package validate_test

import (
	"errors"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
	"github.com/pencelheimer/go-compose-assembler/internal/validate"
)

func svc(name string, f func(*domain.Service)) domain.Service {
	s := domain.Service{Name: name, Image: "img"}
	if f != nil {
		f(&s)
	}
	return s
}

func project(svcs ...domain.Service) *domain.Project {
	p := &domain.Project{Services: map[string]domain.Service{}, Volumes: map[string]domain.Volume{"data": {Name: "data"}}}
	for _, s := range svcs {
		p.Services[s.Name] = s
	}
	return p
}

func TestPorts(t *testing.T) {
	tests := []struct {
		name    string
		ports   [2][]string
		want    error // category sentinel, nil for clean
		wantMsg string
	}{
		{"no conflict", [2][]string{{"8080:80"}, {"8081:80"}}, nil, ""},
		{"same host port", [2][]string{{"8080:80"}, {"8080:90"}}, errs.ErrConflict, "8080/tcp"},
		{"different protocol", [2][]string{{"53:53/udp"}, {"53:53"}}, nil, ""},
		{"different ips", [2][]string{{"127.0.0.1:80:80"}, {"127.0.0.2:80:80"}}, nil, ""},
		{"wildcard vs ip", [2][]string{{"80:80"}, {"127.0.0.1:80:80"}}, errs.ErrConflict, "80/tcp"},
		{"range overlap", [2][]string{{"8000-8010:8000-8010"}, {"8005:80"}}, errs.ErrConflict, "8005/tcp"},
		{"container only", [2][]string{{"80"}, {"80"}}, nil, ""},
		{"ipv6", [2][]string{{"[::1]:80:80"}, {"[::1]:80:81"}}, errs.ErrConflict, "80/tcp"},
		{"garbage", [2][]string{{"${X}:80"}, nil}, errs.ErrInvalidInput, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := project(
				svc("a", func(s *domain.Service) { s.Ports = tc.ports[0] }),
				svc("b", func(s *domain.Service) { s.Ports = tc.ports[1] }),
			)
			got := errors.Join(validate.PortsValidator{}.Validate(p)...)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("unexpected error: %v", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want category %v", got, tc.want)
			}
			var pe errs.PortConflictError
			if tc.want == errs.ErrConflict {
				if !errors.As(got, &pe) || pe.Port != tc.wantMsg || pe.ServiceA != "a" || pe.ServiceB != "b" {
					t.Fatalf("got %+v, want port %s between a and b", pe, tc.wantMsg)
				}
			}
		})
	}
}

func TestNames(t *testing.T) {
	p := project(svc("a", nil))
	if errs := (validate.NamesValidator{}).Validate(p); len(errs) != 0 {
		t.Fatalf("unexpected: %v", errs)
	}
	p.Services["x"] = svc("a", nil) // key != name, and duplicates a
	got := errors.Join(validate.NamesValidator{}.Validate(p)...)
	if !errors.Is(got, errs.ErrConflict) || !errors.Is(got, errs.ErrInvalidInput) {
		t.Fatalf("expected duplicate and mismatch errors, got %v", got)
	}
}

func TestDepends(t *testing.T) {
	dep := func(names ...string) func(*domain.Service) {
		return func(s *domain.Service) {
			s.DependsOn = map[string]domain.ServiceDependency{}
			for _, n := range names {
				s.DependsOn[n] = domain.ServiceDependency{Condition: "service_started"}
			}
		}
	}
	tests := []struct {
		name      string
		svcs      []domain.Service
		wantChain []string
	}{
		{"chain", []domain.Service{svc("a", dep("b")), svc("b", dep("c")), svc("c", nil)}, nil},
		{"missing target ignored", []domain.Service{svc("a", dep("ghost"))}, nil},
		{"self", []domain.Service{svc("a", dep("a"))}, []string{"a", "a"}},
		{"cycle", []domain.Service{svc("a", dep("b")), svc("b", dep("c")), svc("c", dep("a"))}, []string{"a", "b", "c", "a"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validate.DependsValidator{}.Validate(project(tc.svcs...))
			if tc.wantChain == nil {
				if len(got) != 0 {
					t.Fatalf("unexpected: %v", got)
				}
				return
			}
			var ce errs.CyclicDependencyError
			if len(got) != 1 || !errors.As(got[0], &ce) || !errors.Is(got[0], errs.ErrConflict) {
				t.Fatalf("got %v", got)
			}
			if len(ce.Chain) != len(tc.wantChain) {
				t.Fatalf("chain %v, want %v", ce.Chain, tc.wantChain)
			}
			for i := range ce.Chain {
				if ce.Chain[i] != tc.wantChain[i] {
					t.Fatalf("chain %v, want %v", ce.Chain, tc.wantChain)
				}
			}
		})
	}
}

func TestVolumes(t *testing.T) {
	vol := func(v ...string) func(*domain.Service) { return func(s *domain.Service) { s.Volumes = v } }
	tests := []struct {
		name string
		svcs []domain.Service
		bad  bool
	}{
		{"single user", []domain.Service{svc("a", vol("data:/d"))}, false},
		{"bind and anonymous skipped", []domain.Service{svc("a", vol("./x:/x", "/abs:/y", "/anon")), svc("b", vol("./x:/x"))}, false},
		{"shared named", []domain.Service{svc("a", vol("data:/d")), svc("b", vol("data:/e"))}, true},
		{"undeclared", []domain.Service{svc("a", vol("other:/d"))}, true},
		{"same path twice", []domain.Service{svc("a", vol("data:/d", "./x:/d"))}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validate.VolumesValidator{}.Validate(project(tc.svcs...))
			if tc.bad != (len(got) > 0) {
				t.Fatalf("got %v, bad=%v", got, tc.bad)
			}
			for _, e := range got {
				var ve errs.VolumeConflictError
				if !errors.As(e, &ve) || !errors.Is(e, errs.ErrConflict) {
					t.Fatalf("wrong type %T", e)
				}
			}
		})
	}
}

type fake struct {
	name string
	errs []error
	boom bool
}

func (f fake) Name() string { return f.name }
func (f fake) Validate(*domain.Project) []error {
	if f.boom {
		panic("boom")
	}
	return f.errs
}

func TestRunAll(t *testing.T) {
	vs := []validate.Validator{
		fake{name: "z", errs: []error{errors.New("zz")}},
		fake{name: "a", errs: []error{errors.New("aa"), errors.New("ab")}},
		fake{name: "p", boom: true},
		fake{name: "clean"},
	}
	for range 50 { // repeated to shake out ordering and races
		got := validate.RunAll(&domain.Project{}, vs)
		if len(got) != 4 || got[0].Error() != "aa" || got[1].Error() != "ab" {
			t.Fatalf("unexpected results: %v", got)
		}
	}
	if got := validate.RunAll(&domain.Project{}, nil); len(got) != 0 {
		t.Fatalf("expected none, got %v", got)
	}
}
