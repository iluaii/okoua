// SPDX-License-Identifier: AGPL-3.0-or-later

package power

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"
)

// 2026-10-12 00:00 Kyiv (UTC+3).
const day12 = 1791752400

// gpvHours builds a GPV hour map: all "yes" except the given overrides.
func gpvHours(over map[int]string) string {
	var parts []string
	for h := 1; h <= 24; h++ {
		v := "yes"
		if o, ok := over[h]; ok {
			v = o
		}
		parts = append(parts, fmt.Sprintf(`"%d":%q`, h, v))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func TestParseGPV(t *testing.T) {
	q11 := gpvHours(map[int]string{7: "no", 8: "no", 12: "first", 13: "mfirst", 19: "second", 20: "no", 21: "no", 22: "no"})
	q12 := gpvHours(nil)
	body := fmt.Sprintf(`{"regionId":"x","lastUpdated":"2026-10-12T09:57:49Z",
		"fact":{"data":{"%d":{"GPV1.1":%s,"GPV1.2":%s}},"update":"12.10.2026 09:57","today":%d}}`, day12, q11, q12, day12)

	d, err := parseGPV([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Queues, []string{"1.1", "1.2"}) {
		t.Errorf("queues = %v", d.Queues)
	}
	if len(d.Days) != 1 || d.Days[0].Date != "2026-10-12" || d.Days[0].Status != StatusSchedule {
		t.Fatalf("days = %+v", d.Days)
	}
	want := []Outage{{360, 480, false}, {660, 690, false}, {720, 750, true}, {1110, 1320, false}}
	if got := d.Days[0].Queues["1.1"]; !reflect.DeepEqual(got, want) {
		t.Errorf("1.1 = %v, want %v", got, want)
	}
	if got := d.Days[0].Queues["1.2"]; len(got) != 0 {
		t.Errorf("1.2 = %v, want no outages", got)
	}
	if !d.UpdatedAt.Equal(time.Date(2026, 10, 12, 9, 57, 49, 0, time.UTC)) {
		t.Errorf("updatedAt = %v", d.UpdatedAt)
	}
}

func TestParseGPVNoOutagesDay(t *testing.T) {
	body := fmt.Sprintf(`{"lastUpdated":"2026-10-12T09:00:00Z","fact":{"data":{"%d":{"GPV1.1":%s}}}}`, day12, gpvHours(nil))
	d, err := parseGPV([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if d.Days[0].Status != StatusNone {
		t.Errorf("status = %q, want none", d.Days[0].Status)
	}
}

func TestParseGPVEmptyAndBroken(t *testing.T) {
	d, err := parseGPV([]byte(`{"lastUpdated":"2026-10-12T09:00:00Z","fact":{"data":[]}}`))
	if err != nil || len(d.Days) != 0 {
		t.Errorf("empty data: %+v, %v", d, err)
	}
	missing := fmt.Sprintf(`{"fact":{"data":{"%d":{"GPV1.1":{"1":"yes"}}}}}`, day12)
	if _, err := parseGPV([]byte(missing)); err == nil {
		t.Error("missing hours: want error")
	}
}

func TestParseYasno(t *testing.T) {
	body := `{
	  "1.1": {"today": {"slots": [{"start":0,"end":210,"type":"Definite"},{"start":210,"end":1080,"type":"NotPlanned"},
	                              {"start":1080,"end":1320,"type":"Definite"},{"start":1320,"end":1440,"type":"NotPlanned"}],
	                    "date":"2026-10-12T00:00:00+03:00","status":"ScheduleApplies"},
	          "tomorrow": {"slots":[],"date":"2026-10-13T00:00:00+03:00","status":"WaitingForSchedule"},
	          "updatedOn":"2026-10-11T19:41:59+00:00"},
	  "10.1": {"today": {"slots":[{"start":600,"end":690,"type":"Possible"}],"date":"2026-10-12T00:00:00+03:00","status":"ScheduleApplies"},
	           "tomorrow": {"slots":[],"date":"2026-10-13T00:00:00+03:00","status":"WaitingForSchedule"},
	           "updatedOn":"2026-10-12T05:00:00+00:00"},
	  "2.1": {"today": {"slots":[],"date":"2026-10-12T00:00:00+03:00","status":"ScheduleApplies"},
	          "tomorrow": {"slots":[],"date":"2026-10-13T00:00:00+03:00","status":"WaitingForSchedule"},
	          "updatedOn":"2026-10-11T19:41:59+00:00"}
	}`
	d, err := parseYasno([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(d.Queues, []string{"1.1", "2.1", "10.1"}) {
		t.Errorf("queues = %v", d.Queues)
	}
	if !d.UpdatedAt.Equal(time.Date(2026, 10, 12, 5, 0, 0, 0, time.UTC)) {
		t.Errorf("updatedAt = %v, want the latest updatedOn", d.UpdatedAt)
	}
	days := map[string]Day{}
	for _, day := range d.Days {
		days[day.Date] = day
	}
	today, tomorrow := days["2026-10-12"], days["2026-10-13"]
	if today.Status != StatusSchedule || tomorrow.Status != StatusWaiting {
		t.Fatalf("statuses: %q, %q", today.Status, tomorrow.Status)
	}
	if got, want := today.Queues["1.1"], []Outage{{0, 210, false}, {1080, 1320, false}}; !reflect.DeepEqual(got, want) {
		t.Errorf("1.1 = %v, want %v", got, want)
	}
	if got := today.Queues["10.1"]; len(got) != 1 || !got[0].Possible {
		t.Errorf("10.1 = %v, want one possible outage for an unknown slot type", got)
	}
}

func TestParseYasnoRejectsBadSlots(t *testing.T) {
	body := `{"1.1":{"today":{"slots":[{"start":900,"end":600,"type":"Definite"}],"date":"2026-10-12T00:00:00+03:00","status":"ScheduleApplies"}}}`
	if _, err := parseYasno([]byte(body)); err == nil {
		t.Error("want error for an outage that ends before it starts")
	}
}

type fakeSource struct {
	data Data
	err  error
}

func (f *fakeSource) Fetch(context.Context) (Data, error) { return f.data, f.err }

func TestServiceGet(t *testing.T) {
	src := &fakeSource{data: Data{
		Queues: []string{"1.1"},
		Days: []Day{
			{Date: "2026-10-12", Status: StatusSchedule, Queues: map[string][]Outage{"1.1": {{360, 480, false}}}},
			{Date: "2026-10-11", Status: StatusSchedule, Queues: map[string][]Outage{}},
		},
		UpdatedAt: time.Date(2026, 10, 12, 6, 0, 0, 0, time.UTC),
	}}
	s := NewService([]Region{{UID: 25, Source: src, Operator: Link{Name: "op"}}}, 5*time.Minute,
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	clock := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC) // 13:00 Kyiv
	s.now = func() time.Time { return clock }

	if _, err := s.Get(3); !errors.Is(err, ErrUnsupported) {
		t.Errorf("unsupported region: err = %v", err)
	}
	if _, err := s.Get(25); !errors.Is(err, ErrNoData) {
		t.Errorf("before first fetch: err = %v", err)
	}

	s.fetchAll(context.Background())
	sch, err := s.Get(25)
	if err != nil {
		t.Fatal(err)
	}
	if sch.Today.Date != "2026-10-12" || sch.Today.Status != StatusSchedule || sch.Stale {
		t.Errorf("today = %+v, stale = %v", sch.Today, sch.Stale)
	}
	if sch.Tomorrow.Date != "2026-10-13" || sch.Tomorrow.Status != StatusWaiting {
		t.Errorf("tomorrow = %+v, want waiting", sch.Tomorrow)
	}

	// Kyiv midnight passed (21:30 UTC = 00:30 Kyiv): yesterday's data must not show as today.
	clock = time.Date(2026, 10, 12, 21, 30, 0, 0, time.UTC)
	src.err = errors.New("down")
	s.fetchAll(context.Background())
	sch, _ = s.Get(25)
	if sch.Today.Date != "2026-10-13" || sch.Today.Status != StatusUnknown {
		t.Errorf("after midnight: today = %+v, want unknown for 2026-10-13", sch.Today)
	}
	if !sch.Stale {
		t.Error("fetch failing for hours: want stale")
	}
}
