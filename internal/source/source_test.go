package source

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

// --- fsSource / listIDs tests -----------------------------------------------

func TestFsSource_ListIDs_FiltersCorrectly(t *testing.T) {
	memFS := fstest.MapFS{
		"valid.yaml":       {Data: []byte("x")},
		"also-valid.yaml":  {Data: []byte("x")},
		"UPPER.yaml":       {Data: []byte("x")}, // invalid identifier
		"no-ext":           {Data: []byte("x")}, // no .yaml extension
		"readme.txt":       {Data: []byte("x")}, // wrong extension
		"subdir/file.yaml": {Data: []byte("x")}, // inside a subdirectory
	}

	s := &fsSource{name: "test", components: memFS}
	ids, err := s.ListComponents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"also-valid", "valid"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, ids)
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("ids[%d] = %q, want %q", i, id, expected[i])
		}
	}
}

func TestFsSource_ListIDs_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s := &fsSource{name: "test", components: fstest.MapFS{"a.yaml": {Data: []byte("x")}}}
	_, err := s.ListComponents(ctx)
	if err == nil {
		t.Fatal("expected context error")
	}
}

// --- fsSource / loadFile tests -----------------------------------------------

func TestFsSource_LoadFile_ValidFile(t *testing.T) {
	content := []byte("id: mycomp")
	memFS := fstest.MapFS{
		"mycomp.yaml": {Data: content},
	}
	s := &fsSource{name: "test", components: memFS}
	data, err := s.LoadComponent(context.Background(), "mycomp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("data mismatch: got %q", string(data))
	}
}

func TestFsSource_LoadFile_NotFound(t *testing.T) {
	memFS := fstest.MapFS{}
	s := &fsSource{name: "test", components: memFS}
	_, err := s.LoadComponent(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got: %v", err)
	}
}

func TestFsSource_LoadFile_NilFS(t *testing.T) {
	s := &fsSource{name: "test", components: nil}
	_, err := s.LoadComponent(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got: %v", err)
	}
}

func TestFsSource_LoadFile_OversizedFile(t *testing.T) {
	bigData := make([]byte, MaxFileSize+1)
	memFS := fstest.MapFS{
		"big.yaml": {Data: bigData},
	}
	s := &fsSource{name: "test", components: memFS}
	_, err := s.LoadComponent(context.Background(), "big")
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}

// --- Traversal safety --------------------------------------------------------

// trapFS panics if any read is attempted, proving no filesystem access occurs.
type trapFS struct{}

func (trapFS) Open(name string) (fs.File, error) {
	panic("filesystem was accessed with name: " + name)
}

func TestFsSource_TraversalSafety(t *testing.T) {
	badIDs := []string{
		"../etc/passwd",
		"/etc/passwd",
		"foo/bar",
		"UPPERCASE",
		"has.dot",
		"has_underscore",
		"",
		"-starts-with-dash",
		"a b",
	}

	s := &fsSource{name: "test", components: trapFS{}}
	for _, id := range badIDs {
		t.Run(id, func(t *testing.T) {
			_, err := s.LoadComponent(context.Background(), id)
			if err == nil {
				t.Fatal("expected error for bad identifier")
			}
			if !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("expected fs.ErrNotExist, got: %v", err)
			}
		})
	}
}

// --- DirSource ---------------------------------------------------------------

func TestDirSource_MissingBaseDir(t *testing.T) {
	ds := NewDirSource("/nonexistent/path/that/does/not/exist")
	ctx := context.Background()

	ids, err := ds.ListComponents(ctx)
	if err != nil {
		t.Fatalf("expected empty result, got error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}

	ids, err = ds.ListPresets(ctx)
	if err != nil {
		t.Fatalf("expected empty result, got error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}
}

func TestDirSource_MissingSubdirs(t *testing.T) {
	// Use a temp dir with no components/ or presets/ subdirs.
	ds := NewDirSource(t.TempDir())
	ctx := context.Background()

	ids, err := ds.ListComponents(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}
}

// --- EmbeddedSource ----------------------------------------------------------

func TestEmbeddedSource_ShippedDefaults(t *testing.T) {
	// NewEmbeddedSource panics on broken content; if this succeeds,
	// all shipped defaults are valid.
	src := NewEmbeddedSource()
	if src.Name() != "embedded" {
		t.Errorf("expected name 'embedded', got %q", src.Name())
	}

	ctx := context.Background()
	comps, err := src.ListComponents(ctx)
	if err != nil {
		t.Fatalf("listing components: %v", err)
	}
	if len(comps) < 4 {
		t.Errorf("expected at least 4 components, got %d", len(comps))
	}

	presets, err := src.ListPresets(ctx)
	if err != nil {
		t.Fatalf("listing presets: %v", err)
	}
	if len(presets) < 3 {
		t.Errorf("expected at least 3 presets, got %d", len(presets))
	}
}

func TestEmbeddedSource_PanicsOnBrokenComponent(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for broken embedded component")
		}
	}()

	brokenComponents := fstest.MapFS{
		"broken.yaml": {Data: []byte("not: valid: component: yaml: [")},
	}
	validPresets := fstest.MapFS{}
	newEmbeddedSource(brokenComponents, validPresets)
}

func TestEmbeddedSource_PanicsOnBrokenPreset(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for broken embedded preset")
		}
	}()

	validComponents := fstest.MapFS{}
	brokenPresets := fstest.MapFS{
		"broken.yaml": {Data: []byte("not: valid: preset: yaml: [")},
	}
	newEmbeddedSource(validComponents, brokenPresets)
}

func TestEmbeddedSource_PanicsOnInvalidComponent(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid component definition")
		}
	}()

	// Valid YAML but fails component validation (missing image).
	invalidComponent := fstest.MapFS{
		"noimage.yaml": {Data: []byte(`id: noimage
description: missing image
params: []
service:
  ports: ["80:80"]
`)},
	}
	newEmbeddedSource(invalidComponent, fstest.MapFS{})
}
