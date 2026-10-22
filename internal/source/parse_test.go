package source

import (
	"errors"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// --- helpers ----------------------------------------------------------------

// validComponentYAML is a minimal valid component for baseline tests.
const validComponentYAML = `id: testcomp
description: A test component
params:
  - name: TEST_VAR
    type: scalar
    required: true
    description: required var
  - name: TEST_OPT
    type: scalar
    required: false
    default: "val"
service:
  image: "img:${TEST_OPT}"
  ports:
    - "${TEST_VAR}:80"
`

// validPresetYAML is a minimal valid preset for baseline tests.
const validPresetYAML = `name: testpreset
description: A test preset
services:
  - component: postgres
    params:
      SOME_PARAM: "value"
  - component: redis
`

// --- ParseComponent tests ---------------------------------------------------

func TestParseComponent_ValidBaseline(t *testing.T) {
	comp, err := ParseComponent([]byte(validComponentYAML), "test", "testcomp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if comp.ID != "testcomp" {
		t.Errorf("expected id testcomp, got %s", comp.ID)
	}
	if len(comp.Params) != 2 {
		t.Errorf("expected 2 params, got %d", len(comp.Params))
	}
	if comp.Service.Image != "img:${TEST_OPT}" {
		t.Errorf("unexpected image: %s", comp.Service.Image)
	}
}

func TestParseComponent_Rules(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		id   string
	}{
		{
			name: "id does not match identifier pattern",
			yaml: `id: BADID
description: x
params: []
service:
  image: "img"
`,
			id: "BADID",
		},
		{
			name: "id does not match file stem",
			yaml: `id: other
description: x
params: []
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "empty parameter name",
			yaml: `id: testcomp
description: x
params:
  - name: ""
    type: scalar
    required: true
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "duplicate parameter name",
			yaml: `id: testcomp
description: x
params:
  - name: DUP
    type: scalar
    required: true
  - name: DUP
    type: scalar
    required: true
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "invalid parameter type",
			yaml: `id: testcomp
description: x
params:
  - name: MY_PARAM
    type: number
    required: true
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "scalar name does not match env-var pattern",
			yaml: `id: testcomp
description: x
params:
  - name: lower-case
    type: scalar
    required: true
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "scalar name starts with digit",
			yaml: `id: testcomp
description: x
params:
  - name: 1DIGIT
    type: scalar
    required: true
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "scalar declares a target",
			yaml: `id: testcomp
description: x
params:
  - name: MY_PARAM
    type: scalar
    required: true
    target: ports
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "required scalar has a default",
			yaml: `id: testcomp
description: x
params:
  - name: MY_PARAM
    type: scalar
    required: true
    default: "val"
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "non-required scalar has no default",
			yaml: `id: testcomp
description: x
params:
  - name: MY_PARAM
    type: scalar
    required: false
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "list parameter has no target",
			yaml: `id: testcomp
description: x
params:
  - name: MY_LIST
    type: list
    required: false
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "list parameter has a default",
			yaml: `id: testcomp
description: x
params:
  - name: MY_LIST
    type: list
    target: ports
    default: "bad"
service:
  image: "img"
`,
			id: "testcomp",
		},
		{
			name: "empty service image",
			yaml: `id: testcomp
description: x
params: []
service:
  image: ""
`,
			id: "testcomp",
		},
		{
			name: "missing service image",
			yaml: `id: testcomp
description: x
params: []
service:
  ports: ["80:80"]
`,
			id: "testcomp",
		},
		{
			name: "undeclared interpolation reference in image",
			yaml: `id: testcomp
description: x
params: []
service:
  image: "img:${MISSING}"
`,
			id: "testcomp",
		},
		{
			name: "undeclared interpolation reference in port",
			yaml: `id: testcomp
description: x
params:
  - name: MY_IMG
    type: scalar
    required: true
service:
  image: "${MY_IMG}"
  ports:
    - "${MISSING}:80"
`,
			id: "testcomp",
		},
		{
			name: "undeclared interpolation reference in environment",
			yaml: `id: testcomp
description: x
params:
  - name: MY_IMG
    type: scalar
    required: true
service:
  image: "${MY_IMG}"
  environment:
    KEY: "${MISSING}"
`,
			id: "testcomp",
		},
		{
			name: "volume source not declared in top-level volumes",
			yaml: `id: testcomp
description: x
params: []
service:
  image: "img"
  volumes:
    - "mydata:/data"
`,
			id: "testcomp",
		},
		{
			name: "network not declared in top-level networks",
			yaml: `id: testcomp
description: x
params: []
service:
  image: "img"
  networks:
    - mynet
`,
			id: "testcomp",
		},
		{
			name: "duplicate top-level volume name",
			yaml: `id: testcomp
description: x
params: []
service:
  image: "img"
volumes:
  - name: dup
  - name: dup
`,
			id: "testcomp",
		},
		{
			name: "duplicate top-level network name",
			yaml: `id: testcomp
description: x
params: []
service:
  image: "img"
networks:
  - name: dup
  - name: dup
`,
			id: "testcomp",
		},
		{
			name: "missing service template entirely",
			yaml: `id: testcomp
description: x
params: []
`,
			id: "testcomp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseComponent([]byte(tt.yaml), "test", tt.id)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var compErr errs.InvalidComponentError
			if !errors.As(err, &compErr) {
				t.Fatalf("expected InvalidComponentError, got %T: %v", err, err)
			}
			if !errors.Is(err, errs.ErrInvalidDefinition) {
				t.Fatalf("expected error to match ErrInvalidDefinition")
			}
		})
	}
}

