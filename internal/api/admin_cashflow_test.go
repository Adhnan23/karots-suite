package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

func TestAdminCashflow(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009615", "admin")
	var ids []int64
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM money_receipts WHERE note = 'T cf move'`)                //nolint:errcheck
		te.db.Exec(`DELETE FROM locker_ledger WHERE locker_id = ANY($1)`, pq.Array(ids)) //nolint:errcheck
		te.db.Exec(`DELETE FROM lockers WHERE id = ANY($1)`, pq.Array(ids))              //nolint:errcheck
	})
	for _, n := range []string{"T cf a", "T cf b"} {
		ids = append(ids, decode[AdminLocker](t, te.do(http.MethodPost, "/api/v1/admin/lockers", LockerRequest{Name: n, Kind: "safe", OpeningBalance: "100"}, ck)).ID)
	}
	get := func() AdminCashflow { return decode[AdminCashflow](t, te.do(http.MethodGet, "/api/v1/admin/cashflow?preset=today", nil, ck)) }
	d := func(s string) decimal.Decimal { v, _ := decimal.NewFromString(s); return v }
	// The two test lockers' total (not cash on hand: parallel tests sell at open tills).
	pair := func(cf AdminCashflow) string {
		sum := decimal.Zero
		for _, p := range cf.Places {
			if p.Value == "locker:"+strconv.FormatInt(ids[0], 10) || p.Value == "locker:"+strconv.FormatInt(ids[1], 10) {
				sum = sum.Add(d(p.Balance))
			}
		}
		return sum.String()
	}
	before := get()
	if rec := te.do(http.MethodPost, "/api/v1/admin/lockers/transfer", LockerTransferRequest{FromID: ids[0], ToID: ids[1], Amount: "40", Note: "T cf move"}, ck); rec.Code != http.StatusCreated {
		t.Fatalf("transfer: %d %s", rec.Code, rec.Body)
	}
	after := get()
	if after.In != before.In || after.Out != before.Out || !d(after.Moved).Sub(d(before.Moved)).Equal(decimal.NewFromInt(40)) || pair(after) != pair(before) {
		t.Fatalf("transfer moved in/out: before %+v after %+v", before, after)
	}
	rows := decode[[]CashflowRow](t, te.do(http.MethodGet, "/api/v1/admin/cashflow/ledger?preset=today&place=locker:"+strconv.FormatInt(ids[1], 10), nil, ck))
	if len(rows) != 2 || !rows[0].Moved || rows[0].Delta != "40.00" || rows[0].ReceiptID == 0 || rows[1].Moved {
		t.Fatalf("ledger %+v", rows)
	}
	if len(after.NetLines) < 6 || after.NetPosition == "" {
		t.Fatalf("net position %+v", after.NetLines)
	}
}

// Uncleared cheques sit in their own place and never count as cash on hand.
func TestAdminCashflowChequesNotCash(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009615", "admin")
	var cid int64
	if err := te.db.Get(&cid, `SELECT id FROM lockers WHERE kind='cheque'`); err != nil {
		t.Fatal(err)
	}
	get := func() AdminCashflow { return decode[AdminCashflow](t, te.do(http.MethodGet, "/api/v1/admin/cashflow?preset=today", nil, ck)) }
	before := get()
	if _, err := te.db.Exec(`INSERT INTO locker_ledger (locker_id, balance_delta, kind, note) VALUES ($1, 777, 'adjust', 'T cf cheque')`, cid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { te.db.Exec(`DELETE FROM locker_ledger WHERE note = 'T cf cheque'`) }) //nolint:errcheck
	after := get()
	if after.CashOnHand != before.CashOnHand {
		t.Fatalf("cheques counted as cash: %s -> %s", before.CashOnHand, after.CashOnHand)
	}
	shown := false
	for _, p := range after.Places {
		shown = shown || (p.Kind == "cheque" && p.Balance == "777.00")
	}
	if !shown {
		t.Fatal("cheque place not listed")
	}
}
