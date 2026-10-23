# AGENTS.md — Compose Assembler

This file describes the fixed architecture of the project. Before changing the package layout, interfaces, or YAML formats, check against it. If you need a decision that isn't covered here, ask instead of inventing one.

## What this is

A local Go tool for assembling and customizing compose files (Compose Specification) for different project types (web, low-level, mobile, etc.). A university project for the "Go Programming Language" course, built by two people.

Two modes of operation, both consumers of the same core:

- CLI — the interactive TUI (the default command).
- Web — the serve command starts a local HTTP server with a browser UI.

## Course requirements → where they live in the code

| Lab topic | Where it is used |
|---|---|
| Goroutines and channels | source.Resolver.List* (fan-out over sources), validate.RunAll (fan-out/fan-in), optionally an SSE broadcaster in web |
| Errors and custom errors | errs — custom types, wrapping via %w, inspection via errors.Is / errors.As |
| Panics and recover | panic for programmer errors (duplicate registrations, broken embedded content); recover in main and in the HTTP middleware |
| Interfaces and associated functions | Source, Validator, TargetMerger, Exporter |
| Web server | internal/web — REST API + frontend SPA |

## Data flow

1. The frontend (CLI or Web) builds a `GenerateRequest`: either a preset name with optional per-component overrides (`Overrides map[string]map[string]any`) or an ad-hoc list of services (`Services []ServiceSelection`, each with a component ID and parameters `map[string]any`). Preset mode allows overrides but no services list; ad-hoc mode has no preset and no overrides; specifying both or neither yields `InvalidRequestError`. Overrides targeting a component not in the selected preset yield `InvalidRequestError`. In v1, there is a 1:1 mapping between component and service (service name equals component ID); selecting a component twice yields `DuplicateServiceError`.
2. Engine.Generate (taking a `context.Context` and `GenerateRequest`) handles parameter resolution following strict precedence: request overrides > preset values > component defaults. Null values are treated as unset (falling through). It then checks that every required parameter was provided, returning `MissingRequiredParamError` otherwise. Unknown parameters or type mismatches return `UnknownParamError` / `ParamTypeMismatchError`.
3. Scalar parameters are normalized to strings for `.env`: whole numbers become integer digit strings, booleans become `"true"`/`"false"`, strings are kept, and numbers with fractional parts are rejected with `ParamTypeMismatchError` (prompting the user to quote the value). The engine enforces the environment collision policy: scalar parameters follow the component prefix convention; if two distinct components resolve the same `.env` key to different values, the engine halts with `EnvConflictError` (identical values are permitted). List parameters are dispatched to the matching `TargetMerger` registered for their target, and an unregistered target yields `UnknownTargetError`.
4. The engine combines the resulting services into an in-memory `domain.Project`, merging the top-level volumes and networks declared by the components. `Project` represents domain state, not a raw Compose document.
5. validate.RunAll runs every registered Validator against the assembled `domain.Project` in parallel and collects their errors.
   Validators see a copy of the project with `${VAR}` references substituted from the `.env` map (only so ports etc. can be compared); the returned `Project` and the exported files keep the references and Compose does the real substitution.
6. If validation passes, Engine.Generate returns a `domain.GenerateResult` containing the `Project`, the `.env` scalar map, and any non-fatal warnings. The frontend then calls an Exporter to render the Compose specification document (`compose.yaml`) and `.env` file bytes.

A `depends_on` entry naming a service that is not part of the project is not an error: the result carries a warning (Compose will reject such a file, so the frontend should surface it).

Errors from steps 2 to 5 are returned to the frontend, which decides how to present them.

Separation of responsibilities:

- Engine owns parameter resolution (precedence, defaults, overrides), required and type checks, scalar normalization, target dispatch, and environment variable conflict detection.
- Validator answers "does the already-assembled project conflict with itself" (ports across services, duplicate names, depends_on cycles, volume collisions). Under the single-instance policy, the duplicate-name validator acts as a defensive check.
- CLI and Web contain no business logic: they declare the consumer interfaces (catalogs, generator, exporter), populate a GenerateRequest, call Generate, and invoke the Exporter to render output artifacts.

