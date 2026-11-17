// Package screens keeps the screen sizes the shop's tills have reported from the
// Sell page, with who used them, so Appearance can set a Sell page layout per size.
package screens

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	every    = time.Hour           // a device rewrites its size at most this often, unless it changes
	keepFor  = 90 * 24 * time.Hour // listed while seen within this
	maxNames = 6
	maxSizes = 32 // listed (the server keeps as many layouts)
	// maxDevices bounds the throttle's memory; past it, it starts over (a few extra writes).
	maxDevices = 500
)

// Size is one screen size and the last few people seen on it.
type Size struct {
	W        int            `db:"w"`
	H        int            `db:"h"`
	Users    pq.StringArray `db:"users"`
	LastSeen time.Time      `db:"last_seen"`
}

type seen struct {
	w, h int
	at   time.Time
}

type Service struct {
	db   *sqlx.DB
	mu   sync.Mutex
	last map[string]seen // device → what it last wrote
}

func New(db *sqlx.DB) *Service { return &Service{db: db, last: map[string]seen{}} }

// Seen records that device showed the Sell page at w × h for name. Best effort: a
// failure is logged, never returned (a heartbeat must not fail over it).
func (s *Service) Seen(ctx context.Context, device string, w, h int, name string, now time.Time) {
	if w < 300 || h < 300 || w > 8000 || h > 8000 {
		return
	}
	s.mu.Lock()
	p, ok := s.last[device]
	if ok && p.w == w && p.h == h && now.Sub(p.at) < every {
		s.mu.Unlock()
		return
	}
	if len(s.last) >= maxDevices {
		s.last = map[string]seen{}
	}
	s.last[device] = seen{w, h, now}
	s.mu.Unlock()
	if err := s.write(ctx, w, h, name, now); err != nil {
		log.Printf("screens: %v", err)
	}
}

// ponytail: two devices first seen at a new size in the same instant can keep only one
// name; the next hourly write adds the other.
func (s *Service) write(ctx context.Context, w, h int, name string, now time.Time) error {
	var users pq.StringArray
	err := s.db.GetContext(ctx, &users, `SELECT users FROM screen_sizes WHERE w = $1 AND h = $2`, w, h)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO screen_sizes (w, h, users, last_seen) VALUES ($1, $2, $3, $4)
		ON CONFLICT (w, h) DO UPDATE SET users = EXCLUDED.users, last_seen = EXCLUDED.last_seen`,
		w, h, pq.StringArray(addName(users, name)), now)
	return err
}

// List is the sizes seen in the last 90 days, the 32 most recent first.
func (s *Service) List(ctx context.Context, now time.Time) ([]Size, error) {
	out := []Size{}
	err := s.db.SelectContext(ctx, &out, `SELECT w, h, users, last_seen FROM screen_sizes WHERE last_seen >= $1 ORDER BY last_seen DESC LIMIT $2`, now.Add(-keepFor), maxSizes)
	return out, err
}

// addName puts name last (once), keeping the last six.
func addName(names []string, name string) []string {
	if name == "" || slices.Contains(names, name) {
		return names
	}
	names = append(names, name)
	if len(names) > maxNames {
		names = names[len(names)-maxNames:]
	}
	return names
}
