package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/pencelheimer/go-compose-assembler/internal/assemble"
	"github.com/pencelheimer/go-compose-assembler/internal/cli"
	"github.com/pencelheimer/go-compose-assembler/internal/export"
	"github.com/pencelheimer/go-compose-assembler/internal/source"
	"github.com/pencelheimer/go-compose-assembler/internal/web"
)

// defaultAddr is where `assembler serve` listens; ASSEMBLER_ADDR overrides it.
// Loopback only: the tool is local and the API is unauthenticated.
const defaultAddr = "127.0.0.1:8080"

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
	engine := assemble.NewEngine(catalog, catalog, nil)
	d := cli.Deps{
		Components: catalog,
		Presets:    catalog,
		Generator:  engine,
		Exporter:   export.YAMLExporter{},
		Out:        os.Stdout,
		Err:        os.Stderr,
		IsTerminal: cli.StdTerminal,
		TUI:        cli.RunTUI,
		Serve: func(ctx context.Context) error {
			addr := os.Getenv("ASSEMBLER_ADDR")
			if addr == "" {
				addr = defaultAddr
			}
			return web.Serve(ctx, addr, web.Deps{
				Components: catalog,
				Presets:    catalog,
				Generator:  engine,
				Exporter:   export.YAMLExporter{},
				Logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
			}, os.Stdout)
		},
	}
	code := cli.Run(ctx, d, os.Args[1:])
	stop()
	os.Exit(code)
}
