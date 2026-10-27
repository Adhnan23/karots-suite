package main

import (
	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
)

// ensureSystemAdmin re-ensures the hidden recovery admin on every boot, adopting
// the install id this binary was built for (see app.EnsureSystemAdmin).
func ensureSystemAdmin(db *sqlx.DB) error { return app.EnsureSystemAdmin(db, installIDBaked) }
