// SPDX-License-Identifier: AGPL-3.0-or-later

package feedback

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alert-concept/server/internal/ratelimit"
)

func TestClean(t *testing.T) {
	in := "  hi\x1b[31mred\x07\tthere\nline2‎  "
	if got := Clean(in, true); got != "hi[31mred there\nline2" {
		t.Errorf("clean = %q", got)
	}
	if got := Clean("a\nb", false); got != "ab" {
		t.Errorf("single line = %q", got)
	}
}

func newHandler(t *testing.T) (*Handler, *Store) {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &Handler{Store: s, Limiter: NewLimiter(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, s
}

func post(h http.Handler, body, contentType string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHandler(t *testing.T) {
	h, s := newHandler(t)
	h.Limiter.PerClient = []ratelimit.Rule{{N: 100, Per: time.Minute}} // many cases from one test client
	ctx := context.Background()

	cases := []struct {
		name, body, ct string
		code           int
	}{
		{"ok", `{"category":"ai","subcategory":"russian","message":"  відповідь російською  ","page":"/ai/","version":"1"}`, "application/json", 204},
		{"form post", `category=ai`, "application/x-www-form-urlencoded", 415},
		{"bad category", `{"category":"x","subcategory":"other"}`, "application/json", 400},
		{"bad subcategory", `{"category":"ai","subcategory":"missing"}`, "application/json", 400},
		{"too long", `{"category":"ai","subcategory":"other","message":"` + strings.Repeat("я", MaxMessage+1) + `"}`, "application/json", 400},
		{"body too large", `{"category":"ai","subcategory":"other","message":"` + strings.Repeat("x", maxBody) + `"}`, "application/json", 413},
		{"honeypot", `{"category":"ai","subcategory":"other","website":"http://spam"}`, "application/json", 204},
	}
	for _, c := range cases {
		if w := post(h, c.body, c.ct); w.Code != c.code {
			t.Errorf("%s: code %d (%s), want %d", c.name, w.Code, w.Body.String(), c.code)
		}
	}

	list, err := s.List(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("stored %d entries, want only the valid one (honeypot dropped): %+v", len(list), list)
	}
	if e := list[0]; e.Category != "ai" || e.Subcategory != "russian" || e.Message != "відповідь російською" || e.Page != "/ai/" {
		t.Errorf("entry = %+v", e)
	}
}

func TestHandlerMessageLimitCountsCharacters(t *testing.T) {
	h, _ := newHandler(t)
	// 1000 Cyrillic letters are 2000 bytes but within the 1000-character limit.
	body := `{"category":"other","subcategory":"idea","message":"` + strings.Repeat("я", MaxMessage) + `"}`
	if w := post(h, body, "application/json"); w.Code != 204 {
		t.Errorf("code %d (%s), want 204", w.Code, w.Body.String())
	}
}

func TestHandlerRateLimited(t *testing.T) {
	h, _ := newHandler(t)
	body := `{"category":"other","subcategory":"idea"}`
	for range 5 {
		post(h, body, "application/json")
	}
	w := post(h, body, "application/json")
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Errorf("code %d, Retry-After %q", w.Code, w.Header().Get("Retry-After"))
	}
}

func TestPrune(t *testing.T) {
	_, s := newHandler(t)
	ctx := context.Background()
	if err := s.Add(ctx, Entry{Category: "other", Subcategory: "idea"}); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Prune(ctx, time.Hour); err != nil || n != 0 {
		t.Errorf("fresh entry pruned: %d, %v", n, err)
	}
	if n, err := s.Prune(ctx, -time.Hour); err != nil || n != 1 {
		t.Errorf("old entry not pruned: %d, %v", n, err)
	}
}
