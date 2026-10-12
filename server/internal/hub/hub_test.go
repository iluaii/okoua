// SPDX-License-Identifier: AGPL-3.0-or-later

package hub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"alert-concept/server/internal/alerts"
)

type result struct {
	list []alerts.Alert
	err  error
}

// fakeSource returns scripted results one by one.
type fakeSource struct{ results []result }

func (f *fakeSource) Name() string { return "fake" }

func (f *fakeSource) Fetch(context.Context) ([]alerts.Alert, error) {
	r := f.results[0]
	f.results = f.results[1:]
	return r.list, r.err
}

var t0 = time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)

func whole(uid int) alerts.Alert {
	return alerts.Alert{ID: "w", Type: "air_raid", OblastUID: uid, LocationUID: itoa(uid), LocationType: "oblast", StartedAt: t0}
}

func raion(uid int, id string) alerts.Alert {
	return alerts.Alert{ID: id, Type: "air_raid", OblastUID: uid, LocationUID: "r" + id, LocationType: "raion", StartedAt: t0}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func newTestHub(src alerts.Source, clock *time.Time) *Hub {
	h := New(src, 15*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	h.now = func() time.Time { return *clock }
	return h
}

func decode(t *testing.T, b []byte) State {
	t.Helper()
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHubGroupsAndBroadcastsOnChange(t *testing.T) {
	errDown := errors.New("down")
	src := &fakeSource{results: []result{
		{list: []alerts.Alert{whole(16), raion(22, "a"), raion(22, "b"), whole(9), raion(9, "c")}},
		{list: []alerts.Alert{whole(16), raion(22, "a"), raion(22, "b"), whole(9), raion(9, "c")}}, // unchanged
		{err: alerts.ErrNotModified},
		{err: errDown},
		{err: errDown},
	}}
	clock := t0
	h := newTestHub(src, &clock)
	updates, unsubscribe := h.Subscribe()
	defer unsubscribe()

	if s := decode(t, h.Payload()); !s.Stale || len(s.Regions) != 0 {
		t.Fatalf("before first poll: %+v, want stale and empty", s)
	}

	h.poll(context.Background())
	s := decode(t, <-updates)
	if s.Stale || s.Source != "fake" {
		t.Fatalf("after poll: %+v", s)
	}
	got := map[int]string{}
	for _, r := range s.Regions {
		got[r.UID] = r.Status
	}
	want := map[int]string{9: "full", 16: "full", 22: "partial"}
	if len(got) != len(want) || got[9] != "full" || got[16] != "full" || got[22] != "partial" {
		t.Fatalf("regions = %v, want %v", got, want)
	}

	// Same list and 304: no broadcast.
	for range 2 {
		clock = clock.Add(15 * time.Second)
		h.poll(context.Background())
		select {
		case <-updates:
			t.Fatal("unexpected broadcast without changes")
		default:
		}
	}

	// Source down for less than a minute: data is still fresh.
	clock = clock.Add(30 * time.Second)
	h.poll(context.Background())
	select {
	case <-updates:
		t.Fatal("unexpected broadcast")
	default:
	}

	// Over a minute without a successful poll: stale, broadcast.
	clock = clock.Add(31 * time.Second)
	h.poll(context.Background())
	if s := decode(t, <-updates); !s.Stale || len(s.Regions) != 3 {
		t.Fatalf("after outage: %+v, want stale with last known regions", s)
	}
}

func TestSlowSubscriberGetsLatest(t *testing.T) {
	src := &fakeSource{results: []result{
		{list: []alerts.Alert{whole(16)}},
		{list: []alerts.Alert{whole(16), whole(22)}},
	}}
	clock := t0
	h := newTestHub(src, &clock)
	updates, unsubscribe := h.Subscribe()
	defer unsubscribe()

	h.poll(context.Background())
	h.poll(context.Background())
	if s := decode(t, <-updates); len(s.Regions) != 2 {
		t.Fatalf("got %d regions, want latest state with 2", len(s.Regions))
	}
	select {
	case <-updates:
		t.Fatal("stale intermediate state should be dropped")
	default:
	}
}
