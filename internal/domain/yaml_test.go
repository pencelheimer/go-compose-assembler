package domain_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

func TestServiceTemplateInlineYAML(t *testing.T) {
	raw := `
image: "myapp:latest"
ports: ["8080:8080"]
command: "echo hello"
user: "1000:1000"
working_dir: "/app"
`
	var svc domain.ServiceTemplate
	if err := yaml.Unmarshal([]byte(raw), &svc); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if svc.Image != "myapp:latest" {
		t.Errorf("Image = %q, want %q", svc.Image, "myapp:latest")
	}
	if len(svc.Ports) != 1 || svc.Ports[0] != "8080:8080" {
		t.Errorf("Ports = %v, want [8080:8080]", svc.Ports)
	}
	if v, ok := svc.Extra["command"]; !ok || v != "echo hello" {
		t.Errorf("Extra[command] = %v, want %q", v, "echo hello")
	}
	if v, ok := svc.Extra["user"]; !ok || v != "1000:1000" {
		t.Errorf("Extra[user] = %v, want %q", v, "1000:1000")
	}
	if v, ok := svc.Extra["working_dir"]; !ok || v != "/app" {
		t.Errorf("Extra[working_dir] = %v, want %q", v, "/app")
	}
}
