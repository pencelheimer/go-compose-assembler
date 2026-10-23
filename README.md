# Compose Assembler

A local Go tool for assembling and customizing Compose specification files (`compose.yaml` / `docker-compose.yml`) and environment files for different project types (web, mobile, low-level, etc.). It supports both an interactive CLI and a local web interface powered by a shared core engine. Built as a university project for the "Go Programming Language" course.

## Getting Started

Use `just` to list available recipes and modules:

```bash
just
```

### Running

```bash
just run              # interactive terminal UI
just run serve        # web UI on http://127.0.0.1:8080 (ASSEMBLER_ADDR overrides)
```

The web UI updates live: creating, editing or deleting a file under `./assembler-presets/` or the user config directory shows up in the open browser tab without a reload.

The web UI is embedded into the binary at build time, so run `just build` (frontend bundle, then Go binary) for a release build. During frontend work, run `just run serve` and `just frontend dev` side by side: Vite serves the SPA with hot reload and proxies `/api` to the Go server.

### Common Commands

```bash
# Backend (or run directly from root)
just backend run      # Run Go assembler
just backend build    # Build binary to bin/assembler
just backend test     # Run Go tests with race detection

# Frontend
just frontend install # Install web dependencies
just frontend dev     # Start Vite development server
just frontend build   # Build SPA production bundle

# Full build and cleanup
just build            # Build frontend bundle + Go binary
just clean            # Clean all build outputs
```
