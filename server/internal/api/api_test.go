// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/hub"
	"alert-concept/server/internal/power"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

type staticSource struct{}

func (staticSource) Name() string { return "static" }

func (staticSource) Fetch(context.Context) ([]alerts.Alert, error) {
	return []alerts.Alert{{ID: "1", Type: "air_raid", OblastUID: 22, LocationUID: "22", LocationType: "oblast"}}, nil
}

func TestStreamSendsSnapshotOnConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := hub.New(staticSource{}, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	go h.Run(ctx)

	srv := httptest.NewServer(New(h, power.NewService(nil, time.Hour, discard), nil, ""))
	defer srv.Close()

	// Wait for the first poll so the snapshot is not empty.
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(string(h.Payload()), `"uid":22`) {
		if time.Now().After(deadline) {
			t.Fatal("hub did not poll")
		}
		time.Sleep(10 * time.Millisecond)
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}

	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		data, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		var s hub.State
		if err := json.Unmarshal([]byte(data), &s); err != nil {
			t.Fatal(err)
		}
		if len(s.Regions) != 1 || s.Regions[0].UID != 22 || s.Regions[0].Status != "full" {
			t.Fatalf("snapshot = %+v", s)
		}
		return
	}
	t.Fatalf("no data event: %v", sc.Err())
}

func TestAlertsSnapshot(t *testing.T) {
	h := hub.New(staticSource{}, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	New(h, power.NewService(nil, time.Hour, discard), nil, "").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/alerts", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("code %d, headers %v", rec.Code, rec.Header())
	}
	var s hub.State
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil || !s.Stale {
		t.Fatalf("state %+v, err %v (want stale before first poll)", s, err)
	}
}

func TestPowerRoutes(t *testing.T) {
	h := hub.New(staticSource{}, time.Hour, discard)
	handler := New(h, power.NewService([]power.Region{{UID: 25}}, time.Hour, discard), nil, "")

	cases := []struct {
		path string
		code int
		body string
	}{
		{"/api/power", http.StatusOK, `{"regions":[25]}`},
		{"/api/power/3", http.StatusNotFound, "unsupported"},
		{"/api/power/25", http.StatusServiceUnavailable, "no data"},
		{"/api/power/abc", http.StatusBadRequest, "bad region"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.body) {
			t.Errorf("%s: %d %q, want %d containing %q", c.path, rec.Code, rec.Body.String(), c.code, c.body)
		}
	}
}
