// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api serves the alert state over HTTP: a snapshot (JSON) and a stream of changes (SSE).
package api

import (
	"net/http"
	"time"

	"alert-concept/server/internal/hub"
)

const heartbeatEvery = 20 * time.Second

func New(h *hub.Hub, staticDir string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/alerts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(h.Payload())
	})

	mux.HandleFunc("GET /api/stream", func(w http.ResponseWriter, r *http.Request) {
		stream(w, r, h)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})

	if staticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(staticDir)))
	}
	return mux
}

// stream holds an SSE connection: it sends the full snapshot right away, then again after every change.
// Full snapshots rather than deltas, so a client that reconnects has nothing to resync.
func stream(w http.ResponseWriter, r *http.Request, h *hub.Hub) {
	rc := http.NewResponseController(w)
	hd := w.Header()
	hd.Set("Content-Type", "text/event-stream")
	hd.Set("Cache-Control", "no-cache")
	hd.Set("X-Accel-Buffering", "no")

	updates, unsubscribe := h.Subscribe()
	defer unsubscribe()

	send := func(chunks ...[]byte) bool {
		for _, c := range chunks {
			if _, err := w.Write(c); err != nil {
				return false
			}
		}
		return rc.Flush() == nil
	}
	event := func(payload []byte) bool {
		return send([]byte("data: "), payload, []byte("\n\n"))
	}

	if !send([]byte("retry: 5000\n\n")) || !event(h.Payload()) {
		return
	}

	heartbeat := time.NewTicker(heartbeatEvery)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case p := <-updates:
			if !event(p) {
				return
			}
		case <-heartbeat.C:
			if !send([]byte(": ping\n\n")) {
				return
			}
		}
	}
}
