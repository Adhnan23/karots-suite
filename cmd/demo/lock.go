package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"karots-suite/internal/support"
)

const lockCookie = "ks_demo"

// Baked by `make demo` (cmd/bootstrap -demo), same names as cmd/server: the
// demo's own install ID and only the seed derived for it — never the master
// secret. A dev build without them uses a fixed dev seed and prints the code.
var (
	installIDBaked = ""
	supportSeedHex = ""
)

func demoSeed() (seed []byte, installID string, dev bool) {
	if supportSeedHex != "" {
		if b, err := hex.DecodeString(supportSeedHex); err == nil {
			return b, support.Normalise(installIDBaked), false
		}
	}
	return support.DeriveSeed("dev", "DEMO-DEV"), "DEMO-DEV", true
}

func newToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// unlocked reports whether the request carries this run's lock token.
func (l *launcher) unlocked(r *http.Request) bool {
	c, err := r.Cookie(lockCookie)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(l.token)) == 1
}

// guard lets the lock page and its static assets through; everything else needs
// the cookie. A page asks for the lock page, anything else gets 401.
func (l *launcher) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/__demo/lock" || p == "/__demo/unlock" || strings.HasPrefix(p, "/__demo/ui/") || l.unlocked(r) {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodGet && !strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, "/__demo/") {
			http.Redirect(w, r, "/__demo/lock", http.StatusFound)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "locked"})
	})
}

// unlock checks the hourly code (this window or the one either side, as the
// system user's PIN). Five wrong codes in a minute make it wait out the minute.
func (l *launcher) unlock(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	now := l.now()
	l.mu.Lock()
	cut := now.Add(-time.Minute)
	recent := l.misses[:0]
	for _, t := range l.misses {
		if t.After(cut) {
			recent = append(recent, t)
		}
	}
	l.misses = recent
	tooMany := len(l.misses) >= 5
	ok := !tooMany && support.Valid(l.seed, strings.TrimSpace(in.Code), now, 1)
	if !ok && !tooMany {
		l.misses = append(l.misses, now)
	}
	l.mu.Unlock()
	if tooMany {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many tries — wait a minute."})
		return
	}
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "That code isn't right."})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: lockCookie, Value: l.token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func currentCode(seed []byte) string { return support.Code(seed, time.Now()) }
