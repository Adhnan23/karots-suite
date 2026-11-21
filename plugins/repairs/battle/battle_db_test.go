// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide).
package battle

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/repairs"
)

// Repairs' battle tests (docs/rules/plugins/repairs.md): one throwaway shop on the
// real server with this plugin compiled in.

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

var seq atomic.Int64

var tillOnce sync.Once

func openTill(t *testing.T, s *ltest.Shop) { tillOnce.Do(func() { s.OpenTill(t, s.Cashier) }) }

func customer(t *testing.T, s *ltest.Shop) int64 {
	t.Helper()
	var c struct{ ID int64 }
	n := seq.Add(1)
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": fmt.Sprintf("Repair customer %d", n),
		"phone": fmt.Sprintf("078%07d", n), "creditLimit": "0", "openingBalance": "0"}, &c))
	return c.ID
}

// job opens a phone repair for a new customer (no warranty days unless asked).
func job(t *testing.T, s *ltest.Shop, warranty int) repairs.JobDTO {
	t.Helper()
	var j repairs.JobDTO
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/repairs/jobs", map[string]any{"customerId": customer(t, s), "deviceModel": "Galaxy A12",
		"fault": "screen", "warrantyDays": warranty}, &j))
	return j
}

// part puts a new stocked part (Rs 4,000, 10 on hand) on the job; the product id.
func part(t *testing.T, s *ltest.Shop, jobID int64) int64 {
	t.Helper()
	p := s.Product(t, fmt.Sprintf("Screen %d", seq.Add(1)), "4000", "10", nil)
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/parts", jobID), map[string]any{"productId": p, "qty": "1", "warrantyDays": 30}, nil))
	return p
}

func labour(t *testing.T, s *ltest.Shop, jobID int64, amount string) {
	t.Helper()
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/charges", jobID), map[string]string{"label": "Fitting", "amount": amount}, nil))
}

func get(t *testing.T, s *ltest.Shop, jobID int64) repairs.JobDTO {
	t.Helper()
	var j repairs.JobDTO
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/repairs/jobs/%d", jobID), nil, &j))
	return j
}

type sold struct {
	ID    int64
	Lines []struct {
		ID        int64
		ProductID int64
	}
}

// billLines is what Put on the bill rings for the job: its lines tagged with it.
func billLines(t *testing.T, s *ltest.Shop, jobID int64) (repairs.BillDTO, []map[string]any) {
	t.Helper()
	var b repairs.BillDTO
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/bill", jobID), nil, &b))
	var out []map[string]any
	tag := map[string]any{"repairs": map[string]any{"job": jobID}}
	for _, l := range b.Lines {
		it := map[string]any{"productId": l.ProductID, "quantity": "1", "plugin": tag}
		if l.Kind == "labour" {
			it["priceOverride"], it["description"] = l.Price, l.Name
		} else {
			it["quantity"] = l.Qty
		}
		if l.Discount != "" {
			it["discount"], it["discountType"] = l.Discount, l.DiscountType
		}
		out = append(out, it)
	}
	return b, out
}

// pickup sells items to cust, paid by payments; the sale with its lines.
func pickup(t *testing.T, s *ltest.Shop, cust *int64, items []map[string]any, payments ...map[string]any) (sold, error) {
	t.Helper()
	if len(payments) == 0 {
		payments = []map[string]any{{"method": "cash", "amount": "100000"}}
	}
	req := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "items": items, "payments": payments}
	if cust != nil {
		req["customerId"] = *cust
	}
	var r sold
	if err := s.Cashier.Do("POST", "/api/v1/till/sales", req, &r); err != nil {
		return r, err
	}
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", r.ID), nil, &r))
	return r, nil
}

// collect puts the job on the bill and pays it all in cash.
func collect(t *testing.T, s *ltest.Shop, jobID int64) sold {
	t.Helper()
	b, items := billLines(t, s, jobID)
	r, err := pickup(t, s, b.CustomerID, items)
	s.Must(t, err)
	return r
}

