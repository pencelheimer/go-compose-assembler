package web

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func startBroadcaster(t *testing.T) (*broadcaster, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	b := newBroadcaster()
	go b.run(ctx)
	t.Cleanup(func() {
		cancel()
		<-b.done
	})
	return b, cancel
}

func recv(t *testing.T, c <-chan string) (string, bool) {
	t.Helper()
	select {
	case ev, ok := <-c:
		return ev, ok
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for an event")
		return "", false
	}
}

func TestBroadcaster(t *testing.T) {
	t.Run("fans out to every subscriber", func(t *testing.T) {
		b, _ := startBroadcaster(t)
		a, _ := b.subscribe()
		c, _ := b.subscribe()
		b.publish <- eventComponents
		for _, sub := range []chan string{a, c} {
			if ev, ok := recv(t, sub); !ok || ev != eventComponents {
				t.Errorf("got %q, %v", ev, ok)
			}
		}
	})

	t.Run("unsubscribe closes the channel and is idempotent", func(t *testing.T) {
		b, _ := startBroadcaster(t)
		a, _ := b.subscribe()
		b.unsubscribe(a)
		if _, ok := recv(t, a); ok {
			t.Error("channel still open")
		}
		b.unsubscribe(a) // already gone: must not panic or block
	})

	t.Run("slow subscriber is dropped, others keep receiving", func(t *testing.T) {
		b, _ := startBroadcaster(t)
		slow, _ := b.subscribe()
		fast, _ := b.subscribe()
		for range subscriberBuffer + 1 {
			b.publish <- eventPresets
			if _, ok := recv(t, fast); !ok {
				t.Fatal("fast subscriber dropped")
			}
		}
		n := 0
		for range slow {
			n++
		}
		if n != subscriberBuffer {
			t.Errorf("slow subscriber got %d buffered events before being dropped, want %d", n, subscriberBuffer)
		}
	})

	t.Run("stopping closes subscribers and refuses new ones", func(t *testing.T) {
		b, cancel := startBroadcaster(t)
		a, _ := b.subscribe()
		cancel()
		if _, ok := recv(t, a); ok {
			t.Error("subscriber not closed on stop")
		}
		<-b.done
		if _, ok := b.subscribe(); ok {
			t.Error("subscribe succeeded after stop")
		}
	})
}

func TestWatchSources(t *testing.T) {
	oldRetry, oldDebounce := watchRetry, watchDebounce
	watchRetry, watchDebounce = 50*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { watchRetry, watchDebounce = oldRetry, oldDebounce })

	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := make(chan string)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- watchSources(ctx, []string{base, filepath.Join(base, "missing")}, out, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("watchSources: %v", err)
		}
	})
	time.Sleep(100 * time.Millisecond) // let the initial watches settle

	write := func(rel, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(base, rel), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("components/mongo.yaml", "id: mongo\n")
	if ev, _ := recv(t, out); ev != eventComponents {
		t.Errorf("after creating a component: %q", ev)
	}

	write("components/notes.txt", "ignored")
	select {
	case ev := <-out:
		t.Errorf("non-YAML file produced %q", ev)
	case <-time.After(200 * time.Millisecond):
	}

	// presets/ does not exist yet: its appearance is picked up by the retry.
	if err := os.Mkdir(filepath.Join(base, "presets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if ev, _ := recv(t, out); ev != eventPresets {
		t.Errorf("after creating presets/: %q", ev)
	}
	write("presets/api.yaml", "name: api\n")
	if ev, _ := recv(t, out); ev != eventPresets {
		t.Errorf("after creating a preset: %q", ev)
	}

	if err := os.Remove(filepath.Join(base, "components", "mongo.yaml")); err != nil {
		t.Fatal(err)
	}
	if ev, _ := recv(t, out); ev != eventComponents {
		t.Errorf("after removing a component: %q", ev)
	}
}

func TestEventsEndpoint(t *testing.T) {
	b, _ := startBroadcaster(t)
	d := Deps{Components: fakeCat{}, Presets: fakeCat{}, Generator: &fakeGen{}, Exporter: fakeExp{}}.withDefaults()
	srv := httptest.NewServer(newHandler(d, b))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type = %q", ct)
	}

	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	waitFor := func(want string) {
		t.Helper()
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatalf("stream ended before %q", want)
				}
				if strings.TrimSpace(l) == want {
					return
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for %q", want)
			}
		}
	}

	waitFor(": connected") // flushed before any event, so the subscription is live
	b.publish <- eventComponents
	waitFor("event: " + eventComponents)
}

func TestEventsRouteNeedsServe(t *testing.T) {
	rec := do(t, newTestHandler(fakeCat{}, &fakeGen{}), http.MethodGet, "/api/events", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 without a broadcaster", rec.Code)
	}
}
