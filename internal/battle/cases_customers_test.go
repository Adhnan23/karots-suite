package battle

import (
	"context"
	"fmt"
	"testing"

	"github.com/shopspring/decimal"

	"karots-suite/internal/ledgercheck/ltest"
)

// acct is a customer's account as the database holds it.
type acct struct {
	Balance  decimal.Decimal `db:"outstanding_balance"`
	OldDebt  decimal.Decimal `db:"opening_unlinked"`
	Opening  decimal.Decimal `db:"opening_balance"`
	IsActive bool            `db:"is_active"`
}

func (s *shop) acct(t ltest.T, id int64) acct {
	var a acct
	s.must(t, s.db.Get(&a, `SELECT outstanding_balance, opening_unlinked, opening_balance, is_active FROM customers WHERE id = $1`, id))
	return a
}

// newCustomer makes a customer in the back office; its id.
func (s *shop) newCustomer(t ltest.T, phone, limit, opening string) int64 {
	var c struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": "Customer " + phone, "phone": phone,
		"creditLimit": limit, "openingBalance": opening}, &c))
	return c.ID
}

// onAccount sells qty plain (Rs 100 each) to the customer, cash part paid, the rest on account.
func (s *shop) onAccount(t ltest.T, cust int64, qty int, cash string, extra map[string]any) (saleResult, error) {
	total := decimal.NewFromInt(int64(100 * qty))
	c, _ := decimal.NewFromString(cash)
	body := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "customerId": cust,
		"items":    []line{{"productId": s.prod["plain"], "quantity": fmt.Sprint(qty)}},
		"payments": []pay{{"method": "cash", "amount": cash}, {"method": "credit", "amount": total.Sub(c).String()}}}
	for k, v := range extra {
		body[k] = v
	}
	var r saleResult
	err := s.till1.Do("POST", "/api/v1/till/sales", body, &r)
	return r, err
}

func (s *shop) payCash(t ltest.T, cust int64, amount string) {
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", cust), map[string]any{"amount": amount, "method": "cash"}, nil))
}

func wantDec(t *testing.T, rule, what string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Errorf("%s %s = %s, want %s", rule, what, got, want)
	}
}

// CUST-2: the opening is booked once: the till reuses a known phone, the back office refuses it.
func TestCUST2_OpeningOnce(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	id := s.newCustomer(t, "0712000001", "0", "500")
	if err := s.admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": "Again", "phone": "071-200 0001", "creditLimit": "0", "openingBalance": "700"}, nil); err == nil {
		t.Error("CUST-2 the back office made a second customer on a phone in use")
	}
	var till struct{ Customer struct{ ID int64 } }
	s.must(t, s.till1.Do("POST", "/api/v1/till/customers", map[string]string{"name": "At the till", "phone": "0712000001", "creditLimit": "0"}, &till))
	if till.Customer.ID != id {
		t.Fatalf("CUST-2 the till made %d, want %d reused", till.Customer.ID, id)
	}
	wantDec(t, "CUST-2", "balance", s.acct(t, id).Balance, "500")
	if err := s.admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": "Minus", "phone": "0712000002", "creditLimit": "0", "openingBalance": "-1"}, nil); err == nil {
		t.Error("CUST-2 an opening below zero was taken")
	}
	s.audit(t)
}

// CUST-3: two active customers never share a phone — not by a changed phone, not by bringing one back.
func TestCUST3_OnePhoneOneCustomer(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	a, b := s.newCustomer(t, "0712000011", "0", "0"), s.newCustomer(t, "0712000012", "0", "0")
	put := func(id int64, phone string) error {
		return s.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/customers/%d", id), map[string]string{"name": "Renamed", "phone": phone, "creditLimit": "0"}, nil)
	}
	if err := put(b, "071-200 0011"); err == nil {
		t.Error("CUST-3 B took A's phone")
	}
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/customers/%d", a), nil, nil))
	s.must(t, put(b, "0712000011")) // A is disabled: the phone is free
	if err := s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/activate", a), nil, nil); err == nil {
		t.Error("CUST-3 A brought back while B has its phone")
	}
	s.audit(t)
}

