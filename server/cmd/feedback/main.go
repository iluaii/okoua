// SPDX-License-Identifier: AGPL-3.0-or-later

// Command feedback prints the latest user feedback and reports on AI answers from the server database.
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

	"alert-concept/server/internal/ai"
	"alert-concept/server/internal/feedback"
	"alert-concept/server/internal/storage"
)

func main() {
	db := flag.String("db", "var/app.db", "path to the database")
	n := flag.Int("n", 50, "how many entries to show")
	flag.Parse()

	if _, err := os.Stat(*db); err != nil {
		fmt.Fprintln(os.Stderr, "no database:", err)
		os.Exit(1)
	}
	conn, err := storage.Open(*db)
	if err != nil {
		fail(err)
	}
	defer conn.Close()
	store, err := feedback.NewStore(conn)
	if err != nil {
		fail(err)
	}
	ctx := context.Background()

	list, err := store.List(ctx, *n)
	if err != nil {
		fail(err)
	}
	fmt.Println("== Feedback")
	if len(list) == 0 {
		fmt.Println("no feedback yet")
	}
	for _, e := range list {
		fmt.Printf("#%d  %s  %s/%s  page=%s  version=%s\n", e.ID, e.CreatedAt.Local().Format("2006-01-02 15:04"),
			e.Category, e.Subcategory, e.Page, e.Version)
		if e.Contact != "" {
			fmt.Println("    contact: " + e.Contact)
		}
		if e.Message != "" {
			// Messages are cleaned on input; indent them so they can't pass for metadata lines.
			fmt.Println("    " + strings.ReplaceAll(e.Message, "\n", "\n    "))
		}
		fmt.Println()
	}

	reports, err := ai.ListReports(ctx, conn, *n)
	if err != nil {
		fail(err)
	}
	fmt.Printf("== Reports on AI answers (hidden after %d)\n", ai.HideAfter)
	if len(reports) == 0 {
		fmt.Println("no reports yet")
	}
	for _, r := range reports {
		fmt.Printf("#%d  %s  %s  news=%s  model=%s\n", r.ID, r.CreatedAt.Local().Format("2006-01-02 15:04"),
			r.Reason, r.NewsID, r.Model)
		fmt.Println("    Q: " + indent(r.Question))
		fmt.Println("    A: " + indent(r.Answer))
		if r.Message != "" {
			fmt.Println("    > " + indent(r.Message))
		}
		fmt.Println()
	}
}

// indent keeps multiline text visibly inside its entry. Model output is not cleaned on
// input, so control characters are dropped here.
func indent(s string) string {
	return strings.ReplaceAll(feedback.Clean(s, true), "\n", "\n      ")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
