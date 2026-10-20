package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/s3"
)

type fakeCloud map[string][]byte

func (f fakeCloud) Put(_ context.Context, k string, b []byte) error { f[k] = b; return nil }
func (f fakeCloud) Get(_ context.Context, k string) ([]byte, error) {
	if b, ok := f[k]; ok {
		return b, nil
	}
	return nil, s3.ErrNotFound
}
func (f fakeCloud) List(_ context.Context, prefix string) ([]s3.Object, error) {
	var out []s3.Object
	for k, v := range f {
		if strings.HasPrefix(k, prefix) {
			out = append(out, s3.Object{Key: k, Size: int64(len(v))})
		}
	}
	return out, nil
}

func setCloudEnv(t *testing.T) {
	t.Setenv("CLOUD_BACKUP_URL", "https://acct.r2.cloudflarestorage.com/karots-suite-backups")
	t.Setenv("CLOUD_BACKUP_KEY_ID", "kid")
	t.Setenv("CLOUD_BACKUP_SECRET", "sec")
}

// Ticking Karots cloud: a key on the first save that later saves keep, masked
// on the page, the bucket and key in the shop's .env, refused without the
// Builder's bucket; the copy folder is a preset and never the backups folder.
func TestCloudAndCopyFolderProfile(t *testing.T) {
	setCloudEnv(t)
	s := mustShop(t, "al-raj", "linux", "server")
	in := s.Masked()
	in.Backups.Cloud, in.Backups.CopyDir = true, "/media/usb/Karots copies"
	m, err := mergeShop(s, in)
	if err != nil || !hexOf(64).MatchString(m.CloudKey) {
		t.Fatalf("first tick: %q %v", m.CloudKey, err)
	}
	again, _ := mergeShop(m, m.Masked())
	if again.CloudKey != m.CloudKey || m.Masked().CloudKey != maskedValue {
		t.Fatal("cloud key changed on a save or shown on the page")
	}
	env := m.Conf("karots-suite").Env
	if env["CLOUD_BACKUP_FOLDER"] != "al-raj" || env["CLOUD_BACKUP_CRYPT"] != m.CloudKey || env["CLOUD_BACKUP"] != "on" ||
		env["CLOUD_BACKUP_SECRET"] != "sec" || env["BACKUP_COPY_DIR"] != "/media/usb/Karots copies" {
		t.Fatalf("env: %v", env)
	}
	if _, err := m.Check(nil); err != nil {
		t.Fatal(err)
	}
	off := m
	off.Backups.Cloud = false
	if env := off.Conf("x").Env; env["CLOUD_BACKUP"] != "" || env["CLOUD_BACKUP_SECRET"] != "" {
		t.Fatalf("unticked still carries cloud: %v", env)
	}
	t.Setenv("CLOUD_BACKUP_SECRET", "")
	if _, err := m.Check(nil); err == nil || !strings.Contains(err.Error(), "CLOUD_BACKUP_SECRET") {
		t.Fatalf("cloud without the Builder's bucket: %v", err)
	}
	same := off
	same.Backups.CopyDir = "/opt/karots-suite/backups/"
	if _, err := same.Check(nil); err == nil {
		t.Fatal("copy folder = backups folder accepted")
	}
	rel := off
	rel.Backups.CopyDir = "usb"
	if _, err := rel.Check(nil); err == nil {
		t.Fatal("relative copy folder accepted")
	}
	extra := off
	extra.Env = map[string]string{"CLOUD_BACKUP_SECRET": "x"}
	if _, err := extra.Check(nil); err == nil {
		t.Fatal("CLOUD_BACKUP_SECRET in More settings accepted")
	}
}

// Cloud backups → Download: the shop's sealed copy lands opened in dist/<key>/.
func TestCloudBackupsFetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shops.json")
	s := mustShop(t, "al-raj", "linux", "server")
	s.CloudKey = randHex(32)
	SaveShops(path, Shops{Shops: []Shop{s}}) //nolint:errcheck
	key, _ := backup.ParseKey(s.CloudKey)
	sealed, _ := backup.Seal(key, []byte("the backup"))
	name := backup.CloudName(time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local))
	cloud := fakeCloud{"al-raj/" + name: sealed, "other-shop/" + name: sealed}
	t.Setenv("UPDATE_BASE_URL", "http://127.0.0.1:1")
	a := shopsApp{appHandler(nil, func(buildOpts, io.Writer) (buildResult, error) { return buildResult{}, nil },
		shopsCfg{Path: path, Out: filepath.Join(dir, "dist"), Setup: fakeSetup, Env: filepath.Join(dir, ".env"),
			Cloud: func() (backup.CloudStore, error) { return cloud, nil }}), path}

	rec := a.do(t, "/api/shops/al-raj/cloud-backups", "{}")
	var list []backup.BackupFile
	if json.Unmarshal(rec.Body.Bytes(), &list); rec.Code != 200 || len(list) != 1 || list[0].Name != name {
		t.Fatalf("list %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(t, "/api/shops/al-raj/cloud-backups/fetch", `{"name":"../other-shop/`+name+`"}`); rec.Code != 400 {
		t.Fatalf("other shop's copy: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(t, "/api/shops/al-raj/cloud-backups/fetch", `{"name":"`+name+`"}`)
	var out struct{ Path string }
	json.Unmarshal(rec.Body.Bytes(), &out) //nolint:errcheck
	if b, err := os.ReadFile(out.Path); rec.Code != 200 || string(b) != "the backup" || filepath.Dir(out.Path) != filepath.Join(dir, "dist", "al-raj") {
		t.Fatalf("fetch %d %s: %q %v", rec.Code, rec.Body, b, err)
	}
}
