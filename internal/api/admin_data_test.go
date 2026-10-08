package api

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The data endpoints: admin only, a backup is a gzip, file names are never
// paths, and a bad restore changes nothing but still leaves the safety copy.
// (A successful restore wipes the shared test DB, so it is proven on scratch
// databases in internal/backup and end to end in admin-data.spec.ts.)
func TestAdminData(t *testing.T) {
	te := newTestEnv(t)
	te.cfg.BackupDir = t.TempDir()
	ph := func(n int64) string { return fmt.Sprintf("07%08d", (time.Now().UnixNano()+n)%100_000_000) }

	mgr := te.login(t, ph(1), "manager")
	for _, p := range []string{"/api/v1/admin/data", "/api/v1/admin/data/backup"} {
		if rec := te.do(http.MethodGet, p, nil, mgr); rec.Code != http.StatusForbidden {
			t.Fatalf("manager on %s: %d", p, rec.Code)
		}
	}
	if rec := te.upload("/api/v1/admin/data/restore", "backup", "x.json.gz", []byte("x"), mgr); rec.Code != http.StatusForbidden {
		t.Fatalf("manager restore: %d", rec.Code)
	}

	ck := te.login(t, ph(2), "admin")
	rec := te.do(http.MethodGet, "/api/v1/admin/data/backup?setupOnly=1", nil, ck)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "-partial.json.gz") {
		t.Fatalf("setup-only backup: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
	if _, err := gzip.NewReader(rec.Body); err != nil {
		t.Fatalf("backup is not gzip: %v", err)
	}

	var products int
	te.db.Get(&products, `SELECT count(*) FROM products`) //nolint:errcheck
	rec = te.upload("/api/v1/admin/data/restore", "backup", "bad.json.gz", []byte("not a backup"), ck)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad restore: %d %s", rec.Code, rec.Body)
	}
	for _, c := range rec.Result().Cookies() {
		if c.MaxAge < 0 {
			t.Fatal("a failed restore ended the session")
		}
	}
	var after int
	te.db.Get(&after, `SELECT count(*) FROM products`) //nolint:errcheck
	if after < products {
		t.Fatal("a failed restore removed products")
	}

	// A bad file is refused before the safety copy: nothing is written.
	if page := decode[DataPage](t, te.do(http.MethodGet, "/api/v1/admin/data", nil, ck)); len(page.Backups) != 0 {
		t.Fatalf("a bad file left a copy: %+v", page.Backups)
	}
	// A good file that breaks on load (a row pointing at no unit) rolls back,
	// after the safety copy.
	var arc bytes.Buffer
	gw := gzip.NewWriter(&arc)
	gw.Write([]byte(`{"version":1,"tables":[{"name":"products","columns":["id","name","unit_id"],"rows":[["999999","x","999999"]]}]}`)) //nolint:errcheck
	gw.Close()
	if rec := te.upload("/api/v1/admin/data/restore", "backup", "fk.json.gz", arc.Bytes(), ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("fk restore: %d %s", rec.Code, rec.Body)
	}
	te.db.Get(&after, `SELECT count(*) FROM products`) //nolint:errcheck
	if after < products {
		t.Fatal("a failed restore removed products")
	}
	page := decode[DataPage](t, te.do(http.MethodGet, "/api/v1/admin/data", nil, ck))
	if len(page.Backups) != 1 || !page.Backups[0].BeforeRestore || !page.AutoBackups {
		t.Fatalf("data page: %+v", page)
	}
	name := page.Backups[0].Name
	if _, err := os.Stat(filepath.Join(te.cfg.BackupDir, name)); err != nil {
		t.Fatal(err)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/data/backups/"+name, nil, ck); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("download: %d", rec.Code)
	}
	for _, bad := range []string{"..%2Fsecret.json.gz", "%2Fetc%2Fpasswd", "x.txt"} {
		if rec := te.do(http.MethodGet, "/api/v1/admin/data/backups/"+bad, nil, ck); rec.Code != http.StatusBadRequest {
			t.Errorf("download %s: %d", bad, rec.Code)
		}
		if rec := te.do(http.MethodPost, "/api/v1/admin/data/backups/"+bad+"/restore", nil, ck); rec.Code != http.StatusBadRequest {
			t.Errorf("restore %s: %d", bad, rec.Code)
		}
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/data/backups/missing.json.gz", nil, ck); rec.Code != http.StatusNotFound {
		t.Errorf("missing file: %d", rec.Code)
	}
}

func TestAdminDataCheck(t *testing.T) {
	te := newTestEnv(t)
	ph := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	rec := te.do(http.MethodGet, "/api/v1/admin/data/check", nil, te.login(t, ph, "admin"))
	if rec.Code != http.StatusOK {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	for _, f := range decode[[]CheckFinding](t, rec) {
		t.Logf("dev DB finding: %s × %d", f.Key, f.Count)
	}
}

// A settings sheet carries the shop's look and rules, never its identity or
// hardware; an unknown key is reported, not applied.
func TestAdminSettingsSheet(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var before struct {
		InstallID string  `db:"install_id"`
		Footer    *string `db:"receipt_footer"`
	}
	te.db.Get(&before, `SELECT install_id, receipt_footer FROM settings ORDER BY id LIMIT 1`) //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`UPDATE settings SET receipt_footer=$1`, before.Footer) })  //nolint:errcheck

	rec := te.do(http.MethodGet, "/api/v1/admin/data/settings/export", nil, ck)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "install_id,"+before.InstallID+",no") {
		t.Fatalf("export: %d %.200s", rec.Code, rec.Body)
	}
	sheet := "Key,Value\ninstall_id,DEADBEEF\nreceipt_footer,Thank you — sheet\nno_such_setting,1\nreceipt_printer,tcp://x:9100\n"
	rec = te.upload("/api/v1/admin/data/settings/import", "file", "s.csv", []byte(sheet), ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body)
	}
	res := decode[SettingsImportResult](t, rec)
	if res.Applied != 1 || len(res.Unknown) != 1 || len(res.Skipped) != 2 {
		t.Fatalf("result: %+v", res)
	}
	var id, footer string
	te.db.QueryRow(`SELECT install_id, receipt_footer FROM settings ORDER BY id LIMIT 1`).Scan(&id, &footer) //nolint:errcheck
	if id != before.InstallID || footer != "Thank you — sheet" {
		t.Fatalf("after import: install_id %q footer %q", id, footer)
	}
	bad := "key,value\nreceipt_width,wide\nreceipt_footer,should not land\n"
	if rec := te.upload("/api/v1/admin/data/settings/import", "file", "s.csv", []byte(bad), ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad value: %d %s", rec.Code, rec.Body)
	}
	te.db.Get(&footer, `SELECT receipt_footer FROM settings ORDER BY id LIMIT 1`) //nolint:errcheck
	if footer != "Thank you — sheet" {
		t.Fatal("a failed import changed settings")
	}
}
