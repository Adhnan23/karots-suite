package main

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/s3"
)

// envCloud is the Builder's Karots cloud bucket from its .env.
func envCloud() (backup.CloudStore, error) {
	if os.Getenv("CLOUD_BACKUP_URL") == "" || os.Getenv("CLOUD_BACKUP_KEY_ID") == "" || os.Getenv("CLOUD_BACKUP_SECRET") == "" {
		return nil, errors.New("Karots cloud is not set up — add " + strings.Join(cloudEnv, ", ") + " to .env")
	}
	return s3.Client{URL: os.Getenv("CLOUD_BACKUP_URL"), KeyID: os.Getenv("CLOUD_BACKUP_KEY_ID"), Secret: os.Getenv("CLOUD_BACKUP_SECRET")}, nil
}

// mountCloudBackups lets the vendor see a shop's Karots cloud copies and save
// one, opened, into dist/<key>/ — to carry to a PC that can't reach the cloud.
func mountCloudBackups(mux *http.ServeMux, cfg shopsCfg, mu *sync.Mutex, decode func(http.ResponseWriter, *http.Request, any) bool) {
	open := cfg.Cloud
	if open == nil {
		open = envCloud
	}
	// copier is the shop's cloud copy, as the shop itself sees it.
	copier := func(w http.ResponseWriter, key string) *backup.Copier {
		mu.Lock()
		shops, err := LoadShops(cfg.Path)
		mu.Unlock()
		i := slices.IndexFunc(shops.Shops, func(s Shop) bool { return s.Key == key })
		switch {
		case err != nil:
			writeErr(w, http.StatusInternalServerError, err.Error())
			return nil
		case i < 0:
			writeErr(w, http.StatusNotFound, "no shop "+key)
			return nil
		case shops.Shops[i].CloudKey == "":
			writeErr(w, http.StatusBadRequest, key+" has never had Karots cloud backup")
			return nil
		}
		st, err := open()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return nil
		}
		return &backup.Copier{Dir: filepath.Join(cfg.Out, key), S3: st, Now: time.Now,
			Cloud: config.CloudBackup{Folder: key, Crypt: shops.Shops[i].CloudKey}}
	}
	cloud := backup.Copy{Kind: backup.KindCloud}

	mux.HandleFunc("POST /api/shops/{key}/cloud-backups", func(w http.ResponseWriter, r *http.Request) {
		if !decode(w, r, &struct{}{}) {
			return
		}
		c := copier(w, r.PathValue("key"))
		if c == nil {
			return
		}
		list, err := c.Backups(r.Context(), cloud)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Karots cloud: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, list)
	})

	mux.HandleFunc("POST /api/shops/{key}/cloud-backups/fetch", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Name string }
		if !decode(w, r, &in) {
			return
		}
		c := copier(w, r.PathValue("key"))
		if c == nil {
			return
		}
		name, err := c.Fetch(r.Context(), cloud, in.Name)
		if errors.Is(err, backup.ErrNotListed) {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if err != nil {
			writeErr(w, http.StatusBadGateway, "Karots cloud: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"path": filepath.Join(c.Dir, name)})
	})
}
