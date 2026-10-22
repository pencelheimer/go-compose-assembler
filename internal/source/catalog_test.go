package source

import (
	"context"
	"errors"
	"testing"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// --- Conformance: Catalog satisfies the Phase 1 catalog interfaces -----------

// ComponentCatalog mirrors the consumer interface from Phase 1.
type componentCatalog interface {
	ListComponents(ctx context.Context) ([]domain.ComponentSummary, error)
	GetComponent(ctx context.Context, id string) (*domain.ComponentDef, error)
}

// PresetCatalog mirrors the consumer interface from Phase 1.
type presetCatalog interface {
	ListPresets(ctx context.Context) ([]domain.PresetSummary, error)
	GetPreset(ctx context.Context, id string) (*domain.Preset, error)
}

func TestCatalog_SatisfiesComponentCatalog(t *testing.T) {
	var _ componentCatalog = (*Catalog)(nil)
}

func TestCatalog_SatisfiesPresetCatalog(t *testing.T) {
	var _ presetCatalog = (*Catalog)(nil)
}

// --- Catalog functional tests -----------------------------------------------

// validComponentBytes returns valid component YAML for testing.
func validComponentBytes(id string) []byte {
	return []byte(`id: ` + id + `
description: Test ` + id + `
params:
  - name: MY_VAR
    type: scalar
    required: false
    default: "x"
service:
  image: "img:${MY_VAR}"
`)
}

// validPresetBytes returns valid preset YAML for testing.
func validPresetBytes(id string) []byte {
	return []byte(`name: ` + id + `
description: Test preset ` + id + `
services:
  - component: postgres
`)
}

func TestCatalog_ListComponents_Sorted(t *testing.T) {
	src := &mockSource{
		name: "test",
		components: map[string][]byte{
			"zebra": validComponentBytes("zebra"),
			"alpha": validComponentBytes("alpha"),
			"mid":   validComponentBytes("mid"),
		},
	}
	cat := NewCatalog(NewResolver(src))
	summaries, err := cat.ListComponents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(summaries))
	}
	expected := []string{"alpha", "mid", "zebra"}
	for i, s := range summaries {
		if s.ID != expected[i] {
			t.Errorf("summaries[%d].ID = %q, want %q", i, s.ID, expected[i])
		}
	}
}

func TestCatalog_ListComponents_BrokenFileSkippedWithError(t *testing.T) {
	src := &mockSource{
		name: "test",
		components: map[string][]byte{
			"good":   validComponentBytes("good"),
			"broken": []byte("not valid yaml: [[["),
		},
	}
	cat := NewCatalog(NewResolver(src))
	summaries, err := cat.ListComponents(context.Background())

	// Should still return the good one.
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].ID != "good" {
		t.Errorf("expected 'good', got %q", summaries[0].ID)
	}

	// Should also return an error matching ErrInvalidDefinition.
	if err == nil {
		t.Fatal("expected non-nil error for broken file")
	}
	if !errors.Is(err, errs.ErrInvalidDefinition) {
		t.Fatalf("expected error to match ErrInvalidDefinition, got: %v", err)
	}
}

func TestCatalog_GetComponent_Override(t *testing.T) {
	override := &mockSource{
		name:       "override",
		components: map[string][]byte{"mycomp": validComponentBytes("mycomp")},
	}
	embedded := &mockSource{
		name:       "embedded",
		components: map[string][]byte{"mycomp": validComponentBytes("mycomp")},
	}

	cat := NewCatalog(NewResolver(override, embedded))
	comp, err := cat.GetComponent(context.Background(), "mycomp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if comp.ID != "mycomp" {
		t.Errorf("expected id 'mycomp', got %q", comp.ID)
	}
}

