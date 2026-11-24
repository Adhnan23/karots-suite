package catalogue

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
)

// Store is the plugin's tables.
type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// State is the copy's version and its last check.
type State struct {
	Version   int        `db:"version"    json:"version"`
	Count     int        `db:"count"      json:"count"`
	CheckedAt *time.Time `db:"checked_at" json:"checkedAt"`
	SyncedAt  *time.Time `db:"synced_at"  json:"syncedAt"`
	Error     string     `db:"error"      json:"error"`
}

func (s *Store) State(ctx context.Context) (State, error) {
	var st State
	err := s.db.GetContext(ctx, &st, `SELECT version, checked_at, synced_at, error,
		(SELECT count(*) FROM catalogue_items) AS count FROM catalogue_state WHERE id = 1`)
	return st, err
}