// CUST-4: adjusting the opening moves balance and opening alike, the linked part stays, and it is audited.
func TestCUST4_AdjustOpening(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000021", "100000", "1000")
	_, err := s.onAccount(t, id, 3, "0", nil)
	s.must(t, err)
	adjust := func(v string) {
		s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/opening", id), map[string]string{"opening": v}, nil))
	}
	adjust("600")
	a := s.acct(t, id)
	wantDec(t, "CUST-4", "balance", a.Balance, "900")
	wantDec(t, "CUST-4", "old debt", a.OldDebt, "600")
	adjust("-200") // an advance from before the suite
	a = s.acct(t, id)
	wantDec(t, "CUST-4", "balance", a.Balance, "100")
	wantDec(t, "CUST-4", "linked", a.Balance.Sub(a.OldDebt), "300")
	var n int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM audit_log WHERE entity = 'customer' AND entity_id = $1 AND detail LIKE 'opening balance%'`, fmt.Sprint(id)))
	if n != 2 {
		t.Errorf("CUST-4 %d audit rows, want 2", n)
	}
	s.audit(t)
}

// CUST-5: a payment's money lands where its method says, or nothing happens.
func TestCUST5_PaymentLandsItsMoney(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	bank := s.locker(t, "Bank", "bank", "0")
	id := s.newCustomer(t, "0712000031", "100000", "1000")
	if err := s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", id), map[string]any{"amount": "100", "method": "card"}, nil); err == nil {
		t.Error("CUST-5 a card payment with no account was taken")
	}
	if err := s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", id), map[string]any{"amount": "100", "method": "cash", "source": fmt.Sprintf("locker:%d", bank)}, nil); err == nil {
		t.Error("CUST-5 cash into a bank was taken")
	}
	wantDec(t, "CUST-5", "balance after refusals", s.acct(t, id).Balance, "1000")
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", id), map[string]any{"amount": "70", "method": "card", "accountId": s.cardAcct}, nil))
	s.payCash(t, id, "30")
	var card decimal.Decimal
	s.must(t, s.db.Get(&card, `SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, s.cardAcct))
	wantDec(t, "CUST-5", "card account", card, "70")
	var st struct{ Summary struct{ Expected string } }
	s.must(t, s.till1.Do("GET", "/api/v1/till/drawer", nil, &st))
	wantDec(t, "CUST-5", "drawer", dec(t, st.Summary.Expected), "5030")
	wantDec(t, "CUST-5", "balance", s.acct(t, id).Balance, "900")
	s.audit(t)
}

// CUST-6: each slip has its own DP- number; one's after is the next one's before.
func TestCUST6_SlipsChain(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000041", "0", "1000")
	s.payCash(t, id, "100")
	s.payCash(t, id, "250")
	var rows []struct {
		No     string          `db:"receipt_no"`
		Before decimal.Decimal `db:"balance_before"`
		After  decimal.Decimal `db:"balance_after"`
	}
	s.must(t, s.db.Select(&rows, `SELECT receipt_no, balance_before, balance_after FROM customer_payments WHERE customer_id = $1 ORDER BY id`, id))
	if len(rows) != 2 || rows[0].No == rows[1].No {
		t.Fatalf("CUST-6 slips %+v", rows)
	}
	wantDec(t, "CUST-6", "first before", rows[0].Before, "1000")
	wantDec(t, "CUST-6", "second before", rows[1].Before, rows[0].After.String())
	wantDec(t, "CUST-6", "second after", rows[1].After, "650")
	s.audit(t)
}

// CUST-7: paying more than owed leaves an advance; the next sale on account uses it first.
func TestCUST7_OverpayThenAdvanceUsed(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000051", "1000", "0")
	s.payCash(t, id, "500")
	wantDec(t, "CUST-7", "advance", s.acct(t, id).Balance, "-500")
	r, err := s.onAccount(t, id, 3, "0", nil)
	s.must(t, err)
	var slip struct {
		Sale struct{ FromAdvance, Due, Balance string } `json:"sale"`
	}
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/print?data=1", r.ID), nil, &slip))
	if slip.Sale.FromAdvance != "300.00" || slip.Sale.Due != "0.00" || slip.Sale.Balance != "-200.00" {
		t.Errorf("CUST-7 receipt from advance %s, due %s, balance %s; want 300.00, 0.00, -200.00", slip.Sale.FromAdvance, slip.Sale.Due, slip.Sale.Balance)
	}
	s.audit(t)
}

