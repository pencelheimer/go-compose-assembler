package source

import (
	"context"
	"embed"
	"io/fs"
)

//go:embed components/*.yaml
var embeddedComponents embed.FS

//go:embed presets/*.yaml
var embeddedPresets embed.FS

// EmbeddedSource serves the default components and presets shipped in the binary.
type EmbeddedSource struct {
	inner *fsSource
}

// NewEmbeddedSource creates an EmbeddedSource from the real embedded filesystem.
// It eagerly validates all embedded content and panics on the first failure.
func NewEmbeddedSource() *EmbeddedSource {
	componentsFS, err := fs.Sub(embeddedComponents, "components")
	if err != nil {
		panic("embedded: failed to open components: " + err.Error())
	}
	presetsFS, err := fs.Sub(embeddedPresets, "presets")
	if err != nil {
		panic("embedded: failed to open presets: " + err.Error())
	}
	return newEmbeddedSource(componentsFS, presetsFS)
}

// newEmbeddedSource constructs an EmbeddedSource from arbitrary filesystems.
// It eagerly validates all content and panics on the first failure.
// Unexported; accessible from in-package tests to trigger the panic with
// a deliberately broken in-memory filesystem.
func newEmbeddedSource(componentsFS, presetsFS fs.FS) *EmbeddedSource {
	s := &fsSource{
		name:       "embedded",
		components: componentsFS,
		presets:    presetsFS,
	}

	ctx := context.Background()

	// Eagerly validate all components.
	compIDs, err := s.ListComponents(ctx)
	if err != nil {
		panic("embedded: listing components: " + err.Error())
	}
	seenComp := make(map[string]bool)
	for _, id := range compIDs {
		if seenComp[id] {
			panic("embedded: duplicate component id: " + id)
		}
		seenComp[id] = true

		data, err := s.LoadComponent(ctx, id)
		if err != nil {
			panic("embedded: loading component " + id + ": " + err.Error())
		}
		if _, err := ParseComponent(data, "embedded:components/"+id+".yaml", id); err != nil {
			panic("embedded: validating component " + id + ": " + err.Error())
		}
	}

	// Eagerly validate all presets.
	presetIDs, err := s.ListPresets(ctx)
	if err != nil {
		panic("embedded: listing presets: " + err.Error())
	}
	seenPreset := make(map[string]bool)
	for _, id := range presetIDs {
		if seenPreset[id] {
			panic("embedded: duplicate preset id: " + id)
		}
		seenPreset[id] = true

		data, err := s.LoadPreset(ctx, id)
		if err != nil {
			panic("embedded: loading preset " + id + ": " + err.Error())
		}
		if _, err := ParsePreset(data, "embedded:presets/"+id+".yaml", id); err != nil {
			panic("embedded: validating preset " + id + ": " + err.Error())
		}
	}

	return &EmbeddedSource{inner: s}
}

func (e *EmbeddedSource) Name() string { return e.inner.Name() }
func (e *EmbeddedSource) ListComponents(ctx context.Context) ([]string, error) {
	return e.inner.ListComponents(ctx)
}
func (e *EmbeddedSource) LoadComponent(ctx context.Context, id string) ([]byte, error) {
	return e.inner.LoadComponent(ctx, id)
}
func (e *EmbeddedSource) ListPresets(ctx context.Context) ([]string, error) {
	return e.inner.ListPresets(ctx)
}
func (e *EmbeddedSource) LoadPreset(ctx context.Context, id string) ([]byte, error) {
	return e.inner.LoadPreset(ctx, id)
}
