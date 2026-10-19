package main

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"sync"
)

// shopsCfg is where the app keeps shop profiles and puts shop builds.
type shopsCfg struct {
	Path  string // shops.json
	Out   string // dist
	Setup func(osName, arch, out string, log io.Writer) error
}

// mountShops adds the shop-profile routes. Every POST is JSON-only (jsonOnly);
// lists and edit forms only ever see masked secrets — Show (reveal) and
// Download .env are POSTs, so another site can't read them.
func mountShops(mux *http.ServeMux, all []manifest, b func(buildOpts, io.Writer) (buildResult, error), cfg shopsCfg, busy *sync.Mutex) {
	var mu sync.Mutex // load → change → save, one at a time

	// edit runs fn on the named shop under mu and saves when fn returns no error.
	edit := func(w http.ResponseWriter, key string, fn func(shops *Shops, i int) (any, int, string)) {
		mu.Lock()
		defer mu.Unlock()
		shops, err := LoadShops(cfg.Path)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		i := slices.IndexFunc(shops.Shops, func(s Shop) bool { return s.Key == key })
		if i < 0 {
			writeErr(w, http.StatusNotFound, "no shop "+key)
			return
		}
		out, code, msg := fn(&shops, i)
		if code != 0 {
			writeErr(w, code, msg)
			return
		}
		if err := SaveShops(cfg.Path, shops); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if out != nil {
			writeJSON(w, http.StatusOK, out)
		}
	}
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		if !jsonOnly(w, r) {
			return false
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(v); err != nil {
			writeErr(w, http.StatusBadRequest, "bad request: "+err.Error())
			return false
		}
		return true
	}

	mux.HandleFunc("GET /api/shops", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		shops, err := LoadShops(cfg.Path)
		mu.Unlock()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out := struct {
			Path     string              `json:"path"`
			Shops    []Shop              `json:"shops"`
			Warnings map[string][]string `json:"warnings"`
		}{cfg.Path, []Shop{}, map[string][]string{}}
		for _, s := range shops.Shops {
			out.Shops = append(out.Shops, s.Masked())
			if warn, err := s.Check(all); err != nil {
				out.Warnings[s.Key] = []string{err.Error()}
			} else if len(warn) > 0 {
				out.Warnings[s.Key] = warn
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	mux.HandleFunc("POST /api/shops", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Key, Name, OS, Arch, Mode string }
		if !decode(w, r, &in) {
			return
		}
		s, err := NewShop(in.Key, in.Name, in.OS, in.Arch, in.Mode)
		if err == nil {
			_, err = s.Check(all)
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		mu.Lock()
		defer mu.Unlock()
		shops, err := LoadShops(cfg.Path)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if slices.ContainsFunc(shops.Shops, func(x Shop) bool { return x.Key == s.Key }) {
			writeErr(w, http.StatusConflict, "a shop called "+s.Key+" already exists")
			return
		}
		shops.Shops = append(shops.Shops, s)
		if err := SaveShops(cfg.Path, shops); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s.Masked())
	})

	mux.HandleFunc("POST /api/shops/{key}", func(w http.ResponseWriter, r *http.Request) {
		var in Shop
		if !decode(w, r, &in) {
			return
		}
		edit(w, r.PathValue("key"), func(shops *Shops, i int) (any, int, string) {
			merged, err := mergeShop(shops.Shops[i], in)
			if err == nil {
				_, err = merged.Check(all)
			}
			if err != nil {
				return nil, http.StatusBadRequest, err.Error()
			}
			shops.Shops[i] = merged
			return merged.Masked(), 0, ""
		})
	})

	mux.HandleFunc("POST /api/shops/{key}/rotate", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ What string }
		if !decode(w, r, &in) {
			return
		}
		edit(w, r.PathValue("key"), func(shops *Shops, i int) (any, int, string) {
			if err := shops.Shops[i].Rotate(in.What); err != nil {
				return nil, http.StatusBadRequest, err.Error()
			}
			return shops.Shops[i], 0, ""
		})
	})

	mux.HandleFunc("POST /api/shops/{key}/reveal", func(w http.ResponseWriter, r *http.Request) {
		if !decode(w, r, &struct{}{}) {
			return
		}
		edit(w, r.PathValue("key"), func(shops *Shops, i int) (any, int, string) { return shops.Shops[i], 0, "" })
	})

	mux.HandleFunc("POST /api/shops/{key}/env", func(w http.ResponseWriter, r *http.Request) {
		if !decode(w, r, &struct{}{}) {
			return
		}
		key := r.PathValue("key")
		var env []byte
		edit(w, key, func(shops *Shops, i int) (any, int, string) {
			env = shops.Shops[i].Conf(shopBinary).EnvFile()
			return nil, 0, ""
		})
		if env != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="`+key+`.env"`)
			w.Write(env) //nolint:errcheck
		}
	})

	mux.HandleFunc("POST /api/shops/{key}/delete", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Confirm string }
		if !decode(w, r, &in) {
			return
		}
		key := r.PathValue("key")
		edit(w, key, func(shops *Shops, i int) (any, int, string) {
			if in.Confirm != key {
				return nil, http.StatusBadRequest, "type the shop's key to delete it"
			}
			shops.Shops = slices.Delete(shops.Shops, i, i+1)
			return map[string]bool{"deleted": true}, 0, ""
		})
	})

	mux.HandleFunc("POST /api/shops/{key}/build", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Installer bool }
		if !decode(w, r, &in) {
			return
		}
		key := r.PathValue("key")
		mu.Lock()
		shops, err := LoadShops(cfg.Path)
		mu.Unlock()
		i := slices.IndexFunc(shops.Shops, func(s Shop) bool { return s.Key == key })
		switch {
		case err != nil:
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		case i < 0:
			writeErr(w, http.StatusNotFound, "no shop "+key)
			return
		}
		if !busy.TryLock() {
			writeErr(w, http.StatusConflict, "a build is already running")
			return
		}
		defer busy.Unlock()
		s := shops.Shops[i]
		streamBuild(w, func(log io.Writer) (buildResult, error) {
			res, err := buildShop(&s, all, shopBuildOpts{Out: cfg.Out, Installer: in.Installer}, b, cfg.Setup, log)
			if err != nil {
				return res, err
			}
			// Re-read before saving: the profile may have been edited during the build.
			mu.Lock()
			defer mu.Unlock()
			cur, err := LoadShops(cfg.Path)
			if err != nil {
				return res, err
			}
			if j := slices.IndexFunc(cur.Shops, func(x Shop) bool { return x.Key == key }); j >= 0 {
				cur.Shops[j].LastBuilt = s.LastBuilt
			}
			return res, SaveShops(cfg.Path, cur)
		})
	})
}

// mergeShop applies an edit form's (masked) shop over the stored one: masked
// secrets keep their stored value, the identity (key, install id, channel,
// last build) never comes from the form, and a mode change goes through SetMode.
func mergeShop(old, in Shop) (Shop, error) {
	m := old
	if err := m.SetMode(in.Mode); err != nil {
		return old, err
	}
	m.Name, m.OS, m.Arch, m.Plugins, m.Port, m.Install = in.Name, in.OS, in.Arch, in.Plugins, in.Port, in.Install
	if m.Plugins == nil {
		m.Plugins = []string{}
	}
	if m.Mode == "file" && in.FileDBPort != 0 {
		m.FileDBPort = in.FileDBPort
	}
	if m.Mode == "server" && in.DB != nil {
		db := *m.DB
		db.Host, db.Port, db.Name, db.User = in.DB.Host, in.DB.Port, in.DB.Name, in.DB.User
		if in.DB.Password != maskedValue && in.DB.Password != "" {
			db.Password = in.DB.Password
		}
		if in.DB.SuperPassword != maskedValue && in.DB.SuperPassword != "" {
			db.SuperPassword = in.DB.SuperPassword
		}
		m.DB = &db
	}
	if m.Mode == "server" && in.JWTSecret != maskedValue && len(in.JWTSecret) >= 32 {
		m.JWTSecret = in.JWTSecret
	}
	m.Env = nil
	for k, v := range in.Env {
		if v == maskedValue {
			v = old.Env[k]
		}
		if m.Env == nil {
			m.Env = map[string]string{}
		}
		m.Env[k] = v
	}
	return m, nil
}