## Project layout

```
cmd/assembler/main.go        — entrypoint (TUI by default, `serve`), top-level recover

frontend/                    — TanStack Router SPA (Vite + TypeScript)
  src/
  package.json
  vite.config.ts

internal/
  domain/                    — plain structs, no logic and no dependencies on other internal packages
    project.go               — Project, Service, Volume, Network, HealthCheck
    component.go             — ComponentDef, ComponentParam, ComponentSummary, ComponentVolume, ComponentNetwork
    preset.go                — Preset, PresetService, PresetSummary
    request.go               — GenerateRequest, ServiceSelection, GenerateResult
  errs/errors.go             — all custom error types
  source/
    source.go                — Source interface, identifier pattern, shared fs.FS-backed implementation
    parse.go                 — ParseComponent / ParsePreset: strict decoding and load-time validation
    embedded.go              — EmbeddedSource (go:embed), validates all content eagerly, panics if broken
    dir.go                   — DirSource (cwd / config dir); a missing directory behaves as empty
    resolver.go              — ordered lookup (first match wins), parallel List*
    catalog.go               — Catalog: implements ComponentCatalog + PresetCatalog over a Resolver
    components/*.yaml        — built-in components (postgres, redis, nginx, go-app, ...)
    presets/*.yaml           — built-in presets (web, low-level, mobile)
  assemble/
    engine.go                — Engine.Generate
    params.go                — scalar normalization, list type checks
    subst.go                 — ${VAR} expansion used only to build the copy validators inspect
    target.go                — TargetMerger interface + registry + shared merger helpers
    merge_ports.go           — PortsMerger (+ init registration)
    merge_volumes.go         — VolumesMerger
    merge_env.go             — EnvironmentMerger
  validate/
    validator.go             — Validator interface, RunAll (fan-out/fan-in)
    ports.go
    names.go
    depends.go
    volumes.go               — volume name collisions
  export/
    exporter.go              — Exporter interface
    yaml.go                  — compose YAML and .env rendering
    json.go                  — for the web API
  cli/                       — consumer interfaces, Deps, Run (cli.go); full-screen bubbletea session (tui.go);
                               pure request-building helpers for it (wizard.go)
  web/
    server.go                — consumer interfaces, Deps, NewHandler (routes), Serve (graceful shutdown)
    middleware.go            — recover middleware, logging
    handlers.go
    errors.go                — error → HTTP status + structured details
    static.go                — embedded SPA serving with index.html fallback
    sse.go                   — optional
    static/                  — SPA build output (Vite writes here; only .gitkeep is tracked)
```

go:embed cannot see files above the package directory, so the built-in components/ and presets/ live inside internal/source/.

## Key interfaces

```go
// source (internal to source package)
type Source interface {
    Name() string                                                 // "embedded" or the directory path
    ListComponents(ctx context.Context) ([]string, error)
    LoadComponent(ctx context.Context, id string) ([]byte, error) // raw YAML; wraps fs.ErrNotExist if absent
    ListPresets(ctx context.Context) ([]string, error)
    LoadPreset(ctx context.Context, id string) ([]byte, error)
}

// read-side catalog (consumed by CLI and Web)
type ComponentCatalog interface {
    ListComponents(ctx context.Context) ([]domain.ComponentSummary, error)
    GetComponent(ctx context.Context, id string) (*domain.ComponentDef, error)
}

type PresetCatalog interface {
    ListPresets(ctx context.Context) ([]domain.PresetSummary, error)
    GetPreset(ctx context.Context, id string) (*domain.Preset, error)
}

// generator contract (consumed by CLI and Web)
type Generator interface {
    Generate(ctx context.Context, req domain.GenerateRequest) (*domain.GenerateResult, error)
}

// assemble
type TargetMerger interface {
    Target() string                                  // "ports", "volumes", "environment", "checks", ...
    Merge(svc *domain.Service, value any) error      // value arrives decoded from JSON/YAML as scalar or slice
}

// validate
type Validator interface {
    Name() string
    Validate(p *domain.Project) []error
}

// export (consumed by CLI and Web)
type Exporter interface {
    Export(res *domain.GenerateResult) ([]byte, []byte, error) // compose document bytes, .env bytes, error
}
```

