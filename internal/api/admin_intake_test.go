package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminIntake(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	const phone = "0770009581"
	clean := func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone=$1)`, phone) //nolint:errcheck
	}
	clean()
	ck := te.login(t, phone, "manager")
	t.Cleanup(clean)
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID) //nolint:errcheck
	name := fmt.Sprintf("T intake %d", time.Now().UnixNano())

	rec := te.do(http.MethodPost, "/api/v1/admin/inventory/intake/create", IntakeCreateRequest{
		Product: ProductRequest{Name: name, CategoryID: cat, UnitID: 1, CostPrice: "20", SellingPrice: "30"}, Quantity: "12"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	item := decode[IntakeItem](t, rec)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM stock_batches WHERE product_id=$1`, item.ProductID) //nolint:errcheck
		te.db.Exec(`DELETE FROM stock WHERE product_id=$1`, item.ProductID)         //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE id=$1`, item.ProductID)              //nolint:errcheck
	})
	if item.StockQty != "12" || item.Kind != "create" {
		t.Fatalf("created: %+v", item)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/inventory/intake/create", IntakeCreateRequest{Product: ProductRequest{Name: name, CategoryID: cat, UnitID: 1}}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name: %d", rec.Code)
	}

	// Restock 5 at a new selling price: the new lot carries it, the product too.
	r := decode[IntakeItem](t, te.do(http.MethodPost, "/api/v1/admin/inventory/intake/restock", IntakeRestockRequest{ProductID: item.ProductID, Quantity: "5", SellingPrice: "35"}, ck))
	var lotPrice, prodPrice float64
	te.db.Get(&lotPrice, `SELECT selling_price FROM stock_batches WHERE product_id=$1 ORDER BY id DESC LIMIT 1`, item.ProductID) //nolint:errcheck
	te.db.Get(&prodPrice, `SELECT selling_price FROM products WHERE id=$1`, item.ProductID)                                      //nolint:errcheck
	if r.StockQty != "17" || lotPrice != 35 || prodPrice != 35 {
		t.Fatalf("restock: %+v lot %v product %v", r, lotPrice, prodPrice)
	}

	// Undo the create: units off, product disabled.
	if rec := te.do(http.MethodPost, "/api/v1/admin/inventory/intake/undo", IntakeUndoRequest{ProductID: item.ProductID, Qty: "12", Kind: "create"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body)
	}
	var active bool
	var qty float64
	te.db.Get(&active, `SELECT is_active FROM products WHERE id=$1`, item.ProductID)  //nolint:errcheck
	te.db.Get(&qty, `SELECT quantity FROM stock WHERE product_id=$1`, item.ProductID) //nolint:errcheck
	if active || qty != 5 {
		t.Fatalf("after undo: active %v qty %v", active, qty)
	}
}
