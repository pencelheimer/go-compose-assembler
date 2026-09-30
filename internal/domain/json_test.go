package domain_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

func TestGenerateRequestJSON(t *testing.T) {
	// Preset mode: preset set, overrides with entries, no services.
	preset := domain.GenerateRequest{
		Preset: "web-stack",
		Overrides: map[string]map[string]any{
			"postgres": {"POSTGRES_VERSION": "15", "POSTGRES_PORT": "5433"},
		},
	}

	data, err := json.Marshal(preset)
	if err != nil {
		t.Fatalf("marshal preset request: %v", err)
	}

	var got domain.GenerateRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal preset request: %v", err)
	}
	if !reflect.DeepEqual(preset, got) {
		t.Errorf("preset round-trip mismatch:\n  want: %+v\n  got:  %+v", preset, got)
	}

	// Ad-hoc mode: no preset, no overrides, services list with entries.
	adhoc := domain.GenerateRequest{
		Services: []domain.ServiceSelection{
			{Component: "postgres", Params: map[string]any{"POSTGRES_PASSWORD": "secret"}},
			{Component: "redis", Params: map[string]any{"REDIS_PORT": "6379"}},
		},
	}

	data, err = json.Marshal(adhoc)
	if err != nil {
		t.Fatalf("marshal adhoc request: %v", err)
	}

	var got2 domain.GenerateRequest
	if err := json.Unmarshal(data, &got2); err != nil {
		t.Fatalf("unmarshal adhoc request: %v", err)
	}
	if !reflect.DeepEqual(adhoc, got2) {
		t.Errorf("adhoc round-trip mismatch:\n  want: %+v\n  got:  %+v", adhoc, got2)
	}
}

func TestGenerateResultJSON(t *testing.T) {
	result := domain.GenerateResult{
		Project: domain.Project{
			Services: map[string]domain.Service{
				"myapp": {
					Name:  "myapp",
					Image: "myapp:latest",
					Environment: map[string]string{
						"APP_ENV": "production",
					},
					Ports:   []string{"8080:8080"},
					Volumes: []string{"appdata:/data"},
					Extra: map[string]any{
						"command": "echo hello",
					},
				},
			},
		},
		EnvMap: map[string]string{
			"APP_ENV": "production",
		},
		Warnings: []string{"port conflict on 8080"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	var got domain.GenerateResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !reflect.DeepEqual(result, got) {
		t.Errorf("result round-trip mismatch:\n  want: %+v\n  got:  %+v", result, got)
	}

	// Verify Extra fields survive round-trip under the "extra" JSON key.
	svc, ok := got.Project.Services["myapp"]
	if !ok {
		t.Fatal("service 'myapp' not found after round-trip")
	}
	cmd, ok := svc.Extra["command"]
	if !ok {
		t.Fatal("Extra[\"command\"] missing after round-trip")
	}
	if cmd != "echo hello" {
		t.Errorf("Extra[\"command\"] = %v, want \"echo hello\"", cmd)
	}
}

func TestProjectJSON(t *testing.T) {
	project := domain.Project{
		Services: map[string]domain.Service{
			"web": {
				Name:  "web",
				Image: "nginx:latest",
				Ports: []string{"80:80"},
				HealthCheck: &domain.HealthCheck{
					Test:     []string{"CMD", "curl", "-f", "http://localhost"},
					Interval: "30s",
					Timeout:  "10s",
					Retries:  3,
				},
				DependsOn: map[string]domain.ServiceDependency{
					"db": {Condition: "service_healthy"},
				},
				Networks: []string{"frontend"},
			},
			"db": {
				Name:  "db",
				Image: "postgres:16",
			},
		},
		Volumes: map[string]domain.Volume{
			"pgdata": {Name: "pgdata", Driver: "local"},
		},
		Networks: map[string]domain.Network{
			"frontend": {Name: "frontend", Driver: "bridge"},
		},
	}

	data, err := json.Marshal(project)
	if err != nil {
		t.Fatalf("marshal project: %v", err)
	}

	var got domain.Project
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal project: %v", err)
	}
	if !reflect.DeepEqual(project, got) {
		t.Errorf("project round-trip mismatch:\n  want: %+v\n  got:  %+v", project, got)
	}

	// Verify service names survive.
	if _, ok := got.Services["web"]; !ok {
		t.Error("service 'web' not found")
	}
	if _, ok := got.Services["db"]; !ok {
		t.Error("service 'db' not found")
	}

	// Verify volume driver.
	if got.Volumes["pgdata"].Driver != "local" {
		t.Errorf("volume driver = %q, want \"local\"", got.Volumes["pgdata"].Driver)
	}

	// Verify network driver.
	if got.Networks["frontend"].Driver != "bridge" {
		t.Errorf("network driver = %q, want \"bridge\"", got.Networks["frontend"].Driver)
	}
}

func TestServiceExtraJSON(t *testing.T) {
	svc := domain.Service{
		Name:  "test",
		Image: "test:latest",
		Extra: map[string]any{
			"command":     "sleep infinity",
			"user":        "1000:1000",
			"working_dir": "/app",
		},
	}

	data, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("marshal service: %v", err)
	}

	// Verify JSON contains "extra" key.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}
	if _, ok := raw["extra"]; !ok {
		t.Fatal("JSON output missing \"extra\" key")
	}

	// Unmarshal back.
	var got domain.Service
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal service: %v", err)
	}

	// Assert all three extra fields are preserved.
	if got.Extra["command"] != "sleep infinity" {
		t.Errorf("Extra[\"command\"] = %v, want \"sleep infinity\"", got.Extra["command"])
	}
	if got.Extra["user"] != "1000:1000" {
		t.Errorf("Extra[\"user\"] = %v, want \"1000:1000\"", got.Extra["user"])
	}
	if got.Extra["working_dir"] != "/app" {
		t.Errorf("Extra[\"working_dir\"] = %v, want \"/app\"", got.Extra["working_dir"])
	}
}