Registries:

- Engine validators — injected into `assemble.NewEngine`; nil means `validate.Default()`.
- Source and Validator — slices (run all of them, collect the results; order doesn't matter).
- TargetMerger — a map map[string]TargetMerger (lookup by target). Registered via RegisterTarget in init(), panics on duplicates.

## File formats

### Component (components/postgres.yaml)

One file = one service template. ${VAR} substitution is performed by Compose itself from .env; the assembler does not do it.

```yaml
id: postgres
description: PostgreSQL database
params:
  - name: POSTGRES_PASSWORD
    type: scalar
    required: true
    description: Database password
  - name: POSTGRES_VERSION
    type: scalar
    required: false
    default: "16"
  - name: POSTGRES_PORT
    type: scalar
    required: false
    default: "5432"
  - name: EXTRA_PORTS
    type: list
    target: ports            # the TargetMerger that will append the elements
    required: false
    description: Additional host:container mappings
service:
  image: "postgres:${POSTGRES_VERSION}"
  ports: ["${POSTGRES_PORT}:5432"]
  environment:
    POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
  volumes: ["pgdata:/var/lib/postgresql/data"]
volumes:
  - name: pgdata             # the engine adds it to the top-level volumes:
```

- type: scalar → the value goes into .env; if the user didn't set it, default is used; if it's required and missing → `MissingRequiredParamError`. Parameter declaration rules: a parameter with `required: true` must not declare a `default`; every non-required scalar parameter must declare a `default`. Violations are rejected during component loading with `InvalidComponentError`.
- type: list → the value is dispatched to the service via the TargetMerger matching target. List parameters are not injected into .env.

Canonical service structure in component YAML:
- `environment`: map-style key-value pairs (`KEY: "${VAL}"`).
- `depends_on`: map-style with condition (`dependency_name: { condition: service_healthy }`).
- `ports`: short string format (`"${PORT}:5432"`).
- `volumes`: short string format (`"pgdata:/var/lib/postgresql/data"`).
- `networks`: list of network names.
- Arbitrary compose fields (`command`, `restart`, `user`, etc.) are captured via an inline catch-all map so author definitions are preserved. Top-level `volumes` and `networks` in `ComponentDef` are slices of named definitions (`name: ...`).

### Preset (presets/web.yaml)

A preset only lists components and their parameter values; it does not contain ready-made compose YAML.

```yaml
name: web
description: Full web stack
services:
  - component: postgres
    params: { POSTGRES_VERSION: "16", EXTRA_PORTS: ["5433:5432"] }
  - component: nginx
    params: { HTTP_PORT: "80" }
  - component: go-app
    params: { APP_PORT: "8080" }
```

## Source lookup

Identifiers equal file names: `postgres.yaml` holds `id: postgres`, `web.yaml` holds `name: web`; a mismatch is an invalid definition. Identifiers match `[a-z0-9][a-z0-9-]{0,63}`.

Priority order (first match wins: an earlier source fully replaces a later one with the same ID; definitions are never merged):

1. ./assembler-presets/ — cwd, local to the project
2. os.UserConfigDir()/assembler/ — cross-platform (Linux ~/.config, macOS ~/Library/Application Support, Windows %AppData%); do not branch manually on runtime.GOOS
3. Embedded (go:embed) — defaults shipped in the binary

Nothing is cached: every operation reads the sources on demand, so edits are picked up without a restart. A broken higher-priority file is an error and never falls back to a lower-priority definition. Listing is best-effort: it returns every valid entry plus a joined error describing what was skipped (results are usable when the error is non-nil).

Each directory is expected to contain components/ and presets/ subdirectories. A failed lookup → `PresetNotFoundError` / `ComponentNotFoundError` with the list of searched locations (paths or source names). Malformed YAML files → `InvalidPresetError` / `InvalidComponentError` wrapping the parser error.

## Errors

All custom error types live in `internal/errs/errors.go`. Go naming conventions are followed: `XxxError` for error structs, and `ErrXxx` for category sentinels. Each struct implements `Error() string`, uses value receivers, and supports inspection via `errors.Is` / `errors.As` without string comparison.

Category sentinels (allow consumers to map cleanly to HTTP status codes and CLI exit codes):
- `ErrNotFound` (404 / Exit code: component or preset not found)
- `ErrInvalidInput` (400 / Exit code: caller mistake — missing required params, type mismatches, unknown params, invalid requests)
- `ErrInvalidDefinition` (500 / Exit code: broken component/preset definition, unknown merger target, malformed YAML)
- `ErrConflict` (409 / Exit code: port conflicts, volume conflicts, environment variable conflicts, cyclic dependencies, duplicate services)

Typed error structs (implementing `Is(target error) bool` matching their category sentinel):
- `PresetNotFoundError{ID string, SearchedLocations []string}` (matches `ErrNotFound`)
- `ComponentNotFoundError{ID string, SearchedLocations []string}` (matches `ErrNotFound`)
- `InvalidPresetError{Location string, Err error}` (implements `Unwrap() error`, matches `ErrInvalidDefinition`)
- `InvalidComponentError{Location string, Err error}` (implements `Unwrap() error`, matches `ErrInvalidDefinition`)
- `UnknownTargetError{Target string}` (matches `ErrInvalidDefinition`)
- `MissingRequiredParamError{Component string, Param string}` (matches `ErrInvalidInput`)
- `UnknownParamError{Component string, Param string}` (matches `ErrInvalidInput`)
- `ParamTypeMismatchError{Component string, Param string, ExpectedType string, ActualType string}` (matches `ErrInvalidInput`)
- `InvalidRequestError{Reason string}` (matches `ErrInvalidInput`)
- `PortConflictError{Port string, ServiceA string, ServiceB string}` (Port is normalized string, e.g. "8080/tcp", matches `ErrConflict`)
- `VolumeConflictError{Volume string, ServiceA string, ServiceB string, Reason string}` (matches `ErrConflict`)
- `EnvConflictError{Key string, ServiceA string, ValueA string, ServiceB string, ValueB string}` (matches `ErrConflict`)
- `CyclicDependencyError{Chain []string}` (matches `ErrConflict`)
- `DuplicateServiceError{Name string}` (matches `ErrConflict`)

## Panics

Rule: a programmer error or an error in built-in content → panic. A user or environment error → error.

We panic on:
- RegisterTarget with an already-taken key;
- a duplicate id among built-in components/presets when loading the embedded FS;
- broken built-in YAML.

We never panic on user input (user YAML, parameters, a missing file) — only error.

We recover in:
- main — a top-level defer recover() for a readable message instead of a stack trace;
- web/middleware.go — recover() around every handler, respond with 500, the server keeps running.

## Goroutines and channels

- Resolver.ListComponents/ListPresets — one goroutine per source, results via a channel; aggregate without races and without leaking goroutines.
- validate.RunAll — each Validator in its own goroutine, errors collected from a channel; use sync.WaitGroup, close the channel exactly once.
- Optional: an SSE broadcaster in web (fsnotify on the preset directories).
- Verify with go test -race ./...

## Web API (initial set)

```
GET  /api/components               — list available components: {items, skipped}
GET  /api/components/{id}          — ComponentDef (params drive the UI forms)
GET  /api/presets                  — list presets: {items, skipped}
GET  /api/presets/{id}             — Preset (values pre-fill the forms)
POST /api/generate                 — body: GenerateRequest, response: {compose, env, warnings}
GET  /api/events                   — SSE (optional)
```

Listings are best-effort like the catalog: `skipped` describes files that failed to load. Every non-2xx response is `{kind, message, details[]}`: `kind` comes from the category sentinel (`invalid_input` 400, `not_found` 404, `conflict` 409, `invalid_definition` / `internal` 500), and `details` has one entry per underlying error (joined validator errors are split) with the typed fields the UI needs to point at a service or parameter (`component`, `param`, `services`, `value`, `chain`, `location`).

`assembler serve` listens on 127.0.0.1:8080 (loopback only, the API is unauthenticated); the `ASSEMBLER_ADDR` environment variable overrides the address.

The web frontend is a client-side SPA using TanStack Router (SPA mode, without SSR). The Go server serves the compiled static assets from `static/` alongside the REST API. Handlers are thin: parse the request → Engine.Generate → respond.

## Live updates (SSE, optional)

SSE is a one-way push channel (server → browser) over a long-lived HTTP response. The browser still uses fetch for all actions; SSE only tells it that something changed.

- Scope: web layer only. The core (source, assemble, validate, export) knows nothing about SSE or HTTP.
- Event source: a watcher goroutine (fsnotify) on the directory-based sources (cwd and user config dir). Embedded sources never change. When a component or preset file is created, changed, or removed, the watcher emits an event such as components_changed / presets_changed, and the browser refetches GET /api/components or GET /api/presets.
- Broadcaster: one goroutine owns the set of subscribers; each connected browser gets its own buffered channel, and register / unregister / broadcast go through channels (no shared map behind a mutex). Drop slow clients instead of blocking the broadcaster, and clean up when the request context is cancelled.

## Division of work

- Person A — core: domain, errs, source, assemble, validate, export.
- Person B — interfaces: cmd, cli, web (REST, recover middleware, SSE, frontend).

The contract between them is `domain.GenerateRequest`, `domain.GenerateResult`, and the `Engine.Generate(ctx, req)` signature. Person B works against interfaces and mocks without waiting for the core implementation. Changes to domain and to interface signatures require agreement from both.

## What we deliberately do NOT do

- No Storage layer. Presets are YAML files read from sources, not managed state.
- No custom template engine / Renderer. Compose handles ${VAR} substitution.
- No reflection for target. We extend by registering a new TargetMerger, not by naming a struct field with a string.
- No business logic in cli and web.
- No auto-prefixing of volume names. Collisions are caught by the volumes validator.
- No merging of same-ID definitions across sources. The first match wins.

## Code conventions

- go.mod pins the Go version and the module; formatting with gofmt, linting with go vet.
- Packages are small and named by purpose; no import cycles (domain and errs import nothing from internal).
- Interfaces are declared where they are used and kept small.
- Tests are table-driven; validate (including with -race), assemble (required params, mergers), and registration panics (recover in the test) must be covered.
- Code, comments, and user-facing messages are in English.

## CLI decisions

The binary has no argument parsing beyond one word:

- `assembler` (no arguments) — the full-screen interactive session (bubbletea + huh). Needs a terminal.
- `assembler serve` — starts the web server and the browser UI (see Web API).
- Anything else is a usage error (exit 2). No flags, no cobra.

The session is one alt-screen program: choose preset or components, edit parameters (one huh form group per component, shift+tab walks back, secrets are masked), generate, scrollable preview (`c` compose / `e` .env), write. On an input or conflict error it shows the message and returns to the parameters with every value kept. Output goes to the current directory; existing `compose.yaml` / `.env` are only replaced after the user confirms. `.env` is written with mode 0600. The TUI receives its services through `cli.Session` and contains no business logic.

Exit codes follow the category sentinels: 2 invalid input, 3 not found, 4 invalid definition, 5 conflict, 130 aborted, 1 anything else, 70 recovered panic.

## Open questions

1. SSE + fsnotify: do it or not (see "Live updates (SSE, optional)" above; extra dependency, a "wow" feature for the defense).
