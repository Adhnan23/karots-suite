package pocketcashier

import (
	"context"
	"time"
)

// unreachableAfter: a phone not heard from this long can't hand its person
// back, so they stay off the PC (they may be selling offline on it).
const unreachableAfter = 60 * time.Second

// takeoverGuard keeps a person off another device while the phone they are on
// is out of reach; an admin releases a lost or dead phone in Phones. Fails open.
func (s *Store) takeoverGuard(ctx context.Context, userID int64, active string) (bool, string) {
	var row struct {
		Phone    string     `db:"phone"`
		Person   string     `db:"person"`
		LastSeen *time.Time `db:"last_seen"`
	}
	if err := s.db.GetContext(ctx, &row, `SELECT p.name AS phone, u.name AS person, p.last_seen
		FROM pocket_phones p JOIN users u ON u.id = $1 WHERE p.device = $2 AND p.status = 'active'`, userID, active); err != nil {
		return false, "" // not one of our phones (or a read error): don't hold anyone
	}
	if row.LastSeen != nil && time.Since(*row.LastSeen) < unreachableAfter {
		return false, ""
	}
	return true, row.Person + " is signed in on " + row.Phone + ", which isn't reachable. An admin can release it in Phones."
}
