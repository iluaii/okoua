// SPDX-License-Identifier: AGPL-3.0-or-later

package power

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"
)

// Link is a named link shown to users.
type Link struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Region describes where a region's schedule comes from.
type Region struct {
	UID       int    // alerts.in.ua region UID
	Source    Source // where we fetch from
	Operator  Link   // the grid operator that publishes the schedule
	Via       *Link  // a community dataset we read it through, if any
	LookupURL string // where people can find their queue
}

// Schedule is what the client receives for one region.
type Schedule struct {
	Region    int      `json:"region"`
	Queues    []string `json:"queues"`
	Today     Day      `json:"today"`
	Tomorrow  Day      `json:"tomorrow"`
	UpdatedAt string   `json:"updatedAt"` // when the source last published changes (RFC 3339, may be empty)
	CheckedAt string   `json:"checkedAt"` // our last successful fetch (RFC 3339)
	Stale     bool     `json:"stale"`
	Source    Link     `json:"source"`
	Via       *Link    `json:"via,omitempty"`
	LookupURL string   `json:"lookupUrl,omitempty"`
}

var (
	ErrUnsupported = errors.New("power: region not supported")
	ErrNoData      = errors.New("power: no data yet")
)

type entry struct {
	data      Data
	checkedAt time.Time
}

type Service struct {
	regions  map[int]Region
	interval time.Duration
	log      *slog.Logger
	now      func() time.Time

	mu   sync.Mutex
	data map[int]entry
}

func NewService(regions []Region, interval time.Duration, log *slog.Logger) *Service {
	s := &Service{
		regions:  map[int]Region{},
		interval: interval,
		log:      log,
		now:      time.Now,
		data:     map[int]entry{},
	}
	for _, r := range regions {
		s.regions[r.UID] = r
	}
	return s
}

// Supported returns the UIDs of regions with a schedule source.
func (s *Service) Supported() []int {
	return slices.Sorted(maps.Keys(s.regions))
}

// Run fetches all regions until ctx is cancelled. The first fetch happens immediately.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		s.fetchAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Service) fetchAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, r := range s.regions {
		wg.Go(func() {
			fctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			d, err := r.Source.Fetch(fctx)
			if err != nil {
				if ctx.Err() == nil {
					s.log.Warn("power fetch failed", "region", r.UID, "err", err)
				}
				return // keep the last good data
			}
			s.mu.Lock()
			s.data[r.UID] = entry{data: d, checkedAt: s.now()}
			s.mu.Unlock()
		})
	}
	wg.Wait()
}

// Get returns the schedule for today and tomorrow (Kyiv time) for a region.
func (s *Service) Get(uid int) (Schedule, error) {
	r, ok := s.regions[uid]
	if !ok {
		return Schedule{}, ErrUnsupported
	}
	s.mu.Lock()
	e, ok := s.data[uid]
	s.mu.Unlock()
	if !ok {
		return Schedule{}, ErrNoData
	}

	now := s.now()
	today, tomorrow := kyivDate(now), kyivDate(now.Add(24*time.Hour))
	sch := Schedule{
		Region:    uid,
		Queues:    e.data.Queues,
		Today:     pickDay(e.data.Days, today, StatusUnknown),
		Tomorrow:  pickDay(e.data.Days, tomorrow, StatusWaiting),
		CheckedAt: e.checkedAt.UTC().Format(time.RFC3339),
		Source:    r.Operator,
		Via:       r.Via,
		LookupURL: r.LookupURL,
	}
	if !e.data.UpdatedAt.IsZero() {
		sch.UpdatedAt = e.data.UpdatedAt.UTC().Format(time.RFC3339)
	}
	// Stale if we couldn't fetch for a while, or the source itself hasn't changed in over a day.
	sch.Stale = now.Sub(e.checkedAt) > 3*s.interval ||
		(!e.data.UpdatedAt.IsZero() && now.Sub(e.data.UpdatedAt) > 30*time.Hour)
	return sch, nil
}

func pickDay(days []Day, date, missing string) Day {
	for _, d := range days {
		if d.Date == date {
			return d
		}
	}
	return Day{Date: date, Status: missing, Queues: map[string][]Outage{}}
}
