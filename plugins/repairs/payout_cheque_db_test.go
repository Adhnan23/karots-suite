package repairs

import (
	"context"
	"fmt"
	"testing"
	"time"

	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/sales"
)

// The repairer paid by cheque: the payout counts at once; a bounce leaves that
// expense unpaid, so the job's repairer paid drops by it.
func TestPayoutByCheque(t *testing.T) {
	f := newFixture(t)
	var bank int64
	must(t, f.db.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ('BOC', 'bank') RETURNING id`))
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Nokia","repairedBy":"Sunil"}`, f.cust)))
	path := fmt.Sprintf("/jobs/%d/payouts", j.ID)
	day := time.Now().Format("2006-01-02")
	if rec := f.do("POST", path, fmt.Sprintf(`{"amount":"150","method":"cheque","cheques":[{"lockerId":%d,"number":"200001","amount":"100","date":"%s"}]}`, bank, day)); rec.Code < 400 {
		t.Fatal("cheques that don't add up to the payout were taken")
	}
	j = decode[JobDTO](t, f.do("POST", path, fmt.Sprintf(`{"amount":"150","method":"cheque","cheques":[{"lockerId":%d,"number":"200001","amount":"150","date":"%s"}]}`, bank, day)))
	var chq int64
	var bal string
	must(t, f.db.Get(&chq, `SELECT id FROM cheques WHERE expense_id = $1 AND status = 'pending'`, j.Payouts[0].ExpenseID))
	must(t, f.db.Get(&bal, `SELECT COALESCE(SUM(balance_delta), 0)::numeric(14,2)::text FROM locker_ledger WHERE locker_id = $1`, bank))
	if j.RepairerPaid != "150.00" || bal != "0.00" {
		t.Fatalf("paid by cheque: repairer paid %s, bank %s; want 150 / 0", j.RepairerPaid, bal)
	}
	must(t, cheques.NewService(f.db, cashflow.NewService(f.db, f.sales)).Return(context.Background(), chq, "bounced", 0))
	if j = decode[JobDTO](t, f.do("GET", fmt.Sprintf("/jobs/%d", j.ID), "")); j.RepairerPaid != "0.00" {
		t.Fatalf("bounced: repairer paid %s, want 0", j.RepairerPaid)
	}
}

// Pickup is the till's Pay: the job's customer pays it by cheque with no
// change in the plugin — the job is collected, the cheque held in hand.
func TestPickupByCheque(t *testing.T) {
	f := newFixture(t)
	f.sales.WithChequeLeg(cheques.SaleLeg(cashflow.NewService(f.db, f.sales)))
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Galaxy A12"}`, f.cust)))
	decode[JobDTO](t, f.do("POST", fmt.Sprintf("/jobs/%d/charges", j.ID), `{"label":"Fitting","amount":"1500"}`))
	bill := decode[BillDTO](t, f.do("GET", fmt.Sprintf("/jobs/%d/bill", j.ID), ""))
	_, err := f.sales.Create(context.Background(), sales.CreateInput{CustomerID: &f.cust, SaleType: "retail",
		Items: []sales.ItemInput{{ProductID: bill.Lines[0].ProductID, Quantity: "1", PriceOverride: "1500", Plugin: tag(j.ID)}},
		Payments: []sales.PaymentInput{{Method: "cheque", Amount: "1500", Cheque: &sales.ChequeInput{Bank: "BOC", Number: "300001",
			Date: time.Now().Format("2006-01-02")}}}}, f.user)
	must(t, err)
	var status string
	must(t, f.db.Get(&status, `SELECT status FROM cheques WHERE customer_id = $1 AND number = '300001'`, f.cust))
	if j = decode[JobDTO](t, f.do("GET", fmt.Sprintf("/jobs/%d", j.ID), "")); j.Status != "collected" || status != "pending" {
		t.Fatalf("pickup by cheque: job %s, cheque %s", j.Status, status)
	}
}
