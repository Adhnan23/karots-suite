package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/backup"
)

// Setup's two one-shot commands (uninstall, a move, a mode change): Setup never
// touches the shop's data itself — the installed program dumps it with -backup,
// the new one brings it back with -restore. Each prints one line Setup reads.

// backupTo writes one full snapshot into dir (tag "final", "before-move"…).
func backupTo(ctx context.Context, db *sqlx.DB, dir, tag string) (string, error) {
	return backup.WriteSnapshot(ctx, db, dir, time.Now(), tag)
}

// restoreFrom replaces the shop's data with the backup file. A bad file is
// refused before anything is touched.
func restoreFrom(ctx context.Context, db *sqlx.DB, file string) (backup.RestoreReport, error) {
	f, err := os.Open(file)
	if err != nil {
		return backup.RestoreReport{}, err
	}
	defer f.Close()
	arc, err := backup.ReadArchive(f)
	if err != nil {
		return backup.RestoreReport{}, err
	}
	return backup.RestoreArchive(ctx, db, arc)
}

// tillStopped refuses a server-mode -backup / -restore while the till is
// serving (file mode already claims the port in startFileMode).
func tillStopped() error {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3000"
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("the till is running (port %s) — stop it first", port)
	}
	return ln.Close()
}

func printRestore(rep backup.RestoreReport) {
	fmt.Printf("restored: %d rows\n", rep.Rows)
	if len(rep.SkippedTables) > 0 {
		fmt.Printf("skipped tables: %s\n", strings.Join(rep.SkippedTables, ", "))
	}
	if len(rep.DroppedColumns) > 0 {
		fmt.Printf("dropped columns: %s\n", strings.Join(rep.DroppedColumns, ", "))
	}
}
