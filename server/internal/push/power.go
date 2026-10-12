// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"alert-concept/server/internal/alerts"
	"alert-concept/server/internal/power"
)

// PowerRule asks for warnings about one queue's planned outages.
type PowerRule struct {
	Region int    `json:"region"`
	Queue  string `json:"queue"`
	// BeforeOff: minutes before an outage starts.
	BeforeOff int `json:"beforeOff"`
	// BeforeOn: minutes before power is due back; 0 is "when it is due back", -1 is never.
	BeforeOn int `json:"beforeOn"`
}

const (
	maxBeforeOff = 180
	maxBeforeOn  = 120
	// onGrace is how long after power is due back a "power is back" message is still sent
	// (the server may have been down at that minute).
	onGrace = 10 * time.Minute
	// sentRetention: an event older than this can't come up again (schedules span two days).
	sentRetention = 3 * 24 * time.Hour
)

var queuePattern = regexp.MustCompile(`^[0-9]{1,2}\.[0-9]$`)

// Schedules gives power schedules; *power.Service implements it.
type Schedules interface {
	Supported() []int
	Get(uid int) (power.Schedule, error)
}

// checkRule validates a rule against the regions and queues the server knows.
func checkRule(r PowerRule, s Schedules) error {
	if s == nil {
		return fmt.Errorf("power schedules are off")
	}
	if !queuePattern.MatchString(r.Queue) {
		return fmt.Errorf("bad queue")
	}
	if r.BeforeOff < 1 || r.BeforeOff > maxBeforeOff || r.BeforeOn < -1 || r.BeforeOn > maxBeforeOn {
		return fmt.Errorf("bad times")
	}
	sch, err := s.Get(r.Region)
	switch err {
	case nil:
		for _, q := range sch.Queues {
			if q == r.Queue {
				return nil
			}
		}
		return fmt.Errorf("bad queue")
	case power.ErrNoData:
		return nil // just restarted; the pattern check has to do
	default:
		return fmt.Errorf("bad region")
	}
}

// span is one outage in absolute time; outages that touch across midnight are joined.
type span struct{ start, end time.Time }

var kyiv = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		panic(err)
	}
	return loc
}()

// spans returns the definite outages of a queue for today and tomorrow.
func spans(sch power.Schedule, queue string) []span {
	var out []span
	for _, day := range []power.Day{sch.Today, sch.Tomorrow} {
		if day.Status != power.StatusSchedule {
			continue
		}
		d, err := time.ParseInLocation(time.DateOnly, day.Date, kyiv)
		if err != nil {
			continue
		}
		for _, o := range day.Queues[queue] {
			if o.Possible {
				continue
			}
			// time.Date normalizes minutes past midnight by the wall clock, so DST days work.
			s := span{
				time.Date(d.Year(), d.Month(), d.Day(), 0, o.From, 0, 0, kyiv),
				time.Date(d.Year(), d.Month(), d.Day(), 0, o.To, 0, 0, kyiv),
			}
			if n := len(out); n > 0 && !s.start.After(out[n-1].end) {
				out[n-1].end = s.end
				continue
			}
			out = append(out, s)
		}
	}
	return out
}

// due returns the message a rule asks for about an outage at now, with an ID that is the
// same every time it is due, so it is sent once.
func due(r PowerRule, s span, now time.Time) (Message, string, bool) {
	where := fmt.Sprintf("%s · черга %s", alerts.Regions[r.Region], r.Queue)
	m := Message{Region: r.Region, Tag: fmt.Sprintf("power-%d", r.Region), URL: openURL(r.Region)}

	if now.Before(s.start) && s.start.Sub(now) <= time.Duration(r.BeforeOff)*time.Minute {
		m.Title = "🔌 Відключення світла о " + clock(s.start)
		m.Body = where + " · до " + clock(s.end)
		return m, fmt.Sprintf("off:%d:%s:%d", r.Region, r.Queue, s.start.Unix()), true
	}
	if r.BeforeOn < 0 || now.Before(s.start) {
		return Message{}, "", false
	}
	lo, hi := s.end.Add(-time.Duration(r.BeforeOn)*time.Minute), s.end
	if r.BeforeOn == 0 {
		hi = s.end.Add(onGrace)
	}
	if now.Before(lo) || !now.Before(hi) {
		return Message{}, "", false
	}
	if r.BeforeOn == 0 {
		m.Title = "💡 Світло мають увімкнути"
		m.Body = where + " · за графіком о " + clock(s.end)
	} else {
		m.Title = "💡 Світло увімкнуть о " + clock(s.end)
		m.Body = where
	}
	return m, fmt.Sprintf("on:%d:%s:%d", r.Region, r.Queue, s.end.Unix()), true
}

func clock(t time.Time) string { return t.In(kyiv).Format("15:04") }

// PowerNotifier warns subscribers before their queue's outages start and end.
type PowerNotifier struct {
	store     *Store
	schedules Schedules
	send      Sender
	log       *slog.Logger
	now       func() time.Time
	pruned    time.Time
}

func NewPowerNotifier(store *Store, schedules Schedules, send Sender, log *slog.Logger) *PowerNotifier {
	return &PowerNotifier{store: store, schedules: schedules, send: send, log: log, now: time.Now}
}

// Run checks schedules every 30 seconds until ctx is cancelled.
func (n *PowerNotifier) Run(ctx context.Context) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		n.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (n *PowerNotifier) tick(ctx context.Context) {
	now := n.now()
	var out []delivery
	for _, uid := range n.schedules.Supported() {
		sch, err := n.schedules.Get(uid)
		if err != nil {
			continue
		}
		for _, queue := range sch.Queues {
			ss := spans(sch, queue)
			if !anySoon(ss, now) {
				continue // most queues most of the time: no database query
			}
			subs, err := n.store.PowerFor(ctx, uid, queue)
			if err != nil {
				n.log.Error("push: load power subscribers", "region", uid, "err", err)
				continue
			}
			for _, ps := range subs {
				for _, s := range ss {
					msg, id, ok := due(ps.rule, s, now)
					if !ok {
						continue
					}
					first, err := n.store.MarkSent(ctx, ps.sub.Endpoint, id)
					if err != nil {
						n.log.Error("push: mark sent", "err", err)
						continue
					}
					if first {
						payload, _ := json.Marshal(msg)
						out = append(out, delivery{ps.sub, payload, fmt.Sprintf("power-%d", uid)})
					}
				}
			}
		}
	}
	if len(out) > 0 {
		gone := sendAll(ctx, n.send, n.store, n.log, out)
		n.log.Info("power push sent", "messages", len(out), "expired", gone)
	}
	if now.Sub(n.pruned) > time.Hour {
		n.pruned = now
		if err := n.store.PruneSent(ctx, now.Add(-sentRetention)); err != nil {
			n.log.Warn("push: prune sent", "err", err)
		}
	}
}

// anySoon reports whether any rule could be due for these outages now.
func anySoon(ss []span, now time.Time) bool {
	for _, s := range ss {
		if d := s.start.Sub(now); d > 0 && d <= maxBeforeOff*time.Minute {
			return true
		}
		if d := s.end.Sub(now); d > -onGrace && d <= maxBeforeOn*time.Minute {
			return true
		}
	}
	return false
}
