// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage opens the server's SQLite database.
package storage

import (
	"database/sql"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no cgo
)

// Open opens (or creates) the SQLite database at path.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite allows one writer; serialize to avoid busy errors
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
