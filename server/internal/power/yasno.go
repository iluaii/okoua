// SPDX-License-Identifier: AGPL-3.0-or-later

package power

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Yasno reads planned outages from the YASNO (DTEK) public API.
// The API is unofficial and undocumented, so parsing is strict and errors surface as stale data.
type Yasno struct {
	URL    string
	Client *http.Client
}

func NewYasno(regionID, dsoID int) *Yasno {
	return &Yasno{
		URL: fmt.Sprintf("https://app.yasno.ua/api/blackout-service/public/shutdowns/regions/%d/dsos/%d/planned-outages",
			regionID, dsoID),
		Client: &http.Client{Timeout: 15 * time.Second},
	}
}

type yasnoDay struct {
	Slots []struct {
		Start int    `json:"start"`
		End   int    `json:"end"`
		Type  string `json:"type"`
	} `json:"slots"`
	Date   string `json:"date"`
	Status string `json:"status"`
}

type yasnoQueue struct {
	Today     *yasnoDay `json:"today"`
	Tomorrow  *yasnoDay `json:"tomorrow"`
	UpdatedOn time.Time `json:"updatedOn"`
}

func (y *Yasno) Fetch(ctx context.Context) (Data, error) {
	body, err := getJSON(ctx, y.Client, y.URL)
	if err != nil {
		return Data{}, err
	}
	return parseYasno(body)
}

func parseYasno(body []byte) (Data, error) {
	var raw map[string]yasnoQueue
	if err := json.Unmarshal(body, &raw); err != nil {
		return Data{}, fmt.Errorf("yasno: decode: %w", err)
	}
	var d Data
	days := map[string]*Day{} // by date
	var order []string
	for q, v := range raw {
		d.Queues = append(d.Queues, q)
		if v.UpdatedOn.After(d.UpdatedAt) {
			d.UpdatedAt = v.UpdatedOn
		}
		for _, yd := range []*yasnoDay{v.Today, v.Tomorrow} {
			if yd == nil {
				continue
			}
			if len(yd.Date) < 10 {
				return Data{}, fmt.Errorf("yasno: queue %s: bad date %q", q, yd.Date)
			}
			date := yd.Date[:10]
			day := days[date]
			if day == nil {
				day = &Day{Date: date, Status: yasnoStatus(yd.Status), Queues: map[string][]Outage{}}
				days[date] = day
				order = append(order, date)
			}
			var outages []Outage
			for _, s := range yd.Slots {
				switch s.Type {
				case "NotPlanned":
					continue
				case "Definite":
					outages = append(outages, Outage{From: s.Start, To: s.End})
				default:
					// Unknown slot kinds are shown as possible outages rather than hidden.
					outages = append(outages, Outage{From: s.Start, To: s.End, Possible: true})
				}
			}
			day.Queues[q] = mergeOutages(outages)
		}
	}
	sortQueues(d.Queues)
	for _, date := range order {
		d.Days = append(d.Days, *days[date])
	}
	return d, validate(d)
}

func yasnoStatus(s string) string {
	switch s {
	case "ScheduleApplies":
		return StatusSchedule
	case "NoOutages":
		return StatusNone
	case "WaitingForSchedule":
		return StatusWaiting
	default:
		return StatusUnknown
	}
}
