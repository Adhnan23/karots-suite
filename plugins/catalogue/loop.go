package catalogue

import (
	"context"
	"log"
	"time"
)

const (
	runHour    = 3 // the nightly check, 03:30 local
	runMinute  = 30
	staleAfter = 20 * time.Hour
)

// Loop checks a minute after boot when the last check is over 20 hours old (or never), then every night at
// 03:30, until ctx ends (plugin.Registry.AddBackground: the real server only).
func (s *Syncer) Loop(ctx context.Context) {
	wait := time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		var checked *time.Time
		_ = s.db.GetContext(ctx, &checked, `SELECT checked_at FROM catalogue_state WHERE id = 1`)
		if s.base != "" && (checked == nil || time.Since(*checked) > staleAfter) {
			if v, err := s.Sync(ctx); err != nil {
				log.Printf("catalogue: %v", err)
			} else {
				log.Printf("catalogue: version %d", v)
			}
		}
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), runHour, runMinute, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		wait = next.Sub(now)
	}
}
