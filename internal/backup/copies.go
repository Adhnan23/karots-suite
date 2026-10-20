package backup

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/config"

	"github.com/jmoiron/sqlx"
)

// Copy kinds: the owner's copy folder and the vendor's Karots cloud.
const (
	KindFolder = "folder"
	KindCloud  = "cloud"
)

// Copy is one place this PC copies its automatic backups to (backup_copies).
type Copy struct {
	Kind         string     `db:"kind"`
	Enabled      bool       `db:"enabled"`
	Path         string     `db:"path"`
	Preset       string     `db:"preset"`
	LastCopiedAt *time.Time `db:"last_copied_at"`
	LastFile     string     `db:"last_file"`
	LastError    string     `db:"last_error"`
	LastTriedAt  *time.Time `db:"last_tried_at"`
}

// SyncCopies brings backup_copies in line with .env at start-up: a new copy
// folder preset is seeded (the same one again is not — the owner may have
// cleared it), and the cloud row exists exactly when this build carries Karots
// cloud (created with its initial switch; after that the row's switch counts).
func SyncCopies(ctx context.Context, db *sqlx.DB, copyDir string, cloud config.CloudBackup) error {
	if copyDir != "" {
		if _, err := db.ExecContext(ctx, `INSERT INTO backup_copies (kind, enabled, path, preset) VALUES ('folder', true, $1, $1)
			ON CONFLICT (kind) DO UPDATE SET enabled = true, path = $1, preset = $1, last_error = ''
			WHERE backup_copies.preset <> $1`, copyDir); err != nil {
			return err
		}
	}
	if !cloud.Set() {
		_, err := db.ExecContext(ctx, `DELETE FROM backup_copies WHERE kind = 'cloud'`)
		return err
	}
	_, err := db.ExecContext(ctx, `INSERT INTO backup_copies (kind, enabled) VALUES ('cloud', $1) ON CONFLICT (kind) DO NOTHING`, cloud.On)
	return err
}

// Copies lists the copy places, folder first.
func Copies(ctx context.Context, db *sqlx.DB) ([]Copy, error) {
	out := []Copy{}
	err := db.SelectContext(ctx, &out, `SELECT * FROM backup_copies ORDER BY kind DESC`)
	return out, err
}

// CopyOf is one place, nil when it isn't set up.
func CopyOf(ctx context.Context, db *sqlx.DB, kind string) (*Copy, error) {
	var c Copy
	err := db.GetContext(ctx, &c, `SELECT * FROM backup_copies WHERE kind = $1`, kind)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &c, err
}

// SaveFolder sets the copy folder (the Data page); its status starts afresh.
func SaveFolder(ctx context.Context, db *sqlx.DB, path string, enabled bool) error {
	_, err := db.ExecContext(ctx, `INSERT INTO backup_copies (kind, enabled, path) VALUES ('folder', $2, $1)
		ON CONFLICT (kind) DO UPDATE SET enabled = $2, path = $1,
		last_error = CASE WHEN backup_copies.path = $1 THEN backup_copies.last_error ELSE '' END,
		last_copied_at = CASE WHEN backup_copies.path = $1 THEN backup_copies.last_copied_at END,
		last_file = CASE WHEN backup_copies.path = $1 THEN backup_copies.last_file ELSE '' END`, path, enabled)
	return err
}

// ClearFolder forgets the copy folder; the preset stays remembered so it isn't re-seeded.
func ClearFolder(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `UPDATE backup_copies SET path = '', enabled = false, last_error = '',
		last_copied_at = NULL, last_file = '', last_tried_at = NULL WHERE kind = 'folder'`)
	return err
}

// SetEnabled switches a place on or off (who may do which is the API's call).
func SetEnabled(ctx context.Context, db *sqlx.DB, kind string, on bool) error {
	_, err := db.ExecContext(ctx, `UPDATE backup_copies SET enabled = $2 WHERE kind = $1`, kind, on)
	return err
}

// copyTried stores how a copy went (err nil = copied file at now).
func copyTried(ctx context.Context, db *sqlx.DB, kind, file string, now time.Time, err error) {
	if err != nil {
		db.ExecContext(ctx, `UPDATE backup_copies SET last_tried_at = $2, last_error = $3 WHERE kind = $1`, kind, now, err.Error()) //nolint:errcheck // status only
		return
	}
	db.ExecContext(ctx, `UPDATE backup_copies SET last_tried_at = $2, last_copied_at = $2, last_error = '',
		last_file = CASE WHEN $3 = '' THEN last_file ELSE $3 END WHERE kind = $1`, kind, now, file) //nolint:errcheck // status only
}
