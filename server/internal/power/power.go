// SPDX-License-Identifier: AGPL-3.0-or-later

// Package power fetches power outage schedules for regions, normalizes them
// into one shape and serves the latest known schedule per region.
package power

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // Europe/Kyiv must work on hosts without system tzdata
)

// Outage is a span without power in minutes since local midnight (Kyiv), [From, To).
type Outage struct {
	From     int  `json:"from"`
	To       int  `json:"to"`
	Possible bool `json:"possible,omitempty"` // "possible outage" rather than a definite one
}

// Day statuses.
const (
	StatusSchedule = "schedule" // a schedule applies
	StatusNone     = "none"     // no outages planned
	StatusWaiting  = "waiting"  // not published yet
	StatusUnknown  = "unknown"  // the source says something we don't understand, or nothing for this day
)

type Day struct {
	Date   string              `json:"date"` // YYYY-MM-DD, Kyiv time
	Status string              `json:"status"`
	Queues map[string][]Outage `json:"queues"`
}

// Data is what a source returns.
type Data struct {
	Queues    []string
	Days      []Day
	UpdatedAt time.Time // when the source last published changes
}

// Source fetches the schedule of one region.
type Source interface {
	Fetch(ctx context.Context) (Data, error)
}

var kyiv = mustLoadLocation("Europe/Kyiv")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

// kyivDate formats t as a Kyiv calendar date.
func kyivDate(t time.Time) string { return t.In(kyiv).Format(time.DateOnly) }

// halfHours turns 48 half-hour states into merged outages.
type slot uint8

const (
	slotOn slot = iota
	slotOff
	slotPossible
)

func outagesFromSlots(s [48]slot) []Outage {
	out := []Outage{} // never nil: the client gets [] rather than null
	for i := 0; i < len(s); {
		if s[i] == slotOn {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == s[i] {
			j++
		}
		out = append(out, Outage{From: i * 30, To: j * 30, Possible: s[i] == slotPossible})
		i = j
	}
	return out
}

// mergeOutages sorts outages and merges overlapping or touching ones of the same kind.
func mergeOutages(in []Outage) []Outage {
	in = slices.Clone(in)
	slices.SortFunc(in, func(a, b Outage) int { return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To)) })
	out := []Outage{} // never nil: the client gets [] rather than null
	for _, o := range in {
		if n := len(out); n > 0 && out[n-1].Possible == o.Possible && o.From <= out[n-1].To {
			out[n-1].To = max(out[n-1].To, o.To)
			continue
		}
		out = append(out, o)
	}
	return out
}

func validate(d Data) error {
	if len(d.Queues) == 0 {
		return errors.New("no queues")
	}
	for _, day := range d.Days {
		if _, err := time.Parse(time.DateOnly, day.Date); err != nil {
			return fmt.Errorf("bad date %q", day.Date)
		}
		for q, outages := range day.Queues {
			prev := 0
			for _, o := range outages {
				if o.From < 0 || o.To > 1440 || o.From >= o.To {
					return fmt.Errorf("%s queue %s: bad outage %d-%d", day.Date, q, o.From, o.To)
				}
				if o.From < prev {
					return fmt.Errorf("%s queue %s: overlapping outages", day.Date, q)
				}
				prev = o.To
			}
		}
	}
	return nil
}

// sortQueues orders "1.1", "1.2", "2.1", …, "10.1" numerically.
func sortQueues(qs []string) {
	key := func(q string) (int, int) {
		a, b, _ := strings.Cut(q, ".")
		x, _ := strconv.Atoi(a)
		y, _ := strconv.Atoi(b)
		return x, y
	}
	slices.SortFunc(qs, func(p, q string) int {
		pa, pb := key(p)
		qa, qb := key(q)
		return cmp.Or(cmp.Compare(pa, qa), cmp.Compare(pb, qb), cmp.Compare(p, q))
	})
}

const userAgent = "alert-concept (open-source power outage map; github.com)"

func getJSON(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}
