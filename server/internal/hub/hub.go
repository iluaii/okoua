// SPDX-License-Identifier: AGPL-3.0-or-later

// Package hub polls the alert source, keeps the current state
// and pushes it to subscribers (SSE) on every change.
package hub

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"alert-concept/server/internal/alerts"
)

// State is what the client receives.
type State struct {
	Source    string        `json:"source"`    // alertsinua or mock
	Stale     bool          `json:"stale"`     // source has not answered for a while, data may be outdated
	UpdatedAt time.Time     `json:"updatedAt"` // time of the last successful poll
	Regions   []RegionState `json:"regions"`   // only regions with an alert, by ascending UID
}

type RegionState struct {
	UID    int            `json:"uid"`
	Status string         `json:"status"` // full: whole oblast, partial: part of it
	Alerts []alerts.Alert `json:"alerts"`
}

type Hub struct {
	src        alerts.Source
	interval   time.Duration
	staleAfter time.Duration
	log        *slog.Logger
	now        func() time.Time

	mu         sync.Mutex
	state      State
	regionsRaw []byte // encoded Regions, for cheap comparison
	lastOK     time.Time
	payload    []byte
	subs       map[chan []byte]struct{}
}

func New(src alerts.Source, interval time.Duration, log *slog.Logger) *Hub {
	h := &Hub{
		src:        src,
		interval:   interval,
		staleAfter: max(4*interval, time.Minute),
		log:        log,
		now:        time.Now,
		state:      State{Source: src.Name(), Stale: true, Regions: []RegionState{}},
		subs:       make(map[chan []byte]struct{}),
	}
	h.payload = h.encode()
	return h
}

// Run polls the source until ctx is cancelled. The first poll happens immediately.
func (h *Hub) Run(ctx context.Context) {
	t := time.NewTicker(h.interval)
	defer t.Stop()
	for {
		h.poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (h *Hub) poll(ctx context.Context) {
	fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	list, err := h.src.Fetch(fctx)
	cancel()
	if ctx.Err() != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now().UTC()
	changed := false
	switch {
	case errors.Is(err, alerts.ErrNotModified):
		h.lastOK = now
	case err != nil:
		h.log.Warn("alerts fetch failed", "source", h.src.Name(), "err", err)
	default:
		h.lastOK = now
		regions := groupByRegion(list)
		raw, _ := json.Marshal(regions)
		if !bytes.Equal(raw, h.regionsRaw) {
			h.state.Regions = regions
			h.regionsRaw = raw
			changed = true
		}
	}

	stale := h.lastOK.IsZero() || now.Sub(h.lastOK) > h.staleAfter
	if stale != h.state.Stale {
		h.state.Stale = stale
		changed = true
		if stale {
			h.log.Warn("alerts data is stale", "lastOK", h.lastOK)
		}
	}
	h.state.UpdatedAt = h.lastOK
	h.payload = h.encode()
	if changed {
		h.broadcast()
	}
}

func groupByRegion(list []alerts.Alert) []RegionState {
	byUID := map[int]*RegionState{}
	for _, a := range list {
		r := byUID[a.OblastUID]
		if r == nil {
			r = &RegionState{UID: a.OblastUID, Status: "partial"}
			byUID[a.OblastUID] = r
		}
		if a.CoversWholeRegion() {
			r.Status = "full"
		}
		r.Alerts = append(r.Alerts, a)
	}
	regions := make([]RegionState, 0, len(byUID))
	for _, r := range byUID {
		slices.SortFunc(r.Alerts, func(a, b alerts.Alert) int {
			return cmp.Or(a.StartedAt.Compare(b.StartedAt), cmp.Compare(a.ID, b.ID))
		})
		regions = append(regions, *r)
	}
	slices.SortFunc(regions, func(a, b RegionState) int { return cmp.Compare(a.UID, b.UID) })
	return regions
}

func (h *Hub) encode() []byte {
	b, err := json.Marshal(h.state)
	if err != nil {
		panic(err) // State always marshals
	}
	return b
}

// Payload returns the current state as JSON.
func (h *Hub) Payload() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.payload
}

// Subscribe returns a channel that receives the state after every change.
// A slow subscriber only gets the latest state; intermediate ones are dropped.
func (h *Hub) Subscribe() (<-chan []byte, func()) {
	ch := make(chan []byte, 1)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// broadcast must be called with h.mu held.
func (h *Hub) broadcast() {
	for ch := range h.subs {
		select {
		case <-ch:
		default:
		}
		ch <- h.payload
	}
}
