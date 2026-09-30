package domain_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

func TestComponentDefYAML(t *testing.T) {
	raw := `
id: postgres
description: PostgreSQL database
params:
  - name: POSTGRES_PASSWORD
    type: scalar
    required: true
    description: Database password
  - name: POSTGRES_VERSION
    type: scalar
    required: false
    default: "16"
  - name: POSTGRES_PORT
    type: scalar
    required: false
    default: "5432"
  - name: EXTRA_PORTS
    type: list
    target: ports
    required: false
    description: Additional host:container mappings
service:
  image: "postgres:${POSTGRES_VERSION}"
  ports: ["${POSTGRES_PORT}:5432"]
  environment:
    POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
  volumes: ["pgdata:/var/lib/postgresql/data"]
volumes:
  - name: pgdata
`
	var comp domain.ComponentDef
	if err := yaml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if comp.ID != "postgres" {
		t.Errorf("ID = %q, want %q", comp.ID, "postgres")
	}
	if comp.Description != "PostgreSQL database" {
		t.Errorf("Description = %q, want %q", comp.Description, "PostgreSQL database")
	}
	if len(comp.Params) != 4 {
		t.Fatalf("len(Params) = %d, want 4", len(comp.Params))
	}

	// First param: POSTGRES_PASSWORD
	p0 := comp.Params[0]
	if p0.Name != "POSTGRES_PASSWORD" {
		t.Errorf("Params[0].Name = %q, want %q", p0.Name, "POSTGRES_PASSWORD")
	}
	if p0.Type != "scalar" {
		t.Errorf("Params[0].Type = %q, want %q", p0.Type, "scalar")
	}
	if !p0.Required {
		t.Error("Params[0].Required = false, want true")
	}
	if p0.Default != "" {
		t.Errorf("Params[0].Default = %q, want empty", p0.Default)
	}

	// Second param: POSTGRES_VERSION
	p1 := comp.Params[1]
	if p1.Name != "POSTGRES_VERSION" {
		t.Errorf("Params[1].Name = %q, want %q", p1.Name, "POSTGRES_VERSION")
	}
	if p1.Type != "scalar" {
		t.Errorf("Params[1].Type = %q, want %q", p1.Type, "scalar")
	}
	if p1.Required {
		t.Error("Params[1].Required = true, want false")
	}
	if p1.Default != "16" {
		t.Errorf("Params[1].Default = %q, want %q", p1.Default, "16")
	}

	// Fourth param: EXTRA_PORTS
	p3 := comp.Params[3]
	if p3.Name != "EXTRA_PORTS" {
		t.Errorf("Params[3].Name = %q, want %q", p3.Name, "EXTRA_PORTS")
	}
	if p3.Type != "list" {
		t.Errorf("Params[3].Type = %q, want %q", p3.Type, "list")
	}
	if p3.Target != "ports" {
		t.Errorf("Params[3].Target = %q, want %q", p3.Target, "ports")
	}
	if p3.Required {
		t.Error("Params[3].Required = true, want false")
	}

	// Service template
	if comp.Service.Image != "postgres:${POSTGRES_VERSION}" {
		t.Errorf("Service.Image = %q, want %q", comp.Service.Image, "postgres:${POSTGRES_VERSION}")
	}
	if len(comp.Service.Ports) != 1 || comp.Service.Ports[0] != "${POSTGRES_PORT}:5432" {
		t.Errorf("Service.Ports = %v, want [${POSTGRES_PORT}:5432]", comp.Service.Ports)
	}
	if v, ok := comp.Service.Environment["POSTGRES_PASSWORD"]; !ok || v != "${POSTGRES_PASSWORD}" {
		t.Errorf("Service.Environment[POSTGRES_PASSWORD] = %q, want %q", v, "${POSTGRES_PASSWORD}")
	}
	if len(comp.Service.Volumes) != 1 || comp.Service.Volumes[0] != "pgdata:/var/lib/postgresql/data" {
		t.Errorf("Service.Volumes = %v, want [pgdata:/var/lib/postgresql/data]", comp.Service.Volumes)
	}

	// Top-level volumes
	if len(comp.Volumes) != 1 {
		t.Fatalf("len(Volumes) = %d, want 1", len(comp.Volumes))
	}
	if comp.Volumes[0].Name != "pgdata" {
		t.Errorf("Volumes[0].Name = %q, want %q", comp.Volumes[0].Name, "pgdata")
	}
}

func TestPresetYAML(t *testing.T) {
	raw := `
name: web-stack
services:
  - component: postgres
    params: { POSTGRES_VERSION: "16", EXTRA_PORTS: ["5433:5432"] }
  - component: nginx
    params: { HTTP_PORT: "80" }
  - component: go-app
    params: { APP_PORT: "8080" }
`
	var preset domain.Preset
	if err := yaml.Unmarshal([]byte(raw), &preset); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if preset.ID != "web-stack" {
		t.Errorf("ID = %q, want %q", preset.ID, "web-stack")
	}
	if len(preset.Services) != 3 {
		t.Fatalf("len(Services) = %d, want 3", len(preset.Services))
	}

	// First service: postgres
	s0 := preset.Services[0]
	if s0.Component != "postgres" {
		t.Errorf("Services[0].Component = %q, want %q", s0.Component, "postgres")
	}
	if v, ok := s0.Params["POSTGRES_VERSION"]; !ok || v != "16" {
		t.Errorf("Services[0].Params[POSTGRES_VERSION] = %v, want %q", v, "16")
	}
	// EXTRA_PORTS should be a slice
	ep, ok := s0.Params["EXTRA_PORTS"]
	if !ok {
		t.Fatal("Services[0].Params[EXTRA_PORTS] missing")
	}
	epSlice, ok := ep.([]any)
	if !ok {
		t.Fatalf("EXTRA_PORTS is %T, want []any", ep)
	}
	if len(epSlice) != 1 || epSlice[0] != "5433:5432" {
		t.Errorf("EXTRA_PORTS = %v, want [5433:5432]", epSlice)
	}

	// Second service: nginx
	if preset.Services[1].Component != "nginx" {
		t.Errorf("Services[1].Component = %q, want %q", preset.Services[1].Component, "nginx")
	}

	// Third service: go-app
	if preset.Services[2].Component != "go-app" {
		t.Errorf("Services[2].Component = %q, want %q", preset.Services[2].Component, "go-app")
	}
	if v, ok := preset.Services[2].Params["APP_PORT"]; !ok || v != "8080" {
		t.Errorf("Services[2].Params[APP_PORT] = %v, want %q", v, "8080")
	}
}

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

func TestComponentDefWithDependsOn(t *testing.T) {
	raw := `
id: app
description: App with dependency
params: []
service:
  image: "app:latest"
  depends_on:
    postgres:
      condition: service_healthy
    redis:
      condition: service_started
`
	var comp domain.ComponentDef
	if err := yaml.Unmarshal([]byte(raw), &comp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if comp.Service.DependsOn == nil {
		t.Fatal("DependsOn is nil")
	}
	if dep, ok := comp.Service.DependsOn["postgres"]; !ok || dep.Condition != "service_healthy" {
		t.Errorf("DependsOn[postgres].Condition = %q, want %q", dep.Condition, "service_healthy")
	}
	if dep, ok := comp.Service.DependsOn["redis"]; !ok || dep.Condition != "service_started" {
		t.Errorf("DependsOn[redis].Condition = %q, want %q", dep.Condition, "service_started")
	}
}
