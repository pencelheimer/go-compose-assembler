// Package cli implements the command-line frontend: generate, list and lint.
// It holds no business logic; it builds requests, calls the injected
// services and renders their output.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
	"github.com/pencelheimer/go-compose-assembler/internal/errs"
)

// ComponentCatalog is the read-side component catalog.
type ComponentCatalog interface {
	ListComponents(ctx context.Context) ([]domain.ComponentSummary, error)
	GetComponent(ctx context.Context, id string) (*domain.ComponentDef, error)
}

// PresetCatalog is the read-side preset catalog.
type PresetCatalog interface {
	ListPresets(ctx context.Context) ([]domain.PresetSummary, error)
	GetPreset(ctx context.Context, id string) (*domain.Preset, error)
}

// Generator assembles a validated project from a request.
type Generator interface {
	Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResult, error)
}

// Exporter renders compose.yaml and .env bytes.
type Exporter interface {
	Export(res *domain.GenerateResult) ([]byte, []byte, error)
}

// Rendered is a generated and exported project ready to be written.
type Rendered struct {
	Compose, Env []byte
	Warnings     []string
}

// Session is everything the full-screen TUI needs; it holds no logic of its own.
type Session struct {
	Components ComponentCatalog
	Presets    PresetCatalog
	// Render generates and exports a request.
	Render func(ctx context.Context, req domain.GenerateRequest) (*Rendered, error)
	// Exists lists the target files already present in dir.
	Exists func(dir string) []string
	// Write stores the files in dir and returns their paths.
	Write func(dir string, r *Rendered) ([]string, error)

	OutDir string // default output directory
	ErrOut io.Writer
}

// Deps are the collaborators of the CLI.
type Deps struct {
	Components ComponentCatalog
	Presets    PresetCatalog
	Generator  Generator
	Exporter   Exporter

	Out io.Writer
	Err io.Writer

	// IsTerminal reports whether the interactive TUI can run.
	IsTerminal func() bool
	// TUI runs the full-screen session and returns the written paths.
	TUI func(ctx context.Context, s Session) ([]string, error)
	// Serve starts the web server; nil until the web layer exists.
	Serve func(ctx context.Context) error
}

// Exit codes, mapped from the category sentinels in errs.
const (
	ExitOK                = 0
	ExitFailure           = 1
	ExitInvalidInput      = 2
	ExitNotFound          = 3
	ExitInvalidDefinition = 4
	ExitConflict          = 5
	ExitAborted           = 130
)

// ErrAborted is returned when the user cancels the interactive session.
var ErrAborted = errors.New("aborted")

// ExitCode maps an error to a process exit code.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitOK
	case errors.Is(err, ErrAborted):
		return ExitAborted
	case errors.Is(err, errs.ErrInvalidInput):
		return ExitInvalidInput
	case errors.Is(err, errs.ErrNotFound):
		return ExitNotFound
	case errors.Is(err, errs.ErrInvalidDefinition):
		return ExitInvalidDefinition
	case errors.Is(err, errs.ErrConflict):
		return ExitConflict
	}
	return ExitFailure
}

const usage = "usage: assembler          interactive session\n       assembler serve    start the web server"

// Run executes the program: no arguments starts the interactive TUI, "serve"
// starts the web server. It returns the process exit code.
func Run(ctx context.Context, d Deps, args []string) int {
	err := run(ctx, d, args)
	if err != nil && !errors.Is(err, ErrAborted) {
		fmt.Fprintln(d.Err, "error:", err)
	}
	return ExitCode(err)
}

func run(ctx context.Context, d Deps, args []string) error {
	switch {
	case len(args) == 0:
		if d.TUI == nil || d.IsTerminal == nil || !d.IsTerminal() {
			return errs.InvalidRequestError{Reason: "the interactive session needs a terminal"}
		}
		paths, err := d.TUI(ctx, newSession(d))
		for _, p := range paths {
			fmt.Fprintln(d.Out, "wrote", p)
		}
		return err
	case len(args) == 1 && args[0] == "serve":
		if d.Serve == nil {
			return errors.New("the web server is not implemented yet")
		}
		return d.Serve(ctx)
	}
	return errs.InvalidRequestError{Reason: usage}
}

func newSession(d Deps) Session {
	return Session{
		Components: d.Components,
		Presets:    d.Presets,
		Render: func(ctx context.Context, req domain.GenerateRequest) (*Rendered, error) {
			res, err := d.Generator.Generate(ctx, req)
			if err != nil {
				return nil, err
			}
			compose, env, err := d.Exporter.Export(res)
			if err != nil {
				return nil, err
			}
			return &Rendered{Compose: compose, Env: env, Warnings: res.Warnings}, nil
		},
		Exists: existingFiles,
		Write: func(dir string, r *Rendered) ([]string, error) {
			return writeAll(dir, r.Compose, r.Env)
		},
		OutDir: ".",
		ErrOut: d.Err,
	}
}

// existingFiles returns the output files that already exist in dir.
func existingFiles(dir string) []string {
	var found []string
	for _, name := range []string{"compose.yaml", ".env"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return found
}

// writeAll writes compose.yaml (0644) and .env (0600, it may hold secrets).
func writeAll(dir string, compose, env []byte) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, t := range []struct {
		name string
		data []byte
		mode fs.FileMode
	}{{"compose.yaml", compose, 0o644}, {".env", env, 0o600}} {
		p := filepath.Join(dir, t.name)
		if err := os.WriteFile(p, t.data, t.mode); err != nil {
			return written, fmt.Errorf("write %s: %w", p, err)
		}
		if err := os.Chmod(p, t.mode); err != nil { // WriteFile keeps the mode of existing files
			return written, fmt.Errorf("chmod %s: %w", p, err)
		}
		written = append(written, p)
	}
	return written, nil
}
