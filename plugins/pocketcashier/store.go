package pocketcashier

import "github.com/jmoiron/sqlx"

// Store is the plugin's own tables.
type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }
