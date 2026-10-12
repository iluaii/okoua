// SPDX-License-Identifier: AGPL-3.0-or-later

package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Siren reads active alerts from siren.pp.ua, a public, keyless wrapper around the official
// «Повітряна тривога» (Ukraine Alarm) API used by the Home Assistant integration. It is a
// stopgap until an alerts.in.ua token arrives: a community service with no guarantees.
// Region IDs follow the same scheme as alerts.in.ua, except Crimea (9999 there, 29 here).
type Siren struct {
	BaseURL string
	Client  *http.Client

	mu        sync.Mutex
	parent    map[string]int // district/community ID → oblast UID
	names     map[string]string
	loadedAt  time.Time
	refreshIn time.Duration
}

func NewSiren() *Siren {
	return &Siren{BaseURL: "https://siren.pp.ua/api/v3", Client: &http.Client{Timeout: 15 * time.Second}, refreshIn: 24 * time.Hour}
}

func (s *Siren) Name() string { return "sirenppua" }

type sirenRegion struct {
	RegionID   string        `json:"regionId"`
	RegionName string        `json:"regionName"`
	RegionType string        `json:"regionType"`
	Children   []sirenRegion `json:"regionChildIds"`
}

type sirenAlertRegion struct {
	RegionID     string `json:"regionId"`
	RegionName   string `json:"regionName"`
	RegionType   string `json:"regionType"`
	ActiveAlerts []struct {
		RegionID    string    `json:"regionId"`
		RegionType  string    `json:"regionType"`
		Type        string    `json:"type"`
		LastUpdate  time.Time `json:"lastUpdate"`
		AlertLevels []struct {
			CreatedAt time.Time `json:"createdAt"`
		} `json:"activeAlertLevels"`
	} `json:"activeAlerts"`
}

func (s *Siren) Fetch(ctx context.Context) ([]Alert, error) {
	if err := s.loadRegions(ctx); err != nil {
		return nil, err
	}
	var raw []sirenAlertRegion
	if err := s.get(ctx, "/alerts", &raw); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return convertSiren(raw, s.parent), nil
}

// loadRegions builds the district/community → oblast map; refreshed once a day.
func (s *Siren) loadRegions(ctx context.Context) error {
	s.mu.Lock()
	fresh := s.parent != nil && time.Since(s.loadedAt) < s.refreshIn
	s.mu.Unlock()
	if fresh {
		return nil
	}
	var tree struct {
		States []sirenRegion `json:"states"`
	}
	if err := s.get(ctx, "/regions", &tree); err != nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.parent != nil {
			return nil // keep the old map if a refresh fails
		}
		return err
	}
	parent := sirenParents(tree.States)
	s.mu.Lock()
	s.parent, s.loadedAt = parent, time.Now()
	s.mu.Unlock()
	return nil
}

func sirenParents(states []sirenRegion) map[string]int {
	parent := map[string]int{}
	var walk func(r sirenRegion, oblast int)
	walk = func(r sirenRegion, oblast int) {
		parent[r.RegionID] = oblast
		for _, c := range r.Children {
			walk(c, oblast)
		}
	}
	for _, st := range states {
		uid, ok := sirenOblast(st.RegionID)
		if !ok {
			continue // special entries (big-city communities, a test region); they also appear as children
		}
		walk(st, uid)
	}
	return parent
}

// sirenOblast maps a top-level Ukraine Alarm ID to our region UID.
func sirenOblast(id string) (int, bool) {
	if id == "9999" {
		return 29, true // Crimea
	}
	n, err := strconv.Atoi(id)
	if err != nil {
		return 0, false
	}
	_, ok := Regions[n]
	return n, ok
}

var sirenTypes = map[string]string{
	"AIR":          "air_raid",
	"ARTILLERY":    "artillery_shelling",
	"URBAN_FIGHTS": "urban_fights",
	"CHEMICAL":     "chemical",
	"NUCLEAR":      "nuclear",
}

var sirenLocationTypes = map[string]string{
	"State":     "oblast",
	"District":  "raion",
	"Community": "hromada",
}

func convertSiren(raw []sirenAlertRegion, parent map[string]int) []Alert {
	var list []Alert
	for _, r := range raw {
		for _, a := range r.ActiveAlerts {
			oblast, ok := parent[a.RegionID]
			if !ok {
				oblast, ok = sirenOblast(a.RegionID)
			}
			if !ok {
				continue // unknown or test region
			}
			typ, ok := sirenTypes[a.Type]
			if !ok {
				typ = "air_raid"
			}
			started := a.LastUpdate
			if len(a.AlertLevels) > 0 && !a.AlertLevels[0].CreatedAt.IsZero() {
				started = a.AlertLevels[0].CreatedAt
			}
			locType := sirenLocationTypes[a.RegionType]
			if locType == "" {
				locType = "unknown"
			}
			locUID := a.RegionID
			if a.RegionType == "State" {
				locUID = strconv.Itoa(oblast) // so Crimea's 9999 becomes 29 and CoversWholeRegion works
			}
			list = append(list, Alert{
				ID:            a.RegionID + ":" + a.Type,
				Type:          typ,
				OblastUID:     oblast,
				LocationUID:   locUID,
				LocationTitle: r.RegionName,
				LocationType:  locType,
				StartedAt:     started.UTC().Truncate(time.Second),
			})
		}
	}
	return list
}

func (s *Siren) get(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "alert-concept (open-source alert map)")
	resp, err := s.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("siren.pp.ua%s: %s", path, resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(v)
}
