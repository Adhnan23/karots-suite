package recharge

import (
	"context"
	"testing"
)

// Carriers, devices (with the float already on them), rates, the plugin's own
// products made once, and a package quick-added at the till: sold at once,
// flagged until reviewed, never twinned.
func TestStoreSetup(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	db.Exec(`INSERT INTO units (name, abbreviation) VALUES ('Pieces', 'pcs') ON CONFLICT DO NOTHING`) //nolint:errcheck
	var cashier int64
	must(t, db.Get(&cashier, `INSERT INTO users (name, phone, pin_hash, role) VALUES ('Nimal', '0770000001', 'x', 'cashier') RETURNING id`))
	st := NewStore(db)

	cid, err := st.CreateCarrier(ctx, "Dialog")
	must(t, err)
	if _, err := st.CreateCarrier(ctx, "dialog"); err == nil {
		t.Fatal("a second Dialog was added")
	}
	c, err := st.Carrier(ctx, db, cid)
	must(t, err)
	var pt bool
	must(t, db.Get(&pt, `SELECT pass_through FROM products WHERE id = $1`, c.ProductID))
	if !pt {
		t.Fatal("the carrier's product isn't pass-through")
	}

	did, err := st.CreateDevice(ctx, DeviceInput{CarrierID: cid, Label: "Shop phone", Reload: true, Money: true, Opening: d("1000")}, cashier)
	must(t, err)
	if b, _ := balance(ctx, db, did); !b.Equal(d("1000")) {
		t.Fatalf("starting float %s, want 1000", b)
	}

	must(t, st.SetRates(ctx, cid, []Rate{{Kind: KindReload, Percent: d("3")}, {Kind: KindBillpay, Fixed: d("10"), Charge: d("50")}}))
	rs, err := st.Rates(ctx, db, cid)
	must(t, err)
	if len(rs) != len(kinds) || !rateOf(rs, KindReload).Percent.Equal(d("3")) || !rateOf(rs, KindBillpay).Charge.Equal(d("50")) {
		t.Fatalf("rates: %+v", rs)
	}

	ids, err := st.ProductIDs(ctx)
	must(t, err)
	again, err := st.ProductIDs(ctx)
	must(t, err)
	if len(ids) != 4 || ids["cashout"] == 0 || again["cashout"] != ids["cashout"] {
		t.Fatalf("own products %v then %v", ids, again)
	}
	must(t, db.Get(&pt, `SELECT pass_through FROM products WHERE id = $1`, ids["charge"]))
	if pt {
		t.Fatal("the service charge is the shop's revenue, not pass-through")
	}

	comm := d("12")
	p, created, err := st.CreatePackage(ctx, PackageInput{CarrierID: cid, Name: "Data 100", Price: d("100"), Commission: &comm}, cashier)
	must(t, err)
	if !created || !p.FloatCost.Equal(d("88")) || p.AddedBy != "Nimal" {
		t.Fatalf("quick-added package: %+v (created %v)", p, created)
	}
	twin, created, err := st.CreatePackage(ctx, PackageInput{CarrierID: cid, Name: "data 100", Price: d("100"), Commission: &comm}, cashier)
	must(t, err)
	if created || twin.ID != p.ID {
		t.Fatalf("a same-name package made a twin: %+v", twin)
	}
	must(t, st.ReviewPackage(ctx, p.ID))
	ps, err := st.Packages(ctx, false)
	must(t, err)
	if len(ps) != 1 || ps[0].AddedBy != "" {
		t.Fatalf("after review: %+v", ps)
	}
	big := d("150")
	if _, _, err := st.CreatePackage(ctx, PackageInput{CarrierID: cid, Name: "Odd", Price: d("100"), Commission: &big}, 0); err == nil {
		t.Fatal("a commission above the price was taken")
	}
}