// CUST-8: a lowered limit leaves the balance; the next sale on account past it is refused unless confirmed.
func TestCUST8_LimitLowered(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000061", "1000", "0")
	_, err := s.onAccount(t, id, 8, "0", nil)
	s.must(t, err)
	s.must(t, s.till1.Do("PATCH", fmt.Sprintf("/api/v1/till/customers/%d/credit-limit", id), map[string]string{"creditLimit": "500"}, nil))
	wantDec(t, "CUST-8", "balance", s.acct(t, id).Balance, "800")
	if _, err := s.onAccount(t, id, 1, "0", nil); err == nil {
		t.Error("CUST-8 a sale past the limit was taken")
	}
	_, err = s.onAccount(t, id, 1, "0", map[string]any{"allowOverLimit": true})
	s.must(t, err)
	wantDec(t, "CUST-8", "balance", s.acct(t, id).Balance, "900")
	s.audit(t)
}

// CUST-9: old debt first pays the old debt, then the linked part, then leaves an advance;
// a plain payment pays the linked part first.
func TestCUST9_OldDebtFirst(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	safe := s.locker(t, "Safe", "safe", "0")
	id := s.newCustomer(t, "0712000071", "100000", "500")
	_, err := s.onAccount(t, id, 3, "0", nil) // linked 300
	s.must(t, err)
	pay := func(amount string, old bool) {
		s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", id), map[string]any{"amount": amount, "method": "cash",
			"source": fmt.Sprintf("locker:%d", safe), "oldDebtFirst": old}, nil))
	}
	pay("200", false) // linked 300 → 100, old 500
	a := s.acct(t, id)
	wantDec(t, "CUST-9", "old debt after a plain payment", a.OldDebt, "500")
	wantDec(t, "CUST-9", "balance", a.Balance, "600")
	pay("450", true) // old 500 → 50, linked 100
	a = s.acct(t, id)
	wantDec(t, "CUST-9", "old debt", a.OldDebt, "50")
	wantDec(t, "CUST-9", "balance", a.Balance, "150")
	pay("400", true) // old 50 → 0, linked 100 → 0, 250 advance
	a = s.acct(t, id)
	wantDec(t, "CUST-9", "old debt", a.OldDebt, "0")
	wantDec(t, "CUST-9", "balance", a.Balance, "-250")
	s.audit(t)
}

// CUST-10: the statement's running balance ends at the balance, kept change included.
func TestCUST10_StatementEndsAtBalance(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000081", "100000", "1000")
	r, err := s.onAccount(t, id, 3, "100", nil)
	s.must(t, err)
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": id, "items": []line{{"productId": s.prod["plain"], "quantity": "2"}}, "payments": []pay{cash("250")},
		"changeGiven": "30", "onAccountRounding": true}, nil))
	s.giveBack(t, s.till1, r.ID, s.sale(t, s.till1, r.ID).Lines[0].ID, "1")
	s.payCash(t, id, "100")
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/opening", id), map[string]string{"opening": "800"}, nil))
	var st struct {
		Balance string
		Lines   []struct{ Kind, Balance string }
	}
	s.must(t, s.admin.Do("GET", fmt.Sprintf("/api/v1/admin/customers/%d/statement", id), nil, &st))
	bal := s.acct(t, id).Balance
	last := st.Lines[len(st.Lines)-1].Balance
	if !dec(t, last).Equal(bal) || !dec(t, st.Balance).Equal(bal) {
		t.Errorf("CUST-10 statement ends at %s (total %s), the account says %s: %+v", last, st.Balance, bal, st.Lines)
	}
	s.audit(t)
}

