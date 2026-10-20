package backup

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"karots-suite/internal/config"
	appdb "karots-suite/internal/db"
	"karots-suite/migrations"

	"github.com/jmoiron/sqlx"
)

// copiesDB is the dev database, migrated, with backup_copies emptied (before and after).
func copiesDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := appdb.RunMigrations(db.DB, migrations.FS); err != nil {
		t.Fatal(err)
	}
	db.MustExec(`DELETE FROM backup_copies`)
	t.Cleanup(func() { db.MustExec(`DELETE FROM backup_copies`); db.Close() })
	return db
}

func copyRow(t *testing.T, db *sqlx.DB, kind string) *Copy {
	t.Helper()
	all, err := Copies(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	for i := range all {
		if all[i].Kind == kind {
			return &all[i]
		}
	}
	return nil
}

var cloudOn = config.CloudBackup{URL: "https://x/b", KeyID: "k", Secret: "s", Folder: "shop", Crypt: testKeyHex, On: true}

func TestSyncCopiesFolderPreset(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(SyncCopies(ctx, db, "/media/usb", config.CloudBackup{}))
	if f := copyRow(t, db, "folder"); f == nil || f.Path != "/media/usb" || !f.Enabled {
		t.Fatalf("seeded folder = %+v", f)
	}
	must(ClearFolder(ctx, db))
	must(SyncCopies(ctx, db, "/media/usb", config.CloudBackup{}))
	if f := copyRow(t, db, "folder"); f != nil && f.Path != "" {
		t.Fatalf("cleared folder came back: %+v", f)
	}
	must(SyncCopies(ctx, db, "/mnt/share", config.CloudBackup{}))
	if f := copyRow(t, db, "folder"); f == nil || f.Path != "/mnt/share" || !f.Enabled {
		t.Fatalf("new preset = %+v", f)
	}
	must(SaveFolder(ctx, db, "/media/other", false))
	must(SyncCopies(ctx, db, "/mnt/share", config.CloudBackup{}))
	if f := copyRow(t, db, "folder"); f.Path != "/media/other" || f.Enabled {
		t.Fatalf("owner's edit lost: %+v", f)
	}
}

func TestSyncCopiesCloud(t *testing.T) {
	db, ctx := copiesDB(t), context.Background()
	off := cloudOn
	off.On = false
	if err := SyncCopies(ctx, db, "", off); err != nil {
		t.Fatal(err)
	}
	if c := copyRow(t, db, "cloud"); c == nil || c.Enabled {
		t.Fatalf("cloud off = %+v", c)
	}
	if err := SetEnabled(ctx, db, "cloud", true); err != nil {
		t.Fatal(err)
	}
	SyncCopies(ctx, db, "", off) // the row's switch counts after creation
	if c := copyRow(t, db, "cloud"); !c.Enabled {
		t.Fatal("Sync overrode the cloud switch")
	}
	SyncCopies(ctx, db, "", config.CloudBackup{})
	if c := copyRow(t, db, "cloud"); c != nil {
		t.Fatalf("cloud row kept without credentials: %+v", c)
	}
	SyncCopies(ctx, db, "", cloudOn)
	if c := copyRow(t, db, "cloud"); c == nil || !c.Enabled {
		t.Fatalf("cloud on = %+v", c)
	}
}

func TestBackupCopiesNeverDumped(t *testing.T) {
	db := copiesDB(t)
	if !skipTable("backup_copies") {
		t.Fatal("skipTable(backup_copies) = false")
	}
	SyncCopies(context.Background(), db, "/media/usb", cloudOn)
	var buf bytes.Buffer
	if err := Dump(context.Background(), db, "now", &buf, DumpOpts{}); err != nil {
		t.Fatal(err)
	}
	arc, err := ReadArchive(&buf)
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range arc.Tables {
		if strings.Contains(tb.Name, "backup_copies") {
			t.Fatal("backup_copies is in the dump")
		}
	}
}
