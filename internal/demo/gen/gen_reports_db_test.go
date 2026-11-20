package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/app"
	"karots-suite/internal/config"
	"karots-suite/internal/datetime"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/ledgercheck/ltest"
)

// A grocery's year through the real API: every report agrees with the raw rows
// and with each other, month by month and over the whole year (area 7,
// docs/rules/reports.md — ltest.Agree).
func TestYearReportsAgree(t *testing.T) {
	d, now := grocery(t)
	var sales, returns, expenses int
	for q, n := range map[string]*int{`SELECT count(*) FROM sales`: &sales, `SELECT count(*) FROM sale_returns`: &returns,
		`SELECT count(*) FROM expenses`: &expenses} {
		if err := d.Get(n, q); err != nil {
			t.Fatal(err)
		}
	}
	if sales < 1000 || returns == 0 || expenses == 0 {
		t.Fatalf("the year is too thin to prove anything: %d sales, %d returns, %d expenses", sales, returns, expenses)
	}
	t.Logf("a year of %d sales, %d returns, %d expenses", sales, returns, expenses)

	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	v := app.SystemPINValidator(nil)
	h := app.BuildServer(d, cfg, v)
	phone := fmt.Sprintf("0779%06d", time.Now().UnixNano()%1_000_000)
	if _, err := app.NewServices(d, cfg, v).Auth.CreateUser(t.Context(), auth.CreateUserInput{Name: "Year auditor", Phone: phone, Role: "admin", PIN: "1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec(`UPDATE users SET must_change_pin = false WHERE phone = $1`, phone); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Exec(`UPDATE users SET is_active = false WHERE phone = $1`, phone) }) //nolint:errcheck
	var cookies []*http.Cookie
	call := func(method, path string, body []byte, out any) error {
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.RemoteAddr = "127.0.0.1:1"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Ks-Device", "year-auditor")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if set := w.Result().Cookies(); len(set) > 0 {
			cookies = set
		}
		if w.Code < 200 || w.Code > 299 {
			return fmt.Errorf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		if out == nil {
			return nil
		}
		return json.Unmarshal(w.Body.Bytes(), out)
	}
	if err := call("POST", "/api/v1/auth/login", []byte(`{"phone":"`+phone+`","pin":"1234"}`), nil); err != nil {
		t.Fatal(err)
	}
	get := func(path string, out any) error { return call("GET", path, nil, out) }

	today := now.In(datetime.Location)
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, datetime.Location).AddDate(-1, 0, 0)
	for m := first; !m.After(today); m = m.AddDate(0, 1, 0) {
		ltest.Agree(t, d, get, m.Format(time.DateOnly), m.AddDate(0, 1, -1).Format(time.DateOnly))
	}
	ltest.Agree(t, d, get, first.Format(time.DateOnly), today.Format(time.DateOnly))
}
