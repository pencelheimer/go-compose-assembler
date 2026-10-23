package web

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	// subscriberBuffer is how many events a browser may lag behind before it
	// is dropped; EventSource reconnects on its own.
	subscriberBuffer = 8
	heartbeatEvery   = 25 * time.Second
)

// broadcaster fans events out to connected browsers. One goroutine (run)
// owns the subscriber set; register, unregister and publish all go through
// channels, so there is no shared map and no mutex.
type broadcaster struct {
	register   chan chan string
	unregister chan chan string
	publish    chan string
	done       chan struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{
		register:   make(chan chan string),
		unregister: make(chan chan string),
		publish:    make(chan string),
		done:       make(chan struct{}),
	}
}

// run serves the channels until ctx is cancelled, then closes every
// subscriber so their handlers return.
func (b *broadcaster) run(ctx context.Context) {
	defer close(b.done)
	subs := make(map[chan string]struct{})
	defer func() {
		for c := range subs {
			close(c)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case c := <-b.register:
			subs[c] = struct{}{}
		case c := <-b.unregister:
			if _, ok := subs[c]; ok {
				delete(subs, c)
				close(c)
			}
		case ev := <-b.publish:
			for c := range subs {
				select {
				case c <- ev:
				default: // too slow: drop it rather than block everyone else
					delete(subs, c)
					close(c)
				}
			}
		}
	}
}

// subscribe returns a channel of events; false once the broadcaster stopped.
func (b *broadcaster) subscribe() (chan string, bool) {
	c := make(chan string, subscriberBuffer)
	select {
	case b.register <- c:
		return c, true
	case <-b.done:
		return nil, false
	}
}

// unsubscribe is safe to call for a subscriber that was already dropped.
func (b *broadcaster) unsubscribe(c chan string) {
	select {
	case b.unregister <- c:
	case <-b.done:
	}
}

// events streams change notifications as Server-Sent Events. The browser
// refetches what changed; events carry no payload of their own.
func (h *handlers) events(w http.ResponseWriter, r *http.Request) {
	ch, ok := h.hub.subscribe()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Kind: kindInternal, Message: "server is shutting down", Details: []errorDetail{}})
		return
	}
	defer h.hub.unsubscribe(ch)

	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "retry: 2000\n: connected\n\n")
	if err := rc.Flush(); err != nil {
		return
	}

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return // dropped as slow, or shutting down
			}
			fmt.Fprintf(w, "event: %s\ndata: {}\n\n", ev)
		case <-heartbeat.C:
			fmt.Fprint(w, ": ping\n\n") // keeps proxies from closing an idle stream
		}
		if err := rc.Flush(); err != nil {
			return
		}
	}
}
