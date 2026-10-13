// SPDX-License-Identifier: AGPL-3.0-or-later

package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const alertsInUAURL = "https://api.alerts.in.ua/v1/alerts/active.json"

// AlertsInUA reads active alerts from the alerts.in.ua API (https://devs.alerts.in.ua/).
// API limit: up to 12 requests per minute from one IP.
type AlertsInUA struct {
	Token  string
	URL    string
	Client *http.Client

	mu           sync.Mutex
	lastModified string
}

func NewAlertsInUA(token string) *AlertsInUA {
	return &AlertsInUA{
		Token:  token,
		URL:    alertsInUAURL,
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *AlertsInUA) Name() string { return "alertsinua" }

func (s *AlertsInUA) Fetch(ctx context.Context) ([]Alert, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("Accept", "application/json")
	s.mu.Lock()
	if s.lastModified != "" {
		req.Header.Set("If-Modified-Since", s.lastModified)
	}
	s.mu.Unlock()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotModified:
		return nil, ErrNotModified
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("alerts.in.ua: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	list, err := parseAlertsInUA(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.lastModified = resp.Header.Get("Last-Modified")
	s.mu.Unlock()
	return list, nil
}

type alertsInUAResponse struct {
	Alerts []struct {
		ID                flexString `json:"id"`
		LocationTitle     string     `json:"location_title"`
		LocationType      string     `json:"location_type"`
		StartedAt         time.Time  `json:"started_at"`
		FinishedAt        *time.Time `json:"finished_at"`
		AlertType         string     `json:"alert_type"`
		LocationUID       flexString `json:"location_uid"`
		LocationOblastUID flexString `json:"location_oblast_uid"`
	} `json:"alerts"`
}

func parseAlertsInUA(r io.Reader) ([]Alert, error) {
	var resp alertsInUAResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("alerts.in.ua: decode: %w", err)
	}
	list := make([]Alert, 0, len(resp.Alerts))
	for _, a := range resp.Alerts {
		if a.FinishedAt != nil {
			continue
		}
		oblast, err := strconv.Atoi(string(a.LocationOblastUID))
		if err != nil {
			return nil, fmt.Errorf("alerts.in.ua: alert %s: bad location_oblast_uid %q", a.ID, a.LocationOblastUID)
		}
		var raion string
		if a.LocationType == "raion" {
			raion = string(a.LocationUID)
		}
		list = append(list, Alert{
			ID:            string(a.ID),
			Type:          a.AlertType,
			OblastUID:     oblast,
			LocationUID:   string(a.LocationUID),
			LocationTitle: a.LocationTitle,
			LocationType:  a.LocationType,
			RaionUID:      raion,
			StartedAt:     a.StartedAt.UTC(),
		})
	}
	return list, nil
}

// flexString accepts both a JSON string and a number: the API uses both forms for identifiers.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*f = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	*f = flexString(b)
	return nil
}
