package repairs

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// warrantyUntil is the date a repair's warranty runs to: nil when none, else the
// pickup date plus the days.
func warrantyUntil(days int, from time.Time) *time.Time {
	if days <= 0 {
		return nil
	}
	y, m, d := from.Date()
	u := time.Date(y, m, d, 0, 0, 0, 0, from.Location()).AddDate(0, 0, days)
	return &u
}

// jobWarrantyUntil: "days" mode = the whole repair's warranty; "parts" mode = the
// latest part's, so the job stays in warranty while any covered part is.
func jobWarrantyUntil(mode string, d *Detail, from time.Time) *time.Time {
	if mode != "parts" {
		return warrantyUntil(d.Job.WarrantyDays, from)
	}
	var latest *time.Time
	for _, pt := range d.Parts {
		if u := warrantyUntil(pt.WarrantyDays, from); u != nil && (latest == nil || u.After(*latest)) {
			latest = u
		}
	}
	return latest
}

func inWarranty(j Job, now time.Time) bool {
	return j.Status == "collected" && j.WarrantyUntil != nil && !now.After(*j.WarrantyUntil)
}

// dueLabel says how far off the promised date is; due today / tomorrow / overdue
// count as urgent. Closed jobs have none.
func dueLabel(j Job, now time.Time) (string, bool) {
	if j.Status == "collected" || j.Status == "cancelled" || j.PromisedDate == nil {
		return "", j.Urgent && j.Status != "collected" && j.Status != "cancelled"
	}
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	py, pm, pd := j.PromisedDate.Date()
	days := int(time.Date(py, pm, pd, 0, 0, 0, 0, time.UTC).Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return fmt.Sprintf("overdue %dd", -days), true
	case days == 0:
		return "due today", true
	case days == 1:
		return "due tomorrow", true
	}
	return fmt.Sprintf("in %dd", days), j.Urgent
}

var hundred = decimal.NewFromInt(100)

// partDiscount is an old row's frozen discount (fixed is per unit), off
// gross and clamped to it. New rows have none.
func partDiscount(p Part, gross decimal.Decimal) decimal.Decimal {
	var amt decimal.Decimal
	if p.DiscountType == "percent" {
		amt = gross.Mul(p.DiscountValue).Div(hundred)
	} else {
		amt = p.DiscountValue.Mul(p.Qty)
	}
	amt = amt.Round(2)
	if amt.IsNegative() {
		return decimal.Zero
	}
	return decimal.Min(amt, gross)
}

// estimate is the job at today's prices: parts × selling price (less an old
// row's discount) + labour. The cart prices it for real at pickup.
func estimate(d *Detail) decimal.Decimal {
	t := decimal.Zero
	for _, c := range d.Charges {
		t = t.Add(c.Amount)
	}
	for _, p := range d.Parts {
		gross := p.Qty.Mul(p.Price)
		t = t.Add(gross.Sub(partDiscount(p, gross)))
	}
	return t.Round(2)
}

// deposits is the core customer payments taken for the job.
func deposits(d *Detail) decimal.Decimal {
	t := decimal.Zero
	for _, x := range d.Deposits {
		t = t.Add(x.Amount)
	}
	return t
}

// legacyHeld is an old deposit not yet refunded or applied to a sale.
func legacyHeld(d *Detail) decimal.Decimal {
	t := decimal.Zero
	for _, p := range d.Payments {
		if p.Kind == "deposit" {
			t = t.Add(p.Amount)
		} else {
			t = t.Sub(p.Amount)
		}
	}
	return decimal.Max(t, decimal.Zero)
}

// repairerPaid is the old running total plus the job's live expenses.
func repairerPaid(d *Detail) decimal.Decimal {
	t := d.Job.RepairerPaid
	for _, p := range d.Payouts {
		t = t.Add(p.Amount)
	}
	return t
}

// onAccountPrefill is what Put on the bill pre-fills as On account: the job's
// deposits, never more than the customer's advance now (it may have gone on
// other buys) nor the bill. Never adds debt.
func onAccountPrefill(jobDeposits, outstanding, bill decimal.Decimal) decimal.Decimal {
	advance := decimal.Max(outstanding.Neg(), decimal.Zero)
	return decimal.Max(decimal.Min(jobDeposits, advance, bill), decimal.Zero).Round(2)
}