func TestCatalog_GetComponent_BrokenOverrideDoesNotFallback(t *testing.T) {
	// A broken higher-priority source should error, not fall back.
	override := &mockSource{
		name:       "override",
		components: map[string][]byte{"mycomp": []byte("broken yaml: [[[")},
	}
	embedded := &mockSource{
		name:       "embedded",
		components: map[string][]byte{"mycomp": validComponentBytes("mycomp")},
	}

	cat := NewCatalog(NewResolver(override, embedded))
	_, err := cat.GetComponent(context.Background(), "mycomp")
	if err == nil {
		t.Fatal("expected error for broken override, got nil")
	}
	if !errors.Is(err, errs.ErrInvalidDefinition) {
		t.Fatalf("expected ErrInvalidDefinition, got: %v", err)
	}
}

func TestCatalog_GetComponent_NotFound(t *testing.T) {
	src := &mockSource{name: "test", components: map[string][]byte{}}
	cat := NewCatalog(NewResolver(src))
	_, err := cat.GetComponent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestCatalog_ListPresets_Sorted(t *testing.T) {
	src := &mockSource{
		name: "test",
		presets: map[string][]byte{
			"zulu":  validPresetBytes("zulu"),
			"alpha": validPresetBytes("alpha"),
		},
	}
	cat := NewCatalog(NewResolver(src))
	summaries, err := cat.ListPresets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2, got %d", len(summaries))
	}
	if summaries[0].ID != "alpha" || summaries[1].ID != "zulu" {
		t.Errorf("unexpected order: %v", summaries)
	}
}

func TestCatalog_GetPreset_NotFound(t *testing.T) {
	src := &mockSource{name: "test", presets: map[string][]byte{}}
	cat := NewCatalog(NewResolver(src))
	_, err := cat.GetPreset(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestCatalog_GetPreset_BrokenOverrideDoesNotFallback(t *testing.T) {
	override := &mockSource{
		name:    "override",
		presets: map[string][]byte{"web": []byte("broken yaml [[[")},
	}
	embedded := &mockSource{
		name:    "embedded",
		presets: map[string][]byte{"web": validPresetBytes("web")},
	}

	cat := NewCatalog(NewResolver(override, embedded))
	_, err := cat.GetPreset(context.Background(), "web")
	if err == nil {
		t.Fatal("expected error for broken override")
	}
	if !errors.Is(err, errs.ErrInvalidDefinition) {
		t.Fatalf("expected ErrInvalidDefinition, got: %v", err)
	}
}

// --- End-to-end: real embedded content test -----------------------------------

func TestCatalog_EndToEnd_EmbeddedDefaults(t *testing.T) {
	cat := NewCatalog(NewResolver(NewEmbeddedSource()))
	ctx := context.Background()

	// List and fetch all components.
	compSummaries, err := cat.ListComponents(ctx)
	if err != nil {
		t.Fatalf("listing components: %v", err)
	}
	if len(compSummaries) < 4 {
		t.Fatalf("expected at least 4 components, got %d", len(compSummaries))
	}
	for _, s := range compSummaries {
		comp, err := cat.GetComponent(ctx, s.ID)
		if err != nil {
			t.Errorf("GetComponent(%s): %v", s.ID, err)
			continue
		}
		if comp.ID != s.ID {
			t.Errorf("ID mismatch: summary %q vs def %q", s.ID, comp.ID)
		}
		if comp.Description != s.Description {
			t.Errorf("Description mismatch for %s", s.ID)
		}
	}

	// List and fetch all presets.
	presetSummaries, err := cat.ListPresets(ctx)
	if err != nil {
		t.Fatalf("listing presets: %v", err)
	}
	if len(presetSummaries) < 3 {
		t.Fatalf("expected at least 3 presets, got %d", len(presetSummaries))
	}
	for _, s := range presetSummaries {
		preset, err := cat.GetPreset(ctx, s.ID)
		if err != nil {
			t.Errorf("GetPreset(%s): %v", s.ID, err)
			continue
		}
		if preset.ID != s.ID {
			t.Errorf("ID mismatch: summary %q vs def %q", s.ID, preset.ID)
		}
	}
}
