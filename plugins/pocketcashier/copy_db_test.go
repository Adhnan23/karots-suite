package pocketcashier

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func (en *env) withCopy() {
	en.p.core.TillCopy = func(ctx context.Context) (any, error) {
		people := []map[string]any{{"role": "cashier"}}
		v, _ := en.p.core.TillCopyVersion(ctx)
		return map[string]any{"version": v, "people": people}, nil
	}
	en.p.core.TillCopyVersion = func(ctx context.Context) (string, error) {
		var v string
		err := en.db.GetContext(ctx, &v, `SELECT COALESCE(max(id), 0)::text FROM sales`)
		return v, err
	}
}

func body(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	r := io.Reader(rec.Body)
	if rec.Header().Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(rec.Body)
		if err != nil {
			t.Fatal(err)
		}
		r = zr
	}
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestCopyNeedsActivePhone(t *testing.T) {
	en := newEnv(t)
	en.withCopy()
	if rec := en.do(http.MethodGet, "/copy", nil, nil, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no phone: %d", rec.Code)
	}
	secret, device := en.activePhone()
	rec := en.do(http.MethodGet, "/copy", nil, nil, secret)
	var c struct {
		People []struct{ Role string } `json:"people"`
	}
	if err := json.Unmarshal([]byte(body(t, rec)), &c); err != nil || rec.Code != http.StatusOK || len(c.People) == 0 {
		t.Fatalf("copy %d %v", rec.Code, err)
	}
	var copyAt *time.Time
	en.db.Get(&copyAt, `SELECT copy_at FROM pocket_phones WHERE device = $1`, device) //nolint:errcheck
	if copyAt == nil {
		t.Fatal("copy_at not set")
	}
}

func TestCopyETag(t *testing.T) {
	en := newEnv(t)
	en.withCopy()
	secret, _ := en.activePhone()
	first := en.do(http.MethodGet, "/copy", nil, nil, secret)
	tag := first.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag")
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x/pocketcashier/copy", nil)
	req.Header.Set(phoneHeader, secret)
	req.Header.Set("If-None-Match", tag)
	rec := httptest.NewRecorder()
	en.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("same version: %d", rec.Code)
	}
	en.sale()
	rec = httptest.NewRecorder()
	en.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("after a sale: %d", rec.Code)
	}
}

func TestFeedSendsVersion(t *testing.T) {
	en := newEnv(t)
	en.withCopy()
	secret, _ := en.activePhone()
	saved := feedEvery
	feedEvery = 50 * time.Millisecond
	t.Cleanup(func() { feedEvery = saved })
	srv := httptest.NewServer(en.e)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/x/pocketcashier/feed?ticket="+en.ticket(secret), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("feed: %v %v", res, err)
	}
	defer res.Body.Close()
	// Read the stream in the background; the sale (which may t.Fatal) stays on
	// the test's own goroutine, so nothing fails after the test has ended.
	got := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(res.Body)
		for sc.Scan() {
			if sc.Text() == "event: version" {
				got <- true
				return
			}
		}
		got <- false
	}()
	time.Sleep(200 * time.Millisecond)
	en.sale()
	if !<-got {
		t.Fatal("no version event within 5 s")
	}
}

// sale makes one cash sale of a throwaway service product (bumps the version).
func (en *env) sale() {
	en.t.Helper()
	uid, _ := en.user("cashier")
	var pid int64
	if err := en.db.Get(&pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
		VALUES ('T pocket svc', (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 10, true) RETURNING id`); err != nil {
		en.t.Fatal(err)
	}
	if _, err := en.db.Exec(`INSERT INTO sales (receipt_no, sale_type, subtotal, total, paid_amount, cashier_id) VALUES ('T-'||nextval('sales_receipt_seq'), 'retail', 10, 10, 10, $1)`, uid); err != nil {
		en.t.Fatal(err)
	}
	en.t.Cleanup(func() {
		en.db.Exec(`DELETE FROM sales WHERE cashier_id = $1`, uid) //nolint:errcheck
		en.db.Exec(`DELETE FROM products WHERE id = $1`, pid)      //nolint:errcheck
	})
}

// ticket asks for a one-time feed ticket as the phone.
func (en *env) ticket(secret string) string {
	en.t.Helper()
	rec := en.do(http.MethodPost, "/feed-ticket", nil, nil, secret)
	var out struct{ Ticket string }
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Ticket == "" {
		en.t.Fatalf("feed ticket: %d %s", rec.Code, rec.Body)
	}
	return out.Ticket
}

// The phone's key never goes in a URL (logs keep URLs): the feed takes a
// one-time ticket, and the key in ?t= is refused.
func TestFeedTicketOnce(t *testing.T) {
	en := newEnv(t)
	en.withCopy()
	secret, _ := en.activePhone()
	srv := httptest.NewServer(en.e)
	defer srv.Close()
	open := func(q string) int {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/x/pocketcashier/feed?"+q, nil)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		return res.StatusCode
	}
	if c := open("t=" + secret); c != http.StatusUnauthorized {
		t.Fatalf("key in the URL: %d", c)
	}
	tk := en.ticket(secret)
	if c := open("ticket=" + tk); c != http.StatusOK {
		t.Fatalf("ticket: %d", c)
	}
	if c := open("ticket=" + tk); c != http.StatusUnauthorized {
		t.Fatalf("ticket used twice: %d", c)
	}
}

// An open feed keeps the phone "seen", so a live phone never looks unreachable.
func TestFeedKeepsPhoneSeen(t *testing.T) {
	en := newEnv(t)
	en.withCopy()
	secret, device := en.activePhone()
	saved := feedPing
	feedPing = 50 * time.Millisecond
	t.Cleanup(func() { feedPing = saved })
	srv := httptest.NewServer(en.e)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/v1/x/pocketcashier/feed?ticket="+en.ticket(secret), nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	en.db.Exec(`UPDATE pocket_phones SET last_seen = now() - interval '5 minutes' WHERE device = $1`, device) //nolint:errcheck
	time.Sleep(300 * time.Millisecond)
	var fresh bool
	if err := en.db.Get(&fresh, `SELECT last_seen > now() - interval '5 seconds' FROM pocket_phones WHERE device = $1`, device); err != nil || !fresh {
		t.Fatalf("last_seen not kept fresh (%v)", err)
	}
}
