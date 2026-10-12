// SPDX-License-Identifier: AGPL-3.0-or-later

// Command server polls the air-raid alert source and serves the current state to the site.
// Configuration comes from environment variables, see .env.example in the repository root.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/api"
	"alert-concept/server/internal/hub"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	addr := env("ADDR", ":8080")
	interval, err := time.ParseDuration(env("POLL_INTERVAL", "15s"))
	if err != nil || interval < 5*time.Second {
		return fmt.Errorf("POLL_INTERVAL: need a duration of at least 5s, got %q", os.Getenv("POLL_INTERVAL"))
	}

	src, err := source(log)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	h := hub.New(src, interval, log)
	go h.Run(ctx)

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(h, os.Getenv("STATIC_DIR")),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		// Request contexts are cancelled on shutdown; otherwise open SSE connections would block Shutdown.
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", addr, "source", src.Name(), "interval", interval)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func source(log *slog.Logger) (alerts.Source, error) {
	token := os.Getenv("ALERTS_IN_UA_TOKEN")
	name := os.Getenv("ALERTS_SOURCE")
	if name == "" {
		name = "mock"
		if token != "" {
			name = "alertsinua"
		}
	}
	switch name {
	case "alertsinua":
		if token == "" {
			return nil, errors.New("ALERTS_SOURCE=alertsinua requires ALERTS_IN_UA_TOKEN")
		}
		return alerts.NewAlertsInUA(token), nil
	case "mock":
		log.Warn("TEST DATA: using the mock source, alerts are not real")
		return alerts.NewMock(), nil
	default:
		return nil, fmt.Errorf("ALERTS_SOURCE: unknown source %q (want alertsinua or mock)", name)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
