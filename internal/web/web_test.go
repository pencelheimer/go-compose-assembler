package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/pencelheimer/go-compose-assembler/internal/assemble"
	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
	"github.com/pencelheimer/go-compose-assembler/internal/export"
	"github.com/pencelheimer/go-compose-assembler/internal/source"
)

type fakeCat struct {
	listErr error
}

func (f fakeCat) ListComponents(context.Context) ([]domain.ComponentSummary, error) {
	return []domain.ComponentSummary{{ID: "db", Description: "database"}}, f.listErr
}

func (f fakeCat) GetComponent(_ context.Context, id string) (*domain.ComponentDef, error) {
	if id == "boom" {
		panic("broken catalog")
	}
	if id != "db" {
		return nil, errs.ComponentNotFoundError{ID: id, SearchedLocations: []string{"embedded"}}
	}
	return &domain.ComponentDef{ID: "db", Params: []domain.ComponentParam{
		{Name: "DB_PASSWORD", Type: "scalar", Required: true},
	}}, nil
}

func (f fakeCat) ListPresets(context.Context) ([]domain.PresetSummary, error) {
	return nil, f.listErr
}

func (f fakeCat) GetPreset(_ context.Context, id string) (*domain.Preset, error) {
	if id != "web" {
		return nil, errs.PresetNotFoundError{ID: id}
	}
	return &domain.Preset{ID: "web", Services: []domain.PresetService{{Component: "db"}}}, nil
}

type fakeGen struct {
	err error
	got *domain.GenerateRequest
}

func (f *fakeGen) Generate(_ context.Context, r domain.GenerateRequest) (*domain.GenerateResult, error) {
	f.got = &r
	if f.err != nil {
		return nil, f.err
	}
	return &domain.GenerateResult{Warnings: []string{"careful"}}, nil
}

type fakeExp struct{}

func (fakeExp) Export(*domain.GenerateResult) ([]byte, []byte, error) {
	return []byte("services: {}\n"), []byte("A=1\n"), nil
}

func newTestHandler(cat fakeCat, gen *fakeGen) http.Handler {
	return NewHandler(Deps{
		Components: cat,
		Presets:    cat,
		Generator:  gen,
		Exporter:   fakeExp{},
		Assets: fstest.MapFS{
			"index.html":       {Data: []byte("<html>spa</html>")},
			"assets/app-1.js":  {Data: []byte("console.log(1)")},
			"favicon.svg":      {Data: []byte("<svg/>")},
			"assets/nested/.x": {Data: []byte("")},
		},
	})
}

func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, r))
	return rec
}

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()
	var e apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return e
}

