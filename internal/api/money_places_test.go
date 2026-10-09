package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"karots-suite/internal/features/suppliers"
)

// moneyPlaces makes one cashier-usable locker of each kind for a test: a safe,
// a bank account and a card-machine account, all allowed to go negative.
func moneyPlaces(t *testing.T, te *testEnv) (safe, bank, card int64) {
	t.Helper()
	sfx := time.Now().UnixNano()
	for _, p := range []struct {
		id   *int64
		kind string
	}{{&safe, "safe"}, {&bank, "bank"}, {&card, "card"}} {
		if err := te.db.Get(p.id, `INSERT INTO lockers (name, kind, allow_negative, cashier_access) VALUES ($1, $2, true, true) RETURNING id`,
			fmt.Sprintf("T %s %d", p.kind, sfx), p.kind); err != nil {
			t.Fatal(err)
		}
		id := *p.id
		t.Cleanup(func() { te.db.Exec(`UPDATE lockers SET is_active=false WHERE id=$1`, id) }) //nolint:errcheck
	}
	return
}

// A supplier payment or refund in cash uses a till or a cash locker, an online
// or card one a bank account; a card-machine account is never one — on admin
// and at the counter alike.
func TestSupplierMoneyFitsItsPlace(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	safe, bank, card := moneyPlaces(t, te)
	loc := func(id int64) string { return fmt.Sprintf("locker:%d", id) }
	sup, err := suppliers.NewService(te.db).Create(context.Background(), suppliers.CreateInput{Name: fmt.Sprintf("T Place Supplier %d", time.Now().UnixNano()), OpeningBalance: "10000"})
	if err != nil {
		t.Fatal(err)
	}
	admin := func(rest string, body any) int {
		return te.do(http.MethodPost, fmt.Sprintf("/api/v1/admin/suppliers/%d%s", sup.ID, rest), body, ck).Code
	}
	counter := func(rest string, body any) int {
		return te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/suppliers/%d%s", sup.ID, rest), body, ck).Code
	}
	pay := func(method, src string) AdminSupplierPayRequest {
		return AdminSupplierPayRequest{Amount: "10", Mode: "old", Method: method, Source: src}
	}
	// Overpay by an untracked transfer so they owe us: the refund cases below
	// are refused for the place, not for a missing credit.
	if code := admin("/payment", AdminSupplierPayRequest{Amount: "10100", Mode: "old", Method: "online"}); code != http.StatusCreated {
		t.Fatalf("advance: %d", code)
	}
	for _, c := range []struct {
		name string
		code int
		want int
	}{
		{"admin cash from the bank", admin("/payment", pay("cash", loc(bank))), http.StatusUnprocessableEntity},
		{"admin cash from the card account", admin("/payment", pay("cash", loc(card))), http.StatusUnprocessableEntity},
		{"admin online from the safe", admin("/payment", pay("online", loc(safe))), http.StatusUnprocessableEntity},
		{"admin online from the card account", admin("/payment", pay("online", loc(card))), http.StatusUnprocessableEntity},
		{"admin card from the card account", admin("/payment", pay("card", loc(card))), http.StatusUnprocessableEntity},
		{"admin cash from the safe", admin("/payment", pay("cash", loc(safe))), http.StatusCreated},
		{"admin online from the bank", admin("/payment", pay("online", loc(bank))), http.StatusCreated},
		{"admin refund in cash into the bank", admin("/refund", AdminMoneyRequest{Amount: "1", Method: "cash", Source: loc(bank)}), http.StatusUnprocessableEntity},
		{"admin refund online into the safe", admin("/refund", AdminMoneyRequest{Amount: "1", Method: "online", Source: loc(safe)}), http.StatusUnprocessableEntity},
		{"counter cash from the bank", counter("/pay", SupplierPayRequest{CounterMoneyRequest: CounterMoneyRequest{LockerID: bank}, Amount: "10", Mode: "old", Method: "cash"}), http.StatusUnprocessableEntity},
		{"counter online from the safe", counter("/pay", SupplierPayRequest{CounterMoneyRequest: CounterMoneyRequest{LockerID: safe}, Amount: "10", Mode: "old", Method: "online"}), http.StatusUnprocessableEntity},
		{"counter online from the card account", counter("/pay", SupplierPayRequest{CounterMoneyRequest: CounterMoneyRequest{LockerID: card}, Amount: "10", Mode: "old", Method: "online"}), http.StatusUnprocessableEntity},
		{"counter online from the bank", counter("/pay", SupplierPayRequest{CounterMoneyRequest: CounterMoneyRequest{LockerID: bank}, Amount: "10", Mode: "old", Method: "online"}), http.StatusCreated},
		{"counter refund in cash into the bank", counter("/refund", SupplierRefundRequest{CounterMoneyRequest: CounterMoneyRequest{LockerID: bank}, Amount: "1", Method: "cash"}), http.StatusUnprocessableEntity},
	} {
		if c.code != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.code, c.want)
		}
	}
}
