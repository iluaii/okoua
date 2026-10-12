// SPDX-License-Identifier: AGPL-3.0-or-later

package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	clock := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	l := New([]Rule{{5, 10 * time.Minute}, {30, 24 * time.Hour}}, []Rule{{300, time.Hour}})
	l.now = func() time.Time { return clock }

	for i := range 5 {
		if !l.Allow("1.2.3.4") {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("6th attempt in 10 minutes allowed")
	}
	if !l.Allow("5.6.7.8") {
		t.Fatal("another client refused")
	}
	clock = clock.Add(10 * time.Minute)
	if !l.Allow("1.2.3.4") {
		t.Fatal("refused after the window passed")
	}
}

func TestLimiterGlobal(t *testing.T) {
	l := New([]Rule{{5, 10 * time.Minute}, {30, 24 * time.Hour}}, []Rule{{300, time.Hour}})
	l.Global = []Rule{{3, time.Hour}}
	for i := range 3 {
		if !l.Allow("10.0.0." + string(rune('1'+i))) {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	if l.Allow("10.0.0.9") {
		t.Fatal("global limit not enforced")
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.RemoteAddr = "[2001:db8:1:2:3:4:5:6]:4444"
	if got := ClientIP(r, false); got != "2001:db8:1:2::" {
		t.Errorf("ipv6 = %q, want the /64 network", got)
	}
	r.RemoteAddr = "127.0.0.1:5555"
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 9.9.9.9")
	if got := ClientIP(r, false); got != "127.0.0.1" {
		t.Errorf("untrusted proxy: %q", got)
	}
	if got := ClientIP(r, true); got != "9.9.9.9" {
		t.Errorf("trusted proxy: %q, want the address our proxy appended", got)
	}
}
