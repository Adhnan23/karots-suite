package ltest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/config"
	"karots-suite/internal/features/auth"
)

// Shop is a throwaway shop on the real server for tests outside internal/battle
// — a plugin's battle tests (docs/rules/plugins): its own database (NewDB, with
// every plugin the test binary compiles in), an admin and a cashier signed in,
// and a category to put products in. Units 1 (pcs) and 2 (kg) are the seed's.
type Shop struct {
	DB             *sqlx.DB
	H              http.Handler
	Admin, Cashier *Client
	Cat            int64
}

// NewShop opens a shop; skipped with -short, as the battle is.
func NewShop(t T) *Shop {
	helper(t)
	if testing.Short() {
		t.Skip("battle: skipped with -short")
	}
	return NewShopOn(t, NewDB(t))
}

// NewShopOn opens the shop on a database already made (a faulty one, say).
func NewShopOn(t T, db *sqlx.DB) *Shop {
	helper(t)
	ctx := context.Background()
	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	v := app.SystemPINValidator(nil)
	svc := app.NewServices(db, cfg, v)
	s := &Shop{DB: db, H: app.BuildServer(db, cfg, v)}
	for _, u := range []struct{ name, phone, role string }{{"Admin", "0770000001", "admin"}, {"Till One", "0770000002", "cashier"}} {
		if _, err := svc.Auth.CreateUser(ctx, auth.CreateUserInput{Name: u.name, Phone: u.phone, Role: u.role, PIN: "1234",
			CanManageCredit: "on", CanSeeCost: "on"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET must_change_pin = false`); err != nil {
		t.Fatal(err)
	}
	var err error
	if s.Admin, err = NewClient(s.H, "0770000001", "1234", fmt.Sprintf("shop-%p-a", s)); err != nil {
		t.Fatal(err)
	}
	if s.Cashier, err = NewClient(s.H, "0770000002", "1234", fmt.Sprintf("shop-%p-c", s)); err != nil {
		t.Fatal(err)
	}
	var c struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "Shop"}, &c))
	s.Cat = c.ID
	return s
}

// Must fails the test on err.
func (s *Shop) Must(t T, err error) {
	helper(t)
	if err != nil {
		t.Fatal(err)
	}
}

// Product makes a product (pcs, cost 10) with stock on hand ("0" = none) and
// returns its id; extra fields override the defaults.
func (s *Shop) Product(t T, name, price, stock string, extra map[string]any) int64 {
	helper(t)
	body := map[string]any{"name": name, "categoryId": s.Cat, "unitId": 1, "costPrice": "10",
		"sellingPrice": price, "wholesalePrice": price, "reorderAuto": false}
	for k, v := range extra {
		body[k] = v
	}
	var p struct{ ID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/products", body, &p))
	if stock != "0" && stock != "" {
		s.Must(t, s.Admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p.ID, "newQuantity": stock, "note": "shop"}, nil))
	}
	return p.ID
}

// OpenTill starts c's till with Rs 5,000 float.
func (s *Shop) OpenTill(t T, c *Client) {
	helper(t)
	s.Must(t, c.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}}, nil))
}

// Audit fails the test with every rule the shop's data breaks — core's and
// every compiled-in plugin's (ledgercheck.Add).
func (s *Shop) Audit(t T) {
	helper(t)
	Check(t, context.Background(), s.DB)
}

// Client calls the shop's real API in-process, signed in as one person.
type Client struct {
	h      http.Handler
	device string
	cookie []*http.Cookie
}

// NewClient signs in phone / pin on h.
func NewClient(h http.Handler, phone, pin, device string) (*Client, error) {
	c := &Client{h: h, device: device}
	if err := c.Do("POST", "/api/v1/auth/login", map[string]string{"phone": phone, "pin": pin}, nil); err != nil {
		return nil, fmt.Errorf("sign in %s: %w", phone, err)
	}
	return c, nil
}

// Do sends in as JSON and decodes the reply into out (nil = ignore); a reply
// outside 2xx is a *StatusError carrying the API's message.
func (c *Client) Do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, body)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ks-Device", c.device)
	for _, k := range c.cookie {
		req.AddCookie(k)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	if set := w.Result().Cookies(); len(set) > 0 {
		c.cookie = set
	}
	if w.Code < 200 || w.Code > 299 {
		return &StatusError{Code: w.Code, Msg: fmt.Sprintf("%s %s: %d %s", method, path, w.Code, bytes.TrimSpace(w.Body.Bytes()))}
	}
	if out == nil || w.Body.Len() == 0 {
		return nil
	}
	return json.Unmarshal(w.Body.Bytes(), out)
}

// StatusError is a reply outside 2xx.
type StatusError struct {
	Code int
	Msg  string
}

func (e *StatusError) Error() string { return e.Msg }
