package pocketcashier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"karots-suite/internal/config"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/heldsales"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

const testSecret = "test-secret-test-secret-test-secret"

// env is the plugin mounted as the server mounts it: public, cashier and
// admin groups with real JWT and role guards.
type env struct {
	t     *testing.T
	db    *sqlx.DB
	e     *echo.Echo
	auth  *auth.Service
	p     *Plugin
	kicks int
}

func newEnv(t *testing.T) *env {
	t.Helper()
	db := testDB(t)
	en := &env{t: t, db: db, e: echo.New(), auth: auth.NewService(db, testSecret, time.Hour, time.Hour)}
	en.e.Validator = validator.New()
	en.e.HTTPErrorHandler = middleware.ErrorHandler(false)
	middleware.SetUserValidator(middleware.DBUserValidator(db))
	ss := sales.NewService(db).WithDrawerKick(func(context.Context) { en.kicks++ })
	cf := cashflow.NewService(db, ss)
	cr := cashregister.NewService(db, ss).WithAudit(audit.NewService(db)).WithLockerLeg(cf.TillLockerLeg).
		WithDrawerKick(func(context.Context) { en.kicks++ })
	mux := plugin.NewMux()
	en.p = &Plugin{}
	en.p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Cfg: &config.Config{ServerPort: 3000, Version: "test"}, Audit: audit.NewService(db),
		Settings: settings.NewService(db), CashRegister: cr, Sales: ss, Cashflow: cf, Auth: en.auth, Held: heldsales.NewService(db)}, mux, en.e))
	jwt := middleware.JWTAuth(testSecret)
	v1 := en.e.Group("/api/v1")
	mux.Mount(v1.Group("/x"), v1.Group("/x", jwt), v1.Group("/x", jwt, middleware.RequireRole(auth.RoleAdmin, auth.RoleManager)))
	t.Cleanup(func() { plugin.SetOneDeviceForTest(false) })
	return en
}

// user makes an active user with PIN 1234 and returns its id and a session cookie.
func (en *env) user(role string) (int64, *http.Cookie) {
	en.t.Helper()
	ctx := context.Background()
	u, err := en.auth.CreateUser(ctx, auth.CreateUserInput{Name: "T " + role, Phone: fmt.Sprintf("06%08d", time.Now().UnixNano()%100_000_000), Role: role, PIN: "1234"})
	if err != nil {
		en.t.Fatal(err)
	}
	en.t.Cleanup(func() { en.db.Exec(`DELETE FROM users WHERE id = $1`, u.ID) }) //nolint:errcheck
	tok, err := en.auth.AccessTokenFor(u)
	if err != nil {
		en.t.Fatal(err)
	}
	return u.ID, &http.Cookie{Name: middleware.CookieName, Value: tok}
}

// doHeader sends a bare GET with the phone secret and one more header.
func (en *env) doHeader(method, path, phone, k, v string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/x/pocketcashier"+path, nil)
	req.Header.Set(phoneHeader, phone)
	req.Header.Set(k, v)
	rec := httptest.NewRecorder()
	en.e.ServeHTTP(rec, req)
	return rec
}

// do sends a JSON request with optional cookie and phone secret.
func (en *env) do(method, path string, body any, ck *http.Cookie, phone string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, "/api/v1/x/pocketcashier"+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if ck != nil {
		req.AddCookie(ck)
	}
	if phone != "" {
		req.Header.Set(phoneHeader, phone)
	}
	rec := httptest.NewRecorder()
	en.e.ServeHTTP(rec, req)
	return rec
}

func decodeAs[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}

func errCodeOf(rec *httptest.ResponseRecorder) string {
	var b struct{ Error struct{ Code string } }
	json.Unmarshal(rec.Body.Bytes(), &b) //nolint:errcheck
	return b.Error.Code
}

// pairPhone pairs a new phone and returns its id and secret (pending).
func (en *env) pairPhone() (code, secret, device string) {
	en.t.Helper()
	n := time.Now().UnixNano()
	secret, device = fmt.Sprintf("secret-%d-0123456789abcdef0123456789", n), fmt.Sprintf("dev-%d", n)
	rec := en.do(http.MethodPost, "/pair", PairRequest{Name: "Phone T", Device: device, Secret: secret}, nil, "")
	if rec.Code != http.StatusOK {
		en.t.Fatalf("pair %d %s", rec.Code, rec.Body)
	}
	en.t.Cleanup(func() { en.db.Exec(`DELETE FROM pocket_phones WHERE device = $1`, device) }) //nolint:errcheck
	return decodeAs[PairReply](en.t, rec).Code, secret, device
}

// activePhone pairs and approves a phone (as a fresh admin).
func (en *env) activePhone() (secret, device string) {
	en.t.Helper()
	code, secret, device := en.pairPhone()
	_, admin := en.user("admin")
	if rec := en.do(http.MethodPost, "/phones/approve", ApproveRequest{Code: code}, admin, ""); rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		en.t.Fatalf("approve %d %s", rec.Code, rec.Body)
	}
	return secret, device
}
