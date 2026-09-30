package assemble_test

import (
	"strings"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/assemble"
	"github.com/pencelheimer/go-compose-assembler/internal/domain"
)

type mockMerger struct {
	target string
}

func (m mockMerger) Target() string                             { return m.target }
func (m mockMerger) Merge(svc *domain.Service, value any) error { return nil }

func TestRegisterAndGetTarget(t *testing.T) {
	assemble.ResetRegistry()

	assemble.RegisterTarget(mockMerger{target: "ports"})

	if got := assemble.GetTarget("ports"); got == nil {
		t.Fatal("expected GetTarget(\"ports\") to be non-nil")
	} else if got.Target() != "ports" {
		t.Fatalf("expected Target() == \"ports\", got %q", got.Target())
	}

	if got := assemble.GetTarget("nonexistent"); got != nil {
		t.Fatalf("expected GetTarget(\"nonexistent\") to be nil, got %v", got)
	}

	assemble.ResetRegistry()
}

func TestRegisterDuplicatePanics(t *testing.T) {
	assemble.ResetRegistry()

	assemble.RegisterTarget(mockMerger{target: "volumes"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected panic value to be a string, got %T", r)
		}
		if !strings.Contains(msg, "duplicate") {
			t.Fatalf("expected panic message to contain \"duplicate\", got %q", msg)
		}
		assemble.ResetRegistry()
	}()

	assemble.RegisterTarget(mockMerger{target: "volumes"})

	t.Fatal("expected panic on duplicate registration")
}

func TestResetRegistry(t *testing.T) {
	assemble.ResetRegistry()

	assemble.RegisterTarget(mockMerger{target: "test"})

	if got := assemble.GetTarget("test"); got == nil {
		t.Fatal("expected GetTarget(\"test\") to be non-nil after registration")
	}

	assemble.ResetRegistry()

	if got := assemble.GetTarget("test"); got != nil {
		t.Fatalf("expected GetTarget(\"test\") to be nil after ResetRegistry, got %v", got)
	}
}
