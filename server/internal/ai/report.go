// SPDX-License-Identifier: AGPL-3.0-or-later

package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"alert-concept/server/internal/feedback"
	"alert-concept/server/internal/ratelimit"
)

// Reasons are the allowed report reasons. Labels live in the web client.
var Reasons = []string{"wrong_fact", "wrong_region", "outdated", "other"}

const (
	// HideAfter reports from different clients hide an answer; the client then shows the
	// original post instead. A flood of fake reports can at worst hide a summary.
	HideAfter        = 2
	maxReportMessage = 300 // characters
	reportRetention  = 365 * 24 * time.Hour
)

// Reports are kept apart from ai_answers: answers are pruned after days, reports are
// reviewed later. Each report remembers the answer it was about and when that answer
// was made, so reports on a pruned answer don't hide a new one.
const reportSchema = `
CREATE TABLE IF NOT EXISTS ai_reports (
	id                INTEGER PRIMARY KEY,
	created_at        TEXT NOT NULL,
	news_id           TEXT NOT NULL,
	question          TEXT NOT NULL,
	answer            TEXT NOT NULL,
	model             TEXT NOT NULL,
	answer_created_at TEXT NOT NULL,
	reason            TEXT NOT NULL,
	message           TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS ai_reports_answer ON ai_reports (news_id, question, answer_created_at);
`

// newReportLimiters returns the overall report limit (5 per 10 minutes and 30 per day per
// client, 300 per hour in total) and a limiter that lets each client count once per answer.
func newReportLimiters() (all, once *ratelimit.Limiter) {
	all = ratelimit.New(
		[]ratelimit.Rule{{N: 5, Per: 10 * time.Minute}, {N: 30, Per: 24 * time.Hour}},
		[]ratelimit.Rule{{N: 300, Per: time.Hour}},
	)
	once = ratelimit.New([]ratelimit.Rule{{N: 1, Per: 24 * time.Hour}}, nil)
	return all, once
}

type reportRequest struct {
	NewsID   string `json:"newsId"`
	Question string `json:"question"`
	Reason   string `json:"reason"`
	Message  string `json:"message"`
}

// report serves POST /api/ai/report: a complaint about one cached answer.
func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "json only"})
		return
	}
	ip := ratelimit.ClientIP(r, h.trustProxy)
	if !h.reportLimiter.Allow(ip) {
		w.Header().Set("Retry-After", "600")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
		return
	}
	var req reportRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if !slices.Contains(Reasons, req.Reason) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad reason"})
		return
	}
	msg := feedback.Clean(req.Message, true)
	if utf8.RuneCountInString(msg) > maxReportMessage {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message too long"})
		return
	}

	ctx := r.Context()
	var answer, model, created string
	err := h.db.QueryRowContext(ctx, `SELECT answer, model, created_at FROM ai_answers WHERE news_id = ? AND question = ?`,
		req.NewsID, req.Question).Scan(&answer, &model, &created)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such answer"})
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}

	// A repeated report from the same client is accepted but not counted, so one person
	// can't hide an answer alone. The limiter keys by a salted hash; no IP is stored.
	if h.reportOnce.Allow(ip + "\x00" + req.NewsID + "\x00" + req.Question) {
		if _, err := h.db.ExecContext(ctx,
			`INSERT INTO ai_reports (created_at, news_id, question, answer, model, answer_created_at, reason, message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			time.Now().UTC().Format(time.RFC3339), req.NewsID, req.Question, answer, model, created, req.Reason, msg); err != nil {
			h.fail(w, err)
			return
		}
	}
	n, err := h.reportCount(ctx, req.NewsID, req.Question, created)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"hidden": n >= HideAfter})
}

func (h *Handler) reportCount(ctx context.Context, newsID, question, answerCreated string) (int, error) {
	var n int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ai_reports WHERE news_id = ? AND question = ? AND answer_created_at = ?`,
		newsID, question, answerCreated).Scan(&n)
	return n, err
}

// Report is one stored complaint, for review.
type Report struct {
	ID        int64
	CreatedAt time.Time
	NewsID    string
	Question  string
	Answer    string
	Model     string
	Reason    string
	Message   string
}

// ListReports returns the newest reports first. It creates the table if needed, so it
// works on a database the server hasn't touched since reports were added.
func ListReports(ctx context.Context, db *sql.DB, limit int) ([]Report, error) {
	if _, err := db.ExecContext(ctx, reportSchema); err != nil {
		return nil, fmt.Errorf("ai: report schema: %w", err)
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, created_at, news_id, question, answer, model, reason, message FROM ai_reports ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		var created string
		if err := rows.Scan(&r.ID, &created, &r.NewsID, &r.Question, &r.Answer, &r.Model, &r.Reason, &r.Message); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, r)
	}
	return out, rows.Err()
}
