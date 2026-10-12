// SPDX-License-Identifier: AGPL-3.0-or-later

package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"alert-concept/server/internal/ratelimit"
)

// Lookup returns the post with the given news ID, or ErrNotFound.
type Lookup func(ctx context.Context, id string) (Post, error)

var ErrNotFound = errors.New("ai: post not found")

const schema = `
CREATE TABLE IF NOT EXISTS ai_answers (
	news_id    TEXT NOT NULL,
	question   TEXT NOT NULL,
	answer     TEXT NOT NULL,
	questions  TEXT NOT NULL, -- JSON array of suggested follow-ups
	model      TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (news_id, question)
);
`

// NewLimiter limits AI requests per client; answers are cached, so only new questions cost.
func NewLimiter() *ratelimit.Limiter {
	return ratelimit.New(
		[]ratelimit.Rule{{N: 20, Per: 10 * time.Minute}, {N: 100, Per: 24 * time.Hour}},
		[]ratelimit.Rule{{N: 300, Per: time.Hour}},
	)
}

// Handler serves POST /api/ai/news.
type Handler struct {
	db         *sql.DB
	asker      *Asker
	lookup     Lookup
	limiter    *ratelimit.Limiter
	trustProxy bool
	log        *slog.Logger

	reportLimiter *ratelimit.Limiter
	reportOnce    *ratelimit.Limiter

	// DailyLimit caps model calls per UTC day (free models have small daily quotas).
	dailyLimit int
	mu         sync.Mutex
	day        string
	used       int
	inflight   map[string]*sync.Mutex // one model call per (post, question) at a time
}

func NewHandler(db *sql.DB, asker *Asker, lookup Lookup, dailyLimit int, trustProxy bool, log *slog.Logger) (*Handler, error) {
	if _, err := db.Exec(schema + reportSchema); err != nil {
		return nil, fmt.Errorf("ai: schema: %w", err)
	}
	all, once := newReportLimiters()
	return &Handler{db: db, asker: asker, lookup: lookup, limiter: NewLimiter(), trustProxy: trustProxy,
		log: log, reportLimiter: all, reportOnce: once, dailyLimit: dailyLimit, inflight: map[string]*sync.Mutex{}}, nil
}

// Register adds POST /api/ai/news and POST /api/ai/report.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /api/ai/news", h)
	mux.HandleFunc("POST /api/ai/report", h.report)
}

type request struct {
	NewsID   string `json:"newsId"`
	Question string `json:"question"`
}

