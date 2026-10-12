// SPDX-License-Identifier: AGPL-3.0-or-later

package alerts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleActive = `{
  "alerts": [
    {"id": 8757, "location_title": "Луганська область", "location_type": "oblast",
     "started_at": "2022-04-04T16:45:39.000Z", "finished_at": null, "updated_at": "2023-10-29T18:22:37.357Z",
     "alert_type": "air_raid", "location_uid": "16", "location_oblast": "Луганська область",
     "location_oblast_uid": 16, "location_raion": null, "notes": null},
    {"id": "9001", "location_title": "Харківський район", "location_type": "raion",
     "started_at": "2026-10-12T10:05:00.000Z", "finished_at": null,
     "alert_type": "artillery_shelling", "location_uid": "1293", "location_oblast": "Харківська область",
     "location_oblast_uid": "22"},
    {"id": 9002, "location_title": "м. Київ", "location_type": "city",
     "started_at": "2026-10-12T10:00:00.000Z", "finished_at": "2026-10-12T10:30:00.000Z",
     "alert_type": "air_raid", "location_uid": "31", "location_oblast_uid": 31}
  ],
  "meta": {"last_updated_at": "2026-10-12T10:31:00.000Z"}
}`

func TestParseAlertsInUA(t *testing.T) {
	list, err := parseAlertsInUA(strings.NewReader(sampleActive))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d alerts, want 2 (finished one skipped): %+v", len(list), list)
	}
	want := Alert{
		ID: "8757", Type: "air_raid", OblastUID: 16, LocationUID: "16",
		LocationTitle: "Луганська область", LocationType: "oblast",
		StartedAt: time.Date(2022, 4, 4, 16, 45, 39, 0, time.UTC),
	}
	if list[0] != want {
		t.Errorf("alert[0] = %+v, want %+v", list[0], want)
	}
	if list[1].ID != "9001" || list[1].OblastUID != 22 || list[1].CoversWholeRegion() {
		t.Errorf("alert[1] = %+v, want partial alert in 22 with id 9001", list[1])
	}
}

func TestCoversWholeRegion(t *testing.T) {
	cases := []struct {
		a    Alert
		want bool
	}{
		{Alert{LocationType: "oblast", LocationUID: "22", OblastUID: 22}, true},
		{Alert{LocationType: "city", LocationUID: "31", OblastUID: 31}, true},
		{Alert{LocationType: "raion", LocationUID: "1293", OblastUID: 22}, false},
		{Alert{LocationType: "hromada", LocationUID: "564", OblastUID: 14}, false},
	}
	for _, c := range cases {
		if got := c.a.CoversWholeRegion(); got != c.want {
			t.Errorf("%+v: got %v, want %v", c.a, got, c.want)
		}
	}
}

func TestAlertsInUAFetch(t *testing.T) {
	const lastMod = "Mon, 12 Oct 2026 10:31:00 GMT"
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q", got)
		}
		if r.Header.Get("If-Modified-Since") == lastMod {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", lastMod)
		w.Write([]byte(sampleActive))
	}))
	defer srv.Close()

	s := NewAlertsInUA("secret")
	s.URL = srv.URL
	list, err := s.Fetch(context.Background())
	if err != nil || len(list) != 2 {
		t.Fatalf("first fetch: %d alerts, err %v", len(list), err)
	}
	if _, err := s.Fetch(context.Background()); !errors.Is(err, ErrNotModified) {
		t.Fatalf("second fetch: err = %v, want ErrNotModified", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d", calls)
	}
}

func TestAlertsInUAFetchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	s := NewAlertsInUA("secret")
	s.URL = srv.URL
	if _, err := s.Fetch(context.Background()); err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err = %v, want 429 error", err)
	}
}
