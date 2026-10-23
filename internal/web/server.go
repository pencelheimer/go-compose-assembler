// Package web serves the REST API and the browser SPA. Handlers are thin:
// parse the request, call the core, respond. The core knows nothing about HTTP.
package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pencelheimer/go-compose-assembler/internal/domain"
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

// Deps are the collaborators of the web layer.
type Deps struct {
	Components ComponentCatalog
	Presets    PresetCatalog
	Generator  Generator
	Exporter   Exporter

	// Logger receives request logs and recovered panics; nil discards them.
	Logger *slog.Logger
	// Assets holds the built SPA; nil means the assets embedded in the binary.
	Assets fs.FS
	// WatchDirs are the directory sources Serve watches for live updates
	// (GET /api/events); empty disables watching.
	WatchDirs []string
}

func (d Deps) withDefaults() Deps {
	if d.Logger == nil {
		d.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if d.Assets == nil {
		d.Assets = embeddedAssets()
	}
	return d
}

//go:embed all:static
var embedded embed.FS

func embeddedAssets() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		panic(fmt.Sprintf("web: embedded static dir: %v", err)) // a fixed, valid path
	}
	return sub
}

// NewHandler builds the HTTP handler: the API under /api/ and the SPA for
// every other path, wrapped in logging and panic recovery. Live updates need
// background goroutines and are only available through Serve.
func NewHandler(d Deps) http.Handler {
	return newHandler(d.withDefaults(), nil)
}

func newHandler(d Deps, events *broadcaster) http.Handler {
	h := &handlers{d: d, hub: events}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/components", h.listComponents)
	mux.HandleFunc("GET /api/components/{id}", h.getComponent)
	mux.HandleFunc("GET /api/presets", h.listPresets)
	mux.HandleFunc("GET /api/presets/{id}", h.getPreset)
	mux.HandleFunc("POST /api/generate", h.generate)
	if events != nil {
		mux.HandleFunc("GET /api/events", h.events)
	}
	mux.HandleFunc("/api/", h.apiNotFound)
	mux.Handle("/", spaHandler(d.Assets))

	return logRequests(d.Logger, recoverPanics(d.Logger, mux))
}

// Serve listens on addr and serves until ctx is cancelled, then shuts down
// gracefully. It also runs the live-update broadcaster and the file watcher.
// The URL it listens on is written to out.
func Serve(ctx context.Context, addr string, d Deps, out io.Writer) error {
	d = d.withDefaults()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	// Cancelling ctx also cancels every request context, so long-lived event
	// streams end and Shutdown does not wait for them.
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := newBroadcaster()
	wg.Go(func() { events.run(ctx) })
	if len(d.WatchDirs) > 0 {
		wg.Go(func() {
			if err := watchSources(ctx, d.WatchDirs, events.publish, d.Logger); err != nil {
				d.Logger.Warn("live updates disabled", "err", err)
			}
		})
	}

	srv := &http.Server{
		Handler:           newHandler(d, events),
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	fmt.Fprintf(out, "serving on http://%s (ctrl+c to stop)\n", ln.Addr())

	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-done; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
