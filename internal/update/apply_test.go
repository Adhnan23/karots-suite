package update

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeShop struct {
	u        *Updater
	exe      string
	restarts chan struct{}
	backups  []string
	mu       sync.Mutex
}

// newFakeShop serves a signed release of binary (gzipped) whose manifest says sha,
// with handler overriding how the .gz is served when non-nil.
func newFakeShop(t *testing.T, binary []byte, sha string, gz http.HandlerFunc) *fakeShop {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	m := signed(t, priv, "2026.10.19.1530", sha)
	m.File = "karots-suite-2026.10.19.1530.gz"
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write(binary)
	w.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + testChannel + "/manifest.json":
			json.NewEncoder(w).Encode(m)
		case "/" + testChannel + "/" + m.File:
			if gz != nil {
				gz(w, r)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
			w.Write(buf.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	f := &fakeShop{exe: filepath.Join(dir, "karots-suite"), restarts: make(chan struct{}, 1)}
	os.WriteFile(f.exe, []byte("old binary"), 0o755)
	f.u = &Updater{Current: "2026.10.18.0900", BaseURL: srv.URL, Channel: testChannel, PubKey: pub, Exe: f.exe,
		Backup: func(_ context.Context, v string) error {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.backups = append(f.backups, v)
			return nil
		},
		Restart: func() { f.restarts <- struct{}{} },
	}
	if err := f.u.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	return f
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// settle waits until the update leaves the installing state.
func settle(t *testing.T, u *Updater) Status {
	t.Helper()
	for range 500 {
		if s := u.Status(); s.State != StateInstalling {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("update never finished")
	return Status{}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplyHappy(t *testing.T) {
	bin := []byte("new binary " + strings.Repeat("x", 100000))
	f := newFakeShop(t, bin, sum(bin), nil)
	if err := f.u.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-f.restarts:
	case <-time.After(5 * time.Second):
		t.Fatalf("no restart; status %+v", f.u.Status())
	}
	if got := read(t, f.exe); got != string(bin) {
		t.Fatalf("exe not replaced (%d bytes)", len(got))
	}
	if read(t, f.exe+".old") != "old binary" {
		t.Fatal(".old should hold the previous binary")
	}
	if len(f.backups) != 1 || f.backups[0] != "2026.10.19.1530" {
		t.Fatalf("backups = %v", f.backups)
	}
	if s := f.u.Status(); s.Progress != "Restarting…" {
		t.Fatalf("progress = %q", s.Progress)
	}
	if err := f.u.Apply(context.Background()); err == nil {
		t.Fatal("a second Apply while installing must be refused")
	}
}

func TestApplyWithoutCheck(t *testing.T) {
	u := &Updater{Current: "dev"}
	if err := u.Apply(context.Background()); err == nil || !strings.Contains(err.Error(), "check first") {
		t.Fatalf("err = %v", err)
	}
}

func assertUntouched(t *testing.T, f *fakeShop, s Status) {
	t.Helper()
	if s.State != StateFailed || !strings.HasPrefix(s.Message, "Update failed, nothing changed:") {
		t.Fatalf("status = %+v", s)
	}
	if read(t, f.exe) != "old binary" {
		t.Fatal("exe must be untouched")
	}
	for _, p := range []string{f.exe + ".old", f.exe + ".new"} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("%s should not exist", filepath.Base(p))
		}
	}
	if strings.Contains(s.Message, testChannel) {
		t.Fatalf("message leaks the channel: %q", s.Message)
	}
	select {
	case <-f.restarts:
		t.Fatal("must not restart")
	default:
	}
}

func TestApplyBadHash(t *testing.T) {
	bin := []byte("new binary")
	f := newFakeShop(t, bin, sum([]byte("something else")), nil)
	f.u.Apply(context.Background())
	assertUntouched(t, f, settle(t, f.u))
	if len(f.backups) != 0 {
		t.Fatal("no backup before a bad download")
	}
}

func TestApplyStall(t *testing.T) {
	defer func(l, p time.Duration) { stallLimit, stallPoll = l, p }(stallLimit, stallPoll)
	stallLimit, stallPoll = 200*time.Millisecond, 20*time.Millisecond
	release := make(chan struct{})
	defer close(release)
	bin := []byte("new binary")
	f := newFakeShop(t, bin, sum(bin), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write([]byte{0x1f, 0x8b})
		w.(http.Flusher).Flush()
		select { // hang until the client gives up
		case <-r.Context().Done():
		case <-release:
		}
	})
	f.u.Apply(context.Background())
	s := settle(t, f.u)
	assertUntouched(t, f, s)
	if !strings.Contains(s.Message, "stalled") {
		t.Fatalf("message = %q", s.Message)
	}
}

func TestApplyBackupFails(t *testing.T) {
	bin := []byte("new binary")
	f := newFakeShop(t, bin, sum(bin), nil)
	f.u.Backup = func(context.Context, string) error { return errors.New("disk full") }
	f.u.Apply(context.Background())
	s := settle(t, f.u)
	assertUntouched(t, f, s)
	if !strings.Contains(s.Message, "disk full") {
		t.Fatalf("message = %q", s.Message)
	}
}
