package alternatives

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// testAPI mounts the routes the way the server does, with a stand-in for the
// admin guard (?cashier=1 is refused on admin routes).
func testAPI(t *testing.T, db *sqlx.DB) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	mux := plugin.NewMux()
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db)}, mux, e))
	x := e.Group("/api/v1/x")
	admin := e.Group("/api/v1/x", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if c.QueryParam("cashier") == "1" {
				return c.NoContent(403)
			}
			return next(c)
		}
	})
	mux.Mount(x, x, admin)
	return func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/x/alternatives"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("%d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}

func TestAPI(t *testing.T) {
	_, db, tag := testStore(t)
	do := testAPI(t, db)
	king := product(t, db, "Kingston "+tag, "0", false, true)
	sand := product(t, db, "SanDisk "+tag, "7", false, true)

	for _, r := range []struct{ m, p string }{{"GET", "?cashier=1"}, {"POST", "?cashier=1"}, {"GET", "/1?cashier=1"}, {"DELETE", "/members/1?cashier=1"}, {"GET", "/product/1?cashier=1"}} {
		if rec := do(r.m, r.p, "{}"); rec.Code != 403 {
			t.Errorf("%s %s as cashier = %d, want 403", r.m, r.p, rec.Code)
		}
	}
	if rec := do("POST", "", `{"name":" "}`); rec.Code != 422 {
		t.Fatalf("blank name = %d", rec.Code)
	}
	g := decode[GroupDTO](t, do("POST", "", fmt.Sprintf(`{"name":"USB %s"}`, tag)))
	if len(g.Tiers) != 1 || g.Tiers[0].Name != "" {
		t.Fatalf("created: %+v", g)
	}
	tid := g.Tiers[0].ID
	for _, p := range []int64{king, sand} {
		if rec := do("POST", fmt.Sprintf("/tiers/%d/members", tid), fmt.Sprintf(`{"productId":%d}`, p)); rec.Code != 200 {
			t.Fatalf("add member = %d %s", rec.Code, rec.Body)
		}
	}
	other := decode[GroupDTO](t, do("POST", "", fmt.Sprintf(`{"name":"Other %s"}`, tag)))
	rec := do("POST", fmt.Sprintf("/tiers/%d/members", other.Tiers[0].ID), fmt.Sprintf(`{"productId":%d}`, king))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "IN_OTHER_GROUP") || !strings.Contains(rec.Body.String(), "USB "+tag) {
		t.Fatalf("in another group = %d %s", rec.Code, rec.Body)
	}
	g = decode[GroupDTO](t, do("PUT", fmt.Sprintf("/tiers/%d", tid), `{"name":"Genuine","reorderLevel":10}`))
	if g.Tiers[0].Name != "Genuine" || !g.Tiers[0].Low || g.Tiers[0].Total != "7" || len(g.Tiers[0].Members) != 2 {
		t.Fatalf("tier edit: %+v", g)
	}
	m := decode[map[string]MembershipDTO](t, do("GET", fmt.Sprintf("/membership?ids=%d,%d,x", king, sand), ""))
	if m[fmt.Sprint(king)].Group != "USB "+tag || m[fmt.Sprint(sand)].Tier != "Genuine" {
		t.Fatalf("membership: %+v", m)
	}
	if all := decode[map[string]MembershipDTO](t, do("GET", "/membership", "")); all[fmt.Sprint(king)].TierID != tid {
		t.Fatalf("all memberships: %d rows", len(all))
	}
	if p := decode[MembershipDTO](t, do("GET", fmt.Sprintf("/product/%d", king), "")); p.TierID != tid || p.GroupID != g.ID {
		t.Fatalf("product: %+v", p)
	}
	// The till (cashier route) offers SanDisk for the out-of-stock Kingston.
	alts := decode[[]AltDTO](t, do("GET", fmt.Sprintf("/till/%d?cashier=1", king), ""))
	if len(alts) != 1 || alts[0].ProductID != sand || alts[0].Tier != "Genuine" || alts[0].SellingPrice != "100.00" {
		t.Fatalf("till: %+v", alts)
	}
	if rec := do("DELETE", fmt.Sprintf("/members/%d", king), ""); rec.Code != 204 {
		t.Fatalf("remove = %d", rec.Code)
	}
	if rec := do("DELETE", fmt.Sprintf("/%d", g.ID), ""); rec.Code != 204 {
		t.Fatalf("delete = %d", rec.Code)
	}
	if rec := do("GET", fmt.Sprintf("/%d", g.ID), ""); rec.Code != 404 {
		t.Fatalf("deleted group = %d", rec.Code)
	}
	var n int
	db.Get(&n, `SELECT count(*) FROM audit_log WHERE (entity = 'alt_group' AND entity_id = $1) OR (entity = 'alt_tier' AND entity_id = $2)
		OR (entity = 'alt_member' AND entity_id IN ($3, $4))`, fmt.Sprint(g.ID), fmt.Sprint(tid), fmt.Sprint(king), fmt.Sprint(sand)) //nolint:errcheck
	if n != 6 {
		t.Fatalf("audit rows = %d, want 6 (create, 2 adds, tier edit, remove, delete)", n)
	}
}
