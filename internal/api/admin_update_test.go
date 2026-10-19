package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"karots-suite/internal/update"
)

// The Updates card's endpoints: admin only; no updater → the version only;
// check finds a newer signed release; apply before a check is refused.
func TestAdminUpdate(t *testing.T) {
	te := newTestEnv(t)
	te.cfg.Version = "2026.10.18.0900"
	ph := func(n int64) string { return fmt.Sprintf("07%08d", (time.Now().UnixNano()+n)%100_000_000) }
	t.Cleanup(func() { update.Enable(nil) })

	mgr := te.login(t, ph(1), "manager")
	if rec := te.do(http.MethodGet, "/api/v1/admin/data/update", nil, mgr); rec.Code != http.StatusForbidden {
		t.Fatalf("manager: %d", rec.Code)
	}
	admin := te.login(t, ph(2), "admin")

	update.Enable(nil)
	var st UpdateStatusDTO
	rec := te.do(http.MethodGet, "/api/v1/admin/data/update", nil, admin)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &st) != nil || st.Enabled || st.Version != "2026.10.18.0900" {
		t.Fatalf("no updater: %d %s", rec.Code, rec.Body)
	}

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	m := update.Manifest{Version: "2026.10.19.1530", SHA256: "aa", File: "x.gz", Notes: "faster till",
		Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, update.SignedMessage("2026.10.19.1530", "aa")))}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chan/manifest.json" {
			json.NewEncoder(w).Encode(m)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	update.Enable(&update.Updater{Current: "2026.10.18.0900", Kind: "release", BaseURL: srv.URL, Channel: "chan", PubKey: pub, Exe: t.TempDir() + "/karots-suite"})

	rec = te.do(http.MethodPost, "/api/v1/admin/data/update/apply", nil, admin)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("apply before check: %d %s", rec.Code, rec.Body)
	}
	rec = te.do(http.MethodPost, "/api/v1/admin/data/update/check", nil, admin)
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &st) != nil {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	if !st.Enabled || st.State != "available" || st.Latest != "2026.10.19.1530" || st.Notes != "faster till" || st.Kind != "release" || st.CheckedAt == nil {
		t.Fatalf("after check: %+v", st)
	}
	rec = te.do(http.MethodPost, "/api/v1/admin/data/update/dismiss", nil, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("dismiss: %d", rec.Code)
	}
}
