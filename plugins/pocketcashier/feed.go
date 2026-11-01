package pocketcashier

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// The live feed: phones keep an EventSource open and refresh their copy when a
// "version" event arrives, so a change on the PC reaches them in seconds.
// ponytail: polls a fingerprint every 2 s; a core change bus if many phones or big shops need it

var feedEvery = 2 * time.Second

// feedPing keeps the stream open and the phone "seen" (well inside unreachableAfter).
var feedPing = 20 * time.Second

// An EventSource can't send headers, and URLs end up in logs: the phone trades
// its key for a one-time ticket that opens the feed once, within a minute.
type feedTickets struct {
	mu sync.Mutex
	m  map[string]feedTicket
}

type feedTicket struct {
	secret string
	until  time.Time
}

func (f *feedTickets) issue(secret string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	tk := hex.EncodeToString(b)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.m == nil {
		f.m = map[string]feedTicket{}
	}
	now := time.Now()
	for k, v := range f.m { // drop the stale ones as we go
		if now.After(v.until) {
			delete(f.m, k)
		}
	}
	f.m[tk] = feedTicket{secret: secret, until: now.Add(time.Minute)}
	return tk
}

// take spends a ticket: the phone's key, or "" when unknown, used or late.
func (f *feedTickets) take(tk string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.m[tk]
	delete(f.m, tk)
	if !ok || time.Now().After(t.until) {
		return ""
	}
	return t.secret
}

func (h handlers) feedTicket(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"ticket": h.p.tickets.issue(c.Request().Header.Get(phoneHeader))})
}

// feedAuth turns ?ticket= into the phone's key, then phoneAuth checks it as usual.
func (h handlers) feedAuth(next echo.HandlerFunc) echo.HandlerFunc {
	auth := h.phoneAuth(next)
	return func(c echo.Context) error {
		c.Request().Header.Set(phoneHeader, h.p.tickets.take(c.QueryParam("ticket")))
		return auth(c)
	}
}

// feedHub is one shared poller for every open feed, running while anyone listens.
type feedHub struct {
	version func(context.Context) (string, error)

	mu   sync.Mutex
	subs map[chan string]struct{}
	stop context.CancelFunc
}

func (f *feedHub) subscribe() chan string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := make(chan string, 1)
	if f.subs == nil {
		f.subs = map[chan string]struct{}{}
	}
	f.subs[ch] = struct{}{}
	if f.stop == nil {
		ctx, cancel := context.WithCancel(context.Background())
		f.stop = cancel
		go f.poll(ctx)
	}
	return ch
}

func (f *feedHub) unsubscribe(ch chan string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.subs, ch)
	if len(f.subs) == 0 && f.stop != nil {
		f.stop()
		f.stop = nil
	}
}

func (f *feedHub) poll(ctx context.Context) {
	last, _ := f.version(ctx)
	t := time.NewTicker(feedEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		v, err := f.version(ctx)
		if err != nil || v == last {
			continue
		}
		last = v
		f.mu.Lock()
		for ch := range f.subs {
			select { // a slow phone just gets the newest one later
			case ch <- v:
			default:
			}
		}
		f.mu.Unlock()
	}
}

func (h handlers) feed(c echo.Context) error {
	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	w.Flush()
	ch := h.p.feed.subscribe()
	defer h.p.feed.unsubscribe(ch)
	ping := time.NewTicker(feedPing)
	defer ping.Stop()
	for {
		select {
		case <-c.Request().Context().Done():
			return nil
		case v := <-ch:
			fmt.Fprintf(w, "event: version\ndata: %s\n\n", v)
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			if p := phoneFrom(c); p != nil {
				_, _ = h.p.core.DB.ExecContext(c.Request().Context(), `UPDATE pocket_phones SET last_seen = now() WHERE id = $1`, p.ID)
			}
		}
		w.Flush()
	}
}
