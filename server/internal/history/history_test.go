// SPDX-License-Identifier: AGPL-3.0-or-later

package history

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/hub"
	"alert-concept/server/internal/storage"
)

func TestHistory(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r, err := NewRecorder(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	clock := t0
	r.now = func() time.Time { return clock }

	alert := func(id string, region int, started time.Time) alerts.Alert {
		return alerts.Alert{ID: id, OblastUID: region, StartedAt: started}
	}
	state := func(as ...alerts.Alert) []hub.RegionState {
		by := map[int]*hub.RegionState{}
		var out []hub.RegionState
		for _, a := range as {
			if by[a.OblastUID] == nil {
				out = append(out, hub.RegionState{UID: a.OblastUID})
				by[a.OblastUID] = &out[len(out)-1]
			}
			by[a.OblastUID].Alerts = append(by[a.OblastUID].Alerts, a)
		}
		return out
	}

	// Region 25: two overlapping district alerts 10:00–10:40 and 10:20–11:00 (one oblast alert),
	// then another 12:00–12:30. Region 22: one alert still going.
	a1, a2 := alert("a1", 25, t0), alert("a2", 25, t0.Add(20*time.Minute))
	r.Sync(state(a1))
	clock = t0.Add(20 * time.Minute)
	r.Sync(state(a1, a2))
	clock = t0.Add(40 * time.Minute)
	r.Sync(state(a2))
	clock = t0.Add(60 * time.Minute)
	r.Sync(state())
	clock = t0.Add(120 * time.Minute)
	a3, b1 := alert("a3", 25, clock), alert("b1", 22, clock)
	r.Sync(state(a3, b1))
	clock = t0.Add(150 * time.Minute)
	r.Sync(state(b1))

	stats, err := r.Stats(context.Background(), t0.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if s := stats[25]; s != (Stat{Count: 2, TotalMinutes: 90, LongestMinutes: 60}) {
		t.Errorf("region 25 = %+v, want 2 alerts, 90 min, longest 60", s)
	}
	if s := stats[22]; s != (Stat{Count: 1, TotalMinutes: 30, LongestMinutes: 30}) {
		t.Errorf("region 22 = %+v, want the ongoing alert counted up to now", s)
	}

	// A window starting mid-alert counts only the part inside it.
	stats, _ = r.Stats(context.Background(), t0.Add(30*time.Minute))
	if s := stats[25]; s.TotalMinutes != 60 || s.Count != 2 {
		t.Errorf("clipped window: %+v, want 30+30 min in 2 alerts", s)
	}
}
