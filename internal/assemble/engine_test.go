package assemble_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/assemble"
	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

type fakeCatalog struct {
	comps   map[string]*domain.ComponentDef
	presets map[string]*domain.Preset
}

func (f fakeCatalog) GetComponent(_ context.Context, id string) (*domain.ComponentDef, error) {
	if c, ok := f.comps[id]; ok {
		return c, nil
	}
	return nil, errs.ComponentNotFoundError{ID: id}
}

func (f fakeCatalog) GetPreset(_ context.Context, id string) (*domain.Preset, error) {
	if p, ok := f.presets[id]; ok {
		return p, nil
	}
	return nil, errs.PresetNotFoundError{ID: id}
}

func scalar(name, def string, required bool) domain.ComponentParam {
	return domain.ComponentParam{Name: name, Type: "scalar", Required: required, Default: def}
}

func newCatalog() fakeCatalog {
	db := &domain.ComponentDef{
		ID: "db",
		Params: []domain.ComponentParam{
			scalar("DB_PASSWORD", "", true),
			scalar("DB_PORT", "5432", false),
			{Name: "DB_EXTRA_PORTS", Type: "list", Target: "ports"},
			{Name: "DB_BAD", Type: "list", Target: "nope"},
		},
		Service: domain.ServiceTemplate{
			Image: "db", Ports: []string{"${DB_PORT}:5432"},
			Environment: map[string]string{"PW": "${DB_PASSWORD}"},
			HealthCheck: &domain.HealthCheck{Test: []string{"CMD", "true"}},
			Extra:       map[string]any{"labels": []any{"a"}},
		},
		Volumes: []domain.ComponentVolume{{Name: "dbdata"}},
	}
	cache := &domain.ComponentDef{
		ID:      "cache",
		Params:  []domain.ComponentParam{scalar("DB_PORT", "6379", false)}, // same key as db, different default
		Service: domain.ServiceTemplate{Image: "cache", Ports: []string{"${DB_PORT}:6379"}},
	}
	app := &domain.ComponentDef{
		ID:     "app",
		Params: []domain.ComponentParam{scalar("APP_PORT", "8080", false)},
		Service: domain.ServiceTemplate{
			Image: "app", Ports: []string{"${APP_PORT}:8080"},
			DependsOn: map[string]domain.ServiceDependency{"db": {Condition: "service_healthy"}},
			Networks:  []string{"net"},
		},
		Networks: []domain.ComponentNetwork{{Name: "net"}},
	}
	app2 := *app
	app2.ID = "app2"
	app2.Params = []domain.ComponentParam{scalar("APP_PORT", "8080", false)}
	return fakeCatalog{
		comps: map[string]*domain.ComponentDef{"db": db, "cache": cache, "app": app, "app2": &app2},
		presets: map[string]*domain.Preset{
			"p": {ID: "p", Services: []domain.PresetService{
				{Component: "db", Params: map[string]any{"DB_EXTRA_PORTS": []any{"5433:5432"}, "DB_PORT": nil}},
				{Component: "app", Params: map[string]any{"APP_PORT": float64(9000)}},
			}},
		},
	}
}

