package ltest

import (
	"bytes"
	"context"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/backup"
)

// Restored is the install area's auditor (docs/rules/install.md): a full backup
// of db restored into a fresh database must load and pass the rule book there
// (BAK-1). It returns the copy.
func Restored(t T, db *sqlx.DB) *sqlx.DB {
	helper(t)
	ctx := context.Background()
	var buf bytes.Buffer
	if err := backup.Dump(ctx, db, time.Now().Format(time.RFC3339), &buf, backup.DumpOpts{}); err != nil {
		t.Fatalf("BAK-1: backup: %v", err)
	}
	cp := NewDB(t)
	if _, err := backup.Restore(ctx, cp, &buf); err != nil {
		t.Fatalf("BAK-1: the backup doesn't restore: %v", err)
	}
	Check(t, ctx, cp)
	return cp
}

// RoundTrip is Restored for a shop at rest: every table comes back row for row
// (BAK-2; backup_copies describes the PC and never travels, BAK-8).
func RoundTrip(t T, db *sqlx.DB) *sqlx.DB {
	helper(t)
	cp := Restored(t, db)
	before, after := State(t, db), State(t, cp)
	delete(before, "backup_copies")
	delete(after, "backup_copies")
	for _, tb := range Changed(before, after) {
		t.Errorf("BAK-2: %s differs after a backup and restore", tb)
	}
	return cp
}
