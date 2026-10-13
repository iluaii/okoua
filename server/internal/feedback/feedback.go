// SPDX-License-Identifier: AGPL-3.0-or-later

// Package feedback accepts anonymous user feedback, rate-limits it and stores it in SQLite.
// No IP addresses or other identifiers are stored; a contact is kept only if the person leaves one.
package feedback

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"alert-concept/server/internal/ratelimit"
)

// Categories are the allowed category → subcategory IDs. Labels live in the web client.
var Categories = map[string][]string{
	"alerts": {"missing", "false", "time", "region", "delay", "other"},
	"power":  {"mismatch", "outdated", "no_region", "no_queue", "other"},
	"ai":     {"hallucination", "russian", "wrong_region", "outdated", "other"},
	"ui":     {"broken", "inconvenient", "slow", "other"},
	"other":  {"idea", "other"},
}

const (
	MaxMessage = 1000 // characters
	MaxContact = 100  // characters
	maxBody    = 4 << 10
	maxPage    = 200
	maxVersion = 40
)

type request struct {
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Message     string `json:"message"`
	Page        string `json:"page"`
	Version     string `json:"version"`
	Contact     string `json:"contact"`
	// Website is a honeypot: the field is hidden from people, so only bots fill it.
	Website string `json:"website"`
}

// NewLimiter returns the feedback rate limits: 5 per 10 minutes and 30 per day per client,
// 300 per hour in total.
func NewLimiter() *ratelimit.Limiter {
	return ratelimit.New(
		[]ratelimit.Rule{{N: 5, Per: 10 * time.Minute}, {N: 30, Per: 24 * time.Hour}},
		[]ratelimit.Rule{{N: 300, Per: time.Hour}},
	)
}

// Handler serves POST /api/feedback.
type Handler struct {
	Store      *Store
	Limiter    *ratelimit.Limiter
	TrustProxy bool // take the client IP from X-Forwarded-For (set by our reverse proxy)
	Log        *slog.Logger
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// JSON only: browsers can't send it cross-site without a CORS preflight, which we never allow.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "json only")
		return
	}
	// Limit before parsing, so floods of junk are throttled too.
	if !h.Limiter.Allow(ratelimit.ClientIP(r, h.TrustProxy)) {
		w.Header().Set("Retry-After", "600")
		writeError(w, http.StatusTooManyRequests, "too many requests")
		return
	}

	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "too large")
			return
		}
		writeError(w, http.StatusBadRequest, "bad json")
		return
	}
	if req.Website != "" {
		w.WriteHeader(http.StatusNoContent) // pretend success so bots don't adapt
		return
	}

	e, err := validate(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.Store.Add(r.Context(), e); err != nil {
		h.Log.Error("feedback store failed", "err", err)
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validate(req request) (Entry, error) {
	subs, ok := Categories[req.Category]
	if !ok {
		return Entry{}, errors.New("bad category")
	}
	found := false
	for _, s := range subs {
		found = found || s == req.Subcategory
	}
	if !found {
		return Entry{}, errors.New("bad subcategory")
	}
	msg := Clean(req.Message, true)
	if utf8.RuneCountInString(msg) > MaxMessage {
		return Entry{}, errors.New("message too long")
	}
	page := Clean(req.Page, false)
	if !strings.HasPrefix(page, "/") || len(page) > maxPage {
		page = ""
	}
	contact := Clean(req.Contact, false)
	if utf8.RuneCountInString(contact) > MaxContact {
		return Entry{}, errors.New("contact too long")
	}
	version := Clean(req.Version, false)
	if len(version) > maxVersion {
		version = ""
	}
	return Entry{Category: req.Category, Subcategory: req.Subcategory, Message: msg, Page: page, Version: version, Contact: contact}, nil
}

// Clean trims the text and drops control characters (which could, for example, inject
// terminal escape codes into whoever reads the feedback). Newlines are kept if multiline.
func Clean(s string, multiline bool) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if r == '\n' && multiline {
			return r
		}
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
