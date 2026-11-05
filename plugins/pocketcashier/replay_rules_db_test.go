package pocketcashier

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/sales"

	"github.com/shopspring/decimal"
)

func (f *rfix) openAt(at time.Time) {
	f.en.t.Helper()
	if _, err := f.en.p.core.CashRegister.Open(context.Background(), f.cashier, cashregister.OpenInput{OpeningCash: "0", At: &at, Remote: true}); err != nil {
		f.en.t.Fatal(err)
	}
}

func (f *rfix) flags(saleID int64) []sales.Flag {
	f.en.t.Helper()
	fs, err := f.en.p.core.Sales.Flags(context.Background(), saleID)
	if err != nil {
		f.en.t.Fatal(err)
	}
	return fs
}

func hasFlag(fs []sales.Flag, kind string) bool {
	for _, x := range fs {
		if x.Kind == kind {
			return true
		}
	}
	return false
}

func (f *rfix) saleTimes(id int64) (created time.Time, rung *time.Time) {
	f.en.t.Helper()
	var row struct {
		Created time.Time  `db:"created_at"`
		Rung    *time.Time `db:"rung_at"`
	}
	if err := f.en.db.Get(&row, `SELECT created_at, rung_at FROM sales WHERE id = $1`, id); err != nil {
		f.en.t.Fatal(err)
	}
	return row.Created, row.Rung
}

func near(a time.Time) bool { return time.Since(a).Abs() < time.Minute }

func TestReplayLateSaleAfterClose(t *testing.T) {
	f := newRfix(t, "10")
	ctx := context.Background()
	f.openAt(ago(2 * time.Hour))
	closedAt := ago(time.Hour)
	if _, err := f.en.p.core.CashRegister.Close(ctx, f.cashier, cashregister.CloseInput{ClosingCash: "0", At: &closedAt, Remote: true}); err != nil {
		t.Fatal(err)
	}
	var before decimal.Decimal
	f.en.db.Get(&before, `SELECT expected_cash FROM cash_register WHERE user_id = $1`, f.cashier) //nolint:errcheck
	rungAt := ago(90 * time.Minute)
	r := f.send(nil, f.saleAct("late", rungAt, "1", "120"))
	id := r.Results[0].SaleID
	created, rung := f.saleTimes(id)
	if r.Results[0].Status != "applied" || !near(created) || rung == nil || !rung.Equal(rungAt) || !hasFlag(f.flags(id), sales.FlagLate) {
		t.Fatalf("%+v created %v rung %v flags %+v", r.Results, created, rung, f.flags(id))
	}
	var note *string
	f.en.db.Get(&note, `SELECT review_note FROM cash_register WHERE user_id = $1 AND closed_at IS NULL`, f.cashier) //nolint:errcheck
	var after decimal.Decimal
	f.en.db.Get(&after, `SELECT expected_cash FROM cash_register WHERE user_id = $1 AND closed_at IS NOT NULL`, f.cashier) //nolint:errcheck
	if note == nil || !strings.Contains(*note, "late") || !after.Equal(before) {
		t.Fatalf("note %v expected %s→%s", note, before, after)
	}
}

func TestReplayBadClockFlagsAndCountsNow(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	r := f.send(nil, f.saleAct("tomorrow", time.Now().Add(24*time.Hour), "1", "120"))
	id := r.Results[0].SaleID
	created, _ := f.saleTimes(id)
	if !near(created) || !hasFlag(f.flags(id), sales.FlagClock) {
		t.Fatalf("created %v flags %+v", created, f.flags(id))
	}
}

func TestReplayPriceChangedKept(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	a := f.saleAct("price", ago(10*time.Minute), "1", "100")
	a.Body, _ = json.Marshal(SaleBody{SaleType: "retail", Items: []SaleItem{{ProductID: f.product, Quantity: "1", ExpectPrice: "100"}},
		Payments: []SalePayment{{Method: "cash", Amount: "100"}}})
	r := f.send(nil, a)
	if r.Results[0].Status != "applied" || !hasFlag(f.flags(r.Results[0].SaleID), sales.FlagPrice) {
		t.Fatalf("%+v", r.Results)
	}
}

