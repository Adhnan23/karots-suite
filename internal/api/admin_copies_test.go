package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"karots-suite/internal/backup"
)

// Backup copies: never the backups folder, Karots cloud switched on only by
// Karots, and a fetch takes only a name the copy lists, keeping its time.
func TestAdminCopies(t *testing.T) {
	te := newTestEnv(t)
	te.cfg.BackupDir = t.TempDir()
	te.db.MustExec(`DELETE FROM backup_copies`)
	t.Cleanup(func() { te.db.MustExec(`DELETE FROM backup_copies`) })
	ck := te.login(t, "0770009301", "admin")

	for _, p := range []string{te.cfg.BackupDir, filepath.Join(te.cfg.BackupDir, "sub")} {
		rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/folder", CopyFolderRequest{Path: p, Enabled: true}, ck)
		if rec.Code != http.StatusBadRequest || errCode(rec) != "BACKUP_FOLDER_IS_COPY" {
			t.Fatalf("copy folder %s: %d %s", p, rec.Code, rec.Body)
		}
	}
	if rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/folder", CopyFolderRequest{Path: "rel/dir", Enabled: true}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("relative path: %d", rec.Code)
	}

	place := filepath.Join(t.TempDir(), "Karots copies") // created by the save's test
	rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/folder", CopyFolderRequest{Path: place, Enabled: true}, ck)
	if rec.Code != http.StatusOK || decode[BackupCopyRow](t, rec).Path != place {
		t.Fatalf("save folder: %d %s", rec.Code, rec.Body)
	}
	mod := time.Date(2026, 10, 1, 9, 30, 0, 0, time.Local)
	name := backup.FileName("Shop", mod, false)
	os.WriteFile(filepath.Join(place, name), []byte("x"), 0o644) //nolint:errcheck
	os.Chtimes(filepath.Join(place, name), mod, mod)             //nolint:errcheck

	for _, bad := range []string{"..%2Fx.json.gz", "other.json.gz"} {
		if rec := te.do(http.MethodPost, "/api/v1/admin/data/copies/folder/backups/"+bad+"/fetch", nil, ck); rec.Code != http.StatusBadRequest {
			t.Fatalf("fetch %s: %d %s", bad, rec.Code, rec.Body)
		}
	}
	rec = te.do(http.MethodPost, "/api/v1/admin/data/copies/folder/backups/"+name+"/fetch", nil, ck)
	if rec.Code != http.StatusOK || decode[FetchedBackup](t, rec).Name != name {
		t.Fatalf("fetch: %d %s", rec.Code, rec.Body)
	}
	if st, err := os.Stat(filepath.Join(te.cfg.BackupDir, name)); err != nil || !st.ModTime().Equal(mod) {
		t.Fatalf("fetched copy: %v %v", st, err)
	}

	if rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/cloud", CopyCloudRequest{Enabled: true}, ck); rec.Code != http.StatusNotFound {
		t.Fatalf("no cloud row: %d", rec.Code)
	}
	te.db.MustExec(`INSERT INTO backup_copies (kind, enabled) VALUES ('cloud', false)`)
	if rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/cloud", CopyCloudRequest{Enabled: true}, ck); rec.Code != http.StatusForbidden {
		t.Fatalf("admin switched cloud on: %d", rec.Code)
	}
	id := te.seedUser(t, "0770009302", "admin")
	sys := login(t, te, "0770009302") // before flagging: system PINs rotate
	te.db.MustExec(`UPDATE users SET is_system=true WHERE id=$1`, id)
	if rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/cloud", CopyCloudRequest{Enabled: true}, sys); rec.Code != http.StatusOK || !decode[BackupCopyRow](t, rec).Enabled {
		t.Fatalf("system cloud on: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPut, "/api/v1/admin/data/copies/cloud", CopyCloudRequest{Enabled: false}, ck); rec.Code != http.StatusOK {
		t.Fatalf("admin cloud off: %d", rec.Code)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, ck); rec.Code != http.StatusOK {
		t.Fatalf("dashboard: %d %s", rec.Code, rec.Body)
	}
}

func TestStaleCopies(t *testing.T) {
	now := time.Now()
	ago := func(h int) *time.Time { t := now.Add(-time.Duration(h) * time.Hour); return &t }
	got := staleCopies([]backup.Copy{
		{Kind: backup.KindFolder, Enabled: true, Path: "/m", LastCopiedAt: ago(25), LastTriedAt: ago(1)},
		{Kind: backup.KindCloud, Enabled: false, LastCopiedAt: ago(48)},
	}, now)
	if len(got) != 1 || got[0] != "Copy folder" {
		t.Fatalf("stale: %v", got)
	}
	got = staleCopies([]backup.Copy{
		{Kind: backup.KindFolder, Enabled: true, Path: "/m", LastCopiedAt: ago(2)},
		{Kind: backup.KindCloud, Enabled: true}, // never tried yet
	}, now)
	if len(got) != 0 {
		t.Fatalf("fresh: %v", got)
	}
}
