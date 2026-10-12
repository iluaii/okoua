// SPDX-License-Identifier: AGPL-3.0-or-later

package feedback

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

// Entry is one piece of feedback. It deliberately has no IP or user identifier.
type Entry struct {
	ID          int64
	CreatedAt   time.Time
	Category    string
	Subcategory string
	Message     string
	Page        string
	Version     string
}

// Store keeps feedback in SQLite.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS feedback (
	id          INTEGER PRIMARY KEY,
	created_at  TEXT    NOT NULL,
	category    TEXT    NOT NULL,
	subcategory TEXT    NOT NULL,
	message     TEXT    NOT NULL,
	page        TEXT    NOT NULL,
	version     TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS feedback_created_at ON feedback (created_at);
`

// Open opens (or creates) the database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite allows one writer; serialize to avoid busy errors
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("feedback: schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Add(ctx context.Context, e Entry) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feedback (created_at, category, subcategory, message, page, version) VALUES (?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), e.Category, e.Subcategory, e.Message, e.Page, e.Version)
	return err
}

// List returns the newest entries first.
func (s *Store) List(ctx context.Context, limit int) ([]Entry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, created_at, category, subcategory, message, page, version FROM feedback ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var created string
		if err := rows.Scan(&e.ID, &created, &e.Category, &e.Subcategory, &e.Message, &e.Page, &e.Version); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, created)
		out = append(out, e)
	}
	return out, rows.Err()
}

// Prune deletes entries older than maxAge, so feedback isn't kept longer than needed.
func (s *Store) Prune(ctx context.Context, maxAge time.Duration) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM feedback WHERE created_at < ?`,
		time.Now().UTC().Add(-maxAge).Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