func TestReplayOversoldFlagged(t *testing.T) {
	f := newRfix(t, "1")
	f.openAt(ago(time.Hour))
	r := f.send(nil, f.saleAct("over", ago(10*time.Minute), "3", "360"))
	if r.Results[0].Status != "applied" || !hasFlag(f.flags(r.Results[0].SaleID), sales.FlagOversold) {
		t.Fatalf("%+v", r.Results)
	}
}

func TestReplayOverLimitFlagged(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	var cust int64
	f.en.db.Get(&cust, `INSERT INTO customers (name, credit_limit) VALUES ('T pocket limit', 100) RETURNING id`) //nolint:errcheck
	t.Cleanup(func() { f.en.db.Exec(`DELETE FROM customers WHERE id = $1`, cust) })                              //nolint:errcheck
	a := f.saleAct("limit", ago(10*time.Minute), "3", "360")
	a.Body, _ = json.Marshal(SaleBody{SaleType: "retail", CustomerID: &cust, Items: []SaleItem{{ProductID: f.product, Quantity: "3", ExpectPrice: "120"}},
		Payments: []SalePayment{{Method: "credit", Amount: "360"}}})
	r := f.send(nil, a)
	if r.Results[0].Status != "applied" || !hasFlag(f.flags(r.Results[0].SaleID), sales.FlagOverLimit) {
		t.Fatalf("%+v", r.Results)
	}
}

func TestReplayLookalike(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	pcAt := ago(20 * time.Minute)
	pc, err := f.en.p.core.Sales.Create(context.Background(), sales.CreateInput{SaleType: "retail", At: &pcAt,
		Items: []sales.ItemInput{{ProductID: f.product, Quantity: "2"}}, Payments: []sales.PaymentInput{{Method: "cash", Amount: "240"}}}, f.cashier)
	if err != nil {
		t.Fatal(err)
	}
	r := f.send(nil, f.saleAct("twin", pcAt.Add(90*time.Second), "2", "240"))
	fs := f.flags(r.Results[0].SaleID)
	if !hasFlag(fs, sales.FlagLookalike) || !strings.Contains(fs[len(fs)-1].Note, pc.Sale.ReceiptNo) {
		t.Fatalf("flags %+v (pc %s)", fs, pc.Sale.ReceiptNo)
	}
	if hasFlag(f.flags(pc.Sale.ID), sales.FlagLookalike) {
		t.Fatal("the PC sale was flagged, not the replayed one")
	}
}

func TestReplayInactiveUserFlagged(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	f.en.db.MustExec(`UPDATE users SET is_active = false WHERE id = $1`, f.cashier)
	r := f.send(nil, f.saleAct("gone-user", ago(10*time.Minute), "1", "120"))
	if r.Results[0].Status != "applied" || !hasFlag(f.flags(r.Results[0].SaleID), sales.FlagInactiveUser) {
		t.Fatalf("%+v", r.Results)
	}
}

func TestReplayAdminRefused(t *testing.T) {
	f := newRfix(t, "10")
	admin, _ := f.en.user("admin")
	a := f.saleAct("admin", ago(10*time.Minute), "1", "120")
	a.UserID = admin
	r := f.send(nil, a)
	if r.Results[0].Status != "failed" || !strings.Contains(r.Results[0].Note, "cashier account") {
		t.Fatalf("%+v", r.Results)
	}
}

func TestReplaySignedInClaimsDevice(t *testing.T) {
	f := newRfix(t, "10")
	f.send(&f.cashier)
	var dev string
	var onPhone int64
	f.en.db.Get(&dev, `SELECT COALESCE(active_device, '') FROM users WHERE id = $1`, f.cashier)         //nolint:errcheck
	f.en.db.Get(&onPhone, `SELECT COALESCE(user_id, 0) FROM pocket_phones WHERE device = $1`, f.device) //nolint:errcheck
	if dev != f.device || onPhone != f.cashier {
		t.Fatalf("device %q phone user %d", dev, onPhone)
	}
}

