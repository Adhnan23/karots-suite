package recharge

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// One bill with a reload, a package, a bill paid from a bank and a service
// charge; a cash out netted to a payout; a sale paid by wallet; then a failed
// reload and a wrong-number package returned — every float and bank move books
// inside the sale / return, with the carrier's commission, never refused.
func TestMoneyThroughTheSale(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()
	db.Exec(`INSERT INTO units (name, abbreviation) VALUES ('Pieces', 'pcs') ON CONFLICT DO NOTHING`) //nolint:errcheck
	var cashier, bank int64
	must(t, db.Get(&cashier, `INSERT INTO users (name, phone, pin_hash, role) VALUES ('Nimal', '0770000002', 'x', 'cashier') RETURNING id`))
	must(t, db.Get(&bank, `INSERT INTO lockers (name, kind, allow_negative, cashier_access) VALUES ('BOC', 'bank', false, true) RETURNING id`))

	st := NewStore(db)
	cf := cashflow.NewService(db, sales.NewService(db))
	p := &Plugin{core: plugin.Core{DB: db, Cashflow: cf}, store: st}
	cid, err := st.CreateCarrier(ctx, "Dialog")
	must(t, err)
	car, _ := st.Carrier(ctx, db, cid)
	dev, err := st.CreateDevice(ctx, DeviceInput{CarrierID: cid, Label: "Shop phone", Reload: true, Money: true, Opening: d("1000")}, cashier)
	must(t, err)
	must(t, st.SetRates(ctx, cid, []Rate{{Kind: KindReload, Percent: d("3")}, {Kind: KindWalletIn, Percent: d("1")}}))
	fc := d("88")
	pk, _, err := st.CreatePackage(ctx, PackageInput{CarrierID: cid, Name: "Data 100", Price: d("100"), FloatCost: &fc}, 0)
	must(t, err)
	own, err := st.ProductIDs(ctx)
	must(t, err)

	svc := sales.NewService(db).
		WithLineHook(func(ctx context.Context, tx *sqlx.Tx, ev sales.TaggedSale) error {
			out := plugin.SaleLines{SaleID: ev.SaleID, ActorID: ev.ActorID}
			for _, l := range ev.Lines {
				out.Lines = append(out.Lines, plugin.SaleLine{ItemID: l.ItemID, ProductID: l.ProductID, Quantity: l.Quantity, Amount: l.Amount, Data: l.Plugin["recharge"]})
			}
			return p.recordSale(ctx, tx, out)
		}).
		WithPluginTenders(func(m string) bool { return m == walletMethod }, func(ctx context.Context, tx *sqlx.Tx, ev sales.PluginTender) error {
			return p.settleWallet(ctx, tx, plugin.TenderSettle{SaleID: ev.SaleID, Amount: ev.Amount, Meta: ev.Meta, ActorID: ev.ActorID})
		}).
		WithReturnHook(func(ctx context.Context, tx *sqlx.Tx, ev sales.ReturnedSale) error {
			out := plugin.ReturnLines{SaleID: ev.SaleID, ReturnID: ev.ReturnID, ActorID: ev.ActorID}
			for _, l := range ev.Lines {
				out.Lines = append(out.Lines, plugin.ReturnLine{SaleItemID: l.SaleItemID, ProductID: l.ProductID, Quantity: l.Quantity, Refund: l.Refund, Data: l.Plugin["recharge"]})
			}
			return p.recordReturn(ctx, tx, out)
		})
	tag := func(v string) map[string]json.RawMessage { return map[string]json.RawMessage{"recharge": json.RawMessage(v)} }
	ds := strconv.FormatInt(dev, 10)
	float := func(want string) {
		t.Helper()
		if b, _ := balance(ctx, db, dev); !b.Equal(d(want)) {
			t.Fatalf("float %s, want %s", b, want)
		}
	}

	s1, err := svc.Create(ctx, sales.CreateInput{SaleType: "retail", Items: []sales.ItemInput{
		{ProductID: car.ProductID, Quantity: "1", PriceOverride: "100", Plugin: tag(`{"kind":"reload","device":` + ds + `,"number":"0771112223"}`)},
		{ProductID: car.ProductID, Quantity: "1", PriceOverride: "100", Plugin: tag(`{"kind":"package","device":` + ds + `,"package":` + strconv.FormatInt(pk.ID, 10) + `}`)},
		{ProductID: own["billpay"], Quantity: "1", PriceOverride: "2500", Plugin: tag(`{"kind":"billpay","bank":` + strconv.FormatInt(bank, 10) + `,"ref":"0123456","billType":"Electricity"}`)},
		{ProductID: own["charge"], Quantity: "1", PriceOverride: "50", Plugin: tag(`{"kind":"charge"}`)},
	}, Payments: []sales.PaymentInput{{Method: "cash", Amount: "2750"}}}, cashier)
	must(t, err)
	float("827") // 1000 − 100 + 3 − 88 + 12
	var bankBal decimal.Decimal
	must(t, db.Get(&bankBal, `SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, bank))
	if !bankBal.Equal(d("-2500")) {
		t.Fatalf("bank %s, want -2500 (paid in the bank's app, never refused)", bankBal)
	}

	s2, err := svc.Create(ctx, sales.CreateInput{SaleType: "retail", Items: []sales.ItemInput{
		{ProductID: own["cashout"], Quantity: "1", PriceOverride: "500", Payout: true, Plugin: tag(`{"kind":"cashout","device":` + ds + `}`)},
	}}, cashier)
	must(t, err)
	if !s2.Sale.ChangeGiven.Equal(d("500")) {
		t.Fatalf("cash out gave %s", s2.Sale.ChangeGiven)
	}
	float("1327")

	_, err = svc.Create(ctx, sales.CreateInput{SaleType: "retail", Items: []sales.ItemInput{{ProductID: own["charge"], Quantity: "1", PriceOverride: "200"}},
		Payments: []sales.PaymentInput{{Method: walletMethod, Amount: "200", Meta: map[string]string{"device": ds}}}}, cashier)
	must(t, err)
	float("1529") // + 200 + 1% commission

	item := func(i int) int64 { return s1.Items[i].ID }
	_, _, err = svc.PartialReturn(ctx, s1.Sale.ID, sales.PartialReturnInput{Lines: []sales.ReturnLineInput{
		{SaleItemID: item(0), Quantity: "1", Plugin: tag(`{"back":"all"}`)},
		{SaleItemID: item(1), Quantity: "1", Plugin: tag(`{"back":"none"}`)},
	}}, cashier)
	must(t, err)
	float("1626") // the failed reload's 100 back, its 3 commission taken back; the wrong number's float stays gone

	e, err := Earnings(ctx, db, s1.Sale.CreatedAt.AddDate(0, 0, -1), s1.Sale.CreatedAt.AddDate(0, 0, 1))
	must(t, err)
	if !e.Equal(d("14")) { // 3 + 12 + 2 − 3
		t.Fatalf("earnings %s, want 14", e)
	}
}
