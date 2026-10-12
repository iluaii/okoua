// SPDX-License-Identifier: AGPL-3.0-or-later

// Command feedback prints the latest user feedback from the server database.
//
//	go run ./cmd/feedback            # last 50 entries
//	go run ./cmd/feedback -n 200 -db var/app.db
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"alert-concept/server/internal/feedback"
)

func main() {
	db := flag.String("db", "var/app.db", "path to the database")
	n := flag.Int("n", 50, "how many entries to show")
	flag.Parse()

	if _, err := os.Stat(*db); err != nil {
		fmt.Fprintln(os.Stderr, "no database:", err)
		os.Exit(1)
	}
	store, err := feedback.Open(*db)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()

	list, err := store.List(context.Background(), *n)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(list) == 0 {
		fmt.Println("no feedback yet")
		return
	}
	for _, e := range list {
		fmt.Printf("#%d  %s  %s/%s  page=%s  version=%s\n", e.ID, e.CreatedAt.Local().Format("2006-01-02 15:04"),
			e.Category, e.Subcategory, e.Page, e.Version)
		if e.Message != "" {
			// Messages are cleaned on input; indent them so they can't pass for metadata lines.
			fmt.Println("    " + strings.ReplaceAll(e.Message, "\n", "\n    "))
		}
		fmt.Println()
	}
}
