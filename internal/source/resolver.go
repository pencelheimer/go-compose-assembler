package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// Resolver owns an ordered list of sources (highest priority first) and
// applies first-match-wins with no merging. It is safe for concurrent use.
type Resolver struct {
	sources []Source
}

// NewResolver creates a Resolver with the given sources in priority order.
func NewResolver(sources ...Source) *Resolver {
	return &Resolver{sources: sources}
}

// DefaultDirs returns the directory sources in priority order:
//  1. ./assembler-presets/ (current working directory)
//  2. <user-config-dir>/assembler/
//
// A directory whose location cannot be determined is omitted.
func DefaultDirs() []string {
	var dirs []string
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, "assembler-presets"))
	}
	if configDir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(configDir, "assembler"))
	}
	return dirs
}

// NewDefaultResolver builds the standard priority order: the DefaultDirs,
// then the embedded defaults.
func NewDefaultResolver() *Resolver {
	var sources []Source
	for _, dir := range DefaultDirs() {
		sources = append(sources, NewDirSource(dir))
	}
	sources = append(sources, NewEmbeddedSource())
	return NewResolver(sources...)
}

// listResult is the fan-in message for concurrent listing.
type listResult struct {
	index int
	ids   []string
	err   error
}

// ListComponents returns the deduplicated, sorted union of component identifiers
// across all sources. For duplicate identifiers, the highest-priority source wins.
// Individual source failures are collected into a joined error alongside the results.
func (r *Resolver) ListComponents(ctx context.Context) ([]string, error) {
	return r.listAll(ctx, func(s Source, c context.Context) ([]string, error) {
		return s.ListComponents(c)
	})
}

// ListPresets returns the deduplicated, sorted union of preset identifiers.
func (r *Resolver) ListPresets(ctx context.Context) ([]string, error) {
	return r.listAll(ctx, func(s Source, c context.Context) ([]string, error) {
		return s.ListPresets(c)
	})
}

// listAll runs lister on every source concurrently and merges the results.
// It returns promptly if the context is cancelled, even if some sources have
// not finished yet. Goroutines are not leaked because the channel is buffered
// to len(sources), so sends never block after the caller returns.
func (r *Resolver) listAll(ctx context.Context, lister func(Source, context.Context) ([]string, error)) ([]string, error) {
	n := len(r.sources)
	if n == 0 {
		return nil, nil
	}

	ch := make(chan listResult, n)
	var wg sync.WaitGroup

	for i, src := range r.sources {
		wg.Add(1)
		go func(idx int, s Source) {
			defer wg.Done()
			ids, err := lister(s, ctx)
			// Non-blocking send: channel is buffered to n.
			ch <- listResult{index: idx, ids: ids, err: err}
		}(i, src)
	}

	// Close the channel after all goroutines finish.
	go func() {
		wg.Wait()
		close(ch)
	}()

	// Collect results with context-cancellation awareness.
	results := make([]listResult, n)
	var collectedErrs []error
	collected := 0
loop:
	for collected < n {
		select {
		case res := <-ch:
			results[res.index] = res
			collected++
		case <-ctx.Done():
			collectedErrs = append(collectedErrs, ctx.Err())
			break loop
		}
	}

	// Merge whatever results we have (may be partial on cancellation).
	seen := make(map[string]bool)
	var ids []string

	for _, res := range results {
		if res.err != nil {
			collectedErrs = append(collectedErrs, res.err)
		}
		for _, id := range res.ids {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}

	sort.Strings(ids)

	var joined error
	if len(collectedErrs) > 0 {
		joined = errors.Join(collectedErrs...)
	}
	return ids, joined
}

// LoadComponent walks sources in priority order and returns the raw bytes and
// source name for the first match. A "not found" outcome (fs.ErrNotExist) moves
// to the next source; any other error stops the walk. If every source reports
// absence, ComponentNotFoundError is returned.
func (r *Resolver) LoadComponent(ctx context.Context, id string) ([]byte, string, error) {
	var locations []string
	for _, src := range r.sources {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		data, err := src.LoadComponent(ctx, id)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				locations = append(locations, src.Name())
				continue
			}
			return nil, "", err
		}
		return data, src.Name(), nil
	}
	return nil, "", errs.ComponentNotFoundError{ID: id, SearchedLocations: locations}
}

// LoadPreset walks sources in priority order and returns the raw bytes and
// source name for the first match. If every source reports absence,
// PresetNotFoundError is returned.
func (r *Resolver) LoadPreset(ctx context.Context, id string) ([]byte, string, error) {
	var locations []string
	for _, src := range r.sources {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		data, err := src.LoadPreset(ctx, id)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				locations = append(locations, src.Name())
				continue
			}
			return nil, "", err
		}
		return data, src.Name(), nil
	}
	return nil, "", errs.PresetNotFoundError{ID: id, SearchedLocations: locations}
}
