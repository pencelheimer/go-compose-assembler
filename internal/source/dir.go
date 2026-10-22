package source

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// DirSource serves components and presets from a filesystem directory.
// A missing base directory or missing subdirectories behaves as an empty source.
// Permission errors and other real failures are returned.
type DirSource struct {
	name   string
	baseFS fs.FS
}

// NewDirSource creates a DirSource rooted at baseDir.
// It expects optional components/ and presets/ subdirectories inside baseDir.
func NewDirSource(baseDir string) *DirSource {
	return &DirSource{name: baseDir, baseFS: os.DirFS(baseDir)}
}

// newDirSourceFromFS creates a DirSource from an arbitrary fs.FS.
// Accessible from in-package tests for in-memory filesystem testing.
func newDirSourceFromFS(name string, baseFS fs.FS) *DirSource {
	return &DirSource{name: name, baseFS: baseFS}
}

func (d *DirSource) Name() string { return d.name }

func (d *DirSource) ListComponents(ctx context.Context) ([]string, error) {
	return d.listIDs(ctx, "components")
}

func (d *DirSource) LoadComponent(ctx context.Context, id string) ([]byte, error) {
	return d.loadFile(ctx, "components", id)
}

func (d *DirSource) ListPresets(ctx context.Context) ([]string, error) {
	return d.listIDs(ctx, "presets")
}

func (d *DirSource) LoadPreset(ctx context.Context, id string) ([]byte, error) {
	return d.loadFile(ctx, "presets", id)
}

// listIDs reads the subdir within the base FS, filters for .yaml files,
// validates identifiers, and returns a sorted slice.
// A missing base or subdir is not an error (empty result).
// Other failures (e.g. permission errors) are returned.
func (d *DirSource) listIDs(ctx context.Context, subdir string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(d.baseFS, subdir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil // missing → empty
		}
		return nil, fmt.Errorf("source %s: reading %s: %w", d.name, subdir, err)
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		id := strings.TrimSuffix(name, ".yaml")
		if !ValidIdentifier(id) {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// loadFile reads a single definition file from subdir/id.yaml within the base FS.
func (d *DirSource) loadFile(ctx context.Context, subdir string, id string) ([]byte, error) {
	if !ValidIdentifier(id) {
		return nil, fmt.Errorf("source %s: identifier %q: %w", d.name, id, fs.ErrNotExist)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path := subdir + "/" + id + ".yaml"

	info, err := fs.Stat(d.baseFS, path)
	if err != nil {
		if isNotExist(err) {
			return nil, fmt.Errorf("source %s: identifier %q: %w", d.name, id, fs.ErrNotExist)
		}
		return nil, fmt.Errorf("source %s: identifier %q: %w", d.name, id, err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("source %s: identifier %q: file exceeds maximum size of %d bytes", d.name, id, MaxFileSize)
	}

	data, err := fs.ReadFile(d.baseFS, path)
	if err != nil {
		return nil, fmt.Errorf("source %s: identifier %q: %w", d.name, id, err)
	}
	return data, nil
}

// isNotExist checks if an error represents a "not found" condition.
func isNotExist(err error) bool {
	return err != nil && (errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err))
}
