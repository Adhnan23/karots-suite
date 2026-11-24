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
	te.stockTakeOn(t) // intake needs it: never lean on another test having turned it on
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

// The till's quick item and a purchase's new product line take plugin extras like the product form: refused
// before anything is written, saved after with the new product (the Catalogue plugin's picks, CTL-6).
func TestQuickItemAndIntakeProductTakeExtras(t *testing.T) {
	useFakeExtras()
	te := newTestEnv(t)
	te.seedTill(t)
	ck := te.login(t, "0770009533", "admin")
	name := fmt.Sprintf("T qx %d", time.Now().UnixNano())
	count := func(n string) (c int) {
		te.db.Get(&c, `SELECT count(*) FROM products WHERE name = $1`, n) //nolint:errcheck
		return
	}
	extra := func(ok bool) map[string]json.RawMessage {
		return map[string]json.RawMessage{"fake": json.RawMessage(fmt.Sprintf(`{"ok":%v}`, ok))}
	}
	for _, c := range []struct {
		path string
		body func(n string, ok bool) any
	}{
		{"/api/v1/till/quick-item", func(n string, ok bool) any {
			return QuickItemRequest{Name: n, Price: "10", Qty: "1", Extra: extra(ok)}
		}},
		{"/api/v1/till/intake-products", func(n string, ok bool) any {
			return IntakeProductRequest{Name: n, CostPrice: "5", SellingPrice: "10", Extra: extra(ok)}
		}},
	} {
		n := name + " " + c.path
		if rec := te.do(http.MethodPost, c.path, c.body(n, false), ck); rec.Code < 400 || count(n) != 0 {
			t.Fatalf("%s: a refused extra still saved (%d, %d products)", c.path, rec.Code, count(n))
		}
		rec := te.do(http.MethodPost, c.path, c.body(n, true), ck)
		if rec.Code >= 300 {
			t.Fatalf("%s: %d %s", c.path, rec.Code, rec.Body)
		}
		var id int64
		te.db.Get(&id, `SELECT id FROM products WHERE name = $1`, n) //nolint:errcheck
		if x := lastFakeSave(t); x.ProductID != id || !x.IsNew {
			t.Fatalf("%s: Save got %+v, want product %d, new", c.path, x, id)
		}
	}
}
