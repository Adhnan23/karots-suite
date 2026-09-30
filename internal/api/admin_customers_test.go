package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAdminCustomers(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009603", "admin")
	sfx := time.Now().UnixNano()
	var safe, bank int64
	te.db.Get(&safe, `INSERT INTO lockers (name, kind, allow_negative) VALUES ($1, 'safe', true) RETURNING id`, fmt.Sprintf("T cust safe %d", sfx)) //nolint:errcheck
	te.db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ($1, 'bank') RETURNING id`, fmt.Sprintf("T cust bank %d", sfx))                         //nolint:errcheck
	path := func(id int64, rest string) string { return fmt.Sprintf("/api/v1/admin/customers/%d%s", id, rest) }
	get := func(id int64) AdminCustomerDetail { return decode[AdminCustomerDetail](t, te.do(http.MethodGet, path(id, ""), nil, ck)) }
	phone := fmt.Sprintf("07%08d", sfx%100_000_000)

	rec := te.do(http.MethodPost, "/api/v1/admin/customers", CustomerRequest{Name: fmt.Sprintf("T Cust %d", sfx), Phone: phone, CreditLimit: "1000", OpeningBalance: "1500"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	cu := decode[AdminCustomerRow](t, rec)
	if !cu.OverLimit || cu.Owed != "1500.00" {
		t.Fatalf("created: %+v", cu)
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/customers", CustomerRequest{Name: "Someone else", Phone: phone}, ck); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate phone: %d", rec.Code)
	}
	if l := decode[AdminCustomerList](t, te.do(http.MethodGet, "/api/v1/admin/customers?show=over&q="+phone, nil, ck)); l.Total != 1 || l.Owed != "1500.00" {
		t.Fatalf("over-limit list: %+v", l)
	}

	// Cash needs a destination; card can't land in a till; 500 cash into the safe.
	if rec := te.do(http.MethodPost, path(cu.ID, "/payment"), CustomerPayRequest{Amount: "500", Method: "cash"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("cash without destination: %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, path(cu.ID, "/payment"), CustomerPayRequest{Amount: "500", Method: "online", Source: fmt.Sprintf("locker:%d", safe)}, ck); rec.Code < 400 {
		t.Fatalf("online into a safe: %d", rec.Code)
	}
	rec = te.do(http.MethodPost, path(cu.ID, "/payment"), CustomerPayRequest{Amount: "500", Method: "cash", Source: fmt.Sprintf("locker:%d", safe)}, ck)
	if rec.Code != http.StatusCreated || decode[CreditPaymentResult](t, rec).BalanceAfter != "1000.00" {
		t.Fatalf("cash pay: %d %s", rec.Code, rec.Body)
	}
	// 1200 online into the bank: clears it, 200 becomes an advance.
	if rec := te.do(http.MethodPost, path(cu.ID, "/payment"), CustomerPayRequest{Amount: "1200", Method: "online", Source: fmt.Sprintf("locker:%d", bank), Reference: "TXN9"}, ck); rec.Code != http.StatusCreated {
		t.Fatalf("online pay: %d %s", rec.Code, rec.Body)
	}
	var inBank string
	te.db.Get(&inBank, `SELECT COALESCE(SUM(balance_delta),0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id=$1`, bank) //nolint:errcheck
	if d := get(cu.ID); d.Owed != "-200.00" || inBank != "1200.00" || len(d.Recent) != 3 || d.Recent[0].Kind != "Payment" {
		t.Fatalf("after online: %+v, bank %s", d, inBank)
	}

	// Old balance → -100 (a pre-system advance): the statement still reconciles.
	if rec := te.do(http.MethodPost, path(cu.ID, "/opening"), OpeningRequest{Opening: "-100", Note: "was in credit"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("opening: %d %s", rec.Code, rec.Body)
	}
	st := decode[Statement](t, te.do(http.MethodGet, path(cu.ID, "/statement"), nil, ck))
	if last := st.Lines[len(st.Lines)-1]; last.Balance != st.Balance {
		t.Fatalf("statement does not reconcile: last %s vs %s", last.Balance, st.Balance)
	}

	// A fresh customer who was in credit before go-live: opening straight to -100.
	fresh := decode[AdminCustomerRow](t, te.do(http.MethodPost, "/api/v1/admin/customers", CustomerRequest{Name: "T Cust fresh", Phone: fmt.Sprintf("07%08d", (sfx+1)%100_000_000)}, ck))
	te.do(http.MethodPost, path(fresh.ID, "/opening"), OpeningRequest{Opening: "-100"}, ck)
	if st := decode[Statement](t, te.do(http.MethodGet, path(fresh.ID, "/statement"), nil, ck)); len(st.Lines) != 1 || st.Lines[0].Credit != "100.00" || st.Balance != "-100.00" {
		t.Fatalf("negative opening statement: %+v", st)
	}

	if rec := te.do(http.MethodPut, path(cu.ID, ""), CustomerRequest{Name: "T Cust renamed", Phone: phone, CreditLimit: "2000"}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodDelete, path(cu.ID, ""), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("disable: %d", rec.Code)
	}
	if l := decode[AdminCustomerList](t, te.do(http.MethodGet, "/api/v1/admin/customers?disabled=1&q="+phone, nil, ck)); l.Total != 1 || l.Items[0].Active || l.Items[0].CreditLimit != "2000.00" {
		t.Fatalf("disabled: %+v", l)
	}
	if rec := te.do(http.MethodPost, path(cu.ID, "/activate"), nil, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("activate: %d", rec.Code)
	}
}
