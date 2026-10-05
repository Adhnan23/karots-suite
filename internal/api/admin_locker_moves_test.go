package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/lib/pq"
)

func TestParseAmount(t *testing.T) {
	for in, want := range map[string]string{"1,500": "1500", " 2000 ": "2000", "12.34": "12.34", "0.5": "0.5"} {
		if got, err := parseAmount(in, "amount"); err != nil || got.String() != want {
			t.Fatalf("parseAmount(%q) = %v, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "0", "-5", "12.345", "abc", "1e3"} {
		if _, err := parseAmount(in, "amount"); err == nil {
			t.Fatalf("parseAmount(%q) accepted", in)
		}
	}
}

func TestAdminLockerMoves(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009613", "admin")
	var ids []int64
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM money_receipts WHERE created_at IN (SELECT created_at FROM locker_ledger WHERE locker_id = ANY($1))`, pq.Array(ids)) //nolint:errcheck
		te.db.Exec(`DELETE FROM locker_ledger WHERE locker_id = ANY($1)`, pq.Array(ids))                                                             //nolint:errcheck
		te.db.Exec(`DELETE FROM lockers WHERE id = ANY($1)`, pq.Array(ids))                                                                          //nolint:errcheck
	})
	mk := func(name, kind, opening string) int64 {
		l := decode[AdminLocker](t, te.do(http.MethodPost, "/api/v1/admin/lockers", LockerRequest{Name: name, Kind: kind, OpeningBalance: opening}, ck))
		ids = append(ids, l.ID)
		return l.ID
	}
	bal := func(id int64) string {
		var s string
		if err := te.db.Get(&s, `SELECT COALESCE(SUM(balance_delta),0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id=$1`, id); err != nil {
			t.Fatal(err)
		}
		return s
	}
	count := func(q string, args ...any) int {
		var n int
		if err := te.db.Get(&n, q, args...); err != nil {
			t.Fatal(err)
		}
		return n
	}
	receipts := func() int { return count(`SELECT count(*) FROM money_receipts`) }
	safe, bank, card := mk("T mv safe", "safe", "1000"), mk("T mv bank", "bank", "500"), mk("T mv card", "card", "800")
	t.Cleanup(func() { te.db.Exec(`DELETE FROM expenses WHERE description LIKE '%T mv %'`) }) //nolint:errcheck

	// Transfer conserves the total and returns its receipt.
	rec := te.do(http.MethodPost, "/api/v1/admin/lockers/transfer", LockerTransferRequest{FromID: safe, ToID: bank, Amount: "300", Note: "T mv banked"}, ck)
	if rec.Code != http.StatusCreated || decode[ReceiptRef](t, rec).ReceiptID == nil {
		t.Fatalf("transfer: %d %s", rec.Code, rec.Body)
	}
	if bal(safe) != "700.00" || bal(bank) != "800.00" {
		t.Fatalf("after transfer safe=%s bank=%s", bal(safe), bal(bank))
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/lockers/transfer", LockerTransferRequest{FromID: safe, ToID: safe, Amount: "1"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("same locker: %d", rec.Code)
	}

	// Overdraw: 409, nothing written (no ledger row, expense or receipt).
	before, rows := receipts(), count(`SELECT count(*) FROM locker_ledger`)
	if rec := te.do(http.MethodPost, "/api/v1/admin/lockers/transfer", LockerTransferRequest{FromID: safe, ToID: bank, Amount: "5000"}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("overdraw transfer: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/lockers/"+strconv.FormatInt(safe, 10)+"/adjust", LockerAdjustRequest{Type: "bank_charge", Amount: "5000", Note: "T mv too big"}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("overdraw charge: %d", rec.Code)
	}
	if receipts() != before || count(`SELECT count(*) FROM locker_ledger`) != rows || count(`SELECT count(*) FROM expenses WHERE description='T mv too big'`) != 0 {
		t.Fatal("an overdraw left rows behind")
	}

	// Bank charge books an expense in the same move; interest and adjust don't.
	adj := func(typ, amt, note string) int {
		return te.do(http.MethodPost, "/api/v1/admin/lockers/"+strconv.FormatInt(bank, 10)+"/adjust", LockerAdjustRequest{Type: typ, Amount: amt, Note: note}, ck).Code
	}
	if c := adj("bank_charge", "25", "T mv fee"); c != http.StatusCreated {
		t.Fatalf("bank charge: %d", c)
	}
	if count(`SELECT count(*) FROM expenses WHERE description='T mv fee' AND category='Bank charges' AND amount=25`) != 1 {
		t.Fatal("bank charge expense missing")
	}
	if adj("interest", "10", "") != http.StatusCreated || adj("adjust_up", "5", "T mv count") != http.StatusCreated || adj("adjust_down", "2", "T mv count") != http.StatusCreated {
		t.Fatal("interest / adjust failed")
	}
	if adj("adjust_down", "2", " ") != http.StatusUnprocessableEntity || adj("gift", "2", "x") != http.StatusUnprocessableEntity {
		t.Fatal("an adjust without a note, or an unknown type, was accepted")
	}
	if bal(bank) != "788.00" {
		t.Fatalf("bank after adjusts %s", bal(bank))
	}

	// Settle: card −gross, bank +net, one fee expense.
	settle := func(r SettleCardRequest) int {
		return te.do(http.MethodPost, "/api/v1/admin/lockers/settle-card", r, ck).Code
	}
	if c := settle(SettleCardRequest{CardID: card, BankID: bank, Gross: "600", Fee: "12"}); c != http.StatusCreated {
		t.Fatalf("settle: %d", c)
	}
	if bal(card) != "200.00" || bal(bank) != "1376.00" {
		t.Fatalf("after settle card=%s bank=%s", bal(card), bal(bank))
	}
	if count(`SELECT count(*) FROM expenses WHERE category='Card settlement fee' AND amount=12 AND description LIKE '%T mv card%'`) != 1 {
		t.Fatal("settlement fee expense missing")
	}
	if settle(SettleCardRequest{CardID: safe, BankID: bank, Gross: "10"}) != http.StatusUnprocessableEntity ||
		settle(SettleCardRequest{CardID: card, BankID: safe, Gross: "10"}) != http.StatusUnprocessableEntity ||
		settle(SettleCardRequest{CardID: card, BankID: bank, Gross: "10", Fee: "10"}) != http.StatusUnprocessableEntity {
		t.Fatal("a forged kind or fee ≥ gross was accepted")
	}
}