func TestReplayNoDrawerKick(t *testing.T) {
	f := newRfix(t, "10")
	f.send(nil,
		act(f.ref("open"), "open", f.cashier, ago(30*time.Minute), OpenBody{OpeningCash: "500"}),
		f.saleAct("cash", ago(20*time.Minute), "1", "200"),
		act(f.ref("wd"), "withdraw", f.cashier, ago(15*time.Minute), MoveBody{Amount: "50"}),
		act(f.ref("dep"), "deposit", f.cashier, ago(14*time.Minute), MoveBody{Amount: "20"}),
		act(f.ref("close"), "close", f.cashier, ago(10*time.Minute), CloseBody{ClosingCash: "550"}),
	)
	if f.en.kicks != 0 {
		t.Fatalf("%d drawer kicks", f.en.kicks)
	}
}

func TestReplaySalesPersonSaleFlaggedButDrawerRefused(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	f.en.db.MustExec(`UPDATE users SET sends_to_cashier = true WHERE id = $1`, f.cashier)
	t.Cleanup(func() { f.en.db.MustExec(`UPDATE users SET sends_to_cashier = false WHERE id = $1`, f.cashier) })
	r := f.send(nil,
		f.saleAct(f.ref("sp-sale"), ago(20*time.Minute), "1", "120"),
		act(f.ref("sp-wd"), "withdraw", f.cashier, ago(10*time.Minute), MoveBody{Amount: "50"}))
	if r.Results[0].Status != "applied" || !hasFlag(f.flags(r.Results[0].SaleID), sales.FlagInactiveUser) {
		t.Fatalf("the sale must stand, flagged: %+v", r.Results[0])
	}
	if r.Results[1].Status != "failed" {
		t.Fatalf("a sales person moved drawer money: %+v", r.Results[1])
	}
}

// What the desktop till sends about a line must survive the phone's replay.
func TestReplayKeepsWhatTheTillSends(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Minute))
	var svc int64
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM sale_items WHERE product_id = $1`, `DELETE FROM stock_movements WHERE product_id = $1`} {
			f.en.db.Exec(q, svc) //nolint:errcheck
		}
	})
	if err := f.en.db.Get(&svc, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service, pass_through)
		VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 0, true, true) RETURNING id`, "T svc "+f.refStamp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.en.db.Exec(`DELETE FROM products WHERE id = $1`, svc) }) //nolint:errcheck
	seller, _ := f.en.user("cashier")
	t.Cleanup(func() { f.en.db.Exec(`UPDATE sales SET prepared_by = NULL WHERE prepared_by = $1`, seller) }) //nolint:errcheck
	body := map[string]any{
		"saleType": "retail", "preparedBy": seller,
		"items": []map[string]any{
			{"productId": f.product, "quantity": "1", "group": map[string]any{"no": 1, "label": "Deal", "sets": "1"}},
			{"productId": svc, "quantity": "1", "priceOverride": "50", "components": []map[string]any{{"productId": f.product, "quantity": "2"}}},
			{"productId": svc, "quantity": "1", "priceOverride": "30", "payout": true},
		},
		"payments": []map[string]any{{"method": "cash", "amount": "140"}},
	}
	raw, _ := json.Marshal(body)
	r := f.send(nil, Action{Ref: f.ref("s"), Kind: "sale", UserID: f.cashier, At: ago(time.Minute), Body: raw}).Results[0]
	if r.Status != "applied" {
		t.Fatalf("%+v", r)
	}
	var who *int64
	if err := f.en.db.Get(&who, `SELECT prepared_by FROM sales WHERE id = $1`, r.SaleID); err != nil || who == nil || *who != seller {
		t.Fatalf("prepared by: %v %v", who, err)
	}
	var group int
	if err := f.en.db.Get(&group, `SELECT count(*) FROM sale_items WHERE sale_id = $1 AND line_group = 1 AND line_group_label = 'Deal'`, r.SaleID); err != nil || group != 1 {
		t.Fatalf("group lines: %d %v", group, err)
	}
	var payout string
	if err := f.en.db.Get(&payout, `SELECT subtotal::text FROM sale_items WHERE sale_id = $1 AND subtotal < 0`, r.SaleID); err != nil {
		t.Fatalf("no payout line: %v", err)
	}
	var stock float64
	if err := f.en.db.Get(&stock, `SELECT quantity FROM stock WHERE product_id = $1`, f.product); err != nil || stock != 7 {
		t.Fatalf("stock %v: 10 − 1 sold − 2 used by the service: %v", stock, err)
	}
}