type response struct {
	Question  string   `json:"question"` // label as shown in the chat
	Answer    string   `json:"answer"`
	Questions []string `json:"questions"`
	Model     string   `json:"model"`
	Cached    bool     `json:"cached"`
	// Hidden: people reported this answer, so it is withheld (Answer and Questions are
	// empty) and the client points to the original post instead.
	Hidden bool `json:"hidden,omitempty"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "json only"})
		return
	}
	var req request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	ctx := r.Context()
	post, err := h.lookup(ctx, req.NewsID)
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no such news"})
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}

	// No free-form questions: only the summary or a follow-up the model suggested for this post.
	if req.Question != Summary {
		ok, err := h.suggested(ctx, req.NewsID, req.Question)
		if err != nil {
			h.fail(w, err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown question"})
			return
		}
	}
	label := req.Question
	if req.Question == Summary {
		label = SummaryLabel
	}

	if res, ok, err := h.cached(ctx, req.NewsID, req.Question); err != nil {
		h.fail(w, err)
		return
	} else if ok {
		res.Question = label
		writeJSON(w, http.StatusOK, res)
		return
	}

	// Only uncached answers cost a model call, so only they are rate-limited.
	if !h.limiter.Allow(ratelimit.ClientIP(r, h.trustProxy)) {
		w.Header().Set("Retry-After", "600")
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "too many requests"})
		return
	}

	lock := h.lock(req.NewsID + "\x00" + req.Question)
	lock.Lock()
	defer lock.Unlock()
	// Another request may have answered while we waited.
	if res, ok, err := h.cached(ctx, req.NewsID, req.Question); err == nil && ok {
		res.Question = label
		writeJSON(w, http.StatusOK, res)
		return
	}
	if !h.spend() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "daily limit"})
		return
	}

	asked, err := h.askedQuestions(ctx, req.NewsID)
	if err != nil {
		h.fail(w, err)
		return
	}
	if req.Question != Summary {
		asked = append(asked, req.Question)
	}
	ans, err := h.asker.Ask(ctx, post, req.Question, asked)
	if err != nil {
		h.log.Warn("ai: no answer", "news", req.NewsID, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "no answer"})
		return
	}
	qs, _ := json.Marshal(ans.Questions)
	if _, err := h.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO ai_answers (news_id, question, answer, questions, model, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		req.NewsID, req.Question, ans.Text, string(qs), ans.Model, time.Now().UTC().Format(time.RFC3339)); err != nil {
		h.log.Warn("ai: cache store failed", "err", err)
	}
	writeJSON(w, http.StatusOK, response{Question: label, Answer: ans.Text, Questions: ans.Questions, Model: ans.Model})
}

func (h *Handler) cached(ctx context.Context, newsID, question string) (response, bool, error) {
	var res response
	var qs, created string
	err := h.db.QueryRowContext(ctx, `SELECT answer, questions, model, created_at FROM ai_answers WHERE news_id = ? AND question = ?`,
		newsID, question).Scan(&res.Answer, &qs, &res.Model, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return response{}, false, nil
	}
	if err != nil {
		return response{}, false, err
	}
	res.Cached = true
	// A hidden answer stays hidden until it is pruned; it is not regenerated, since
	// another model would likely misread the same post the same way.
	n, err := h.reportCount(ctx, newsID, question, created)
	if err != nil {
		return response{}, false, err
	}
	if n >= HideAfter {
		res.Answer, res.Hidden = "", true
		return res, true, nil
	}
	json.Unmarshal([]byte(qs), &res.Questions)
	return res, true, nil
}

// suggested reports whether question was offered as a follow-up for this post.
func (h *Handler) suggested(ctx context.Context, newsID, question string) (bool, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT questions FROM ai_answers WHERE news_id = ?`, newsID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return false, err
		}
		var qs []string
		json.Unmarshal([]byte(raw), &qs)
		if containsFold(qs, question) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (h *Handler) askedQuestions(ctx context.Context, newsID string) ([]string, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT question FROM ai_answers WHERE news_id = ? AND question <> ?`, newsID, Summary)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var q string
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// spend counts one model call against the daily limit.
func (h *Handler) spend() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if day := time.Now().UTC().Format(time.DateOnly); day != h.day {
		h.day, h.used = day, 0
	}
	if h.used >= h.dailyLimit {
		return false
	}
	h.used++
	return true
}

func (h *Handler) lock(key string) *sync.Mutex {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.inflight[key]
	if m == nil {
		m = &sync.Mutex{}
		h.inflight[key] = m
	}
	return m
}

// Prune deletes cached answers older than maxAge (news itself is kept for a week)
// and reports older than a year.
func (h *Handler) Prune(ctx context.Context, maxAge time.Duration) error {
	now := time.Now().UTC()
	_, err := h.db.ExecContext(ctx, `DELETE FROM ai_answers WHERE created_at < ?`, now.Add(-maxAge).Format(time.RFC3339))
	if _, err2 := h.db.ExecContext(ctx, `DELETE FROM ai_reports WHERE created_at < ?`,
		now.Add(-reportRetention).Format(time.RFC3339)); err == nil {
		err = err2
	}
	// Drop per-question locks nobody holds, so the map doesn't grow forever.
	h.mu.Lock()
	for k, m := range h.inflight {
		if m.TryLock() {
			delete(h.inflight, k)
			m.Unlock()
		}
	}
	h.mu.Unlock()
	return err
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	h.log.Error("ai handler", "err", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