func TestLedgerChecksFire(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 30)
	part(t, s, j.ID)
	labour(t, s, j.ID, "1500")
	collect(t, s, j.ID)
	open := job(t, s, 0)
	other := customer(t, s)
	var pay struct{ PaymentID int64 }
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", *open.CustomerID), map[string]any{"amount": "500", "method": "cash"}, &pay))
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/deposits", open.ID), map[string]any{"paymentId": pay.PaymentID}, nil))
	svc := s.Service(t, fmt.Sprintf("Fire service %d", seq.Add(1)), "10")
	dup := s.Product(t, fmt.Sprintf("Fire dup %d", seq.Add(1)), "10", "0", nil)
	for _, x := range []struct{ rule, bad string }{
		{"REP-1", fmt.Sprintf(`UPDATE repair_jobs SET status = 'cancelled', cancelled_at = now() WHERE id = %d`, j.ID)},
		{"REP-1", fmt.Sprintf(`UPDATE repair_jobs SET collected_at = now() WHERE id = %d`, open.ID)},
		{"REP-5", fmt.Sprintf(`UPDATE repair_jobs SET sale_id = 999999999 WHERE id = %d`, j.ID)},
		{"REP-2", fmt.Sprintf(`UPDATE repair_parts SET product_id = %d WHERE job_id = %d`, svc, j.ID)},
		{"REP-2", fmt.Sprintf(`UPDATE repair_parts SET qty = 0 WHERE job_id = %d`, j.ID)},
		{"REP-7", fmt.Sprintf(`UPDATE repair_jobs SET customer_id = %d WHERE id = %d`, other, open.ID)},
		{"REP-7", fmt.Sprintf(`INSERT INTO repair_payments (job_id, amount, kind) VALUES (%d, 100, 'refund')`, open.ID)},
		{"REP-8", fmt.Sprintf(`UPDATE repair_pickup_lines SET job_id = %d WHERE job_id = %d`, open.ID, j.ID)},
		{"REP-10", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id IN (SELECT product_id FROM repair_parts WHERE job_id = %d)`, dup, j.ID)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(x.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == x.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", x.rule, x.bad)
		}
	}
}

// REP-3, REP-4, REP-5, REP-7: a deposit, Put on the bill and Pay — the job is
// collected on that sale, its warranty starts, the deposit goes on the sale as the
// customer's advance.
func TestREP5_PickupCollects(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 90)
	part(t, s, j.ID)
	labour(t, s, j.ID, "1500")
	var pay struct{ PaymentID int64 }
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/customers/%d/payment", *j.CustomerID), map[string]any{"amount": "1000", "method": "cash"}, &pay))
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/deposits", j.ID), map[string]any{"paymentId": pay.PaymentID}, nil))
	b, items := billLines(t, s, j.ID)
	if b.OnAccount != "1000.00" || len(items) != 2 {
		t.Fatalf("REP-3: bill %+v; want two lines and 1000.00 on account", b)
	}
	r, err := pickup(t, s, b.CustomerID, items, map[string]any{"method": "credit", "amount": "1000"}, map[string]any{"method": "cash", "amount": "4500"})
	s.Must(t, err)
	got := get(t, s, j.ID)
	if got.Status != "collected" || got.SaleID == nil || *got.SaleID != r.ID || got.WarrantyUntil == "" || !got.InWarranty {
		t.Errorf("REP-5: after pickup %+v", got.RowDTO)
	}
	if got.Outstanding != "0.00" {
		t.Errorf("REP-4: the customer owes %s after the deposit went on the sale; want 0.00", got.Outstanding)
	}
	s.Audit(t)
}

// REP-5: a closed job rung again (a second sale, a cancelled job) is left as it
// is — the sale stands.
func TestREP5_ClosedJobLeftAlone(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 0)
	labour(t, s, j.ID, "800")
	_, items := billLines(t, s, j.ID)
	first := collect(t, s, j.ID)
	if _, err := pickup(t, s, nil, items); err != nil {
		t.Errorf("REP-5: a second sale of a collected job was refused: %v", err)
	}
	c := job(t, s, 0)
	labour(t, s, c.ID, "300")
	_, citems := billLines(t, s, c.ID)
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/cancel", c.ID), nil, nil))
	if _, err := pickup(t, s, nil, citems); err != nil {
		t.Errorf("REP-5: a sale of a cancelled job was refused: %v", err)
	}
	if g := get(t, s, j.ID); *g.SaleID != first.ID {
		t.Errorf("REP-5: collected job moved to sale %d (want %d)", *g.SaleID, first.ID)
	}
	if g := get(t, s, c.ID); g.Status != "cancelled" || g.SaleID != nil {
		t.Errorf("REP-5: cancelled job now %s on sale %v", g.Status, g.SaleID)
	}
	s.Audit(t)
}

// REP-1: a closed job never changes.
func TestREP1_ClosedJobNeverChanges(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 0)
	p := part(t, s, j.ID)
	collect(t, s, j.ID)
	base := fmt.Sprintf("/api/v1/x/repairs/jobs/%d", j.ID)
	for _, x := range []struct {
		name, method, path string
		body               any
	}{
		{"status", "POST", "/status", map[string]string{"status": "ready"}},
		{"edit", "PUT", "", map[string]any{"customerId": *j.CustomerID, "deviceModel": "Changed"}},
		{"urgent", "POST", "/urgent", map[string]any{"urgent": true}},
		{"cancel", "POST", "/cancel", nil},
		{"part", "POST", "/parts", map[string]any{"productId": p, "qty": "1"}},
		{"labour", "POST", "/charges", map[string]string{"amount": "100"}},
		{"bill", "GET", "/bill", nil},
	} {
		err := s.Cashier.Do(x.method, base+x.path, x.body, nil)
		if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("REP-1 %s on a collected job: %v, want a clear 4xx", x.name, err)
		}
	}
	if g := get(t, s, j.ID); g.Status != "collected" || len(g.Parts) != 1 || len(g.Charges) != 0 || g.DeviceModel != "Galaxy A12" {
		t.Errorf("REP-1: collected job changed: %+v", g)
	}
}

// REP-1: a cancel (or a status) landing with the pickup sale never leaves a
// collected job cancelled or set back.
func TestREP1_CancelAtOnceWithPickup(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	for range 8 {
		j := job(t, s, 0)
		labour(t, s, j.ID, "200")
		b, items := billLines(t, s, j.ID)
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = pickup(t, s, b.CustomerID, items) })
		wg.Go(func() { _ = s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/cancel", j.ID), nil, nil) })
		wg.Go(func() {
			_ = s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/status", j.ID), map[string]string{"status": "in_progress"}, nil)
		})
		wg.Wait()
		if g := get(t, s, j.ID); (g.SaleID != nil) != (g.Status == "collected") {
			t.Errorf("REP-1: job %s ends %s with sale %v", g.TicketNo, g.Status, g.SaleID)
		}
	}
	s.Audit(t)
}

// REP-6: a pickup returned in full leaves the job collected, its warranty standing.
func TestREP6_ReturnedPickupStaysCollected(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 60)
	part(t, s, j.ID)
	labour(t, s, j.ID, "700")
	r := collect(t, s, j.ID)
	var lines []map[string]any
	for _, l := range r.Lines {
		lines = append(lines, map[string]any{"saleItemId": l.ID, "quantity": "1"})
	}
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{"lines": lines}, nil))
	if g := get(t, s, j.ID); g.Status != "collected" || g.WarrantyUntil == "" || *g.SaleID != r.ID {
		t.Errorf("REP-6: after a full return %+v", g.RowDTO)
	}
	s.Audit(t)
}

// REP-7: an old held deposit spent at pickup never past what's held; handed
// back once, all of it, only once the job is closed — six at once = one refund.
func TestREP7_OldHeldDeposit(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 0)
	labour(t, s, j.ID, "1000")
	s.Must(t, s.DB.Get(new(int), `INSERT INTO repair_payments (job_id, amount, kind) VALUES ($1, 300, 'deposit') RETURNING 1`, j.ID))
	b, items := billLines(t, s, j.ID)
	meta := map[string]string{"job": fmt.Sprint(j.ID)}
	if _, err := pickup(t, s, b.CustomerID, items, map[string]any{"method": "deposit", "amount": "400", "meta": meta},
		map[string]any{"method": "cash", "amount": "600"}); err == nil {
		t.Error("REP-4: spent 400 of a 300 old deposit")
	}
	if err := s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/refund-held", j.ID), nil, nil); err == nil {
		t.Error("REP-7: an open job's old deposit was handed back")
	}
	_, err := pickup(t, s, b.CustomerID, items, map[string]any{"method": "deposit", "amount": "200", "meta": meta},
		map[string]any{"method": "cash", "amount": "800"})
	s.Must(t, err)
	var ok atomic.Int64
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			if s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/refund-held", j.ID), nil, nil) == nil {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if g := get(t, s, j.ID); ok.Load() != 1 || g.LegacyHeld != "0.00" {
		t.Errorf("REP-7: %d refunds, %s still held; want one and 0.00", ok.Load(), g.LegacyHeld)
	}
	s.Audit(t)
}

// REP-9: paying the repairer is a core expense linked to the job.
func TestREP9_PayTheRepairer(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 0)
	var cashier int64
	s.Must(t, s.DB.Get(&cashier, `SELECT id FROM users WHERE phone = '0770000002'`))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/repairs/jobs/%d/payouts", j.ID), map[string]string{"amount": "250",
		"source": fmt.Sprintf("till:%d", cashier), "note": "Nimal"}, nil))
	if g := get(t, s, j.ID); g.RepairerPaid != "250.00" || len(g.Payouts) != 1 {
		t.Errorf("REP-9: paid %s in %d payouts; want 250.00 in one", g.RepairerPaid, len(g.Payouts))
	}
	s.Audit(t)
}

// REP-10: a merge moves the job's parts to the kept product.
func TestREP10_ProductMerge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	j := job(t, s, 0)
	p := part(t, s, j.ID)
	kept := s.Product(t, fmt.Sprintf("Kept screen %d", seq.Add(1)), "4000", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", p, kept), map[string]string{"lotPrice": "follow"}, nil))
	if g := get(t, s, j.ID); g.Parts[0].ProductID != kept {
		t.Errorf("REP-10: part on %d, want %d", g.Parts[0].ProductID, kept)
	}
	s.Audit(t)
}

// REP-8: two jobs collected on one sale (and the same part sold loose beside them)
// each count only their own lines.
func TestREP8_TwoJobsOneSale(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	a, b := job(t, s, 0), job(t, s, 0)
	p := part(t, s, a.ID)
	labour(t, s, a.ID, "1000")
	labour(t, s, b.ID, "2000")
	_, ia := billLines(t, s, a.ID)
	_, ib := billLines(t, s, b.ID)
	items := append(append(ia, ib...), map[string]any{"productId": p, "quantity": "1"})
	_, err := pickup(t, s, nil, items)
	s.Must(t, err)
	var rep repairs.ReportDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/repairs/report?preset=today", nil, &rep))
	want := map[int64]string{a.ID: "5000.00", b.ID: "2000.00"}
	for _, r := range rep.Rows {
		if w, ok := want[r.ID]; ok && r.Revenue != w {
			t.Errorf("REP-8: job %s revenue %s, want %s", r.TicketNo, r.Revenue, w)
		}
	}
	s.Audit(t)
}
