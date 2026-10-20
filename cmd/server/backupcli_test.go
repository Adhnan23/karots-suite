package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"

	appdb "karots-suite/internal/db"
)

// scratchDB is an empty database of its own (DATABASE_URL's server), dropped after the test.
func scratchDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	drop := func() { admin.Exec(`DROP DATABASE IF EXISTS backupcli_db WITH (FORCE)`) }
	drop()
	if _, err := admin.Exec(`CREATE DATABASE backupcli_db`); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(dsn)
	u.Path = "/backupcli_db"
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); drop(); admin.Close() })
	return db
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	db := scratchDB(t)
	ctx := context.Background()
	db.MustExec(`CREATE TABLE things (id serial PRIMARY KEY, name text)`)
	db.MustExec(`INSERT INTO things (name) VALUES ('soap')`)

	path, err := backupTo(ctx, db, t.TempDir(), "final")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "-final.json.gz") {
		t.Fatalf("path %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	db.MustExec(`DELETE FROM things`)
	rep, err := restoreFrom(ctx, db, path)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	db.Get(&n, `SELECT count(*) FROM things WHERE name = 'soap'`)
	if rep.Rows == 0 || n != 1 {
		t.Fatalf("rows %d, soap %d", rep.Rows, n)
	}
}

func TestRestoreRefusesBadFile(t *testing.T) {
	db := scratchDB(t)
	db.MustExec(`CREATE TABLE things (id serial PRIMARY KEY, name text)`)
	db.MustExec(`INSERT INTO things (name) VALUES ('soap')`)
	bad := filepath.Join(t.TempDir(), "x.json.gz")
	os.WriteFile(bad, []byte("hello"), 0o644)
	if _, err := restoreFrom(context.Background(), db, bad); err == nil || !strings.Contains(err.Error(), "not a valid backup file") {
		t.Fatalf("err %v", err)
	}
	var n int
	db.Get(&n, `SELECT count(*) FROM things`)
	if n != 1 {
		t.Fatalf("data touched: %d rows", n)
	}
}
