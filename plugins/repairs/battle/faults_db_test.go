package battle

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// REP-1, REP-2, REP-5, REP-7, REP-9, PLG-3, PLG-5: bad requests are refused with
// a clear 4xx and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	openTill(t, s)
	a, b := job(t, s, 0), job(t, s, 0)
	partB := part(t, s, b.ID)
	labour(t, s, b.ID, "100")
	gb := get(t, s, b.ID)
	cancelled := job(t, s, 0)
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/cancel", cancelled.ID), nil, nil))
	svc := s.Service(t, fmt.Sprintf("Refuse service %d", seq.Add(1)), "10")
	var otherPay struct{ PaymentID int64 }
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", *b.CustomerID), map[string]any{"amount": "50", "method": "cash"}, &otherPay))
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `SELECT id FROM users WHERE phone = '0770000002'`))
	jp := func(id int64, rest string) string { return fmt.Sprintf("/api/v1/x/repairs/jobs/%d%s", id, rest) }
	before := ltest.State(t, s.DB)
	for _, x := range []struct {
		name string
		do   func() error
	}{
		{"job no customer", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"deviceModel": "Phone"}, nil)
		}},
		{"job missing customer", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"customerId": 999999, "deviceModel": "Phone"}, nil)
		}},
		{"job no device", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"customerId": *a.CustomerID}, nil)
		}},
		{"job bad date", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"customerId": *a.CustomerID, "deviceModel": "P", "promisedDate": "31/12"}, nil)
		}},
		{"job warranty too long", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"customerId": *a.CustomerID, "deviceModel": "P", "warrantyDays": 4000}, nil)
		}},
		{"no such job", func() error { return s.Cashier.Do("GET", jp(999999, ""), nil, nil) }},
		{"bad id", func() error { return s.Cashier.Do("GET", "/api/v1/x/repairs/jobs/abc", nil, nil) }},
		{"bad status", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/status"), map[string]string{"status": "collected"}, nil)
		}},
		{"status of a cancelled job", func() error {
			return s.Cashier.Do("POST", jp(cancelled.ID, "/status"), map[string]string{"status": "ready"}, nil)
		}},
		{"part qty 0", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/parts"), map[string]any{"productId": partB, "qty": "0"}, nil)
		}},
		{"part a service", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/parts"), map[string]any{"productId": svc, "qty": "1"}, nil)
		}},
		{"part missing", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/parts"), map[string]any{"productId": 999999, "qty": "1"}, nil)
		}},
		{"part warranty", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/parts"), map[string]any{"productId": partB, "qty": "1", "warrantyDays": -1}, nil)
		}},
		{"remove another job's part", func() error {
			return s.Cashier.Do("DELETE", jp(a.ID, fmt.Sprintf("/parts/%d", gb.Parts[0].ID)), nil, nil)
		}},
		{"remove another job's labour", func() error {
			return s.Cashier.Do("DELETE", jp(a.ID, fmt.Sprintf("/charges/%d", gb.Charges[0].ID)), nil, nil)
		}},
		{"labour negative", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/charges"), map[string]string{"amount": "-5"}, nil)
		}},
		{"bill with nothing on it", func() error { return s.Cashier.Do("GET", jp(a.ID, "/bill"), nil, nil) }},
		{"deposit of another customer", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/deposits"), map[string]any{"paymentId": otherPay.PaymentID}, nil)
		}},
		{"deposit no payment", func() error { return s.Cashier.Do("POST", jp(a.ID, "/deposits"), map[string]any{"paymentId": 0}, nil) }},
		{"re-repair of an open job", func() error { return s.Cashier.Do("POST", jp(a.ID, "/rework"), nil, nil) }},
		{"refund held of an open job", func() error { return s.Cashier.Do("POST", jp(a.ID, "/refund-held"), nil, nil) }},
		{"refund held with none", func() error { return s.Cashier.Do("POST", jp(cancelled.ID, "/refund-held"), nil, nil) }},
		{"payout of a cancelled job", func() error {
			return s.Admin.Do("POST", jp(cancelled.ID, "/payouts"), map[string]string{"amount": "10", "source": fmt.Sprintf("till:%d", cashier)}, nil)
		}},
		{"payout 0", func() error {
			return s.Admin.Do("POST", jp(a.ID, "/payouts"), map[string]string{"amount": "0", "source": fmt.Sprintf("till:%d", cashier)}, nil)
		}},
		{"payout from nowhere", func() error {
			return s.Admin.Do("POST", jp(a.ID, "/payouts"), map[string]string{"amount": "10", "source": "bank"}, nil)
		}},
		{"cashier pays the repairer", func() error {
			return s.Cashier.Do("POST", jp(a.ID, "/payouts"), map[string]string{"amount": "10", "source": fmt.Sprintf("till:%d", cashier)}, nil)
		}},
		{"settings bad mode", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/repairs/config", map[string]any{"warrantyMode": "years", "defaultWarrantyDays": 1}, nil)
		}},
		{"settings by a cashier", func() error {
			return s.Cashier.Do("PUT", "/api/v1/x/repairs/config", map[string]any{"warrantyMode": "days", "defaultWarrantyDays": 1}, nil)
		}},
	} {
		if err := x.do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", x.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", x.name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

// PLG-4: a failure at any statement leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	openTill(t, s.Shop)
	var j int64
	var cust *int64
	var items []map[string]any
	var payID int64
	fresh := func() {
		x := job(t, s.Shop, 30)
		j, cust = x.ID, x.CustomerID
		part(t, s.Shop, j)
		labour(t, s.Shop, j, "500")
		_, items = billLines(t, s.Shop, j)
	}
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `SELECT id FROM users WHERE phone = '0770000002'`))
	c := customer(t, s.Shop)
	s.Sweep(t, "open a job", nil, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"customerId": c, "deviceModel": "Sweep phone"}, nil)
	})
	var spare int64
	s.Sweep(t, "a part", func() { fresh(); spare = s.Product(t, fmt.Sprintf("Sweep part %d", seq.Add(1)), "100", "5", nil) }, func() error {
		return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/parts", j), map[string]any{"productId": spare, "qty": "2"}, nil)
	})
	s.Sweep(t, "cancel", fresh, func() error {
		return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/cancel", j), nil, nil)
	})
	s.Sweep(t, "deposit", func() {
		fresh()
		var p struct{ PaymentID int64 }
		s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", *cust), map[string]any{"amount": "200", "method": "cash"}, &p))
		payID = p.PaymentID
	}, func() error {
		return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/deposits", j), map[string]any{"paymentId": payID}, nil)
	})
	s.Sweep(t, "pickup", fresh, func() error {
		return s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"customerId": *cust, "items": items, "payments": []map[string]any{{"method": "cash", "amount": "4500"}}}, nil)
	})
	s.Sweep(t, "refund held", func() {
		fresh()
		s.Must(t, s.DB.Get(new(int), `INSERT INTO repair_payments (job_id, amount, kind) VALUES ($1, 300, 'deposit') RETURNING 1`, j))
		s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/cancel", j), nil, nil))
	}, func() error {
		return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/refund-held", j), nil, nil)
	})
	s.Sweep(t, "pay the repairer", fresh, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/payouts", j), map[string]string{"amount": "150", "source": fmt.Sprintf("till:%d", cashier)}, nil)
	})
}

// PLG-6: a deposit linked twice at once is on the job once.
func TestPLG6_DepositAtOnce(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 0)
	var p struct{ PaymentID int64 }
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", *j.CustomerID), map[string]any{"amount": "400", "method": "cash"}, &p))
	var ok atomic.Int64
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/deposits", j.ID), map[string]any{"paymentId": p.PaymentID}, nil) == nil {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if g := get(t, s, j.ID); ok.Load() != 1 || g.DepositTotal != "400.00" {
		t.Errorf("PLG-6: %d links accepted, deposits %s; want one and 400.00", ok.Load(), g.DepositTotal)
	}
}

func TestPLG7_DataGroup(t *testing.T) {
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "repairs")
}
