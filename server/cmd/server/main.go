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
	"path/filepath"
	"syscall"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/api"
	"alert-concept/server/internal/feedback"
	"alert-concept/server/internal/hub"
	"alert-concept/server/internal/power"
	"alert-concept/server/internal/push"
	"alert-concept/server/internal/storage"
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

	powerInterval, err := time.ParseDuration(env("POWER_INTERVAL", "5m"))
	if err != nil || powerInterval < time.Minute {
		return fmt.Errorf("POWER_INTERVAL: need a duration of at least 1m, got %q", os.Getenv("POWER_INTERVAL"))
	}
	p := power.NewService(power.DefaultRegions(), powerInterval, log)
	go p.Run(ctx)

	dataDir := env("DATA_DIR", "var")
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return err
	}
	db, err := storage.Open(filepath.Join(dataDir, "app.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	trustProxy := os.Getenv("TRUST_PROXY") == "1"

	store, err := feedback.NewStore(db)
	if err != nil {
		return err
	}
	go pruneFeedback(ctx, store, log)
	fb := &feedback.Handler{Store: store, Limiter: feedback.NewLimiter(), TrustProxy: trustProxy, Log: log}

	pushStore, err := push.NewStore(db)
	if err != nil {
		return err
	}
	keys, err := push.LoadOrCreateKeys(filepath.Join(dataDir, "vapid.json"))
	if err != nil {
		return err
	}
	subject := os.Getenv("VAPID_SUBJECT")
	if subject == "" {
		subject = "https://localhost"
		log.Warn("VAPID_SUBJECT is not set; set it to a contact URL or e-mail before deploying")
	}
	notifier := push.NewNotifier(pushStore, push.WebPushSender(keys, subject), log)
	h.OnChange(notifier.Changed)
	go notifier.Run(ctx)
	go h.Run(ctx)
	pushHandler := &push.Handler{Store: pushStore, PublicKey: keys.Public, Limiter: push.NewLimiter(), TrustProxy: trustProxy, Log: log}

	srv := &http.Server{
		Addr:              addr,
		Handler:           api.New(api.Options{Hub: h, Power: p, Feedback: fb, Push: pushHandler, StaticDir: os.Getenv("STATIC_DIR")}),
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

// feedbackRetention is how long feedback is kept; the privacy policy must match it.
const feedbackRetention = 365 * 24 * time.Hour

func pruneFeedback(ctx context.Context, store *feedback.Store, log *slog.Logger) {
	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		if n, err := store.Prune(ctx, feedbackRetention); err != nil {
			log.Warn("feedback prune failed", "err", err)
		} else if n > 0 {
			log.Info("feedback pruned", "deleted", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
