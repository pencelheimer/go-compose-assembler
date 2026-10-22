package source

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// MaxFileSize is the maximum allowed size for a definition file (1 MiB).
const MaxFileSize = 1 << 20

// identifierPattern validates component/preset identifiers.
// 1–64 chars: lowercase ASCII letters, digits and hyphens, starting with a letter or digit.
var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// ValidIdentifier reports whether id matches the identifier pattern.
func ValidIdentifier(id string) bool {
	return identifierPattern.MatchString(id)
}

// Source is a read-only provider of raw definition files from one location.
type Source interface {
	// Name returns a human-readable location label (directory path or "embedded").
	Name() string
	// ListComponents returns all component identifiers available in this source.
	ListComponents(ctx context.Context) ([]string, error)
	// LoadComponent returns the raw bytes of the component definition for the given id.
	// Returns an error wrapping fs.ErrNotExist if the component is not found.
	LoadComponent(ctx context.Context, id string) ([]byte, error)
	// ListPresets returns all preset identifiers available in this source.
	ListPresets(ctx context.Context) ([]string, error)
	// LoadPreset returns the raw bytes of the preset definition for the given id.
	// Returns an error wrapping fs.ErrNotExist if the preset is not found.
	LoadPreset(ctx context.Context, id string) ([]byte, error)
}

// fsSource is the shared implementation for EmbeddedSource and DirSource.
// It reads component and preset YAML files from an fs.FS.
type fsSource struct {
	name       string
	components fs.FS // rooted at the "components" subdirectory (may be nil)
	presets    fs.FS // rooted at the "presets" subdirectory (may be nil)
}

func (s *fsSource) Name() string { return s.name }

func (s *fsSource) ListComponents(ctx context.Context) ([]string, error) {
	return s.listIDs(ctx, s.components)
}

func (s *fsSource) LoadComponent(ctx context.Context, id string) ([]byte, error) {
	return s.loadFile(ctx, s.components, id)
}

func (s *fsSource) ListPresets(ctx context.Context) ([]string, error) {
	return s.listIDs(ctx, s.presets)
}

func (s *fsSource) LoadPreset(ctx context.Context, id string) ([]byte, error) {
	return s.loadFile(ctx, s.presets, id)
}

// listIDs reads the directory from subFS, filters for .yaml files, validates
// identifiers, and returns a sorted slice.
func (s *fsSource) listIDs(ctx context.Context, subFS fs.FS) ([]string, error) {
	if subFS == nil {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(subFS, ".")
	if err != nil {
		return nil, fmt.Errorf("source %s: reading directory: %w", s.name, err)
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

// loadFile validates the identifier, reads id+".yaml" from subFS, checks
// MaxFileSize, and returns the raw bytes.
func (s *fsSource) loadFile(ctx context.Context, subFS fs.FS, id string) ([]byte, error) {
	if !ValidIdentifier(id) {
		return nil, fmt.Errorf("source %s: identifier %q: %w", s.name, id, fs.ErrNotExist)
	}
	if subFS == nil {
		return nil, fmt.Errorf("source %s: identifier %q: %w", s.name, id, fs.ErrNotExist)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	filename := id + ".yaml"

	info, err := fs.Stat(subFS, filename)
	if err != nil {
		return nil, fmt.Errorf("source %s: identifier %q: %w", s.name, id, err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("source %s: identifier %q: file exceeds maximum size of %d bytes", s.name, id, MaxFileSize)
	}

	data, err := fs.ReadFile(subFS, filename)
	if err != nil {
		return nil, fmt.Errorf("source %s: identifier %q: %w", s.name, id, err)
	}
	return data, nil
}
