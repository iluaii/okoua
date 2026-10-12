// SPDX-License-Identifier: AGPL-3.0-or-later

package power

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GPV reads the "GPV" JSON format used by community outage data repositories
// (Baskerville42/outage-data-ua, yaroslav2901/OE_OUTAGE_DATA):
// fact.data[<unix start of day>]["GPV1.1"]["1".."24"] = yes | no | first | second | mfirst | msecond | maybe.
type GPV struct {
	URL    string
	Client *http.Client
}

func NewGPV(url string) *GPV {
	return &GPV{URL: url, Client: &http.Client{Timeout: 15 * time.Second}}
}

func (g *GPV) Fetch(ctx context.Context) (Data, error) {
	body, err := getJSON(ctx, g.Client, g.URL)
	if err != nil {
		return Data{}, err
	}
	return parseGPV(body)
}

type gpvFile struct {
	LastUpdated time.Time `json:"lastUpdated"`
	Fact        struct {
		// An object keyed by day, or an empty array when there are no schedules.
		Data json.RawMessage `json:"data"`
	} `json:"fact"`
}

func parseGPV(body []byte) (Data, error) {
	var f gpvFile
	if err := json.Unmarshal(body, &f); err != nil {
		return Data{}, fmt.Errorf("gpv: decode: %w", err)
	}
	d := Data{UpdatedAt: f.LastUpdated}
	var byDay map[string]map[string]map[string]string
	if s := strings.TrimSpace(string(f.Fact.Data)); s != "[]" && s != "" && s != "null" {
		if err := json.Unmarshal(f.Fact.Data, &byDay); err != nil {
			return Data{}, fmt.Errorf("gpv: decode fact.data: %w", err)
		}
	}

	queues := map[string]bool{}
	for ts, groups := range byDay {
		sec, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return Data{}, fmt.Errorf("gpv: bad day key %q", ts)
		}
		day := Day{Date: kyivDate(time.Unix(sec, 0)), Status: StatusNone, Queues: map[string][]Outage{}}
		for name, hours := range groups {
			q, ok := strings.CutPrefix(name, "GPV")
			if !ok {
				continue
			}
			var slots [48]slot
			for h := 1; h <= 24; h++ {
				v, ok := hours[strconv.Itoa(h)]
				if !ok {
					return Data{}, fmt.Errorf("gpv: %s queue %s: hour %d missing", day.Date, q, h)
				}
				first, second := gpvHalves(v)
				slots[2*(h-1)], slots[2*(h-1)+1] = first, second
			}
			outages := outagesFromSlots(slots)
			if len(outages) > 0 {
				day.Status = StatusSchedule
			}
			day.Queues[q] = outages
			queues[q] = true
		}
		d.Days = append(d.Days, day)
	}
	for q := range queues {
		d.Queues = append(d.Queues, q)
	}
	sortQueues(d.Queues)
	if len(byDay) == 0 {
		// No days at all: we can't tell "no outages" from a broken scraper.
		return d, nil
	}
	return d, validate(d)
}

// gpvHalves maps an hour value to the states of its two half-hours.
// Unknown values become possible outages rather than being hidden.
func gpvHalves(v string) (slot, slot) {
	switch v {
	case "yes":
		return slotOn, slotOn
	case "no":
		return slotOff, slotOff
	case "first":
		return slotOff, slotOn
	case "second":
		return slotOn, slotOff
	case "mfirst":
		return slotPossible, slotOn
	case "msecond":
		return slotOn, slotPossible
	default: // "maybe" and anything new
		return slotPossible, slotPossible
	}
}
