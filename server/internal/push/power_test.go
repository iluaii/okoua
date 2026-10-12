// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alert-concept/server/internal/power"
)

// fakeSchedules serves one schedule for Chernihiv (25).
type fakeSchedules struct{ sch power.Schedule }

func (f fakeSchedules) Supported() []int { return []int{25} }
func (f fakeSchedules) Get(uid int) (power.Schedule, error) {
	if uid != 25 {
		return power.Schedule{}, power.ErrUnsupported
	}
	return f.sch, nil
}

// testSchedule: queue 3.1 is off 14:00–18:00 and 22:00–24:00 on 12.10, then 00:00–02:00
// on 13.10, plus a possible outage at 20:00 (ignored).
func testSchedule() power.Schedule {
	return power.Schedule{
		Region: 25,
		Queues: []string{"3.1", "3.2"},
		Today: power.Day{Date: "2026-10-12", Status: power.StatusSchedule, Queues: map[string][]power.Outage{
			"3.1": {{From: 14 * 60, To: 18 * 60}, {From: 20 * 60, To: 21 * 60, Possible: true}, {From: 22 * 60, To: 24 * 60}},
		}},
		Tomorrow: power.Day{Date: "2026-10-13", Status: power.StatusSchedule, Queues: map[string][]power.Outage{
			"3.1": {{From: 0, To: 2 * 60}},
		}},
	}
}

func at(day, hour, min int) time.Time { return time.Date(2026, 10, day, hour, min, 0, 0, kyiv) }

func TestSpans(t *testing.T) {
	got := spans(testSchedule(), "3.1")
	want := []span{{at(12, 14, 0), at(12, 18, 0)}, {at(12, 22, 0), at(13, 2, 0)}}
	if len(got) != len(want) {
		t.Fatalf("spans = %v, want %v (possible skipped, midnight joined)", got, want)
	}
	for i := range want {
		if !got[i].start.Equal(want[i].start) || !got[i].end.Equal(want[i].end) {
			t.Errorf("span %d = %v–%v, want %v–%v", i, got[i].start, got[i].end, want[i].start, want[i].end)
		}
	}
}

func TestDue(t *testing.T) {
	s := span{at(12, 14, 0), at(12, 18, 0)}
	r := PowerRule{Region: 25, Queue: "3.1", BeforeOff: 30, BeforeOn: 15}
	cases := []struct {
		now   time.Time
		title string // "" = nothing due
	}{
		{at(12, 13, 29), ""},
		{at(12, 13, 30), "🔌 Відключення світла о 14:00"},
		{at(12, 13, 59), "🔌 Відключення світла о 14:00"},
		{at(12, 14, 0), ""},
		{at(12, 17, 44), ""},
		{at(12, 17, 45), "💡 Світло увімкнуть о 18:00"},
		{at(12, 18, 0), ""},
	}
	for _, c := range cases {
		m, _, ok := due(r, s, c.now)
		if (c.title == "") == ok || m.Title != c.title {
			t.Errorf("%s: %q %v, want %q", c.now.Format("15:04"), m.Title, ok, c.title)
		}
	}
	m, _, _ := due(r, s, at(12, 13, 45))
	if m.Body != "Чернігівська область · черга 3.1 · до 18:00" {
		t.Errorf("body = %q", m.Body)
	}

	r.BeforeOn = 0
	if m, _, ok := due(r, s, at(12, 18, 5)); !ok || m.Title != "💡 Світло мають увімкнути" {
		t.Errorf("when due back: %q %v", m.Title, ok)
	}
	if _, _, ok := due(r, s, at(12, 18, 10)); ok {
		t.Error("power-back message sent after the grace period")
	}
	r.BeforeOn = -1
	if _, _, ok := due(r, s, at(12, 18, 0)); ok {
		t.Error("power-back message sent though not asked for")
	}
}

func TestPowerNotifierSendsOnce(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	if err := s.Save(ctx, Subscription{Endpoint: "https://fcm.googleapis.com/a", P256dh: p256dh, Auth: auth,
		Power: []PowerRule{{Region: 25, Queue: "3.1", BeforeOff: 60, BeforeOn: -1}}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, Subscription{Endpoint: "https://fcm.googleapis.com/b", P256dh: p256dh, Auth: auth,
		Power: []PowerRule{{Region: 25, Queue: "3.2", BeforeOff: 60, BeforeOn: -1}}}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var sent []string
	send := func(_ context.Context, payload []byte, sub Subscription, topic string) (int, error) {
		var m Message
		json.Unmarshal(payload, &m)
		mu.Lock()
		sent = append(sent, sub.Endpoint+" "+m.Title)
		mu.Unlock()
		return http.StatusCreated, nil
	}
	n := NewPowerNotifier(s, fakeSchedules{testSchedule()}, send, discard)
	for _, now := range []time.Time{at(12, 13, 10), at(12, 13, 10).Add(30 * time.Second), at(12, 13, 40)} {
		n.now = func() time.Time { return now }
		n.tick(ctx)
	}
	if len(sent) != 1 || sent[0] != "https://fcm.googleapis.com/a 🔌 Відключення світла о 14:00" {
		t.Errorf("sent %v, want one warning to the 3.1 subscriber", sent)
	}
}

func TestHandlerPowerRules(t *testing.T) {
	s := newStore(t)
	h := &Handler{Store: s, PublicKey: "pub", Limiter: NewLimiter(), Log: discard, Power: fakeSchedules{testSchedule()}}
	mux := http.NewServeMux()
	h.Register(mux)
	post := func(power string) int {
		body := `{"subscription":{"endpoint":"https://fcm.googleapis.com/p","keys":{"p256dh":"` + p256dh + `","auth":"` + auth + `"}},"regions":[],"power":` + power + `}`
		r := httptest.NewRequest(http.MethodPost, "/api/push/subscribe", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	cases := []struct {
		name, power string
		code        int
	}{
		{"ok", `[{"region":25,"queue":"3.1","beforeOff":30,"beforeOn":0}]`, 204},
		{"unknown queue", `[{"region":25,"queue":"9.9","beforeOff":30,"beforeOn":0}]`, 400},
		{"unsupported region", `[{"region":3,"queue":"3.1","beforeOff":30,"beforeOn":0}]`, 400},
		{"bad times", `[{"region":25,"queue":"3.1","beforeOff":0,"beforeOn":0}]`, 400},
		{"two queues in one region", `[{"region":25,"queue":"3.1","beforeOff":30,"beforeOn":0},{"region":25,"queue":"3.2","beforeOff":30,"beforeOn":0}]`, 400},
	}
	for _, c := range cases {
		if code := post(c.power); code != c.code {
			t.Errorf("%s: %d, want %d", c.name, code, c.code)
		}
	}
	ctx := context.Background()
	if subs, _ := s.PowerFor(ctx, 25, "3.1"); len(subs) != 1 || subs[0].rule.BeforeOff != 30 {
		t.Errorf("PowerFor = %+v", subs)
	}
	// Power rules alone keep the subscription; dropping them too unsubscribes.
	if code := post(`[]`); code != 204 {
		t.Fatalf("clear: %d", code)
	}
	if n, _ := s.Count(ctx); n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}
