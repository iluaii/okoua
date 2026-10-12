// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/hub"
	"alert-concept/server/internal/storage"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

const (
	p256dh = "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM"
	auth   = "tBHItJI5svbpez7KI4CCXg"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCheckEndpoint(t *testing.T) {
	ok := []string{
		"https://fcm.googleapis.com/fcm/send/abc",
		"https://updates.push.services.mozilla.com/wpush/v2/abc",
		"https://web.push.apple.com/QGuQyavXutnMH",
		"https://wns2-db5p.notify.windows.com/w/?token=abc",
	}
	bad := []string{
		"http://fcm.googleapis.com/fcm/send/abc",   // not https
		"https://evil.example/fcm.googleapis.com",  // wrong host
		"https://fcm.googleapis.com.evil.example/", // suffix trick
		"https://notify.windows.com.evil.example/",
		"https://user@fcm.googleapis.com/x",
		"https://fcm.googleapis.com:8443/x",
		"https://127.0.0.1/x",
	}
	for _, u := range ok {
		if err := checkEndpoint(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	for _, u := range bad {
		if checkEndpoint(u) == nil {
			t.Errorf("%s: accepted", u)
		}
	}
}

func TestStore(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	a := Subscription{Endpoint: "https://fcm.googleapis.com/a", P256dh: p256dh, Auth: auth, Regions: []int{25, 31}}
	b := Subscription{Endpoint: "https://fcm.googleapis.com/b", P256dh: p256dh, Auth: auth, Regions: []int{25}}
	for _, sub := range []Subscription{a, b} {
		if err := s.Save(ctx, sub); err != nil {
			t.Fatal(err)
		}
	}
	if subs, _ := s.ForRegion(ctx, 25); len(subs) != 2 {
		t.Errorf("region 25: %d subscribers, want 2", len(subs))
	}
	// Re-saving replaces the region list.
	a.Regions = []int{9}
	if err := s.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	if subs, _ := s.ForRegion(ctx, 31); len(subs) != 0 {
		t.Errorf("region 31 after re-save: %d, want 0", len(subs))
	}
	if err := s.Delete(ctx, b.Endpoint); err != nil {
		t.Fatal(err)
	}
	if subs, _ := s.ForRegion(ctx, 25); len(subs) != 0 {
		t.Errorf("region 25 after delete: %d, want 0 (regions cascade)", len(subs))
	}
	if n, _ := s.Count(ctx); n != 1 {
		t.Errorf("count = %d", n)
	}
}

func TestHandler(t *testing.T) {
	s := newStore(t)
	h := &Handler{Store: s, PublicKey: "pub", Limiter: NewLimiter(), Log: discard}
	mux := http.NewServeMux()
	h.Register(mux)

	post := func(path, body string) int {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w.Code
	}
	sub := func(endpoint, regions string) string {
		return `{"subscription":{"endpoint":"` + endpoint + `","keys":{"p256dh":"` + p256dh + `","auth":"` + auth + `"}},"regions":` + regions + `}`
	}

	cases := []struct {
		name, path, body string
		code             int
	}{
		{"ok", "/api/push/subscribe", sub("https://fcm.googleapis.com/x", "[25,25,31]"), 204},
		{"ssrf", "/api/push/subscribe", sub("https://169.254.169.254/latest", "[25]"), 400},
		{"bad region", "/api/push/subscribe", sub("https://fcm.googleapis.com/y", "[999]"), 400},
		{"bad keys", "/api/push/subscribe", `{"subscription":{"endpoint":"https://fcm.googleapis.com/z","keys":{"p256dh":"<script>","auth":"x"}},"regions":[25]}`, 400},
		{"no regions", "/api/push/subscribe", sub("https://fcm.googleapis.com/x", "[]"), 204},
		{"unsubscribe", "/api/push/unsubscribe", `{"endpoint":"https://fcm.googleapis.com/x"}`, 204},
	}
	for _, c := range cases {
		if code := post(c.path, c.body); code != c.code {
			t.Errorf("%s: %d, want %d", c.name, code, c.code)
		}
	}
	if n, _ := s.Count(context.Background()); n != 0 {
		t.Errorf("count = %d, want 0 (empty regions unsubscribes)", n)
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/push/key", nil))
	if !strings.Contains(w.Body.String(), `"pub"`) {
		t.Errorf("key: %s", w.Body.String())
	}
}

func region(uid int, status string, started time.Time, places ...string) hub.RegionState {
	r := hub.RegionState{UID: uid, Status: status}
	if len(places) == 0 {
		places = []string{alerts.Regions[uid]}
	}
	for _, p := range places {
		r.Alerts = append(r.Alerts, alerts.Alert{Type: "air_raid", OblastUID: uid, LocationTitle: p, StartedAt: started})
	}
	return r
}

func TestEvents(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	n := NewNotifier(nil, nil, discard)
	n.now = func() time.Time { return now }

	prev := []hub.RegionState{region(16, "full", now.Add(-72*time.Hour)), region(9, "full", now.Add(-40*time.Minute))}
	next := []hub.RegionState{region(16, "full", now.Add(-72*time.Hour)), region(25, "partial", now, "Ніжинський район")}
	ev := n.events(prev, next)
	if len(ev) != 2 {
		t.Fatalf("events = %+v, want a start in 25 and an end in 9", ev)
	}
	got := map[int]Message{}
	for _, e := range ev {
		got[e.region] = e.msg
	}
	if m := got[25]; m.Title != "🔴 Повітряна тривога" || m.Body != "Чернігівська область: Ніжинський район" || m.URL != "/?region=25" {
		t.Errorf("start = %+v", m)
	}
	if m := got[9]; m.Title != "🟢 Відбій тривоги" || m.Body != "Дніпропетровська область · тривала 40 хв" {
		t.Errorf("end = %+v", m)
	}
}

func TestDeliverRemovesExpired(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	for _, ep := range []string{"https://fcm.googleapis.com/live", "https://fcm.googleapis.com/gone"} {
		if err := s.Save(ctx, Subscription{Endpoint: ep, P256dh: p256dh, Auth: auth, Regions: []int{25}}); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	var sent []Message
	send := func(_ context.Context, payload []byte, sub Subscription, topic string) (int, error) {
		var m Message
		json.Unmarshal(payload, &m)
		mu.Lock()
		sent = append(sent, m)
		mu.Unlock()
		if strings.HasSuffix(sub.Endpoint, "/gone") {
			return http.StatusGone, nil
		}
		return http.StatusCreated, nil
	}
	n := NewNotifier(s, send, discard)
	n.deliver(ctx, event{25, Message{Title: "t", Region: 25}})

	if len(sent) != 2 {
		t.Errorf("sent %d, want 2", len(sent))
	}
	if subs, _ := s.ForRegion(ctx, 25); len(subs) != 1 || subs[0].Endpoint != "https://fcm.googleapis.com/live" {
		t.Errorf("after 410: %+v, want only the live subscription", subs)
	}
}

func TestDuration(t *testing.T) {
	cases := map[time.Duration]string{
		20 * time.Second:           "менше хвилини",
		40 * time.Minute:           "40 хв",
		65 * time.Minute:           "1 год 5 хв",
		2 * time.Hour:              "2 год",
		76 * time.Hour:             "3 дн 4 год",
		72*time.Hour + time.Minute: "3 дн",
	}
	for d, want := range cases {
		if got := duration(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}
