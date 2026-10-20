package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/s3"
)

type fakeCloud struct {
	objs map[string][]byte
	err  error
}

func (f fakeCloud) Put(context.Context, string, []byte) error {
	return errors.New("setup never uploads")
}
func (f fakeCloud) Get(_ context.Context, k string) ([]byte, error) {
	if b, ok := f.objs[k]; ok {
		return b, nil
	}
	return nil, s3.ErrNotFound
}
func (f fakeCloud) List(_ context.Context, prefix string) ([]s3.Object, error) {
	var out []s3.Object
	for k, v := range f.objs {
		if strings.HasPrefix(k, prefix) {
			out = append(out, s3.Object{Key: k, Size: int64(len(v))})
		}
	}
	return out, f.err
}

const testCloudKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

// withCloud gives the job Karots cloud with fake as its bucket.
func withCloud(t *testing.T, j *Job, fake fakeCloud) {
	t.Helper()
	for k, v := range map[string]string{"CLOUD_BACKUP_URL": "https://a.r2/b", "CLOUD_BACKUP_KEY_ID": "k", "CLOUD_BACKUP_SECRET": "s",
		"CLOUD_BACKUP_FOLDER": "al-raj", "CLOUD_BACKUP_CRYPT": testCloudKey, "CLOUD_BACKUP": "on"} {
		j.Conf.Env[k] = v
	}
	cloudStore = func(config.CloudBackup) backup.CloudStore { return fake }
	cloudSeen.Lock()
	cloudSeen.key = ""
	cloudSeen.Unlock()
	t.Cleanup(func() {
		cloudStore = func(c config.CloudBackup) backup.CloudStore {
			return s3.Client{URL: c.URL, KeyID: c.KeyID, Secret: c.Secret}
		}
	})
}

func TestCopyDirChoice(t *testing.T) {
	c := conf("linux", "server", true)
	c.Env["BACKUP_DIR"] = "/opt/karots-suite/backups"
	if _, err := applyChoice(c, t.TempDir(), Choice{CopyDir: "/opt/karots-suite/backups/"}); err == nil {
		t.Fatal("copy folder = backup folder accepted")
	}
	if _, err := applyChoice(c, t.TempDir(), Choice{CopyDir: "usb"}); err == nil {
		t.Fatal("relative copy folder accepted")
	}
	j, err := applyChoice(c, t.TempDir(), Choice{CopyDir: "/media/usb/Karots copies"})
	if err != nil || j.Conf.Env["BACKUP_COPY_DIR"] != "/media/usb/Karots copies" {
		t.Fatalf("%v %v", err, j.Conf.Env)
	}
}

// Bring back a shop also offers the copy folder's and Karots cloud's backups;
// a cloud that can't be reached is a note, not a failure.
func TestFindBackupsFromCopies(t *testing.T) {
	j, _, r := linuxJob(t, "file", false)
	copyDir := "/media/usb/Karots copies"
	os.MkdirAll(filepath.Join(r, copyDir), 0o755)
	os.WriteFile(filepath.Join(r, copyDir, "al-raj-20261019-090000.json.gz"), []byte("x"), 0o644)
	j.Conf.Env["BACKUP_COPY_DIR"] = copyDir
	key, _ := backup.ParseKey(testCloudKey)
	sealed, _ := backup.Seal(key, []byte("cloud backup"))
	name := backup.CloudName(time.Date(2026, 10, 20, 9, 0, 0, 0, time.Local))
	withCloud(t, j, fakeCloud{objs: map[string][]byte{"al-raj/" + name: sealed}})

	places := map[string]string{}
	for _, b := range findBackups(j) {
		places[b.Place] = b.Path
	}
	if places["copy"] != copyDir+"/al-raj-20261019-090000.json.gz" || places["cloud"] != "cloud:"+name || j.cloudNote != "" {
		t.Fatalf("found %v (note %q)", places, j.cloudNote)
	}

	j.Restore = "cloud:../other/" + name
	if err := fetchCloud(j); err == nil {
		t.Fatal("a name the cloud doesn't list was fetched")
	}
	j.Restore = "cloud:" + name
	if err := fetchCloud(j); err != nil {
		t.Fatal(err)
	}
	want := "/home/shop/Karots backups/cloud-20261020-090000.json.gz"
	if b, _ := os.ReadFile(filepath.Join(r, want)); j.Restore != want || string(b) != "cloud backup" {
		t.Fatalf("restore %q holds %q", j.Restore, b)
	}

	j2, _, _ := linuxJob(t, "file", false)
	withCloud(t, j2, fakeCloud{err: errors.New("no network")})
	for _, b := range findBackups(j2) {
		if b.Place == "cloud" {
			t.Fatal("cloud row without the cloud")
		}
	}
	if !strings.Contains(j2.cloudNote, "no network") {
		t.Fatalf("note %q", j2.cloudNote)
	}
}
