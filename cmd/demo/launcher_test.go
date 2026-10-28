package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/demo/shops"
	"karots-suite/internal/support"
)

// fakeRunner is a "child" that is an httptest server.
type fakeRunner struct {
	srv    *httptest.Server
	shop   *shops.Profile
	stops  int
	starts int
	gate   chan struct{} // when set, Start waits on it
}

func (f *fakeRunner) Start(p shops.Profile, progress func(int, string)) error {
	if f.gate != nil {
		<-f.gate
	}
	f.starts++
	f.shop = &p
	progress(100, "ready")
	return nil
}
func (f *fakeRunner) Stop() error     { f.stops++; f.shop = nil; return nil }
func (f *fakeRunner) URL() string     { return f.srv.URL }
func (f *fakeRunner) Control() string { return "" }

func newTestLauncher(t *testing.T, upstream http.HandlerFunc) (*launcher, *fakeRunner, time.Time) {
	t.Helper()
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	r := &fakeRunner{srv: srv}
	now := time.Date(2026, 10, 27, 10, 30, 0, 0, time.UTC)
	l := newLauncher(support.DeriveSeed("test", "DEMO1234"), "DEMO1234", r, nil, nil)
	l.now = func() time.Time { return now }
	return l, r, now
}

func unlock(t *testing.T, h http.Handler, code string) *http.Cookie {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/__demo/unlock", strings.NewReader(`{"code":"`+code+`"}`)))
	for _, c := range w.Result().Cookies() {
		if c.Name == lockCookie {
			return c
		}
	}
	return nil
}

func TestUnlockAcceptsCurrentAndPreviousWindow(t *testing.T) {
	l, _, now := newTestLauncher(t, nil)
	seed := support.DeriveSeed("test", "DEMO1234")
	for _, at := range []time.Time{now, now.Add(-time.Hour)} {
		if c := unlock(t, l.handler(), support.Code(seed, at)); c == nil {
			t.Fatalf("code for %v refused", at)
		}
	}
}

func TestUnlockRejectsOlder(t *testing.T) {
	l, _, now := newTestLauncher(t, nil)
	seed := support.DeriveSeed("test", "DEMO1234")
	if c := unlock(t, l.handler(), support.Code(seed, now.Add(-3*time.Hour))); c != nil {
		t.Fatal("a three-hour-old code unlocked the demo")
	}
}

func TestUnlockSlowsGuessing(t *testing.T) {
	l, _, now := newTestLauncher(t, nil)
	for i := 0; i < 5; i++ {
		unlock(t, l.handler(), "000000")
	}
	if c := unlock(t, l.handler(), support.Code(support.DeriveSeed("test", "DEMO1234"), now)); c != nil {
		t.Fatal("the right code after five wrong ones in a minute should wait")
	}
}

func TestEveryRouteNeedsCookie(t *testing.T) {
	l, _, _ := newTestLauncher(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "child") })
	h := l.handler()
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/", http.StatusFound},
		{"GET", "/till", http.StatusFound},
		{"GET", "/print/receipt", http.StatusFound},
		{"GET", "/api/v1/me", http.StatusUnauthorized},
		{"GET", "/__demo/state", http.StatusUnauthorized},
		{"POST", "/__demo/cheat/busy", http.StatusUnauthorized},
		{"GET", "/__demo/prints", http.StatusUnauthorized},
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/__demo/lock", nil))
	if w.Code != http.StatusOK {
		t.Errorf("lock page = %d, want 200", w.Code)
	}
}

func startShop(t *testing.T, l *launcher, now time.Time) (http.Handler, *http.Cookie) {
	t.Helper()
	h := l.handler()
	c := unlock(t, h, support.Code(support.DeriveSeed("test", "DEMO1234"), now))
	req := httptest.NewRequest("POST", "/__demo/start", strings.NewReader(`{"shop":"pharmacy"}`))
	req.AddCookie(c)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start = %d %s", w.Code, w.Body)
	}
	l.waitIdle()
	return h, c
}

