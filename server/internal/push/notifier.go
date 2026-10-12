// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/hub"
)

// Message is the push payload; the service worker turns it into a notification.
// Texts are Ukrainian because they are shown to users.
type Message struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Region int    `json:"region"`
	// Tag makes a newer notification for the same region replace the older one.
	Tag string `json:"tag"`
	URL string `json:"url"`
}

// Sender delivers one encrypted push message. It returns the push service's HTTP status.
type Sender func(ctx context.Context, payload []byte, sub Subscription, topic string) (int, error)

// Notifier turns alert changes into push messages for the regions' subscribers.
type Notifier struct {
	store *Store
	send  Sender
	log   *slog.Logger
	now   func() time.Time
	queue chan event
}

type event struct {
	region int
	msg    Message
}

func NewNotifier(store *Store, send Sender, log *slog.Logger) *Notifier {
	return &Notifier{store: store, send: send, log: log, now: time.Now, queue: make(chan event, 256)}
}

// WebPushSender sends real Web Push messages signed with our VAPID keys.
// subject is a contact for push services: an https URL or an e-mail address.
func WebPushSender(keys Keys, subject string) Sender {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(ctx context.Context, payload []byte, sub Subscription, topic string) (int, error) {
		resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
		}, &webpush.Options{
			HTTPClient:      client,
			Subscriber:      subject,
			VAPIDPublicKey:  keys.Public,
			VAPIDPrivateKey: keys.Private,
			TTL:             15 * 60, // an alert notification older than 15 minutes is useless
			Urgency:         webpush.UrgencyHigh,
			Topic:           topic,
		})
		if err != nil {
			return 0, err
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}
}

// Changed is the hub's OnChange callback. It only queues work and never blocks polling.
func (n *Notifier) Changed(prev, next []hub.RegionState) {
	for _, e := range n.events(prev, next) {
		select {
		case n.queue <- e:
		default:
			n.log.Warn("push queue full, dropping", "region", e.region)
		}
	}
}

func (n *Notifier) events(prev, next []hub.RegionState) []event {
	was := map[int]hub.RegionState{}
	for _, r := range prev {
		was[r.UID] = r
	}
	is := map[int]bool{}
	var out []event
	for _, r := range next {
		is[r.UID] = true
		if _, ok := was[r.UID]; !ok {
			out = append(out, event{r.UID, startMessage(r)})
		}
	}
	for _, r := range prev {
		if !is[r.UID] {
			out = append(out, event{r.UID, endMessage(r, n.now())})
		}
	}
	return out
}

// Run sends queued messages until ctx is cancelled.
func (n *Notifier) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case e := <-n.queue:
			n.deliver(ctx, e)
		}
	}
}

func (n *Notifier) deliver(ctx context.Context, e event) {
	subs, err := n.store.ForRegion(ctx, e.region)
	if err != nil {
		n.log.Error("push: load subscribers", "region", e.region, "err", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	payload, _ := json.Marshal(e.msg)
	topic := fmt.Sprintf("region-%d", e.region) // collapses an unsent start+end into the latest
	out := make([]delivery, len(subs))
	for i, sub := range subs {
		out[i] = delivery{sub, payload, topic}
	}
	gone := sendAll(ctx, n.send, n.store, n.log, out)
	n.log.Info("push sent", "region", e.region, "title", e.msg.Title, "subscribers", len(subs), "expired", gone)
}

type delivery struct {
	sub     Subscription
	payload []byte
	topic   string
}

// sendAll sends the messages, 16 at a time, and deletes subscriptions the push service
// says are gone. It returns how many were deleted.
func sendAll(ctx context.Context, send Sender, store *Store, log *slog.Logger, ds []delivery) int {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	var gone []string
	var mu sync.Mutex
	for _, d := range ds {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			sctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			status, err := send(sctx, d.payload, d.sub, d.topic)
			switch {
			case err != nil:
				log.Warn("push send failed", "err", err)
			case status == http.StatusNotFound || status == http.StatusGone:
				// The browser unsubscribed or the subscription expired.
				mu.Lock()
				if !slices.Contains(gone, d.sub.Endpoint) {
					gone = append(gone, d.sub.Endpoint)
				}
				mu.Unlock()
			case status >= 400:
				log.Warn("push rejected", "status", status, "host", hostOf(d.sub.Endpoint))
			}
		}()
	}
	wg.Wait()
	for _, ep := range gone {
		if err := store.Delete(ctx, ep); err != nil {
			log.Warn("push: delete expired subscription", "err", err)
		}
	}
	return len(gone)
}

var alertTitles = map[string]string{
	"air_raid":           "Повітряна тривога",
	"artillery_shelling": "Загроза артобстрілу",
	"urban_fights":       "Загроза вуличних боїв",
	"chemical":           "Хімічна загроза",
	"nuclear":            "Радіаційна загроза",
}

func startMessage(r hub.RegionState) Message {
	title := "Тривога"
	if len(r.Alerts) > 0 {
		if t, ok := alertTitles[r.Alerts[0].Type]; ok {
			title = t
		}
	}
	body := alerts.Regions[r.UID]
	if r.Status == "partial" {
		var places []string
		for _, a := range r.Alerts {
			if a.LocationTitle != "" && !slices.Contains(places, a.LocationTitle) {
				places = append(places, a.LocationTitle)
			}
		}
		if len(places) > 0 {
			body += ": " + strings.Join(places, ", ")
		}
	}
	return Message{Title: "🔴 " + title, Body: body, Region: r.UID, Tag: tag(r.UID), URL: openURL(r.UID)}
}

func endMessage(r hub.RegionState, now time.Time) Message {
	body := alerts.Regions[r.UID]
	if len(r.Alerts) > 0 {
		start := r.Alerts[0].StartedAt
		for _, a := range r.Alerts {
			if a.StartedAt.Before(start) {
				start = a.StartedAt
			}
		}
		body += " · тривала " + duration(now.Sub(start))
	}
	return Message{Title: "🟢 Відбій тривоги", Body: body, Region: r.UID, Tag: tag(r.UID), URL: openURL(r.UID)}
}

func tag(region int) string     { return fmt.Sprintf("region-%d", region) }
func openURL(region int) string { return fmt.Sprintf("/?region=%d", region) }

// duration formats like the site: «40 хв», «1 год 5 хв», «3 дн 4 год».
func duration(d time.Duration) string {
	m := int(d.Minutes())
	if m < 1 {
		return "менше хвилини"
	}
	days, hours, mins := m/1440, m%1440/60, m%60
	switch {
	case days > 0 && hours > 0:
		return fmt.Sprintf("%d дн %d год", days, hours)
	case days > 0:
		return fmt.Sprintf("%d дн", days)
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%d год %d хв", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%d год", hours)
	default:
		return fmt.Sprintf("%d хв", mins)
	}
}

func hostOf(endpoint string) string {
	rest, _ := strings.CutPrefix(endpoint, "https://")
	host, _, _ := strings.Cut(rest, "/")
	return host
}
