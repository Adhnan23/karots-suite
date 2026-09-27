package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"karots-suite/internal/config"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

type testEnv struct {
	e  *echo.Echo
	db *sqlx.DB
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := &config.Config{JWTSecret: "test-secret-test-secret-test-secret", JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)

	middleware.SetUserValidator(middleware.DBUserValidator(db))

	a := auth.NewService(db, cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	Register(e, Deps{DB: db, Cfg: cfg, Auth: a, Settings: settings.NewService(db),
		CashRegister: cashregister.NewService(db, sales.NewService(db))})
	return &testEnv{e: e, db: db}
}

// seedUser creates an active user with PIN 1234 and removes it after the test.
func (te *testEnv) seedUser(t *testing.T, phone, role string) int64 {
	t.Helper()
	ctx := context.Background()
	te.db.ExecContext(ctx, `DELETE FROM users WHERE phone=$1`, phone) //nolint:errcheck
	u, err := auth.NewService(te.db, "x", time.Hour, time.Hour).CreateUser(ctx,
		auth.CreateUserInput{Name: "T " + role, Phone: phone, Role: role, PIN: "1234"})
	if err != nil {
		t.Fatal(err)
	}
	te.db.ExecContext(ctx, `UPDATE users SET must_change_pin=false WHERE id=$1`, u.ID)  //nolint:errcheck
	t.Cleanup(func() { te.db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, u.ID) }) //nolint:errcheck
	return u.ID
}

// do sends a JSON request, carrying cookie (may be nil); returns recorder.
func (te *testEnv) do(method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	te.e.ServeHTTP(rec, req)
	return rec
}

func sessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == middleware.CookieName && c.Value != "" {
			return c
		}
	}
	t.Fatalf("no session cookie; status %d body %s", rec.Code, rec.Body.String())
	return nil
}

func errCode(rec *httptest.ResponseRecorder) string {
	var b struct{ Error struct{ Code string } }
	json.Unmarshal(rec.Body.Bytes(), &b) //nolint:errcheck
	return b.Error.Code
}
