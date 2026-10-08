package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/datacheck"
	"karots-suite/migrations"
)

// TestRestoreKarotsPosBackup restores a real karots-pos backup into a freshly
// migrated suite database and checks it adds up (spec §11 M6: "done when a
// karots-pos backup restores into karots-suite"). Opt-in, never in a normal run:
//
//	KAROTS_POS_BACKUP=~/shop.json.gz             a backup file from karots-pos, or
//	KAROTS_POS_DSN=postgres://…/pos_db           a live karots-pos database to dump first
//	KAROTS_POS_KEEP=1                            keep the restored DB (kpos_restore_db) to run the suite on it
//
// It checks every restored table has exactly the archive's rows, the next
// S- / CR- / DP- numbers follow the restored ones, and lists the data check.
func TestRestoreKarotsPosBackup(t *testing.T) {
	file, dsn := os.Getenv("KAROTS_POS_BACKUP"), os.Getenv("KAROTS_POS_DSN")
	if file == "" && dsn == "" {
		t.Skip("set KAROTS_POS_BACKUP or KAROTS_POS_DSN")
	}
	ctx := context.Background()
	var raw []byte
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		raw = b
	} else {
		src, err := appdb.Connect(dsn)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Dump(ctx, src, time.Now().Format(time.RFC3339), &buf, DumpOpts{}); err != nil {
			t.Fatal(err)
		}
		src.Close()
		raw = buf.Bytes()
		if out := os.Getenv("KAROTS_POS_SAVE"); out != "" {
			os.WriteFile(out, raw, 0o644) //nolint:errcheck
		}
	}
	arc, err := ReadArchive(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}

	var db = scratchDB(t, "kpos_restore_db")
	if os.Getenv("KAROTS_POS_KEEP") != "" {
		t.Cleanup(func() { t.Log("kept database kpos_restore_db") })
		keepScratch = true
	}
	if err := appdb.RunMigrations(db.DB, migrations.FS); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	rep, err := RestoreArchive(ctx, db, arc)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	t.Logf("restored %d rows in %d tables in %s", rep.Rows, rep.Tables, time.Since(start).Round(time.Millisecond))
	if len(rep.SkippedTables) > 0 {
		t.Logf("left out (not in the suite yet — plugins come in M7): %s", strings.Join(rep.SkippedTables, ", "))
	}
	if len(rep.DroppedColumns) > 0 {
		t.Errorf("columns the suite doesn't have: %s", strings.Join(rep.DroppedColumns, ", "))
	}

	have := map[string]bool{}
	tables, _ := publicTables(ctx, db)
	for _, n := range tables {
		have[n] = true
	}
	for _, tb := range arc.Tables {
		if !have[tb.Name] || skipTable(tb.Name) {
			continue
		}
		var n int
		if err := db.Get(&n, `SELECT count(*) FROM `+quoteIdent(tb.Name)); err != nil {
			t.Fatal(err)
		}
		if n != len(tb.Rows) {
			t.Errorf("%s: %d rows, the backup has %d", tb.Name, n, len(tb.Rows))
		}
	}
	for _, rs := range receiptSequences {
		var maxNo, next int64
		db.Get(&maxNo, fmt.Sprintf(`SELECT COALESCE(MAX(NULLIF(regexp_replace(%s, '\D', '', 'g'), '')::bigint), 0) FROM %s`, rs.Col, rs.Table)) //nolint:errcheck
		if err := db.Get(&next, `SELECT nextval($1)`, rs.Seq); err != nil {
			t.Fatal(err)
		}
		if next <= maxNo {
			t.Errorf("%s: next number %d would collide (highest restored %d)", rs.Seq, next, maxNo)
		}
	}
	found, err := datacheck.Run(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range found {
		t.Logf("data check: %s × %d", f.Title, f.Count)
	}
}
