package battle

import (
	"fmt"
	"regexp"
	"testing"
)

// A refused request changes nothing: each bad request below is refused (4xx),
// every table is exactly as before, and the auditor still passes. These are the
// refusals no case reached (coverage push, 2026-11-21). Not here: a bill discount
// above the goods is capped, not refused (DISC-3); a negative note count reads as
// none, so an open or close can't be negative through the API.

// bad is one request that must be refused.
type bad struct {
	name   string
	c      *client
	method string
	path   string
	body   any
}

func (s *sweepShop) refuseAll(t *testing.T, list []bad) {
	t.Helper()
	for _, b := range list {
		before := s.state(t)
		err := b.c.Do(b.method, b.path, b.body, nil)
		if err == nil {
			t.Errorf("%s: went through, want it refused", b.name)
			continue
		}
		if regexp.MustCompile(`: 5\d\d `).MatchString(err.Error()) {
			t.Errorf("%s: %v, want a refusal (4xx), not a server error", b.name, err)
		}
		for tb, h := range s.state(t) {
			if before[tb] != h {
				t.Errorf("%s: refused but changed %s", b.name, tb)
			}
		}
	}
	s.audit(t)
}

func TestRefusalsSalesAndDrawer(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	s.open(t, s.till1)
	sale := func(items []line, extra map[string]any) map[string]any {
		b := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "items": items, "payments": []pay{cash("100000")}}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	plain, serial := s.prod["plain"], s.prod["serial"]
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": serial, "newQuantity": "5"}, nil))
	s.sell(t, s.till1, "0", []line{{"productId": serial, "quantity": "1", "serials": []string{"IMEI-RF1"}}}, cash("15000"))
	r := s.sell(t, s.till1, "0", []line{{"productId": plain, "quantity": "2"}}, cash("1000"))
	item := s.sale(t, s.till1, r.ID).Lines[0].ID
	s.giveBack(t, s.till1, r.ID, item, "2")
	s.open(t, s.till2) // two tills open: a ticket has to name its cashier
	s.refuseAll(t, []bad{
		{"a quantity of 0", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "0"}}, nil)},
		{"half a piece", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "0.5"}}, nil)},
		{"1.5 serial phones", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": serial, "quantity": "1.5"}}, nil)},
		{"one serial twice", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": serial, "quantity": "2", "serials": []string{"IMEI-X", "IMEI-X"}}}, nil)},
		{"a serial already sold", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": serial, "quantity": "1", "serials": []string{"IMEI-RF1"}}}, nil)},
		{"a line discount of -5", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "1", "discount": "-5"}}, nil)},
		{"a bill discount of -5", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "1"}}, map[string]any{"discount": "-5"})},
		{"a payment of abc", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "1"}}, map[string]any{"payments": []pay{{"method": "cash", "amount": "abc"}}})},
		{"a customer that doesn't exist", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "1"}}, map[string]any{"customerId": 999999})},
		{"change given of abc", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "1"}}, map[string]any{"changeGiven": "abc"})},
		{"a cheque with no bank", s.till1, "POST", "/api/v1/till/sales", sale([]line{{"productId": plain, "quantity": "1"}}, map[string]any{
			"payments": []pay{{"method": "cheque", "amount": "100", "cheque": map[string]string{"number": "1", "date": "2026-12-01"}}}})},
		{"return a sale that doesn't exist", s.till1, "POST", "/api/v1/till/sales/999999/return", map[string]any{"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}},
		{"return 0", s.till1, "POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{"lines": []map[string]any{{"saleItemId": item, "quantity": "0"}}}},
		{"return what's all back already", s.till1, "POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{"lines": []map[string]any{{"saleItemId": item, "quantity": "1"}}}},
		{"a refund slip for no return", s.till1, "POST", "/api/v1/till/sales/999999/refund-print?data=1", nil},
		{"withdraw 0", s.till1, "POST", "/api/v1/till/drawer/withdraw", map[string]any{"amount": "0", "reason": "x"}},
		{"close a till that doesn't exist", s.admin, "POST", "/api/v1/admin/tills/999999/close", map[string]any{"breakdown": map[string]int{"100": 1}}},
		{"hold nothing", s.till1, "POST", "/api/v1/till/held", map[string]any{"label": "x", "saleType": "retail", "discount": "0", "cart": "", "itemCount": 0, "total": "0"}},
		{"send to no cashier", s.till1, "POST", "/api/v1/till/held", map[string]any{"label": "x", "saleType": "retail", "discount": "0", "cart": "[{}]", "itemCount": 1, "total": "10", "toCashier": true}},
		{"send to a cashier whose till is shut", s.till1, "POST", "/api/v1/till/held", map[string]any{"label": "x", "saleType": "retail", "discount": "0", "cart": "[{}]", "itemCount": 1, "total": "10",
			"toCashier": true, "forCashier": s.userID(t, "0770000001")}},
		{"drop a held sale that doesn't exist", s.till1, "DELETE", "/api/v1/till/held/999999", nil},
		{"open a ticket that doesn't exist", s.till1, "POST", "/api/v1/till/tickets/999999/open", nil},
		{"pass a ticket that doesn't exist", s.till1, "POST", "/api/v1/till/tickets/999999/pass", nil},
	})
}
