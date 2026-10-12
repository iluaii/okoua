// SPDX-License-Identifier: AGPL-3.0-or-later

// Package history records when alerts start and end, and summarizes them per region.
package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"alert-concept/server/internal/hub"
)

const schema = `
CREATE TABLE IF NOT EXISTS alert_history (
	alert_id   TEXT    NOT NULL,
	region     INTEGER NOT NULL,
	started_at TEXT    NOT NULL,
	ended_at   TEXT,
	PRIMARY KEY (alert_id, started_at)
);
CREATE INDEX IF NOT EXISTS alert_history_region ON alert_history (region, started_at);
CREATE INDEX IF NOT EXISTS alert_history_open ON alert_history (ended_at);
`

// Keep is how long history is kept.
const Keep = 30 * 24 * time.Hour

type Recorder struct {
	db  *sql.DB
	log *slog.Logger
	now func() time.Time
}

func NewRecorder(db *sql.DB, log *slog.Logger) (*Recorder, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("history: schema: %w", err)
	}
	return &Recorder{db: db, log: log, now: time.Now}, nil
}

type key struct{ id, started string }

// Sync is the hub's OnState callback: it records new alerts and closes the ones that are
// no longer active. Alerts that ended while the server was down are closed at the next sync.
func (r *Recorder) Sync(regions []hub.RegionState) {
	if err := r.sync(context.Background(), regions); err != nil {
		r.log.Warn("history sync failed", "err", err)
	}
}

func (r *Recorder) sync(ctx context.Context, regions []hub.RegionState) error {
	now := r.now().UTC().Format(time.RFC3339)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	active := map[key]bool{}
	for _, reg := range regions {
		for _, a := range reg.Alerts {
			k := key{a.ID, a.StartedAt.UTC().Format(time.RFC3339)}
			active[k] = true
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO alert_history (alert_id, region, started_at) VALUES (?, ?, ?)`,
				k.id, reg.UID, k.started); err != nil {
				return err
			}
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT alert_id, started_at FROM alert_history WHERE ended_at IS NULL`)
	if err != nil {
		return err
	}
	var ended []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.id, &k.started); err != nil {
			rows.Close()
			return err
		}
		if !active[k] {
			ended = append(ended, k)
		}
	}
	rows.Close()
	for _, k := range ended {
		if _, err := tx.ExecContext(ctx, `UPDATE alert_history SET ended_at = ? WHERE alert_id = ? AND started_at = ?`,
			now, k.id, k.started); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM alert_history WHERE ended_at IS NOT NULL AND ended_at < ?`,
		r.now().UTC().Add(-Keep).Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// Stat summarizes a region's alerts over a period. Overlapping alerts in one region
// (for example in several districts at once) count as one.
type Stat struct {
	Count          int `json:"count"`
	TotalMinutes   int `json:"totalMinutes"`
	LongestMinutes int `json:"longestMinutes"`
}

type span struct{ from, to time.Time }

// Stats returns per-region summaries of alerts that overlapped [since, now].
func (r *Recorder) Stats(ctx context.Context, since time.Time) (map[int]Stat, error) {
	now := r.now().UTC()
	rows, err := r.db.QueryContext(ctx,
		`SELECT region, started_at, ended_at FROM alert_history WHERE ended_at IS NULL OR ended_at > ?`,
		since.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spans := map[int][]span{}
	for rows.Next() {
		var region int
		var startedS string
		var endedS sql.NullString
		if err := rows.Scan(&region, &startedS, &endedS); err != nil {
			return nil, err
		}
		from, _ := time.Parse(time.RFC3339, startedS)
		to := now
		if endedS.Valid {
			to, _ = time.Parse(time.RFC3339, endedS.String)
		}
		from = maxTime(from, since)
		if to.After(from) {
			spans[region] = append(spans[region], span{from, to})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[int]Stat{}
	for region, ss := range spans {
		slices.SortFunc(ss, func(a, b span) int { return a.from.Compare(b.from) })
		var merged []span
		for _, s := range ss {
			if n := len(merged); n > 0 && !s.from.After(merged[n-1].to) {
				merged[n-1].to = maxTime(merged[n-1].to, s.to)
				continue
			}
			merged = append(merged, s)
		}
		var st Stat
		for _, m := range merged {
			d := int(m.to.Sub(m.from).Minutes())
			st.Count++
			st.TotalMinutes += d
			st.LongestMinutes = max(st.LongestMinutes, d)
		}
		out[region] = st
	}
	return out, nil
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// Register adds GET /api/stats/alerts?hours=N (1–168, default 24).
func (r *Recorder) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stats/alerts", func(w http.ResponseWriter, req *http.Request) {
		hours := 24
		if v := req.URL.Query().Get("hours"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 168 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "hours must be 1–168"})
				return
			}
			hours = n
		}
		since := r.now().UTC().Add(-time.Duration(hours) * time.Hour)
		stats, err := r.Stats(req.Context(), since)
		if err != nil {
			r.log.Error("history stats", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
			return
		}
		byUID := map[string]Stat{}
		for uid, s := range stats {
			byUID[strconv.Itoa(uid)] = s
		}
		writeJSON(w, http.StatusOK, map[string]any{"since": since.Format(time.RFC3339), "hours": hours, "regions": byUID})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
