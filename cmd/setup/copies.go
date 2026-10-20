package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/s3"
)

// Backup copies (M8c2) as Setup sees them: the copy folder's backups and the
// shop's Karots cloud ones are offered under "Bring back a shop".

// cloudOf is the shop's Karots cloud from install.conf (Set() false = none).
func cloudOf(j *Job) config.CloudBackup {
	e := j.Conf.Env
	return config.CloudBackup{URL: e["CLOUD_BACKUP_URL"], KeyID: e["CLOUD_BACKUP_KEY_ID"], Secret: e["CLOUD_BACKUP_SECRET"],
		Folder: e["CLOUD_BACKUP_FOLDER"], Crypt: e["CLOUD_BACKUP_CRYPT"]}
}

// cloudStore opens the bucket (a fake in tests).
var cloudStore = func(c config.CloudBackup) backup.CloudStore {
	return s3.Client{URL: c.URL, KeyID: c.KeyID, Secret: c.Secret}
}

func cloudCopier(j *Job) *backup.Copier {
	c := cloudOf(j)
	return &backup.Copier{Dir: at(j.keepDir()), Cloud: c, S3: cloudStore(c), Now: time.Now}
}

// ponytail: the page asks for its plan on every change; the cloud list is
// kept a minute so an offline PC doesn't wait 10 s each time.
var cloudSeen struct {
	sync.Mutex
	at   time.Time
	key  string
	list []backup.BackupFile
	err  error
}

// cloudBackups lists the shop's Karots cloud backups (10 s at most).
func cloudBackups(j *Job) ([]backup.BackupFile, error) {
	c := cloudOf(j)
	cloudSeen.Lock()
	defer cloudSeen.Unlock()
	if cloudSeen.key == c.URL+"/"+c.Folder && time.Since(cloudSeen.at) < time.Minute {
		return cloudSeen.list, cloudSeen.err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	list, err := cloudCopier(j).Backups(ctx, backup.Copy{Kind: backup.KindCloud})
	cloudSeen.at, cloudSeen.key, cloudSeen.list, cloudSeen.err = time.Now(), c.URL+"/"+c.Folder, list, err
	return list, err
}

// copyBackups adds the copy folder's and Karots cloud's backups to found.
// A cloud that can't be reached is a note, never a failure.
func copyBackups(j *Job, found []foundBackup) []foundBackup {
	if d := j.Conf.Env["BACKUP_COPY_DIR"]; d != "" {
		if list, err := backup.ListBackups(at(d)); err == nil {
			for _, f := range list {
				found = append(found, foundBackup{Path: joinOS(j, d, f.Name), Name: f.Name, Modified: f.ModifiedAt, Place: "copy"})
			}
		}
	}
	if cloudOf(j).Set() {
		list, err := cloudBackups(j)
		if err != nil {
			j.cloudNote = "Karots cloud: couldn't reach it — " + err.Error()
		}
		for _, f := range list {
			found = append(found, foundBackup{Path: cloudPrefix + f.Name, Name: f.Name, Modified: f.ModifiedAt, Place: "cloud"})
		}
	}
	return found
}

// cloudPrefix marks a Restore that is a Karots cloud backup's name.
const cloudPrefix = "cloud:"

// fetchCloud brings a "cloud:<name>" backup down, opened, into the Karots
// backups folder and points the restore at it. Only a name the shop's cloud
// folder lists is taken.
func fetchCloud(j *Job) error {
	name, ok := strings.CutPrefix(j.Restore, cloudPrefix)
	if !ok {
		return nil
	}
	if !cloudOf(j).Set() {
		return fmt.Errorf("this shop has no Karots cloud backup")
	}
	j.note("Fetching " + name + " from Karots cloud")
	if err := os.MkdirAll(at(j.keepDir()), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	local, err := cloudCopier(j).Fetch(ctx, backup.Copy{Kind: backup.KindCloud}, name)
	if err != nil {
		return fmt.Errorf("couldn't fetch %s from Karots cloud: %w", name, err)
	}
	j.Restore = joinOS(j, j.keepDir(), local)
	return nil
}

// sameDir compares two folders on the shop's OS (Windows ignores case).
func sameDir(osName, a, b string) bool {
	a, b = strings.TrimRight(a, `/\`), strings.TrimRight(b, `/\`)
	if osName == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
