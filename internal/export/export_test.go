package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/export"
)

func result() *domain.GenerateResult {
	return &domain.GenerateResult{
		Project: domain.Project{
			Services: map[string]domain.Service{
				"web": {Name: "web", Image: "nginx", Ports: []string{"${P}:80"}, Restart: "always",
					Extra: map[string]any{"command": "run"}},
			},
			Volumes:  map[string]domain.Volume{"data": {Name: "data"}, "other": {Name: "custom"}},
			Networks: map[string]domain.Network{"net": {Name: "net"}},
		},
		EnvMap:   map[string]string{"B": "x y", "A": "1"},
		Warnings: []string{"w"},
	}
}

func TestYAMLExporter(t *testing.T) {
	compose, env, err := export.YAMLExporter{}.Export(result())
	if err != nil {
		t.Fatal(err)
	}
	c := string(compose)
	if strings.Contains(c, "name: web") || strings.Contains(c, "name: data") {
		t.Errorf("redundant name emitted:\n%s", c)
	}
	for _, want := range []string{"name: custom", "command: run", "image: nginx", "${P}:80", "restart: always"} {
		if !strings.Contains(c, want) {
			t.Errorf("missing %q in:\n%s", want, c)
		}
	}
	if string(env) != "A=1\nB='x y'\n" {
		t.Errorf("env = %q", env)
	}
	if _, _, err := (export.YAMLExporter{}).Export(nil); err == nil {
		t.Error("expected error for nil result")
	}
}

func TestEnvQuoting(t *testing.T) {
	for in, want := range map[string]string{
		"plain-1.2": "K=plain-1.2", "": "K=", "a b": "K='a b'", "a#b": "K='a#b'", "$x": "K='$x'",
		"it's": `K="it's"`, "a$b'c": `K="a\$b'c"`, "l1\nl2": `K="l1\nl2"`,
	} {
		_, env, _ := export.YAMLExporter{}.Export(&domain.GenerateResult{EnvMap: map[string]string{"K": in}})
		if strings.TrimSuffix(string(env), "\n") != want {
			t.Errorf("%q -> %q, want %q", in, env, want)
		}
	}
}

func TestBundleJSON(t *testing.T) {
	data, err := export.BundleJSON(export.YAMLExporter{}, result())
	if err != nil {
		t.Fatal(err)
	}
	var b export.Bundle
	if err := json.Unmarshal(data, &b); err != nil || b.Env == "" || b.Compose == "" || len(b.Warnings) != 1 {
		t.Fatalf("bundle = %+v err=%v", b, err)
	}
}
