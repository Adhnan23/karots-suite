// Package presence is the live "who's on now" view: every signed-in screen sends a
// heartbeat, and the admin reads who is active, idle, locked, away or signed out,
// where in the app and with what in the cart.
//
// ponytail: kept in memory — a restart forgets it and live screens refill it within
// one heartbeat (20 s). Move to a table only if history is ever wanted.
package presence

import (
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// AwayAfter: no heartbeat for this long = the browser closed, the PC is off or the network is down.
	AwayAfter = 60 * time.Second
	// IdleAfter: no input for this long = idle.
	IdleAfter = 2 * time.Minute
	// DropAfter: away / signed-out devices leave the list.
	DropAfter = 12 * time.Hour
)

// Beat is one heartbeat from a device.
type Beat struct {
	Device    string
	UserID    int64
	Name      string
	Role      string
	Locked    bool
	Path      string
	IdleSec   int
	CartItems float64
	CartTotal string
	Note      string
	IP        string
	UA        string
}

// Row is one device as the admin sees it.
type Row struct {
	Beat
	State    string // active | idle | locked | away | signedOut
	LastSeen time.Time
	Label    string // "Windows · Chrome"
}

type entry struct {
	b         Beat
	seen      time.Time
	signedOut bool
}

// Store holds the latest heartbeat per device.
type Store struct {
	mu sync.Mutex
	m  map[string]*entry
}

func New() *Store { return &Store{m: map[string]*entry{}} }

// Beat records a heartbeat (a new sign-in on the device replaces the old user). The
// same user on the same IP and browser under a new id (a private window, cleared
// storage) is the same machine: its old row goes.
func (s *Store) Beat(b Beat, now time.Time) {
	if b.Device == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.m {
		if k != b.Device && e.b.UserID == b.UserID && e.b.IP == b.IP && e.b.UA == b.UA {
			delete(s.m, k)
		}
	}
	s.m[b.Device] = &entry{b: b, seen: now}
}

// SignOut marks a device signed out (Sign out pressed there).
func (s *Store) SignOut(device string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[device]; e != nil {
		e.signedOut, e.seen = true, now
	}
}

// List is every device seen in the last 12 h: on-line ones first (by name, then active /
// idle / locked), then the rest, most recently seen first.
func (s *Store) List(now time.Time) []Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Row{}
	for k, e := range s.m {
		if now.Sub(e.seen) > DropAfter {
			delete(s.m, k)
			continue
		}
		out = append(out, Row{Beat: e.b, State: state(e, now), LastSeen: e.seen, Label: label(e.b.UA)})
	}
	on := func(r Row) bool { return r.State != "away" && r.State != "signedOut" }
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if on(a) != on(b) {
			return on(a)
		}
		if on(a) {
			if na, nb := strings.ToLower(a.Name), strings.ToLower(b.Name); na != nb {
				return na < nb
			}
			return rank[a.State] < rank[b.State] // one person's busiest device first (the Users page shows it)
		}
		return a.LastSeen.After(b.LastSeen)
	})
	return out
}

var rank = map[string]int{"active": 0, "idle": 1, "locked": 2}

func state(e *entry, now time.Time) string {
	switch {
	case e.signedOut:
		return "signedOut"
	case now.Sub(e.seen) > AwayAfter:
		return "away"
	case e.b.Locked:
		return "locked"
	case time.Duration(e.b.IdleSec)*time.Second >= IdleAfter:
		return "idle"
	}
	return "active"
}

// label is a short "OS · browser" from a User-Agent ("" when it says nothing useful).
func label(ua string) string {
	pick := func(pairs ...string) string {
		for i := 0; i < len(pairs); i += 2 {
			if strings.Contains(ua, pairs[i]) {
				return pairs[i+1]
			}
		}
		return ""
	}
	os := pick("Android", "Android", "iPhone", "iPhone", "iPad", "iPad", "Windows", "Windows", "Mac OS", "Mac", "CrOS", "ChromeOS", "Linux", "Linux")
	br := pick("Edg/", "Edge", "Firefox/", "Firefox", "Chrome/", "Chrome", "Chromium/", "Chromium", "Safari/", "Safari")
	switch {
	case os != "" && br != "":
		return os + " · " + br
	case os != "":
		return os
	}
	return br
}
