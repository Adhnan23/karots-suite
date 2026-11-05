package aicatalog

import (
	"context"
	"fmt"
	"testing"

	"karots-suite/internal/features/products"
)

func setupEnv(t *testing.T, name string) (*Plugin, int64) {
	t.Helper()
	db := freshDB(t, name)
	p, _ := testAPI(t, db)
	p.jobs = []Job{setupJob()}
	return p, cat(t, db, "Goods", nil)
}

func TestSetupExpiryAndNull(t *testing.T) {
	p, c := setupEnv(t, "aicatalog_setup_expiry")
	ctx := context.Background()
	a, b, n := prod(t, p.store.db, "Panadol", c), prod(t, p.store.db, "Already yes", c), prod(t, p.store.db, "Unsure", c)
	mustExec(t, p.store.db, nil, `UPDATE products SET has_expiry = true WHERE id = $1`, b)
	batch := Batch{Products: []JobProduct{{ID: a}, {ID: b}, {ID: n}}}
	rows, err := setupParse(ctx, p, batch, fmt.Sprintf(`{"items":[{"id":%d,"has_expiry":true,"pack_name":"","pack_size":0},{"id":%d,"has_expiry":true},{"id":%d,"has_expiry":null},{"id":777,"has_expiry":true}]}`, a, b, n))
	if err != nil || len(rows) != 1 {
		t.Fatalf("same value and unsure give nothing: %+v %v", rows, err)
	}
	if r := rows[0]; r.Ticked || !r.Change || r.Old != "No" || r.New != "Yes" {
		t.Fatalf("a change needs a tick: %+v", r)
	}
	res, err := p.applyJob(ctx, p.job("setup"), JobApply{Rows: rows, Asked: []int64{a}, Suggested: []int64{a}}, 0)
	if err != nil || res.Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	var on bool
	if err := p.store.db.Get(&on, `SELECT has_expiry FROM products WHERE id = $1`, a); err != nil || on {
		t.Fatalf("undo: %v %v", on, err)
	}
}

func TestSetupPackRules(t *testing.T) {
	p, c := setupEnv(t, "aicatalog_setup_rules")
	db := p.store.db
	ctx := context.Background()
	ok, loose, has := prod(t, db, "Card item", c), prod(t, db, "Rice", c), prod(t, db, "Has card", c)
	mustExec(t, db, nil, `UPDATE products SET unit_id = (SELECT id FROM units WHERE allow_decimal LIMIT 1) WHERE id = $1`, loose)
	if _, err := p.core.Products.SavePack(ctx, has, packIn("Card", "10")); err != nil {
		t.Fatal(err)
	}
	batch := Batch{Products: []JobProduct{{ID: ok}, {ID: loose}, {ID: has}}}
	rows, err := setupParse(ctx, p, batch, fmt.Sprintf(`{"items":[
		{"id":%d,"pack_name":"Card","pack_size":10},
		{"id":%d,"pack_name":"Sack","pack_size":25},
		{"id":%d,"pack_name":"card","pack_size":12},
		{"id":%d,"pack_name":"Single","pack_size":1}]}`, ok, loose, has, ok))
	if err != nil || len(rows) != 1 || rows[0].ProductID != ok || rows[0].New != "Card ×10" {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestSetupPackBlankPriceAndUndo(t *testing.T) {
	p, c := setupEnv(t, "aicatalog_setup_pack")
	db := p.store.db
	ctx := context.Background()
	a, b := prod(t, db, "A", c), prod(t, db, "B", c)
	row := func(id int64) JobRow {
		return JobRow{ProductID: id, Kind: "pack", Label: "Pack", New: "Card ×10", Extra: map[string]string{"name": "Card", "size": "10"}}
	}
	res, err := p.applyJob(ctx, p.job("setup"), JobApply{Rows: []JobRow{row(a), row(b)}, Asked: []int64{a, b}, Suggested: []int64{a, b}}, 0)
	if err != nil || res.Applied != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	packs, err := p.core.Products.Packs(ctx, a)
	if err != nil || len(packs) != 1 || !packs[0].SellingPrice.IsZero() || !packs[0].WholesalePrice.IsZero() {
		t.Fatalf("a blank price follows the unit price: %+v %v", packs, err)
	}
	// Someone priced b's pack: undo leaves it.
	mustExec(t, db, nil, `UPDATE product_packs SET selling_price = 99 WHERE product_id = $1`, b)
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db, `SELECT count(*) FROM product_packs WHERE product_id = $1`, a); n != 0 {
		t.Fatalf("undo left a's pack: %d", n)
	}
	if n := count(t, db, `SELECT count(*) FROM product_packs WHERE product_id = $1`, b); n != 1 {
		t.Fatalf("undo removed a pack someone priced: %d", n)
	}
}

func packIn(name, size string) products.PackInput { return products.PackInput{Name: name, Size: size} }