func TestProxyInjectsChromeIntoHTML(t *testing.T) {
	l, _, now := newTestLauncher(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, `<!doctype html><html><head><title>K</title></head><body></body></html>`)
	})
	h, c := startShop(t, l, now)
	req := httptest.NewRequest("GET", "/login", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body := w.Body.String()
	for _, want := range []string{
		`<meta name="ks-plugins" content="pharmacy,alternatives,productplus,requests">`,
		`/__demo/ui/overlay.js`,
		`/__demo/ui/overlay.css`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("page lacks %q:\n%s", want, body)
		}
	}
	if cl := w.Header().Get("Content-Length"); cl != "" && cl != itoa(len(body)) {
		t.Fatalf("Content-Length %s, body %d", cl, len(body))
	}
}

func TestProxyLeavesJSONAlone(t *testing.T) {
	l, _, now := newTestLauncher(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"head":"</head>"}`)
	})
	h, c := startShop(t, l, now)
	req := httptest.NewRequest("GET", "/api/v1/me", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Body.String() != `{"head":"</head>"}` {
		t.Fatalf("JSON changed: %s", w.Body)
	}
}

func TestStateAndSwitch(t *testing.T) {
	l, r, now := newTestLauncher(t, nil)
	h, c := startShop(t, l, now)
	var st State
	req := httptest.NewRequest("GET", "/__demo/state", nil)
	req.AddCookie(c)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st.Phase != "ready" || st.Shop == nil || st.Shop.Key != "pharmacy" || st.Owner.Phone == "" {
		t.Fatalf("state = %+v", st)
	}
	before := r.stops
	req = httptest.NewRequest("POST", "/__demo/switch", nil)
	req.AddCookie(c)
	h.ServeHTTP(httptest.NewRecorder(), req)
	l.waitIdle()
	if r.stops != before+1 || r.shop != nil || l.state().Phase != "picker" {
		t.Fatalf("after switch: stops %d, phase %s", r.stops, l.state().Phase)
	}
}

func TestDoubleClickBuildsOnce(t *testing.T) {
	l, r, _ := newTestLauncher(t, nil)
	r.gate = make(chan struct{})
	p, _ := shops.ByKey("pharmacy")
	l.restart(p)
	l.restart(p) // the second click of a double-click
	close(r.gate)
	l.waitIdle()
	if r.starts != 1 || l.state().Phase != "ready" {
		t.Fatalf("starts %d, phase %s", r.starts, l.state().Phase)
	}
}

func TestNewestPickWins(t *testing.T) {
	l, r, _ := newTestLauncher(t, nil)
	r.gate = make(chan struct{})
	a, _ := shops.ByKey("pharmacy")
	b := shops.BuiltIn()[0]
	if b.Key == a.Key {
		b = shops.BuiltIn()[1]
	}
	l.restart(a)
	l.restart(b)
	close(r.gate)
	l.waitIdle()
	st := l.state()
	if st.Phase != "ready" || st.Shop.Key != b.Key || r.shop.Key != b.Key || st.Progress != 100 {
		t.Fatalf("state %s/%s, child %s", st.Phase, st.Shop.Key, r.shop.Key)
	}
}

func TestCustomShopRejectsUnknownPicks(t *testing.T) {
	l, _, now := newTestLauncher(t, nil)
	c := unlock(t, l.handler(), support.Code(support.DeriveSeed("test", "DEMO1234"), now))
	for _, body := range []string{
		`{"custom":{"plugins":["nope"]}}`,
		`{"custom":{"plugins":[],"palette":"x\"><script>"}}`,
		`{"custom":{"plugins":[],"receiptStyle":"../x"}}`,
	} {
		req := httptest.NewRequest("POST", "/__demo/start", strings.NewReader(body))
		req.AddCookie(c)
		w := httptest.NewRecorder()
		l.handler().ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	if strings.Contains(string(inject([]byte("</head>"), `"><x`)), `"><x`) {
		t.Error("plugins not escaped")
	}
}
