package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

type fakeCat struct {
	listErr error
}

func (f fakeCat) ListComponents(context.Context) ([]domain.ComponentSummary, error) {
	return []domain.ComponentSummary{{ID: "db", Description: "database"}}, f.listErr
}

func (f fakeCat) GetComponent(_ context.Context, id string) (*domain.ComponentDef, error) {
	if id != "db" {
		return nil, errs.ComponentNotFoundError{ID: id}
	}
	return &domain.ComponentDef{ID: "db", Params: []domain.ComponentParam{
		{Name: "DB_PASSWORD", Type: "scalar", Required: true},
		{Name: "DB_PORTS", Type: "list", Target: "ports"},
	}}, nil
}

func (f fakeCat) ListPresets(context.Context) ([]domain.PresetSummary, error) {
	return []domain.PresetSummary{{ID: "p", Description: "preset"}}, nil
}

func (f fakeCat) GetPreset(context.Context, string) (*domain.Preset, error) { return nil, nil }

type fakeGen struct {
	reqs []domain.GenerateRequest
	errs []error // consumed in order
}

func (f *fakeGen) Generate(_ context.Context, r domain.GenerateRequest) (*domain.GenerateResult, error) {
	f.reqs = append(f.reqs, r)
	if len(f.errs) > 0 {
		e := f.errs[0]
		f.errs = f.errs[1:]
		if e != nil {
			return nil, e
		}
	}
	return &domain.GenerateResult{Warnings: []string{"careful"}}, nil
}

type fakeExp struct{}

func (fakeExp) Export(*domain.GenerateResult) ([]byte, []byte, error) {
	return []byte("services: {}\n"), []byte("A=1\n"), nil
}

func newDeps(gen Generator) (Deps, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	c := fakeCat{}
	return Deps{Components: c, Presets: c, Generator: gen, Exporter: fakeExp{}, Out: &out, Err: &errb}, &out, &errb
}

func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{nil, ExitOK},
		{errors.New("boom"), ExitFailure},
		{errs.InvalidRequestError{}, ExitInvalidInput},
		{errs.PresetNotFoundError{}, ExitNotFound},
		{errs.InvalidComponentError{}, ExitInvalidDefinition},
		{errs.PortConflictError{}, ExitConflict},
		{fmt.Errorf("wrapped: %w", ErrAborted), ExitAborted},
		{errors.Join(errs.PortConflictError{}), ExitConflict},
	} {
		if got := ExitCode(tc.err); got != tc.want {
			t.Errorf("ExitCode(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

func TestBuildWizardRequest(t *testing.T) {
	t.Run("preset: only changes become overrides", func(t *testing.T) {
		got := buildWizardRequest("web", []componentAnswers{
			{ID: "db", Params: []paramAnswer{
				{Name: "PW", Type: "scalar", Required: true, Prefill: "", Value: "s"},
				{Name: "VER", Type: "scalar", Prefill: "16", Value: "16"},
				{Name: "EXTRA", Type: "list", Prefill: "5433:5432", Value: "5433:5432"},
			}},
			{ID: "web", Params: []paramAnswer{{Name: "EXTRA", Type: "list", Prefill: "a", Value: ""}}},
			{ID: "untouched"},
			{ID: "ver", Params: []paramAnswer{{Name: "VERSION", Type: "scalar", Prefill: "16", Value: "  "}}}, // cleared optional scalar
		})
		want := domain.GenerateRequest{Preset: "web", Overrides: map[string]map[string]any{
			"db":  {"PW": "s"},
			"web": {"EXTRA": []any{}},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v\nwant %+v", got, want)
		}
	})
	t.Run("ad hoc: required and non-defaults", func(t *testing.T) {
		got := buildWizardRequest("", []componentAnswers{
			{ID: "db", Params: []paramAnswer{
				{Name: "PW", Type: "scalar", Required: true, Value: "s"},
				{Name: "VER", Type: "scalar", Prefill: "16", Value: "16"},
				{Name: "PORT", Type: "scalar", Prefill: "1", Value: "2"},
				{Name: "EXTRA", Type: "list", Value: " 1:1 , 2:2"},
			}},
			{ID: "cache", Params: []paramAnswer{{Name: "V", Type: "scalar", Prefill: "7", Value: "7"}, {Name: "W", Type: "scalar", Prefill: "8", Value: ""}}},
		})
		want := domain.GenerateRequest{Services: []domain.ServiceSelection{
			{Component: "db", Params: map[string]any{"PW": "s", "PORT": "2", "EXTRA": []any{"1:1", "2:2"}}},
			{Component: "cache"},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v\nwant %+v", got, want)
		}
	})
}

func TestRun(t *testing.T) {
	gen := &fakeGen{}
	d, out, errb := newDeps(gen)
	d.Out = out
	dir := t.TempDir()
	var sess Session
	d.IsTerminal = func() bool { return true }
	d.TUI = func(ctx context.Context, s Session) ([]string, error) {
		sess = s
		r, err := s.Render(ctx, domain.GenerateRequest{Preset: "p"})
		if err != nil {
			return nil, err
		}
		if len(s.Exists(dir)) != 0 {
			t.Error("nothing should exist yet")
		}
		return s.Write(dir, r)
	}
	if code := Run(context.Background(), d, nil); code != 0 {
		t.Fatalf("code %d: %s", code, errb)
	}
	if !strings.Contains(out.String(), filepath.Join(dir, ".env")) || len(sess.Exists(dir)) != 2 {
		t.Errorf("out=%q exists=%v", out, sess.Exists(dir))
	}
	if st, err := os.Stat(filepath.Join(dir, ".env")); err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf(".env: %v %v", st, err)
	}

	// Errors map to exit codes; aborting is silent.
	d.TUI = func(context.Context, Session) ([]string, error) { return nil, ErrAborted }
	errb.Reset()
	if code := Run(context.Background(), d, nil); code != ExitAborted || errb.Len() != 0 {
		t.Errorf("abort: code %d, stderr %q", code, errb)
	}
	d.TUI = func(context.Context, Session) ([]string, error) { return nil, errs.PortConflictError{} }
	if code := Run(context.Background(), d, nil); code != ExitConflict {
		t.Errorf("conflict: code %d", code)
	}
}

func TestRunRejectsBadUsage(t *testing.T) {
	d, _, _ := newDeps(&fakeGen{})
	d.TUI = func(context.Context, Session) ([]string, error) { return nil, nil }
	d.IsTerminal = func() bool { return false }
	if code := Run(context.Background(), d, nil); code != ExitInvalidInput {
		t.Errorf("no terminal: got %d", code)
	}
	d.IsTerminal = func() bool { return true }
	for _, args := range [][]string{{"generate"}, {"serve", "x"}, {"--help"}} {
		if code := Run(context.Background(), d, args); code != ExitInvalidInput {
			t.Errorf("%v: got %d", args, code)
		}
	}
}

func TestServe(t *testing.T) {
	d, _, errb := newDeps(&fakeGen{})
	if code := Run(context.Background(), d, []string{"serve"}); code != ExitFailure || !strings.Contains(errb.String(), "not implemented") {
		t.Errorf("unwired serve: code %d, %q", code, errb)
	}
	called := false
	d.Serve = func(context.Context) error { called = true; return nil }
	if code := Run(context.Background(), d, []string{"serve"}); code != 0 || !called {
		t.Errorf("wired serve: code %d called=%v", code, called)
	}
}
