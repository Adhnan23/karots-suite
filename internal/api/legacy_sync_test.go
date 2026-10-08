package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The Stock Capture app: log in at the karots-pos path, read the token from
// the body, pull the catalogue with it as a Bearer header.
func TestLegacySync(t *testing.T) {
	te := newTestEnv(t)
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	te.seedUser(t, phone, "cashier")
	rec := te.do(http.MethodPost, "/api/auth/login", map[string]string{"phone": phone, "pin": "1234"}, nil)
	var login struct {
		Data struct {
			AccessToken string `json:"access_token"`
		}
	}
	json.Unmarshal(rec.Body.Bytes(), &login) //nolint:errcheck
	if rec.Code != http.StatusOK || login.Data.AccessToken == "" {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, "/api/auth/login", map[string]string{"phone": phone, "pin": "9999"}, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong PIN: %d", rec.Code)
	}
	get := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/sync/catalog", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r := httptest.NewRecorder()
		te.e.ServeHTTP(r, req)
		return r
	}
	if r := get(""); r.Code != http.StatusUnauthorized {
		t.Fatalf("no token: %d", r.Code)
	}
	r := get(login.Data.AccessToken)
	var cat struct{ Data []map[string]any }
	json.Unmarshal(r.Body.Bytes(), &cat) //nolint:errcheck
	if r.Code != http.StatusOK || len(cat.Data) == 0 {
		t.Fatalf("catalog: %d %.200s", r.Code, r.Body)
	}
	for _, k := range []string{"id", "name", "category", "unit", "selling_price", "stock_qty", "is_service"} {
		if _, ok := cat.Data[0][k]; !ok {
			t.Errorf("catalog row has no %q: %v", k, cat.Data[0])
		}
	}
}
