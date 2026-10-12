// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ratelimit is an in-memory, privacy-preserving rate limiter keyed by client IP.
package ratelimit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Rule allows at most N events per Per.
type Rule struct {
	N   int
	Per time.Duration
}

// Limiter is an in-memory sliding-window rate limiter keyed by client IP.
// IPs are never stored as-is: the key is a hash with a salt that changes every day,
// and all per-client state is dropped when the salt changes.
type Limiter struct {
	PerClient []Rule
	Global    []Rule
	// MaxClients caps memory use under a flood of distinct addresses; beyond it requests are refused.
	MaxClients int

	mu      sync.Mutex
	now     func() time.Time
	salt    [16]byte
	saltDay string
	clients map[string][]time.Time
	all     []time.Time
	calls   int
}

// New returns a limiter with the given rules. Keep per-client limits generous:
// mobile carriers put many people behind one IP.
func New(perClient, global []Rule) *Limiter {
	return &Limiter{PerClient: perClient, Global: global, MaxClients: 100_000, now: time.Now}
}

// Allow records an attempt from ip and reports whether it is within the limits.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()

	if day := now.UTC().Format(time.DateOnly); day != l.saltDay {
		rand.Read(l.salt[:])
		l.saltDay = day
		l.clients = map[string][]time.Time{}
	}
	l.calls++
	if l.calls%1000 == 0 {
		l.sweep(now)
	}

	key := l.key(ip)
	hits := prune(l.clients[key], now, longest(l.PerClient))
	l.all = prune(l.all, now, longest(l.Global))
	if hits == nil && len(l.clients) >= l.MaxClients {
		return false
	}
	ok := within(hits, now, l.PerClient) && within(l.all, now, l.Global)
	if ok {
		hits = append(hits, now)
		l.all = append(l.all, now)
	}
	if len(hits) > 0 {
		l.clients[key] = hits
	}
	return ok
}

func (l *Limiter) key(ip string) string {
	sum := sha256.Sum256(append(l.salt[:], ip...))
	return hex.EncodeToString(sum[:12])
}

func (l *Limiter) sweep(now time.Time) {
	window := longest(l.PerClient)
	for k, hits := range l.clients {
		if hits = prune(hits, now, window); len(hits) == 0 {
			delete(l.clients, k)
		} else {
			l.clients[k] = hits
		}
	}
}

func within(hits []time.Time, now time.Time, rules []Rule) bool {
	for _, r := range rules {
		n := 0
		for _, t := range hits {
			if now.Sub(t) < r.Per {
				n++
			}
		}
		if n >= r.N {
			return false
		}
	}
	return true
}

// prune drops hits older than window; hits are in time order.
func prune(hits []time.Time, now time.Time, window time.Duration) []time.Time {
	i := 0
	for i < len(hits) && now.Sub(hits[i]) >= window {
		i++
	}
	if i == len(hits) {
		return nil
	}
	return hits[i:]
}

func longest(rules []Rule) time.Duration {
	var d time.Duration
	for _, r := range rules {
		d = max(d, r.Per)
	}
	return d
}

// ClientIP returns the client address. IPv6 addresses are reduced to their /64 network,
// since one client usually controls a whole /64.
func ClientIP(r *http.Request, trustProxy bool) string {
	addr := r.RemoteAddr
	if trustProxy {
		// Our proxy appends the address it saw as the last X-Forwarded-For entry.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			addr = strings.TrimSpace(parts[len(parts)-1])
		}
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	ip := net.ParseIP(addr)
	if ip == nil {
		return addr
	}
	if ip.To4() == nil {
		return ip.Mask(net.CIDRMask(64, 128)).String()
	}
	return ip.String()
}
