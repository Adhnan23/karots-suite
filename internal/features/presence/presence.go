// Package presence is the live "who's on now" view: every signed-in screen sends a
// heartbeat, and the admin reads who is active, idle, locked, away or signed out,
// where in the app and with what in the cart.
//
// ponytail: kept in memory — a restart forgets it and live screens refill it within
// one heartbeat (20 s). Move to a table only if history is ever wanted.
package presence

import (
	"errors"
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
	// ServingWithin: a cart that changed this recently is a sale going on — never locked from afar.
	ServingWithin = 5 * time.Minute
	// UntouchedAfter: a cart left as it is this long is flagged (an item parked to look busy).
	UntouchedAfter = 10 * time.Minute
)

var (
	ErrGone    = errors.New("that screen isn't on any more")
	ErrServing = errors.New("they're serving a customer — the cart changed in the last few minutes")
	ErrNotOpen = errors.New("that screen is already locked or away")
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
	// Serving: active or idle with a cart that changed in the last 5 min (no remote lock).
	Serving bool
	// UntouchedMin: minutes the cart has sat unchanged, once it's 10 or more (else 0).
	UntouchedMin int
	// LockPending: an admin asked to lock it; it locks on its next heartbeat.
	LockPending bool
}

type entry struct {
	b         Beat
	seen      time.Time
	signedOut bool
	cartAt    time.Time // when the cart last changed (items or total)
	lockAsked bool
}

// Store holds the latest heartbeat per device.
type Store struct {
	mu sync.Mutex
	m  map[string]*entry
}

func New() *Store { return &Store{m: map[string]*entry{}} }

// Beat records a heartbeat (a new sign-in on the device replaces the old user). The
// same user on the same IP and browser under a new id (a private window, cleared
// storage) is the same machine: its old row goes. lock = an admin asked this screen
// to lock (asked again on every beat until a locked one arrives).
func (s *Store) Beat(b Beat, now time.Time) (lock bool) {
	if b.Device == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.m {
		if k != b.Device && e.b.UserID == b.UserID && e.b.IP == b.IP && e.b.UA == b.UA {
			delete(s.m, k)
		}
	}
	e := &entry{b: b, seen: now, cartAt: now}
	if prev := s.m[b.Device]; prev != nil && prev.b.UserID == b.UserID && !prev.signedOut {
		if prev.b.CartItems == b.CartItems && prev.b.CartTotal == b.CartTotal {
			e.cartAt = prev.cartAt
		}
		e.lockAsked = prev.lockAsked && !b.Locked
	}
	s.m[b.Device] = e
	return e.lockAsked
}

// Lock asks a device to lock itself on its next heartbeat. Refused while that screen is
// serving (never stop a sale) or already locked / away. Returns who is on it.
func (s *Store) Lock(device string, now time.Time) (Beat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[device]
	if e == nil {
		return Beat{}, ErrGone
	}
	r := row(e, now)
	switch {
	case r.State != "active" && r.State != "idle":
		return e.b, ErrNotOpen
	case r.Serving:
		return e.b, ErrServing
	}
	e.lockAsked = true
	return e.b, nil
}

// Serving reports whether any screen of userID is serving a customer right now (cart
// changed in the last 5 min) — a till closed for them must wait.
func (s *Store) Serving(userID int64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.m {
		if e.b.UserID == userID && row(e, now).Serving {
			return true
		}
	}
	return false
}

func row(e *entry, now time.Time) Row {
	r := Row{Beat: e.b, State: state(e, now), LastSeen: e.seen, Label: label(e.b.UA), LockPending: e.lockAsked}
	if e.b.CartItems > 0 {
		still := now.Sub(e.cartAt)
		r.Serving = (r.State == "active" || r.State == "idle") && still < ServingWithin
		if still >= UntouchedAfter {
			r.UntouchedMin = int(still / time.Minute)
		}
	}
	return r
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
		out = append(out, row(e, now))
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
