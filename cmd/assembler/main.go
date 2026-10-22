package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/pencelheimer/go-compose-assembler/internal/assemble"
	"github.com/pencelheimer/go-compose-assembler/internal/cli"
	"github.com/pencelheimer/go-compose-assembler/internal/export"
	"github.com/pencelheimer/go-compose-assembler/internal/source"
)

func main() {
	// Top-level recover: a readable message instead of a stack trace. Broken
	// embedded content panics at catalog construction and ends up here.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "internal error: %v\n", r)
			os.Exit(70)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	catalog := source.NewDefaultCatalog()
	d := cli.Deps{
		Components: catalog,
		Presets:    catalog,
		Generator:  assemble.NewEngine(catalog, catalog, nil),
		Exporter:   export.YAMLExporter{},
		Out:        os.Stdout,
		Err:        os.Stderr,
		IsTerminal: cli.StdTerminal,
		TUI:        cli.RunTUI,
	}
	code := cli.Run(ctx, d, os.Args[1:])
	stop()
	os.Exit(code)
}
