package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"sync"
	"time"
)

//go:embed page.html
var pageHTML []byte

// pageHandler serves the install page. Setup runs as admin, so every request
// must carry the random token from the page's address (?t= or X-Setup-Token):
// no other site or local user can drive it.
func pageHandler(token string, mk func(Choice) (*Job, error), steps func(*Job) []Step, quit func()) http.Handler {
	var running sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(pageHTML) //nolint:errcheck
	})
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) }) //nolint:errcheck
	mux.HandleFunc("GET /api/plan", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		keep, _ := strconv.Atoi(q.Get("backupKeep"))
		yes := func(k string) bool { return q.Get(k) == "true" }
		j, err := mk(Choice{Role: Role(q.Get("role")), Address: q.Get("address"), Open: q.Get("open"), BackupDir: q.Get("backupDir"), BackupEvery: q.Get("backupEvery"), BackupKeep: keep,
			Uninstall: yes("uninstall"), RemovePostgres: yes("removePostgres"), ChangeMode: yes("changeMode"), Restore: q.Get("restore"), KeepDir: q.Get("keepDir")})
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		type step struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Detail string `json:"detail"`
		}
		out := struct {
			Shop        string   `json:"shop"`
			InstallID   string   `json:"installId"`
			Mode        string   `json:"mode"`
			Open        string   `json:"open"`
			BackupDir   string   `json:"backupDir"`
			BackupEvery string   `json:"backupEvery"`
			BackupKeep  int      `json:"backupKeep"`
			Updating    bool     `json:"updating"`
			Kept        []string `json:"kept"`
			Address     string   `json:"address"`
			LanAddress  string   `json:"lanAddress"`
			TillURL     string   `json:"tillUrl"`
			Steps       []step   `json:"steps"`

			FromMode string        `json:"fromMode"` // an Update that changes the mode: the installed one
			KeepDir  string        `json:"keepDir"`  // the Karots backups folder
			Backups  []foundBackup `json:"backups"`  // newest first
			LocalDB  bool          `json:"localDb"`  // server mode with PostgreSQL on this PC (uninstall can remove it)
		}{j.Conf.Shop, j.Conf.InstallID, j.Conf.Mode, j.Conf.Open, j.backupDir(), orDefault(j.Conf.Env["BACKUP_INTERVAL"], "6h"), atoiOr(j.Conf.Env["BACKUP_KEEP"], 28), j.Updating, j.Kept, j.Address, j.Conf.LanAddress, j.tillURL(), nil,
			j.FromMode, j.keepDir(), findBackups(j), j.localDB()}
		for _, s := range steps(j) {
			d := s.Title
			if s.Detail != nil {
				d = s.Detail(j)
			}
			out.Steps = append(out.Steps, step{s.ID, s.Title, d})
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /api/test-address", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Address string }
		if !decode(w, r, &in) {
			return
		}
		addr := withPort(in.Address, 3000)
		if j, err := mk(Choice{Role: RoleCashier, Address: in.Address}); err == nil {
			addr = j.Address
		}
		c := http.Client{Timeout: 3 * time.Second}
		res, err := c.Get("http://" + addr + "/health")
		ok := err == nil && res.StatusCode == http.StatusOK
		if res != nil {
			res.Body.Close()
		}
		msg := "The main PC answers at " + addr + "."
		if !ok {
			msg = "Nothing answers at " + addr + " — check the main PC is on, Karots is running there, and the address is right."
		}
		writeJSON(w, map[string]any{"ok": ok, "message": msg})
	})
	mux.HandleFunc("POST /api/run", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Choice
			SuperPassword string `json:"superPassword"`
		}
		if !decode(w, r, &in) {
			return
		}
		j, err := mk(in.Choice)
		if err == nil {
			err = checkChoice(j)
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if !running.TryLock() {
			writeErr(w, http.StatusConflict, "Setup is already running")
			return
		}
		defer running.Unlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		log := flushWriter{w}
		j.Log, j.SuperPassword = log, in.SuperPassword
		var failed error
		for _, s := range steps(j) {
			fmt.Fprintf(log, "@@STEP %s run\n", s.ID)
			j.lastNote = ""
			if err := s.Do(j); err != nil {
				failed = err
				fmt.Fprintf(log, "\n@@STEP %s fail %s\n", s.ID, oneLine(err.Error()))
				if errors.Is(err, ErrNeedSuperPassword) {
					fmt.Fprintln(log, "@@NEED superPassword")
				}
				break
			}
			fmt.Fprintf(log, "\n@@STEP %s ok\n", s.ID)
		}
		if failed == nil {
			fmt.Fprintln(log, "@@DONE")
		}
	})
	mountFolders(mux)
	mux.HandleFunc("POST /api/close", func(w http.ResponseWriter, r *http.Request) {
		if decode(w, r, &struct{}{}) {
			writeJSON(w, map[string]bool{"ok": true})
			go quit()
		}
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Setup-Token")
		if got == "" {
			got = r.URL.Query().Get("t")
		}
		if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func atoiOr(v string, d int) int {
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		return n
	}
	return d
}

func oneLine(s string) string {
	for i, c := range s {
		if c == '\n' || c == '\r' {
			return s[:i]
		}
	}
	return s
}

// decode accepts JSON only: a cross-site page can't send it without a CORS
// preflight, which this server never answers.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" {
		writeErr(w, http.StatusUnsupportedMediaType, "send JSON")
		return false
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

// flushWriter pushes each write to the page so the log streams live.
type flushWriter struct{ w http.ResponseWriter }

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if fl, ok := f.w.(http.Flusher); ok {
		fl.Flush()
	}
	return n, err
}
