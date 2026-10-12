// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api serves the alert state over HTTP: a snapshot (JSON) and a stream of changes (SSE).
package api

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alert-concept/server/internal/hub"
	"alert-concept/server/internal/power"
)

const heartbeatEvery = 20 * time.Second

// Options are the parts the HTTP API is built from. Feedback and Push may be nil.
type Options struct {
	Hub       *hub.Hub
	Power     *power.Service
	Feedback  http.Handler
	Push      interface{ Register(*http.ServeMux) }
	News      interface{ Register(*http.ServeMux) }
	History   interface{ Register(*http.ServeMux) }
	AI        interface{ Register(*http.ServeMux) } // nil: AI is disabled (no API key)
	StaticDir string
}

func New(o Options) http.Handler {
	h, p, staticDir := o.Hub, o.Power, o.StaticDir
	mux := http.NewServeMux()

	if o.Feedback != nil {
		mux.Handle("POST /api/feedback", o.Feedback)
	}
	if o.Push != nil {
		o.Push.Register(mux)
	}
	if o.News != nil {
		o.News.Register(mux)
	}
	if o.History != nil {
		o.History.Register(mux)
	}
	if o.AI != nil {
		o.AI.Register(mux)
	} else {
		disabled := func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ai disabled"})
		}
		mux.HandleFunc("POST /api/ai/news", disabled)
		mux.HandleFunc("POST /api/ai/report", disabled)
	}

	mux.HandleFunc("GET /api/alerts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.Write(h.Payload())
	})

	mux.HandleFunc("GET /api/stream", func(w http.ResponseWriter, r *http.Request) {
		stream(w, r, h)
	})

	mux.HandleFunc("GET /api/power", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]int{"regions": p.Supported()})
	})

	mux.HandleFunc("GET /api/power/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid, err := strconv.Atoi(r.PathValue("uid"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad region"})
			return
		}
		sch, err := p.Get(uid)
		switch {
		case errors.Is(err, power.ErrUnsupported):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unsupported"})
		case errors.Is(err, power.ErrNoData):
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no data yet"})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		default:
			writeJSON(w, http.StatusOK, sch)
		}
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})

	if staticDir != "" {
		mux.Handle("GET /", static(staticDir))
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func init() {
	// Not in Go's built-in table; browsers expect this type for the PWA manifest.
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// static serves the site build with cache headers suited to it.
func static(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/_app/immutable/"):
			// File names contain a content hash, so they never change.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasPrefix(r.URL.Path, "/icons/"):
			w.Header().Set("Cache-Control", "public, max-age=86400")
		default:
			// Pages, the service worker and the manifest: always revalidate so updates arrive.
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
