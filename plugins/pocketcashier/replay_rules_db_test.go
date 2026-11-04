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