func TestParseComponent_StrictTopLevel(t *testing.T) {
	yaml := `id: testcomp
description: x
params: []
service:
  image: "img"
bogus_key: true
`
	_, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err == nil {
		t.Fatal("expected error for unknown top-level key, got nil")
	}
	var compErr errs.InvalidComponentError
	if !errors.As(err, &compErr) {
		t.Fatalf("expected InvalidComponentError, got %T", err)
	}
}

func TestParseComponent_UnknownServiceKeyPreserved(t *testing.T) {
	yaml := `id: testcomp
description: x
params: []
service:
  image: "img"
  user: postgres
  working_dir: /app
`
	comp, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.Service.Extra == nil {
		t.Fatal("expected Extra map to contain unknown keys")
	}
	if comp.Service.Extra["user"] != "postgres" {
		t.Errorf("expected Extra[user]=postgres, got %v", comp.Service.Extra["user"])
	}
	if comp.Service.Extra["working_dir"] != "/app" {
		t.Errorf("expected Extra[working_dir]=/app, got %v", comp.Service.Extra["working_dir"])
	}
}

func TestParseComponent_InterpolationWithModifiers(t *testing.T) {
	// ${VAR:-default} form should be accepted when VAR is declared.
	yaml := `id: testcomp
description: x
params:
  - name: MY_VAR
    type: scalar
    required: true
service:
  image: "img:${MY_VAR:-latest}"
`
	_, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err != nil {
		t.Fatalf("expected ${VAR:-default} to be accepted: %v", err)
	}
}

func TestParseComponent_EscapedDollarIgnored(t *testing.T) {
	// $${NOTAVAR} should not be treated as a reference.
	yaml := `id: testcomp
description: x
params: []
service:
  image: "img"
  environment:
    KEY: "$${NOTAVAR}"
`
	_, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err != nil {
		t.Fatalf("expected escaped $$ to be ignored: %v", err)
	}
}

func TestParseComponent_BindMountSkipped(t *testing.T) {
	tests := []struct {
		name   string
		volume string
	}{
		{"absolute path", "/host/path:/container"},
		{"relative dot", "./data:/container"},
		{"relative dotdot", "../data:/container"},
		{"home shortcut", "~/data:/container"},
		{"interpolation ref", "${MY_VOL}:/container"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the interpolation ref case we must declare the param so
			// the interpolation check passes; the volume check should still
			// skip the entry because its source contains ${...}.
			params := "params: []"
			if tt.name == "interpolation ref" {
				params = `params:
  - name: MY_VOL
    type: scalar
    required: true`
			}
			yaml := `id: testcomp
description: x
` + params + `
service:
  image: "img"
  volumes:
    - "` + tt.volume + `"
`
			_, err := ParseComponent([]byte(yaml), "test", "testcomp")
			if err != nil {
				t.Fatalf("expected bind-mount volume %q to be skipped: %v", tt.volume, err)
			}
		})
	}
}

func TestParseComponent_AnonymousVolumeSkipped(t *testing.T) {
	// A volume entry with no colon is anonymous and should be skipped.
	yaml := `id: testcomp
description: x
params: []
service:
  image: "img"
  volumes:
    - "/data"
`
	_, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err != nil {
		t.Fatalf("expected anonymous volume to be skipped: %v", err)
	}
}

func TestParseComponent_DefaultNetworkSkipped(t *testing.T) {
	// The "default" network does not need a top-level declaration.
	yaml := `id: testcomp
description: x
params: []
service:
  image: "img"
  networks:
    - default
`
	_, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err != nil {
		t.Fatalf("expected 'default' network to be skipped: %v", err)
	}
}

func TestParseComponent_ValidWithVolumesAndNetworks(t *testing.T) {
	yaml := `id: testcomp
description: x
params:
  - name: DB_PORT
    type: scalar
    required: false
    default: "5432"
service:
  image: "postgres:16"
  ports:
    - "${DB_PORT}:5432"
  volumes:
    - "pgdata:/var/lib/postgresql/data"
    - "./init:/docker-entrypoint-initdb.d"
  networks:
    - backend
    - default
volumes:
  - name: pgdata
networks:
  - name: backend
`
	comp, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comp.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(comp.Volumes))
	}
	if len(comp.Networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(comp.Networks))
	}
}

func TestParseComponent_ListParamRef_NotInInterpolation(t *testing.T) {
	// A list param referenced via ${EXTRA_PORTS} in a typed field should fail
	// since only scalar params may be interpolated.
	yaml := `id: testcomp
description: x
params:
  - name: EXTRA_PORTS
    type: list
    target: ports
service:
  image: "img"
  ports:
    - "${EXTRA_PORTS}:80"
`
	_, err := ParseComponent([]byte(yaml), "test", "testcomp")
	if err == nil {
		t.Fatal("expected error for interpolation of list param")
	}
}

