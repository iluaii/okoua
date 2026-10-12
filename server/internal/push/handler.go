// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/ratelimit"
)

const (
	maxBody          = 4 << 10
	maxSubscriptions = 500_000 // refuse new ones beyond this, to bound storage under abuse
)

// NewLimiter returns the subscribe/unsubscribe rate limits. Following a region re-sends
// the whole subscription, so per-client limits are generous.
func NewLimiter() *ratelimit.Limiter {
	return ratelimit.New(
		[]ratelimit.Rule{{N: 30, Per: 10 * time.Minute}, {N: 200, Per: 24 * time.Hour}},
		[]ratelimit.Rule{{N: 5000, Per: time.Hour}},
	)
}

// Handler serves the push API.
type Handler struct {
	Store      *Store
	PublicKey  string
	Limiter    *ratelimit.Limiter
	TrustProxy bool
	Log        *slog.Logger
}

// Register adds the push routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/push/key", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"key": h.PublicKey})
	})
	mux.HandleFunc("POST /api/push/subscribe", h.subscribe)
	mux.HandleFunc("POST /api/push/unsubscribe", h.unsubscribe)
}

type subscribeRequest struct {
	Subscription struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	} `json:"subscription"`
	Regions []int `json:"regions"`
}

func (h *Handler) subscribe(w http.ResponseWriter, r *http.Request) {
	var req subscribeRequest
	if !h.decode(w, r, &req) {
		return
	}
	sub := Subscription{
		Endpoint: req.Subscription.Endpoint,
		P256dh:   req.Subscription.Keys.P256dh,
		Auth:     req.Subscription.Keys.Auth,
	}
	if err := checkEndpoint(sub.Endpoint); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !validKey(sub.P256dh, 80, 100) || !validKey(sub.Auth, 16, 30) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad keys"})
		return
	}
	for _, uid := range req.Regions {
		if _, ok := alerts.Regions[uid]; !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad region"})
			return
		}
		if !slices.Contains(sub.Regions, uid) {
			sub.Regions = append(sub.Regions, uid)
		}
	}
	if len(sub.Regions) == 0 {
		// Following nothing is the same as unsubscribing.
		h.store(w, r, h.Store.Delete(r.Context(), sub.Endpoint))
		return
	}
	if n, err := h.Store.Count(r.Context()); err == nil && n >= maxSubscriptions {
		h.Log.Warn("push subscriptions limit reached", "count", n)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "full"})
		return
	}
	h.store(w, r, h.Store.Save(r.Context(), sub))
}

func (h *Handler) unsubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if !h.decode(w, r, &req) {
		return
	}
	h.store(w, r, h.Store.Delete(r.Context(), req.Endpoint))
}

// decode applies the shared checks: JSON only, rate limit, body size.
func (h *Handler) decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "json only"})
		return false
	}
	if !h.Limiter.Allow(ratelimit.ClientIP(r, h.TrustProxy)) {
		w.Header().Set("Retry-After", "600")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return false
	}
	return true
}

func (h *Handler) store(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		h.Log.Error("push store failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validKey checks a base64url key from the browser by length and alphabet.
func validKey(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '=') {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