func TestCatalogRoutes(t *testing.T) {
	tests := []struct {
		name     string
		cat      fakeCat
		target   string
		status   int
		contains string
	}{
		{"list components", fakeCat{}, "/api/components", 200, `"items":[{"id":"db","description":"database"}],"skipped":[]`},
		{"list keeps items when a file is skipped", fakeCat{listErr: errors.Join(errors.New("bad.yaml: oops"))}, "/api/components", 200, `"skipped":["bad.yaml: oops"]`},
		{"nil list becomes empty array", fakeCat{}, "/api/presets", 200, `"items":[]`},
		{"get component", fakeCat{}, "/api/components/db", 200, `"name":"DB_PASSWORD"`},
		{"missing component", fakeCat{}, "/api/components/nope", 404, `"type":"component_not_found","message":"component not found: \"nope\" (searched: embedded)","component":"nope"`},
		{"get preset", fakeCat{}, "/api/presets/web", 200, `"services":[{"component":"db"}]`},
		{"missing preset", fakeCat{}, "/api/presets/nope", 404, `"kind":"not_found"`},
		{"unknown api route is JSON", fakeCat{}, "/api/nothing", 404, `"kind":"not_found"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestHandler(tt.cat, &fakeGen{}), http.MethodGet, tt.target, "")
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.status, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tt.contains) {
				t.Errorf("body %s does not contain %s", rec.Body, tt.contains)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
				t.Errorf("content type = %q", ct)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		genErr  error
		status  int
		kind    string
		details []errorDetail
	}{
		{
			name:   "ok",
			body:   `{"preset":"web","overrides":{"db":{"DB_PASSWORD":"x"}}}`,
			status: 200,
		},
		{
			name:    "malformed json",
			body:    `{"preset":`,
			status:  400,
			kind:    kindInvalidInput,
			details: []errorDetail{{Type: "invalid_request"}},
		},
		{
			name:    "unknown field",
			body:    `{"presets":"web"}`,
			status:  400,
			kind:    kindInvalidInput,
			details: []errorDetail{{Type: "invalid_request"}},
		},
		{
			name:    "missing param",
			body:    `{"preset":"web"}`,
			genErr:  errs.MissingRequiredParamError{Component: "db", Param: "DB_PASSWORD"},
			status:  400,
			kind:    kindInvalidInput,
			details: []errorDetail{{Type: "missing_param", Component: "db", Param: "DB_PASSWORD"}},
		},
		{
			name:    "wrapped type mismatch",
			body:    `{"preset":"web"}`,
			genErr:  fmt.Errorf("component %q: %w", "db", errs.ParamTypeMismatchError{Component: "db", Param: "P", ExpectedType: "scalar", ActualType: "list"}),
			status:  400,
			kind:    kindInvalidInput,
			details: []errorDetail{{Type: "param_type", Component: "db", Param: "P", Expected: "scalar", Actual: "list"}},
		},
		{
			name: "joined validator conflicts",
			body: `{"preset":"web"}`,
			genErr: errors.Join(
				errs.PortConflictError{Port: "8080/tcp", ServiceA: "a", ServiceB: "b"},
				errs.CyclicDependencyError{Chain: []string{"a", "b"}},
			),
			status: 409,
			kind:   kindConflict,
			details: []errorDetail{
				{Type: "port_conflict", Value: "8080/tcp", Services: []string{"a", "b"}},
				{Type: "cyclic_dependency", Chain: []string{"a", "b"}},
			},
		},
		{
			name:    "broken definition",
			body:    `{"preset":"web"}`,
			genErr:  errs.InvalidComponentError{Location: "/x/db.yaml", Err: errors.New("yaml: bad")},
			status:  500,
			kind:    kindInvalidDefinition,
			details: []errorDetail{{Type: "invalid_component", Location: "/x/db.yaml"}},
		},
		{
			name:    "unexpected error",
			body:    `{"preset":"web"}`,
			genErr:  errors.New("disk on fire"),
			status:  500,
			kind:    kindInternal,
			details: []errorDetail{{Type: "error"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &fakeGen{err: tt.genErr}
			rec := do(t, newTestHandler(fakeCat{}, gen), http.MethodPost, "/api/generate", tt.body)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.status, rec.Body)
			}
			if tt.status == 200 {
				var b export.Bundle
				if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
					t.Fatal(err)
				}
				want := export.Bundle{Compose: "services: {}\n", Env: "A=1\n", Warnings: []string{"careful"}}
				if !reflect.DeepEqual(b, want) {
					t.Errorf("bundle = %+v, want %+v", b, want)
				}
				if gen.got == nil || gen.got.Overrides["db"]["DB_PASSWORD"] != "x" {
					t.Errorf("request not passed through: %+v", gen.got)
				}
				return
			}
			e := decodeError(t, rec)
			if e.Kind != tt.kind {
				t.Errorf("kind = %q, want %q", e.Kind, tt.kind)
			}
			for i := range e.Details {
				e.Details[i].Message = "" // messages are covered by errs tests
			}
			if !reflect.DeepEqual(e.Details, tt.details) {
				t.Errorf("details = %+v, want %+v", e.Details, tt.details)
			}
		})
	}
}

func TestRecoverMiddleware(t *testing.T) {
	h := newTestHandler(fakeCat{}, &fakeGen{})
	rec := do(t, h, http.MethodGet, "/api/components/boom", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if e := decodeError(t, rec); e.Kind != kindInternal {
		t.Errorf("kind = %q", e.Kind)
	}
	// The server keeps serving after a panic.
	if rec := do(t, h, http.MethodGet, "/api/components", ""); rec.Code != http.StatusOK {
		t.Errorf("after panic: status = %d", rec.Code)
	}
}

func TestSPA(t *testing.T) {
	h := newTestHandler(fakeCat{}, &fakeGen{})
	tests := []struct {
		name, method, target string
		status               int
		body, cache          string
	}{
		{"root", "GET", "/", 200, "<html>spa</html>", "no-cache"},
		{"client route falls back to index", "GET", "/catalog/components/db", 200, "<html>spa</html>", "no-cache"},
		{"hashed asset is immutable", "GET", "/assets/app-1.js", 200, "console.log(1)", "public, max-age=31536000, immutable"},
		{"plain file", "GET", "/favicon.svg", 200, "<svg/>", ""},
		{"directory falls back to index", "GET", "/assets/nested", 200, "<html>spa</html>", "no-cache"},
		{"post is rejected", "POST", "/", 405, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, tt.method, tt.target, "")
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if tt.body != "" && rec.Body.String() != tt.body {
				t.Errorf("body = %q, want %q", rec.Body, tt.body)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cache {
				t.Errorf("cache-control = %q, want %q", got, tt.cache)
			}
		})
	}
}

func TestSPANotBuilt(t *testing.T) {
	h := NewHandler(Deps{Components: fakeCat{}, Presets: fakeCat{}, Generator: &fakeGen{}, Exporter: fakeExp{}, Assets: fstest.MapFS{}})
	rec := do(t, h, http.MethodGet, "/", "")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "Frontend not built") {
		t.Errorf("got %d %q", rec.Code, rec.Body)
	}
}

// TestEndToEnd runs the real catalog, engine and exporter over the embedded
// content, so the API contract the SPA relies on is checked against the core.
func TestEndToEnd(t *testing.T) {
	cat := source.NewCatalog(source.NewResolver(source.NewEmbeddedSource()))
	h := NewHandler(Deps{
		Components: cat,
		Presets:    cat,
		Generator:  assemble.NewEngine(cat, cat, nil),
		Exporter:   export.YAMLExporter{},
		Assets:     fstest.MapFS{},
	})

	rec := do(t, h, http.MethodPost, "/api/generate", `{"preset":"web"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("without password: status = %d, body %s", rec.Code, rec.Body)
	}
	if e := decodeError(t, rec); len(e.Details) != 1 || e.Details[0].Type != "missing_param" || e.Details[0].Param != "POSTGRES_PASSWORD" {
		t.Errorf("details = %+v", e.Details)
	}

	rec = do(t, h, http.MethodPost, "/api/generate", `{"preset":"web","overrides":{"postgres":{"POSTGRES_PASSWORD":"s3cret"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var b export.Bundle
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.Compose, "postgres:") || !strings.Contains(b.Env, "POSTGRES_PASSWORD=s3cret") {
		t.Errorf("unexpected bundle: %+v", b)
	}

	rec = do(t, h, http.MethodPost, "/api/generate",
		`{"services":[{"component":"redis","params":{"REDIS_PORT":"8080"}},{"component":"nginx","params":{"NGINX_HTTP_PORT":"8080"}}]}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("port clash: status = %d, body %s", rec.Code, rec.Body)
	}
	if e := decodeError(t, rec); len(e.Details) == 0 || e.Details[0].Type != "port_conflict" {
		t.Errorf("details = %+v", e.Details)
	}
}
