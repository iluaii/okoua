// SPDX-License-Identifier: AGPL-3.0-or-later

package news

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Channel is a whitelisted public channel. Name and Kind are shown in the UI.
type Channel struct {
	Username string
	Name     string // Ukrainian, shown to users
	Kind     string // official | energy | media
}

// DefaultSources are the whitelisted channels per region UID (checked 2026-10-12).
// Only official bodies and established local media; never "monitoring" channels.
func DefaultSources() map[int][]Channel {
	return map[int][]Channel{
		25: { // Chernihiv oblast
			{Username: "chernigivskaODA", Name: "Чернігівська ОВА", Kind: "official"},
			{Username: "chernigivoblenergo", Name: "Чернігівобленерго", Kind: "energy"},
			{Username: "suspilnechernihiv", Name: "Суспільне Чернігів", Kind: "media"},
		},
	}
}

// Item is a news item as served to clients: a title and a preview, never the whole post.
type Item struct {
	ID          string `json:"id"`
	Region      int    `json:"region"`
	Title       string `json:"title"`
	Text        string `json:"text"`
	URL         string `json:"url"`
	Source      Source `json:"source"`
	PublishedAt string `json:"publishedAt"`
}

type Source struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

var (
	ErrUnsupported = errors.New("news: region not supported")
	ErrNotFound    = errors.New("news: item not found")
)

const (
	keepFor      = 7 * 24 * time.Hour
	titleRunes   = 120
	previewRunes = 400
)

const schema = `
CREATE TABLE IF NOT EXISTS news (
	id           TEXT PRIMARY KEY,
	region       INTEGER NOT NULL,
	channel      TEXT NOT NULL,
	url          TEXT NOT NULL,
	text         TEXT NOT NULL,
	published_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS news_region_published ON news (region, published_at);
`

// Service polls the channels and stores their posts.
type Service struct {
	db       *sql.DB
	sources  map[int][]Channel
	byUser   map[string]Channel
	interval time.Duration
	client   *http.Client
	log      *slog.Logger
}

func NewService(db *sql.DB, sources map[int][]Channel, interval time.Duration, log *slog.Logger) (*Service, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("news: schema: %w", err)
	}
	s := &Service{db: db, sources: sources, byUser: map[string]Channel{}, interval: interval,
		client: &http.Client{Timeout: 20 * time.Second}, log: log}
	for _, chans := range sources {
		for _, c := range chans {
			s.byUser[strings.ToLower(c.Username)] = c
		}
	}
	return s, nil
}

// Run fetches all channels until ctx is cancelled. The first fetch happens immediately.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		s.fetchAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Service) fetchAll(ctx context.Context) {
	for region, chans := range s.sources {
		for _, c := range chans {
			posts, err := FetchChannel(ctx, s.client, c.Username)
			if err != nil {
				if ctx.Err() == nil {
					s.log.Warn("news fetch failed", "channel", c.Username, "err", err)
				}
				continue
			}
			for _, p := range posts {
				if isThreatTracking(p.Text) {
					continue
				}
				if err := s.save(ctx, region, p); err != nil {
					s.log.Warn("news save failed", "err", err)
				}
			}
			// Be polite to t.me: one channel at a time with a pause.
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM news WHERE published_at < ?`,
		time.Now().UTC().Add(-keepFor).Format(time.RFC3339)); err != nil && ctx.Err() == nil {
		s.log.Warn("news prune failed", "err", err)
	}
}

func (s *Service) save(ctx context.Context, region int, p Post) error {
	// Upsert: channels edit posts, and the latest text wins.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO news (id, region, channel, url, text, published_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET text = excluded.text`,
		p.ID, region, p.Channel, p.URL, p.Text, p.PublishedAt.UTC().Format(time.RFC3339))
	return err
}

// Supported reports whether a region has news sources.
func (s *Service) Supported(region int) bool {
	_, ok := s.sources[region]
	return ok
}

// List returns the newest items of a region.
func (s *Service) List(ctx context.Context, region, limit int) ([]Item, error) {
	if !s.Supported(region) {
		return nil, ErrUnsupported
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, channel, url, text, published_at FROM news WHERE region = ? ORDER BY published_at DESC LIMIT ?`,
		region, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var id, channel, url, text, published string
		if err := rows.Scan(&id, &channel, &url, &text, &published); err != nil {
			return nil, err
		}
		if isThreatTracking(text) {
			continue // also hides posts saved before a filter change
		}
		items = append(items, s.item(id, region, channel, url, text, published))
	}
	return items, rows.Err()
}

// Post returns an item together with its full text, for the AI.
func (s *Service) Post(ctx context.Context, id string) (Item, string, error) {
	var channel, url, text, published string
	var region int
	err := s.db.QueryRowContext(ctx, `SELECT region, channel, url, text, published_at FROM news WHERE id = ?`, id).
		Scan(&region, &channel, &url, &text, &published)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, "", ErrNotFound
	}
	if err != nil {
		return Item{}, "", err
	}
	return s.item(id, region, channel, url, text, published), text, nil
}

func (s *Service) item(id string, region int, channel, url, text, published string) Item {
	c := s.byUser[strings.ToLower(channel)]
	title, rest := splitTitle(text)
	return Item{
		ID:          id,
		Region:      region,
		Title:       title,
		Text:        truncate(rest, previewRunes),
		URL:         url,
		Source:      Source{Name: c.Name, Kind: c.Kind},
		PublishedAt: published,
	}
}

// splitTitle uses the first line as the title; very long first lines are cut at a word.
func splitTitle(text string) (title, rest string) {
	first, rest, _ := strings.Cut(text, "\n")
	if utf8.RuneCountInString(first) <= titleRunes {
		return first, strings.TrimSpace(rest)
	}
	t := truncate(first, titleRunes)
	return t, text
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndexAny(string(r), " \n"); i > len(string(r))/2 {
		return strings.TrimSpace(string(r)[:i]) + "…"
	}
	return string(r) + "…"
}

// isThreatTracking spots real-time threat tracking ("drone heading to …") and alert/all-clear
// notices. The map already shows alerts, and we deliberately don't relay where drones or
// missiles are heading: that is what monitoring channels do, and it is not our job.
func isThreatTracking(text string) bool {
	t := strings.ToLower(text)
	first, _, _ := strings.Cut(t, "\n")
	if containsAny(first, "повітряна тривога", "повітряної тривоги", "відбій тривоги", "тривога в ", "тривога у ") {
		return true
	}
	if utf8.RuneCountInString(t) > 400 {
		return false // longer posts are reports, not live tracking
	}
	weapon := containsAny(t, "бпла", "шахед", "ракет", "дрон", "авіабомб", "каб ")
	moving := containsAny(t, "курсом", "напрямк", "напрямок", "повз ", "рухає", "рухаю", "летить", "летять",
		"на північ", "на південь", "на захід", "на схід", "у бік", "в бік",
		"з брянськ", "з курськ", "з білорус", "за інформацією повітряних сил")
	return weapon && moving
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
