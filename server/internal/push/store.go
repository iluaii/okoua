// SPDX-License-Identifier: AGPL-3.0-or-later

// Package push stores Web Push subscriptions per region and notifies subscribers
// when an alert starts or ends in their regions, and before planned power outages.
package push

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Subscription is a browser's push endpoint plus the regions it follows for alerts and
// the queues it follows for power outages. The endpoint is an opaque, browser-issued URL;
// we store nothing else about the user.
type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
	Regions  []int
	Power    []PowerRule
}

type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS push_subscriptions (
	endpoint   TEXT PRIMARY KEY,
	p256dh     TEXT NOT NULL,
	auth       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS push_regions (
	endpoint TEXT    NOT NULL REFERENCES push_subscriptions (endpoint) ON DELETE CASCADE,
	region   INTEGER NOT NULL,
	PRIMARY KEY (endpoint, region)
);
CREATE INDEX IF NOT EXISTS push_regions_region ON push_regions (region);
CREATE TABLE IF NOT EXISTS push_power (
	endpoint   TEXT    NOT NULL REFERENCES push_subscriptions (endpoint) ON DELETE CASCADE,
	region     INTEGER NOT NULL,
	queue      TEXT    NOT NULL,
	before_off INTEGER NOT NULL,
	before_on  INTEGER NOT NULL,
	PRIMARY KEY (endpoint, region)
);
CREATE INDEX IF NOT EXISTS push_power_queue ON push_power (region, queue);
-- Power messages already sent, so each is sent once even across restarts.
CREATE TABLE IF NOT EXISTS push_power_sent (
	endpoint TEXT NOT NULL REFERENCES push_subscriptions (endpoint) ON DELETE CASCADE,
	event    TEXT NOT NULL,
	sent_at  TEXT NOT NULL,
	PRIMARY KEY (endpoint, event)
);
`

// NewStore creates the tables if needed. The caller owns db (opened with foreign keys on).
func NewStore(db *sql.DB) (*Store, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("push: schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Save creates or replaces a subscription, its regions and power rules.
func (s *Store) Save(ctx context.Context, sub Subscription) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO push_subscriptions (endpoint, p256dh, auth, created_at, updated_at) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (endpoint) DO UPDATE SET p256dh = excluded.p256dh, auth = excluded.auth, updated_at = excluded.updated_at`,
		sub.Endpoint, sub.P256dh, sub.Auth, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_regions WHERE endpoint = ?`, sub.Endpoint); err != nil {
		return err
	}
	for _, r := range sub.Regions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO push_regions (endpoint, region) VALUES (?, ?)`, sub.Endpoint, r); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM push_power WHERE endpoint = ?`, sub.Endpoint); err != nil {
		return err
	}
	for _, p := range sub.Power {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO push_power (endpoint, region, queue, before_off, before_on) VALUES (?, ?, ?, ?, ?)`,
			sub.Endpoint, p.Region, p.Queue, p.BeforeOff, p.BeforeOn); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type powerSub struct {
	sub  Subscription
	rule PowerRule
}

// PowerFor returns the subscriptions that follow a queue, each with its rule.
func (s *Store) PowerFor(ctx context.Context, region int, queue string) ([]powerSub, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.endpoint, s.p256dh, s.auth, p.before_off, p.before_on FROM push_subscriptions s
		JOIN push_power p ON p.endpoint = s.endpoint WHERE p.region = ? AND p.queue = ?`, region, queue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []powerSub
	for rows.Next() {
		ps := powerSub{rule: PowerRule{Region: region, Queue: queue}}
		if err := rows.Scan(&ps.sub.Endpoint, &ps.sub.P256dh, &ps.sub.Auth, &ps.rule.BeforeOff, &ps.rule.BeforeOn); err != nil {
			return nil, err
		}
		out = append(out, ps)
	}
	return out, rows.Err()
}

// MarkSent records that event was sent to endpoint. It reports false if it already was.
func (s *Store) MarkSent(ctx context.Context, endpoint, event string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO push_power_sent (endpoint, event, sent_at) VALUES (?, ?, ?)`,
		endpoint, event, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// PruneSent forgets sent events from before t.
func (s *Store) PruneSent(ctx context.Context, t time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_power_sent WHERE sent_at < ?`, t.UTC().Format(time.RFC3339))
	return err
}

// Delete removes a subscription with its regions and power rules.
func (s *Store) Delete(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// ForRegion returns subscriptions that follow a region (without their region lists).
func (s *Store) ForRegion(ctx context.Context, region int) ([]Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.endpoint, s.p256dh, s.auth FROM push_subscriptions s
		JOIN push_regions r ON r.endpoint = s.endpoint WHERE r.region = ?`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Subscription
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// Count returns the number of subscriptions.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM push_subscriptions`).Scan(&n)
	return n, err
}

// pushHosts are the push services browsers use. Endpoints elsewhere are refused,
// so nobody can make our server send requests to arbitrary URLs.
var pushHosts = []string{
	"fcm.googleapis.com",                // Chrome, Edge on Android, most Chromium browsers
	"updates.push.services.mozilla.com", // Firefox
	"web.push.apple.com",                // Safari, iOS home-screen apps
	".notify.windows.com",               // Edge on Windows (*.notify.windows.com)
	".push.apple.com",                   // other Apple hosts
}

var errBadEndpoint = errors.New("endpoint is not a known push service")

func checkEndpoint(raw string) error {
	if len(raw) > 1000 {
		return errBadEndpoint
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return errBadEndpoint
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range pushHosts {
		if host == h || (strings.HasPrefix(h, ".") && strings.HasSuffix(host, h)) {
			return nil
		}
	}
	return errBadEndpoint
}