func TestGenerateSuccess(t *testing.T) {
	cat := newCatalog()
	eng := assemble.NewEngine(cat, cat, nil)
	res, err := eng.Generate(context.Background(), domain.GenerateRequest{
		Preset:    "p",
		Overrides: map[string]map[string]any{"db": {"DB_PASSWORD": "s3"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantEnv := map[string]string{"DB_PASSWORD": "s3", "DB_PORT": "5432", "APP_PORT": "9000"}
	for k, v := range wantEnv {
		if res.EnvMap[k] != v {
			t.Errorf("env %s = %q, want %q", k, res.EnvMap[k], v)
		}
	}
	db := res.Project.Services["db"]
	if got := db.Ports; len(got) != 2 || got[1] != "5433:5432" || got[0] != "${DB_PORT}:5432" {
		t.Errorf("db ports = %v", got)
	}
	if _, ok := res.Project.Volumes["dbdata"]; !ok {
		t.Error("missing top-level volume")
	}
	if _, ok := res.Project.Networks["net"]; !ok {
		t.Error("missing top-level network")
	}
	// The template must not be mutated by merging or reuse.
	if len(cat.comps["db"].Service.Ports) != 1 {
		t.Error("component template was mutated")
	}
	db.HealthCheck.Test[0] = "X"
	db.Extra["labels"].([]any)[0] = "X"
	if cat.comps["db"].Service.HealthCheck.Test[0] != "CMD" || cat.comps["db"].Service.Extra["labels"].([]any)[0] != "a" {
		t.Error("deep copy shares memory with template")
	}
}

func TestGenerateErrors(t *testing.T) {
	cat := newCatalog()
	eng := assemble.NewEngine(cat, cat, nil)
	pw := map[string]any{"DB_PASSWORD": "x"}
	sel := func(c string, p map[string]any) domain.ServiceSelection {
		return domain.ServiceSelection{Component: c, Params: p}
	}
	tests := []struct {
		name string
		req  domain.GenerateRequest
		is   error
		as   any
	}{
		{"neither", domain.GenerateRequest{}, errs.ErrInvalidInput, new(errs.InvalidRequestError)},
		{"both", domain.GenerateRequest{Preset: "p", Services: []domain.ServiceSelection{sel("db", pw)}}, errs.ErrInvalidInput, new(errs.InvalidRequestError)},
		{"adhoc with overrides", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", pw)}, Overrides: map[string]map[string]any{"db": pw}}, errs.ErrInvalidInput, new(errs.InvalidRequestError)},
		{"override outside preset", domain.GenerateRequest{Preset: "p", Overrides: map[string]map[string]any{"cache": {"DB_PORT": "1"}}}, errs.ErrInvalidInput, new(errs.InvalidRequestError)},
		{"preset not found", domain.GenerateRequest{Preset: "zz"}, errs.ErrNotFound, new(errs.PresetNotFoundError)},
		{"component not found", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("zz", nil)}}, errs.ErrNotFound, new(errs.ComponentNotFoundError)},
		{"duplicate", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", pw), sel("db", pw)}}, errs.ErrConflict, new(errs.DuplicateServiceError)},
		{"missing required", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", nil)}}, errs.ErrInvalidInput, new(errs.MissingRequiredParamError)},
		{"nil falls through to missing", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": nil})}}, errs.ErrInvalidInput, new(errs.MissingRequiredParamError)},
		{"unknown param", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": "x", "NOPE": 1})}}, errs.ErrInvalidInput, new(errs.UnknownParamError)},
		{"fractional", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": "x", "DB_PORT": 1.27})}}, errs.ErrInvalidInput, new(errs.ParamTypeMismatchError)},
		{"scalar given list", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": []any{"x"}})}}, errs.ErrInvalidInput, new(errs.ParamTypeMismatchError)},
		{"list given scalar", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": "x", "DB_EXTRA_PORTS": "1:1"})}}, errs.ErrInvalidInput, new(errs.ParamTypeMismatchError)},
		{"unknown target", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": "x", "DB_BAD": []any{"a"}})}}, errs.ErrInvalidDefinition, new(errs.UnknownTargetError)},
		{"merger rejects element", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", map[string]any{"DB_PASSWORD": "x", "DB_EXTRA_PORTS": []any{1}})}}, errs.ErrInvalidInput, new(errs.InvalidRequestError)},
		{"env conflict", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("db", pw), sel("cache", nil)}}, errs.ErrConflict, new(errs.EnvConflictError)},
		{"port conflict", domain.GenerateRequest{Services: []domain.ServiceSelection{sel("app", nil), sel("app2", nil)}}, errs.ErrConflict, new(errs.PortConflictError)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := eng.Generate(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tc.is) {
				t.Errorf("errors.Is(%v, %v) = false", err, tc.is)
			}
			if !errors.As(err, tc.as) {
				t.Errorf("errors.As(%v, %T) = false", err, tc.as)
			}
		})
	}
}

func TestEnvConflictAllowsIdenticalValues(t *testing.T) {
	cat := newCatalog()
	eng := assemble.NewEngine(cat, cat, nil)
	res, err := eng.Generate(context.Background(), domain.GenerateRequest{Services: []domain.ServiceSelection{
		{Component: "db", Params: map[string]any{"DB_PASSWORD": "x", "DB_PORT": "7000"}},
		{Component: "cache", Params: map[string]any{"DB_PORT": "7000"}},
	}})
	// Identical env values are legal, but both services then publish host port 7000.
	if !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("want port conflict, got res=%v err=%v", res, err)
	}
	var pe errs.PortConflictError
	if !errors.As(err, &pe) || pe.Port != "7000/tcp" {
		t.Fatalf("want port conflict on 7000/tcp, got %v", err)
	}
}

func TestNormalizeScalars(t *testing.T) {
	cat := newCatalog()
	eng := assemble.NewEngine(cat, cat, nil)
	cases := map[string]any{
		"7001": 7001, "7002": float64(7002), "7003": int64(7003), "7004": uint64(7004), "7005": json.Number("7005"), "7006": "7006",
	}
	for want, in := range cases {
		res, err := eng.Generate(context.Background(), domain.GenerateRequest{Services: []domain.ServiceSelection{
			{Component: "app", Params: map[string]any{"APP_PORT": in}},
		}})
		if err != nil || res.EnvMap["APP_PORT"] != want {
			t.Errorf("%T(%v): env=%v err=%v, want %q", in, in, res.EnvMap, err, want)
		}
	}
	res, err := eng.Generate(context.Background(), domain.GenerateRequest{Services: []domain.ServiceSelection{
		{Component: "db", Params: map[string]any{"DB_PASSWORD": true}},
	}})
	if err != nil || res.EnvMap["DB_PASSWORD"] != "true" {
		t.Errorf("bool: %v %v", res, err)
	}
}

func TestDanglingDependencyWarns(t *testing.T) {
	cat := newCatalog()
	eng := assemble.NewEngine(cat, cat, nil)
	res, err := eng.Generate(context.Background(), domain.GenerateRequest{Services: []domain.ServiceSelection{{Component: "app"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings = %v", res.Warnings)
	}
}

func TestGenerateCancelled(t *testing.T) {
	cat := newCatalog()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := assemble.NewEngine(cat, cat, nil).Generate(ctx, domain.GenerateRequest{Services: []domain.ServiceSelection{{Component: "app"}}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestMergers(t *testing.T) {
	svc := domain.Service{Ports: []string{"1:1"}}
	for _, tc := range []struct {
		target string
		value  any
		ok     bool
	}{
		{"ports", []any{"1:1", "2:2"}, true},
		{"volumes", []string{"a:/a"}, true},
		{"environment", []any{"K=V=W"}, true},
		{"environment", []any{"nokey"}, false},
		{"ports", []any{""}, false},
		{"ports", "scalar", false},
	} {
		err := assemble.GetTarget(tc.target).Merge(&svc, tc.value)
		if (err == nil) != tc.ok {
			t.Errorf("%s %v: err=%v", tc.target, tc.value, err)
		}
		if err != nil && !errors.Is(err, errs.ErrInvalidInput) {
			t.Errorf("%s: merger error lacks category: %v", tc.target, err)
		}
	}
	if len(svc.Ports) != 2 || svc.Environment["K"] != "V=W" {
		t.Errorf("svc = %+v", svc)
	}
}