// --- ParsePreset tests ------------------------------------------------------

func TestParsePreset_ValidBaseline(t *testing.T) {
	preset, err := ParsePreset([]byte(validPresetYAML), "test", "testpreset")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if preset.ID != "testpreset" {
		t.Errorf("expected id testpreset, got %s", preset.ID)
	}
	if len(preset.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(preset.Services))
	}
}

func TestParsePreset_Rules(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		id   string
	}{
		{
			name: "name does not match identifier pattern",
			yaml: `name: BAD_NAME
services:
  - component: postgres
`,
			id: "BAD_NAME",
		},
		{
			name: "name does not match file stem",
			yaml: `name: other
services:
  - component: postgres
`,
			id: "testpreset",
		},
		{
			name: "no services",
			yaml: `name: testpreset
services: []
`,
			id: "testpreset",
		},
		{
			name: "duplicate component",
			yaml: `name: testpreset
services:
  - component: postgres
  - component: postgres
`,
			id: "testpreset",
		},
		{
			name: "component does not match identifier pattern",
			yaml: `name: testpreset
services:
  - component: BAD_COMP
`,
			id: "testpreset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePreset([]byte(tt.yaml), "test", tt.id)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var presetErr errs.InvalidPresetError
			if !errors.As(err, &presetErr) {
				t.Fatalf("expected InvalidPresetError, got %T: %v", err, err)
			}
			if !errors.Is(err, errs.ErrInvalidDefinition) {
				t.Fatalf("expected error to match ErrInvalidDefinition")
			}
		})
	}
}

func TestParsePreset_StrictTopLevel(t *testing.T) {
	yaml := `name: testpreset
services:
  - component: postgres
extra_field: oops
`
	_, err := ParsePreset([]byte(yaml), "test", "testpreset")
	if err == nil {
		t.Fatal("expected error for unknown top-level key, got nil")
	}
	var presetErr errs.InvalidPresetError
	if !errors.As(err, &presetErr) {
		t.Fatalf("expected InvalidPresetError, got %T", err)
	}
}

func TestParsePreset_ErrorPreservesUnderlying(t *testing.T) {
	// A malformed YAML should produce an error whose Unwrap chain reveals the cause.
	yaml := `name: [invalid yaml`
	_, err := ParsePreset([]byte(yaml), "test", "testpreset")
	if err == nil {
		t.Fatal("expected error")
	}
	var presetErr errs.InvalidPresetError
	if !errors.As(err, &presetErr) {
		t.Fatalf("expected InvalidPresetError, got %T", err)
	}
	if presetErr.Unwrap() == nil {
		t.Fatal("expected underlying error from Unwrap()")
	}
}

// --- ValidIdentifier tests --------------------------------------------------

func TestValidIdentifier(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"postgres", true},
		{"go-app", true},
		{"a", true},
		{"abc123", true},
		{"a-b-c", true},
		{"1start", true},
		{"", false},
		{"-start", false},
		{"UPPER", false},
		{"has.dot", false},
		{"has/slash", false},
		{"has_underscore", false},
		{"ok" + string(make([]byte, 63)), false}, // too long
		{"a b", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := ValidIdentifier(tt.id)
			if got != tt.valid {
				t.Errorf("ValidIdentifier(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

// --- Embedded defaults test -------------------------------------------------

func TestParseEmbeddedComponents(t *testing.T) {
	// Verify every shipped component YAML passes parsing.
	src := NewEmbeddedSource()
	ctx := t.Context()

	ids, err := src.ListComponents(ctx)
	if err != nil {
		t.Fatalf("listing embedded components: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected at least one embedded component")
	}

	for _, id := range ids {
		data, err := src.LoadComponent(ctx, id)
		if err != nil {
			t.Errorf("loading component %s: %v", id, err)
			continue
		}
		comp, err := ParseComponent(data, "embedded:"+id, id)
		if err != nil {
			t.Errorf("parsing component %s: %v", id, err)
			continue
		}
		if comp.ID != id {
			t.Errorf("component %s: id mismatch: got %s", id, comp.ID)
		}
	}
}

func TestParseEmbeddedPresets(t *testing.T) {
	// Verify every shipped preset YAML passes parsing.
	src := NewEmbeddedSource()
	ctx := t.Context()

	ids, err := src.ListPresets(ctx)
	if err != nil {
		t.Fatalf("listing embedded presets: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected at least one embedded preset")
	}

	for _, id := range ids {
		data, err := src.LoadPreset(ctx, id)
		if err != nil {
			t.Errorf("loading preset %s: %v", id, err)
			continue
		}
		preset, err := ParsePreset(data, "embedded:"+id, id)
		if err != nil {
			t.Errorf("parsing preset %s: %v", id, err)
			continue
		}
		if preset.ID != id {
			t.Errorf("preset %s: id mismatch: got %s", id, preset.ID)
		}
	}
}
