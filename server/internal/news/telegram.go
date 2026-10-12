// SPDX-License-Identifier: AGPL-3.0-or-later

// Package news collects posts from a whitelist of public regional channels.
package news

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// Post is one channel post as read from the public web preview (t.me/s/<channel>).
type Post struct {
	ID          string // "<channel>/<number>", unique across channels
	Channel     string
	URL         string
	Text        string
	PublishedAt time.Time
}

const userAgent = "alert-concept (open-source regional news reader)"

// FetchChannel reads the latest posts (usually about 20) of a public channel.
func FetchChannel(ctx context.Context, client *http.Client, channel string) ([]Post, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://t.me/s/"+channel, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("t.me/s/%s: %s", channel, resp.Status)
	}
	return parsePreview(io.LimitReader(resp.Body, 4<<20))
}

// parsePreview extracts posts from the t.me/s/ page.
func parsePreview(r io.Reader) ([]Post, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var posts []Post
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "tgme_widget_message") {
			if id := attr(n, "data-post"); id != "" {
				if p, ok := parsePost(n, id); ok {
					posts = append(posts, p)
				}
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return posts, nil
}

func parsePost(n *html.Node, id string) (Post, bool) {
	channel, _, _ := strings.Cut(id, "/")
	p := Post{ID: id, Channel: channel, URL: "https://t.me/" + id}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		// A quoted reply carries its own text block; it is not this post's text.
		if hasClass(n, "tgme_widget_message_reply") {
			return
		}
		if p.Text == "" && n.Data == "div" && hasClass(n, "tgme_widget_message_text") {
			p.Text = cleanText(textOf(n))
			return
		}
		if n.Data == "time" && p.PublishedAt.IsZero() {
			p.PublishedAt, _ = time.Parse(time.RFC3339, attr(n, "datetime"))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return p, p.Text != "" && !p.PublishedAt.IsZero()
}

// textOf returns the visible text with <br> as newlines.
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && n.Data == "br":
			b.WriteByte('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// cleanText trims lines and collapses runs of blank lines.
func cleanText(s string) string {
	var lines []string
	blank := false
	for _, l := range strings.Split(s, "\n") {
		l = strings.Join(strings.Fields(l), " ")
		if l == "" {
			blank = len(lines) > 0
			continue
		}
		if blank {
			lines = append(lines, "")
			blank = false
		}
		lines = append(lines, l)
	}
	return strings.Join(lines, "\n")
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}
