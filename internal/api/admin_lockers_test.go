package api

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"karots-suite/internal/features/cashflow"

	"github.com/shopspring/decimal"
)

func TestAdminLockers(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009611", "admin")
	rec := te.do(http.MethodPost, "/api/v1/admin/lockers", LockerRequest{Name: "T locker safe", Kind: "safe", OpeningBalance: "1,500", CashierAccess: true}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	l := decode[AdminLocker](t, rec)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM locker_ledger WHERE locker_id=$1`, l.ID) //nolint:errcheck
		te.db.Exec(`DELETE FROM lockers WHERE id=$1`, l.ID)              //nolint:errcheck
	})
	base := "/api/v1/admin/lockers/" + strconv.FormatInt(l.ID, 10)

	// The opening balance is the locker's balance and its first ledger row.
	if l.Balance != "1500.00" || !l.CashierAccess || l.Overdraft {
		t.Fatalf("created %+v", l)
	}
	if v := decode[AdminLockerView](t, te.do(http.MethodGet, base+"?preset=today", nil, ck)); v.Net != "1500.00" || v.Locker.Balance != "1500.00" {
		t.Fatalf("view %+v", v)
	}
	rows := decode[[]LockerLedgerRow](t, te.do(http.MethodGet, base+"/ledger?preset=today", nil, ck))
	if len(rows) != 1 || rows[0].Kind != "open_balance" || rows[0].Delta != "1500.00" {
		t.Fatalf("ledger %+v", rows)
	}
	if rows := decode[[]LockerLedgerRow](t, te.do(http.MethodGet, base+"/ledger?preset=today&offset=1", nil, ck)); len(rows) != 0 {
		t.Fatalf("offset page %+v", rows)
	}
	// A move's ledger row links the CR- receipt it wrote.
	rc, err := cashflow.NewService(te.db, nil).Move(context.Background(), cashflow.MoveInput{From: cashflow.Locker(l.ID), To: cashflow.External(), Amount: decimal.NewFromInt(100), Reason: "T link"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { te.db.Exec(`DELETE FROM money_receipts WHERE id=$1`, rc.ID) }) //nolint:errcheck
	if rows := decode[[]LockerLedgerRow](t, te.do(http.MethodGet, base+"/ledger", nil, ck)); len(rows) != 2 || rows[0].ReceiptID != rc.ID || rows[0].ReceiptNo != rc.ReceiptNo || rows[1].ReceiptID != 0 {
		t.Fatalf("receipt link %+v", rows)
	}
	if v := decode[AdminLockerView](t, te.do(http.MethodGet, base, nil, ck)); v.Net != "1400.00" {
		t.Fatalf("net after move %+v", v)
	}

	// Edit keeps the balance; a bad kind is refused.
	if rec := te.do(http.MethodPut, base, LockerRequest{Name: "T locker bank", Kind: "bank", Overdraft: true}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPut, base, LockerRequest{Name: "x", Kind: "vault"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad kind: %d", rec.Code)
	}

	inSources := func() bool {
		for _, s := range decode[[]MoneySource](t, te.do(http.MethodGet, "/api/v1/admin/money-sources", nil, ck)) {
			if s.Value == "locker:"+strconv.FormatInt(l.ID, 10) {
				return true
			}
		}
		return false
	}
	listed := func(q string) *AdminLocker {
		for _, r := range decode[AdminLockerList](t, te.do(http.MethodGet, "/api/v1/admin/lockers"+q, nil, ck)).Items {
			if r.ID == l.ID {
				return &r
			}
		}
		return nil
	}
	if r := listed(""); r == nil || r.Name != "T locker bank" || r.Kind != "bank" || !r.Overdraft || r.CashierAccess || r.Balance != "1400.00" {
		t.Fatalf("listed %+v", r)
	}

	// Archive hides it from the pickers and the default list; reactivate brings it back.
	if rec := te.do(http.MethodDelete, base, nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("archive: %d", rec.Code)
	}
	if inSources() || listed("") != nil || listed("?disabled=1") == nil {
		t.Fatal("archived locker still offered, or missing under Show disabled")
	}
	// An archived locker can't become the till's default.
	if rec := te.do(http.MethodPut, "/api/v1/admin/settings/till-cash", TillCashRequest{DefaultLockerID: l.ID}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("archived default: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, base+"/activate", nil, ck); rec.Code != http.StatusNoContent || !inSources() {
		t.Fatalf("reactivate: %d", rec.Code)
	}
}

func TestAdminTillCashSettings(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009612", "admin")
	var lid int64
	if err := te.db.Get(&lid, `INSERT INTO lockers (name, kind) VALUES ('T till default', 'safe') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	var orig struct {
		Def   *int64 `db:"default_locker_id"`
		Allow bool   `db:"allow_untracked_cash"`
	}
	if err := te.db.Get(&orig, `SELECT default_locker_id, allow_untracked_cash FROM settings WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		te.db.Exec(`UPDATE settings SET default_locker_id=$1, allow_untracked_cash=$2 WHERE id=1`, orig.Def, orig.Allow) //nolint:errcheck
		te.db.Exec(`DELETE FROM lockers WHERE id=$1`, lid)                                                               //nolint:errcheck
	})
	others := func() string {
		var s string
		if err := te.db.Get(&s, `SELECT (to_jsonb(s) - 'default_locker_id' - 'allow_untracked_cash' - 'updated_at')::text FROM settings s WHERE id=1`); err != nil {
			t.Fatal(err)
		}
		return s
	}
	before := others()
	if rec := te.do(http.MethodPut, "/api/v1/admin/settings/till-cash", TillCashRequest{DefaultLockerID: lid, AllowUntracked: !orig.Allow}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if others() != before {
		t.Fatal("till-cash PUT touched other settings")
	}
	got := decode[AdminLockerList](t, te.do(http.MethodGet, "/api/v1/admin/lockers", nil, ck))
	if got.TillCash.DefaultLockerID != lid || got.TillCash.AllowUntracked == orig.Allow {
		t.Fatalf("till cash %+v", got.TillCash)
	}
	for _, r := range got.Items {
		if r.TillDefault != (r.ID == lid) {
			t.Fatalf("till default badge on %+v", r)
		}
	}
	// Archiving the default locker clears it, keeping the untracked switch.
	if rec := te.do(http.MethodDelete, "/api/v1/admin/lockers/"+strconv.FormatInt(lid, 10), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("archive: %d", rec.Code)
	}
	if got := decode[AdminLockerList](t, te.do(http.MethodGet, "/api/v1/admin/lockers", nil, ck)); got.TillCash.DefaultLockerID != 0 || got.TillCash.AllowUntracked == orig.Allow {
		t.Fatalf("archived default kept %+v", got.TillCash)
	}
	te.db.Exec(`UPDATE lockers SET is_active=true WHERE id=$1`, lid) //nolint:errcheck
	// 0 = no default.
	if rec := te.do(http.MethodPut, "/api/v1/admin/settings/till-cash", TillCashRequest{}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("clear: %d", rec.Code)
	}
	if got := decode[AdminLockerList](t, te.do(http.MethodGet, "/api/v1/admin/lockers", nil, ck)); got.TillCash.DefaultLockerID != 0 {
		t.Fatalf("not cleared %+v", got.TillCash)
	}
}
