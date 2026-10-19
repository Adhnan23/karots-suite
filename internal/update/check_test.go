package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testChannel = "c0ffeec0ffeec0ffeec0ffeec0ffee00"

// feed serves m as <channel>/manifest.json; nil m answers 404.
func feed(t *testing.T, m *Manifest) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || r.URL.Path != "/"+testChannel+"/manifest.json" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(m)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckStates(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	_, otherPriv, _ := ed25519.GenerateKey(rand.Reader)
	newer := signed(t, priv, "2026.10.19.1530", "aa")
	newer.Notes = "faster till"
	newer.Changelog = []Change{{Version: "2026.10.19.1530", Notes: "faster till"}}
	same := signed(t, priv, "2026.10.18.0900", "aa")
	forged := signed(t, otherPriv, "2026.10.19.1530", "aa")

	cases := []struct {
		name string
		m    *Manifest
		down bool
		want State
		msg  string
	}{
		{"newer", &newer, false, StateAvailable, ""},
		{"same", &same, false, StateUpToDate, ""},
		{"404", nil, false, StateNotPublished, "No update published for this shop yet"},
		{"forged", &forged, false, StateBadSignature, "not signed"},
		{"down", nil, true, StateOffline, "Couldn't reach the update server"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := feed(t, c.m)
			if c.down {
				srv.Close()
			}
			u := &Updater{Current: "2026.10.18.0900", BaseURL: srv.URL, Channel: testChannel, PubKey: pub}
			u.Check(context.Background())
			s := u.Status()
			if s.State != c.want {
				t.Fatalf("state = %q, want %q (message %q)", s.State, c.want, s.Message)
			}
			if c.msg != "" && !strings.Contains(s.Message, c.msg) {
				t.Fatalf("message = %q, want it to contain %q", s.Message, c.msg)
			}
			if strings.Contains(s.Message, testChannel) {
				t.Fatalf("message leaks the channel: %q", s.Message)
			}
			if s.CheckedAt.IsZero() {
				t.Fatal("CheckedAt not set")
			}
			if c.want == StateAvailable && (s.Latest != "2026.10.19.1530" || !strings.Contains(s.Notes, "faster till")) {
				t.Fatalf("available: latest %q notes %q", s.Latest, s.Notes)
			}
			if c.want != StateAvailable && s.Latest != "" {
				t.Fatalf("latest should be empty, got %q", s.Latest)
			}
		})
	}
}

func TestStatusBeforeCheck(t *testing.T) {
	u := &Updater{Current: "2026.10.18.0900", Kind: "test"}
	if s := u.Status(); s.State != StateUnchecked || s.Current != "2026.10.18.0900" || s.Kind != "test" {
		t.Fatalf("status = %+v", s)
	}
}
