# Load .env file if present
set dotenv-load

# Go services, tests, linting
mod backend

# TanStack Router SPA
mod frontend

# List all recipes (including modules)
default:
    @just --list --list-submodules

# Build both frontend and backend
build:
    just frontend build
    just backend build

# Run the assembler
run *args:
    just backend run {{args}}

# Run all tests with race detector
test:
    just backend test

# Check code with go vet
vet:
    just backend vet
alias lint := vet

# Format Go source files
fmt:
    just backend fmt

# Clean all build artifacts
clean:
    just backend clean
    just frontend clean
