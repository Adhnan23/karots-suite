package main

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// buildFn is build, or a fake in tests.
type buildFn = func(buildOpts, io.Writer) (buildResult, error)

func (c shopsCfg) envPath() string {
	if c.Env != "" {
		return c.Env
	}
	return ".env"
}

func (c shopsCfg) openStore(log io.Writer) (store, error) {
	if c.Store != nil {
		return c.Store(log)
	}
	r, err := r2FromEnv(log)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (c shopsCfg) dirty() (bool, error) {
	if c.Dirty != nil {
		return c.Dirty()
	}
	return treeDirty(".")
}

const notPublishedWarn = "! This shop has no release yet — the first install at a shop should be a Release"

// shopRun is one shop's Test build (kind "test": local only) or Release (kind
// "release": build, then publish). A release whose upload fails keeps its
// build — the caller saves LastBuilt — and Publish sends it later.
func shopRun(s *Shop, all []manifest, b buildFn, cfg shopsCfg, kind, version, notes string, installer bool, log io.Writer) (buildResult, error) {
	priv, pub, err := releaseKey(cfg.envPath(), log)
	if err != nil {
		return buildResult{}, err
	}
	if kind == "test" && s.Published == nil {
		fmt.Fprintln(log, notPublishedWarn)
	}
	res, err := buildShop(s, all, shopBuildOpts{Out: cfg.Out, Installer: installer, Version: version, Kind: kind, PubKey: pub}, b, cfg.Setup, log)
	if err != nil || kind != "release" {
		return res, err
	}
	s.LastBuilt.Notes = notes
	if err := publishBuilt(s, cfg, priv, log); err != nil {
		return res, fmt.Errorf("built %s, but not published: %w — press Publish to try again", version, err)
	}
	return res, nil
}

// publishBuilt uploads the release build already in <out>/<key>.
func publishBuilt(s *Shop, cfg shopsCfg, priv ed25519.PrivateKey, log io.Writer) error {
	base := os.Getenv("UPDATE_BASE_URL")
	if base == "" {
		return errors.New("set UPDATE_BASE_URL in .env (the bucket's public URL)")
	}
	st, err := cfg.openStore(log)
	if err != nil {
		return err
	}
	return publish(s, filepath.Join(cfg.Out, s.Key), s.LastBuilt.Version, s.LastBuilt.Notes, s.LastBuilt.Commit, base, st, priv, log)
}

// mountRelease adds Release, Publish, Release all and the notes for the dialog.
// The Test build is POST /api/shops/{key}/build (mountShops).
func mountRelease(mux *http.ServeMux, all []manifest, b buildFn, cfg shopsCfg, busy, mu *sync.Mutex,
	decode func(http.ResponseWriter, *http.Request, any) bool, runOne func(http.ResponseWriter, string, func(*Shop, io.Writer) (buildResult, error))) {
	load := func(w http.ResponseWriter) (Shops, bool) {
		mu.Lock()
		defer mu.Unlock()
		shops, err := LoadShops(cfg.Path)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
		}
		return shops, err == nil
	}
	clean := func(w http.ResponseWriter) bool {
		dirty, err := cfg.dirty()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return false
		}
		if dirty {
			writeErr(w, http.StatusConflict, `uncommitted changes — commit first, so the shops' "what's new" matches what they get (shops.json and the plugin lists don't count)`)
			return false
		}
		return true
	}

	mux.HandleFunc("GET /api/shops/{key}/notes", func(w http.ResponseWriter, r *http.Request) {
		shops, ok := load(w)
		if !ok {
			return
		}
		i := slices.IndexFunc(shops.Shops, func(s Shop) bool { return s.Key == r.PathValue("key") })
		if i < 0 {
			writeErr(w, http.StatusNotFound, "no shop "+r.PathValue("key"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"notes": defaultNotes(shops.Shops[i], all)})
	})

	mux.HandleFunc("POST /api/shops/{key}/release", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Installer bool
			Notes     string
		}
		if !decode(w, r, &in) || !clean(w) {
			return
		}
		version := versionAt(time.Now())
		runOne(w, r.PathValue("key"), func(s *Shop, log io.Writer) (buildResult, error) {
			return shopRun(s, all, b, cfg, "release", version, strings.TrimSpace(in.Notes), in.Installer, log)
		})
	})

	mux.HandleFunc("POST /api/shops/{key}/publish", func(w http.ResponseWriter, r *http.Request) {
		if !decode(w, r, &struct{}{}) {
			return
		}
		shops, ok := load(w)
		if !ok {
			return
		}
		i := slices.IndexFunc(shops.Shops, func(s Shop) bool { return s.Key == r.PathValue("key") })
		switch {
		case i < 0:
			writeErr(w, http.StatusNotFound, "no shop "+r.PathValue("key"))
			return
		case !unpublished(shops.Shops[i]):
			writeErr(w, http.StatusConflict, "nothing to publish — the last build is a test build or already published; press Release")
			return
		}
		runOne(w, r.PathValue("key"), func(s *Shop, log io.Writer) (buildResult, error) {
			priv, _, err := releaseKey(cfg.envPath(), log)
			if err == nil {
				err = publishBuilt(s, cfg, priv, log)
			}
			return buildResult{Folder: filepath.Join(cfg.Out, s.Key)}, err
		})
	})

	mux.HandleFunc("POST /api/shops/release-all", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Installer bool
			Notes     string
		}
		if !decode(w, r, &in) || !clean(w) {
			return
		}
		shops, ok := load(w)
		if !ok {
			return
		}
		if !busy.TryLock() {
			writeErr(w, http.StatusConflict, "a build is already running")
			return
		}
		defer busy.Unlock()
		version := versionAt(time.Now()) // one version for every shop
		streamBuild(w, func(log io.Writer) (buildResult, error) {
			var failed []string
			for _, s := range shops.Shops {
				fmt.Fprintf(log, "\n=== %s (%s) ===\n", s.Name, s.Key)
				before := s.LastBuilt
				_, err := shopRun(&s, all, b, cfg, "release", version, strings.TrimSpace(in.Notes), in.Installer, log)
				if s.LastBuilt != before {
					if serr := saveShopState(cfg, mu, s); serr != nil {
						return buildResult{}, serr
					}
				}
				if err != nil {
					fmt.Fprintf(log, "✗ %s: %v\n", s.Key, err)
					failed = append(failed, s.Key)
					continue
				}
				fmt.Fprintf(log, "✓ %s → %s\n", s.Key, filepath.Join(cfg.Out, s.Key))
			}
			if len(failed) > 0 {
				return buildResult{}, fmt.Errorf("%d of %d shops failed: %s", len(failed), len(shops.Shops), strings.Join(failed, ", "))
			}
			return buildResult{Folder: fmt.Sprintf("%s — %d shops released as %s", cfg.Out, len(shops.Shops), version)}, nil
		})
	})
}

// unpublished: the last build is a release the bucket doesn't have yet.
func unpublished(s Shop) bool {
	return s.LastBuilt != nil && s.LastBuilt.Kind == "release" && (s.Published == nil || s.Published.Version != s.LastBuilt.Version)
}

// saveShopState records a shop's LastBuilt and Published, re-reading first:
// the profile may have been edited during the build.
func saveShopState(cfg shopsCfg, mu *sync.Mutex, s Shop) error {
	mu.Lock()
	defer mu.Unlock()
	cur, err := LoadShops(cfg.Path)
	if err != nil {
		return err
	}
	if j := slices.IndexFunc(cur.Shops, func(x Shop) bool { return x.Key == s.Key }); j >= 0 {
		cur.Shops[j].LastBuilt, cur.Shops[j].Published = s.LastBuilt, s.Published
	}
	return SaveShops(cfg.Path, cur)
}
