# Compose Assembler

A local Go tool for assembling and customizing Compose specification files (`compose.yaml` / `docker-compose.yml`) and environment files for different project types (web, mobile, low-level, etc.). It supports both an interactive CLI and a local web interface powered by a shared core engine. Built as a university project for the "Go Programming Language" course.

## Getting Started

Use `just` to list available recipes and modules:

```bash
just
```

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
just build-all        # Build frontend bundle + Go binary
just clean            # Clean all build outputs
```