// CUST-11: an advance refund hands back at most the advance, and never more than the till holds.
func TestCUST11_AdvanceRefundBounds(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000091", "0", "0")
	s.payCash(t, id, "300")
	refund := func(amount string) error {
		return s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/refund", id), map[string]any{"amount": amount}, nil)
	}
	if err := refund("300.01"); err == nil {
		t.Error("CUST-11 more than the advance handed back")
	}
	safe := s.locker(t, "Safe", "safe", "0")
	s.must(t, s.till1.Do("POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "5200", "reason": "to safe", "lockerId": safe}, nil))
	if err := refund("200"); err == nil { // the drawer holds 100
		t.Error("CUST-11 an advance refund took the drawer below zero")
	}
	s.must(t, refund("100"))
	wantDec(t, "CUST-11", "advance left", s.acct(t, id).Balance, "-200")
	s.audit(t)
}

// CUST-12: dues list everyone who owes (a disabled customer too); advances are counted apart.
func TestCUST12_DuesAndAdvances(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	owes, gone, ahead := s.newCustomer(t, "0712000101", "0", "400"), s.newCustomer(t, "0712000102", "0", "250"), s.newCustomer(t, "0712000103", "0", "0")
	s.payCash(t, ahead, "90")
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/customers/%d", gone), nil, nil))
	var d struct {
		Total string
		Rows  []struct {
			ID       int64
			Disabled bool
		}
	}
	s.must(t, s.admin.Do("GET", "/api/v1/admin/reports/customer-dues", nil, &d))
	seen := map[int64]bool{}
	for _, r := range d.Rows {
		seen[r.ID] = true
		if r.ID == gone && !r.Disabled {
			t.Error("CUST-12 the disabled debtor isn't marked disabled")
		}
	}
	if !seen[owes] || !seen[gone] || seen[ahead] {
		t.Errorf("CUST-12 dues rows %+v", d.Rows)
	}
	var recv, adv decimal.Decimal
	s.must(t, s.db.Get(&recv, `SELECT COALESCE(SUM(outstanding_balance), 0) FROM customers WHERE outstanding_balance > 0`))
	s.must(t, s.db.Get(&adv, `SELECT COALESCE(-SUM(outstanding_balance), 0) FROM customers WHERE outstanding_balance < 0`))
	wantDec(t, "CUST-12", "dues total", dec(t, d.Total), recv.String())
	wantDec(t, "CUST-12", "advances", adv, "90")
	s.audit(t)
}

// CUST-14: a disabled customer keeps their account, can pay and be handed an advance back.
func TestCUST14_DisabledCustomerPaysAndIsRefunded(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000111", "0", "300")
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/customers/%d", id), nil, nil))
	s.payCash(t, id, "400")
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/refund", id), map[string]any{"amount": "100"}, nil))
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/activate", id), nil, nil))
	a := s.acct(t, id)
	wantDec(t, "CUST-14", "balance", a.Balance, "0")
	if !a.IsActive {
		t.Error("CUST-14 not brought back")
	}
	s.audit(t)
}

// CUST-15: an advance they hold pays a new old debt down at once.
func TestCUST15_AdvancePaysNewOldDebt(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000121", "0", "0")
	s.payCash(t, id, "500")
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/opening", id), map[string]string{"opening": "1000"}, nil))
	a := s.acct(t, id)
	wantDec(t, "CUST-15", "balance", a.Balance, "500")
	wantDec(t, "CUST-15", "old debt", a.OldDebt, "500")
	s.payCash(t, id, "100") // a plain payment: nothing linked, so the old debt
	wantDec(t, "CUST-15", "old debt after a payment", s.acct(t, id).OldDebt, "400")
	s.audit(t)
}

// CUST-16: the same payment sent twice is one payment.
func TestCUST16_PaymentSentTwiceIsOne(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000131", "0", "1000")
	body := map[string]any{"amount": "100", "method": "cash", "clientRef": "pay-once-1"}
	var first, second struct{ PaymentID int64 }
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", id), body, &first))
	s.must(t, s.till1.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", id), body, &second))
	if first.PaymentID != second.PaymentID {
		t.Errorf("CUST-16 two payments %d and %d", first.PaymentID, second.PaymentID)
	}
	wantDec(t, "CUST-16", "balance", s.acct(t, id).Balance, "900")
	var st struct{ Summary struct{ Expected string } }
	s.must(t, s.till1.Do("GET", "/api/v1/till/drawer", nil, &st))
	wantDec(t, "CUST-16", "drawer", dec(t, st.Summary.Expected), "5100")
	s.audit(t)
}

// CUST-17: a sale on account to a disabled customer goes through, flagged for review.
func TestCUST17_DisabledCustomerSaleFlagged(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	id := s.newCustomer(t, "0712000141", "1000", "0")
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/customers/%d", id), nil, nil))
	r, err := s.onAccount(t, id, 2, "0", nil)
	s.must(t, err)
	var kinds []string
	s.must(t, s.db.SelectContext(context.Background(), &kinds, `SELECT kind FROM sale_flags WHERE sale_id = $1`, r.ID))
	if len(kinds) != 1 || kinds[0] != "customer_disabled" {
		t.Errorf("CUST-17 flags %v, want [customer_disabled]", kinds)
	}
	s.audit(t)
}
