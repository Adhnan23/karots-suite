package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/products"
	"karots-suite/internal/plugin"
)

// fakeExtras is a stand-in plugin's product extras: it refuses {"ok":false} and
// records every Save. Registered once for the package (hooks are global).
var (
	fakeExtrasOnce sync.Once
	fakeExtrasMu   sync.Mutex
	fakeSaves      []plugin.ExtraCtx
)

func useFakeExtras() {
	fakeExtrasOnce.Do(func() {
		plugin.NewRegistry(plugin.Core{}, plugin.NewMux(), nil).AddProductExtras(plugin.ProductExtras{
			Key: "fake",
			Validate: func(_ context.Context, _ plugin.ExtraCtx, raw json.RawMessage) error {
				var v struct{ OK bool }
				if json.Unmarshal(raw, &v) != nil || !v.OK {
					return apperr.Validation("fake says no")
				}
				return nil
			},
			Save: func(_ context.Context, x plugin.ExtraCtx, _ json.RawMessage) error {
				fakeExtrasMu.Lock()
				defer fakeExtrasMu.Unlock()
				fakeSaves = append(fakeSaves, x)
				return nil
			},
		})
	})
}

func lastFakeSave(t *testing.T) plugin.ExtraCtx {
	t.Helper()
	fakeExtrasMu.Lock()
	defer fakeExtrasMu.Unlock()
	if len(fakeSaves) == 0 {
		t.Fatal("Save was not called")
	}
	return fakeSaves[len(fakeSaves)-1]
}

func TestProductExtras(t *testing.T) {
	useFakeExtras()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009532", "admin")
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID) //nolint:errcheck
	name := fmt.Sprintf("T extras %d", time.Now().UnixNano())
	count := func() (n int) {
		te.db.Get(&n, `SELECT count(*) FROM products WHERE name LIKE $1`, name+"%") //nolint:errcheck
		return
	}
	extra := func(ok bool) map[string]json.RawMessage {
		return map[string]json.RawMessage{"fake": json.RawMessage(fmt.Sprintf(`{"ok":%v}`, ok))}
	}
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, name+"%") //nolint:errcheck
		te.db.Exec(`DELETE FROM stock_batches WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, name+"%")   //nolint:errcheck
		te.db.Exec(`DELETE FROM stock WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, name+"%")           //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE name LIKE $1`, name+"%")                                                      //nolint:errcheck
	})

	// Refused before anything is written.
	req := ProductRequest{Name: name, CategoryID: cat, UnitID: 1, SellingPrice: "10", Extra: extra(false)}
	if rec := te.do(http.MethodPost, "/api/v1/admin/products", req, ck); rec.Code != http.StatusUnprocessableEntity || count() != 0 {
		t.Fatalf("refused extra: %d, %d rows", rec.Code, count())
	}
	// A key no plugin owns.
	req.Extra = map[string]json.RawMessage{"nope": json.RawMessage(`{}`)}
	if rec := te.do(http.MethodPost, "/api/v1/admin/products", req, ck); rec.Code != http.StatusBadRequest || count() != 0 {
		t.Fatalf("unknown key: %d, %d rows", rec.Code, count())
	}
	// Accepted: saved after the product, with its id.
	req.Extra = extra(true)
	rec := te.do(http.MethodPost, "/api/v1/admin/products", req, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	id := decode[ProductSaved](t, rec).ID
	if x := lastFakeSave(t); x.ProductID != id || !x.IsNew || x.CategoryID != cat {
		t.Fatalf("create save: %+v", x)
	}
	// Update: not new; a refusal leaves the product as it was.
	path := "/api/v1/admin/products/" + strconv.FormatInt(id, 10)
	req.SellingPrice = "99"
	req.Extra = extra(false)
	if rec := te.do(http.MethodPut, path, req, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("refused update: %d", rec.Code)
	}
	if got := decode[AdminProduct](t, te.do(http.MethodGet, path, nil, ck)); got.SellingPrice != "10.00" {
		t.Fatalf("refused update wrote: %s", got.SellingPrice)
	}
	req.Extra = extra(true)
	if rec := te.do(http.MethodPut, path, req, ck); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if x := lastFakeSave(t); x.ProductID != id || x.IsNew {
		t.Fatalf("update save: %+v", x)
	}
	// No extra at all: nothing called, works as before.
	req.Extra = nil
	if rec := te.do(http.MethodPut, path, req, ck); rec.Code != http.StatusOK {
		t.Fatalf("plain update: %d", rec.Code)
	}
	// Intake create carries extras too.
	rec = te.do(http.MethodPost, "/api/v1/admin/inventory/intake/create", IntakeCreateRequest{
		Product: ProductRequest{Name: name + " intake", CategoryID: cat, UnitID: 1, SellingPrice: "5", Extra: extra(true)}, Quantity: "2"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("intake: %d %s", rec.Code, rec.Body)
	}
	if x := lastFakeSave(t); x.ProductID != decode[IntakeItem](t, rec).ProductID || !x.IsNew {
		t.Fatalf("intake save: %+v", x)
	}
}

func TestProductListMeta(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009533", "admin")
	var name string
	te.db.Get(&name, `SELECT name FROM products WHERE id=$1`, f.plainID) //nolint:errcheck
	list := func() ProductList {
		return decode[ProductList](t, te.do(http.MethodGet, "/api/v1/admin/products?q="+url.QueryEscape(name), nil, ck))
	}
	for _, r := range list().Items {
		if r.Meta != "" {
			t.Fatalf("meta with no provider: %+v", r)
		}
	}
	calls := 0
	products.MetaProvider = func(_ context.Context, ids []int64) map[int64]string {
		calls++
		return map[int64]string{f.plainID: "Grade: B"}
	}
	t.Cleanup(func() { products.MetaProvider = nil })
	l := list()
	var got string
	for _, r := range l.Items {
		if r.ID == f.plainID {
			got = r.Meta
		}
	}
	if got != "Grade: B" || calls != 1 {
		t.Fatalf("meta %q after %d calls (want one call for the page)", got, calls)
	}
}
