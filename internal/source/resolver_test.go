package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// --- mock source for resolver tests ------------------------------------------

// mockSource is a test double implementing Source.
type mockSource struct {
	name       string
	components map[string][]byte // id → content
	presets    map[string][]byte
	listErr    error           // error returned by ListComponents/ListPresets
	loadErr    error           // error returned by LoadComponent/LoadPreset (non-not-found)
	blockUntil <-chan struct{} // if non-nil, blocks until closed
}

func (m *mockSource) Name() string { return m.name }

func (m *mockSource) ListComponents(ctx context.Context) ([]string, error) {
	if m.blockUntil != nil {
		select {
		case <-m.blockUntil:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.listErr != nil {
		return nil, m.listErr
	}
	var ids []string
	for id := range m.components {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockSource) LoadComponent(ctx context.Context, id string) ([]byte, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	data, ok := m.components[id]
	if !ok {
		return nil, fmt.Errorf("source %s: %q: %w", m.name, id, fs.ErrNotExist)
	}
	return data, nil
}

func (m *mockSource) ListPresets(ctx context.Context) ([]string, error) {
	if m.blockUntil != nil {
		select {
		case <-m.blockUntil:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.listErr != nil {
		return nil, m.listErr
	}
	var ids []string
	for id := range m.presets {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *mockSource) LoadPreset(ctx context.Context, id string) ([]byte, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	data, ok := m.presets[id]
	if !ok {
		return nil, fmt.Errorf("source %s: %q: %w", m.name, id, fs.ErrNotExist)
	}
	return data, nil
}

// --- Resolver listing tests --------------------------------------------------

func TestResolver_ListComponents_PriorityAndDedup(t *testing.T) {
	high := &mockSource{
		name:       "high",
		components: map[string][]byte{"postgres": []byte("high"), "nginx": []byte("high")},
	}
	low := &mockSource{
		name:       "low",
		components: map[string][]byte{"postgres": []byte("low"), "redis": []byte("low")},
	}

	r := NewResolver(high, low)
	ids, err := r.ListComponents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"nginx", "postgres", "redis"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, ids)
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("ids[%d] = %q, want %q", i, id, expected[i])
		}
	}
}

func TestResolver_ListComponents_Deterministic(t *testing.T) {
	src := &mockSource{
		name:       "src",
		components: map[string][]byte{"c": nil, "a": nil, "b": nil},
	}
	r := NewResolver(src)

	var first []string
	for i := range 50 {
		ids, err := r.ListComponents(context.Background())
		if err != nil {
			t.Fatalf("run %d: unexpected error: %v", i, err)
		}
		if i == 0 {
			first = ids
		} else {
			if len(ids) != len(first) {
				t.Fatalf("run %d: length changed", i)
			}
			for j := range ids {
				if ids[j] != first[j] {
					t.Fatalf("run %d: ids[%d] = %q, want %q", i, j, ids[j], first[j])
				}
			}
		}
	}
}

func TestResolver_ListComponents_FailingSourceStillReturnsOthers(t *testing.T) {
	good := &mockSource{
		name:       "good",
		components: map[string][]byte{"postgres": nil},
	}
	bad := &mockSource{
		name:    "bad",
		listErr: fmt.Errorf("disk on fire"),
	}

	r := NewResolver(good, bad)
	ids, err := r.ListComponents(context.Background())

	// Should still get results from the good source.
	if len(ids) != 1 || ids[0] != "postgres" {
		t.Errorf("expected [postgres], got %v", ids)
	}
	// Should also get an error.
	if err == nil {
		t.Fatal("expected non-nil joined error")
	}
}

func TestResolver_ListComponents_ContextCancellation(t *testing.T) {
	block := make(chan struct{})
	var goroutineReturned atomic.Bool

	blocking := &mockSource{
		name:       "blocking",
		components: map[string][]byte{"x": nil},
		blockUntil: block,
	}

	r := NewResolver(blocking)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = r.ListComponents(ctx)
		goroutineReturned.Store(true)
	}()

	// Cancel the context.
	cancel()

	// The call should return promptly.
	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("ListComponents did not return after context cancellation")
	}

	// Unblock the source so its goroutine can exit.
	close(block)

	// Give goroutines a moment to clean up.
	time.Sleep(50 * time.Millisecond)
}

// --- Resolver loading tests --------------------------------------------------

func TestResolver_LoadComponent_FirstMatchWins(t *testing.T) {
	high := &mockSource{
		name:       "high",
		components: map[string][]byte{"postgres": []byte("from-high")},
	}
	low := &mockSource{
		name:       "low",
		components: map[string][]byte{"postgres": []byte("from-low")},
	}

	r := NewResolver(high, low)
	data, srcName, err := r.LoadComponent(context.Background(), "postgres")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "from-high" {
		t.Errorf("expected data from high source, got %q", string(data))
	}
	if srcName != "high" {
		t.Errorf("expected source name 'high', got %q", srcName)
	}
}

func TestResolver_LoadComponent_FallsThrough(t *testing.T) {
	// High source doesn't have it, low source does.
	high := &mockSource{
		name:       "high",
		components: map[string][]byte{},
	}
	low := &mockSource{
		name:       "low",
		components: map[string][]byte{"redis": []byte("from-low")},
	}

	r := NewResolver(high, low)
	data, srcName, err := r.LoadComponent(context.Background(), "redis")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "from-low" {
		t.Errorf("expected data from low source, got %q", string(data))
	}
	if srcName != "low" {
		t.Errorf("expected source name 'low', got %q", srcName)
	}
}

func TestResolver_LoadComponent_NonAbsenceErrorStopsWalk(t *testing.T) {
	permErr := fmt.Errorf("permission denied")
	high := &mockSource{
		name:    "high",
		loadErr: permErr,
	}
	low := &mockSource{
		name:       "low",
		components: map[string][]byte{"postgres": []byte("from-low")},
	}

	r := NewResolver(high, low)
	_, _, err := r.LoadComponent(context.Background(), "postgres")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, permErr) {
		t.Fatalf("expected permission error, got: %v", err)
	}
}

func TestResolver_LoadComponent_NotFound_SearchedLocations(t *testing.T) {
	s1 := &mockSource{name: "dir1", components: map[string][]byte{}}
	s2 := &mockSource{name: "dir2", components: map[string][]byte{}}
	s3 := &mockSource{name: "embedded", components: map[string][]byte{}}

	r := NewResolver(s1, s2, s3)
	_, _, err := r.LoadComponent(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}

	var notFound errs.ComponentNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected ComponentNotFoundError, got %T: %v", err, err)
	}
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatal("expected error to match ErrNotFound")
	}
	if len(notFound.SearchedLocations) != 3 {
		t.Fatalf("expected 3 searched locations, got %d: %v", len(notFound.SearchedLocations), notFound.SearchedLocations)
	}
	if notFound.SearchedLocations[0] != "dir1" {
		t.Errorf("first location should be dir1, got %q", notFound.SearchedLocations[0])
	}
}

func TestResolver_LoadPreset_NotFound(t *testing.T) {
	s := &mockSource{name: "src", presets: map[string][]byte{}}
	r := NewResolver(s)
	_, _, err := r.LoadPreset(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}

	var notFound errs.PresetNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected PresetNotFoundError, got %T: %v", err, err)
	}
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatal("expected error to match ErrNotFound")
	}
}

func TestResolver_LoadPreset_FirstMatchWins(t *testing.T) {
	high := &mockSource{
		name:    "high",
		presets: map[string][]byte{"web": []byte("from-high")},
	}
	low := &mockSource{
		name:    "low",
		presets: map[string][]byte{"web": []byte("from-low")},
	}

	r := NewResolver(high, low)
	data, _, err := r.LoadPreset(context.Background(), "web")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "from-high" {
		t.Errorf("expected from-high, got %q", string(data))
	}
}

func TestResolver_ListComponents_CancellationReportsAndLeaksNothing(t *testing.T) {
	before := runtime.NumGoroutine()
	r := NewResolver(
		&mockSource{name: "fast", components: map[string][]byte{"a": nil}},
		&mockSource{name: "stuck", blockUntil: make(chan struct{})},
	)
	for range 20 {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := r.ListComponents(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("want context.Canceled in error, got %v", err)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("goroutines leaked: before=%d after=%d", before, after)
	}
}

func TestDefaultResolver_CwdAndConfigDirOverrideEmbedded(t *testing.T) {
	cwd, cfg := t.TempDir(), t.TempDir()
	t.Chdir(cwd)
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", cfg)

	write := func(path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	comp := func(id, desc string) string {
		return "id: " + id + "\ndescription: " + desc + "\nservice:\n  image: x\n"
	}
	// cwd shadows the embedded redis; the config dir adds a brand new component.
	write(filepath.Join(cwd, "assembler-presets", "components", "redis.yaml"), comp("redis", "from cwd"))
	write(filepath.Join(cfg, "assembler", "components", "extra.yaml"), comp("extra", "from config"))

	cat := NewDefaultCatalog()
	ctx := context.Background()
	def, err := cat.GetComponent(ctx, "redis")
	if err != nil || def.Description != "from cwd" {
		t.Fatalf("cwd override: %+v, %v", def, err)
	}
	list, err := cat.ListComponents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range list {
		ids = append(ids, c.ID)
	}
	if !slices.Contains(ids, "extra") || !slices.Contains(ids, "postgres") {
		t.Fatalf("expected user and embedded components, got %v", ids)
	}
	// Edits are picked up without restart (no caching).
	write(filepath.Join(cwd, "assembler-presets", "components", "redis.yaml"), comp("redis", "edited"))
	if def, _ := cat.GetComponent(ctx, "redis"); def == nil || def.Description != "edited" {
		t.Fatalf("expected edited definition, got %+v", def)
	}
}
