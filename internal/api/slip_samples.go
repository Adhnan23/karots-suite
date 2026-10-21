package api

import (
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/sales"

	"github.com/shopspring/decimal"
)

// Made-up slips for the sample-slip previews (Settings, Appearance). The busy
// sale carries every line kind at once (pack, discount, warranty + serial, line
// group, payout, two tenders, rounding kept) — the one a style must not clip.

func sampleSale() sales.Detail {
	n := decimal.NewFromInt
	return sales.Detail{
		Sale: sales.Sale{ReceiptNo: "S-00000", CreatedAt: time.Now(), CashierName: "Cashier", Status: "completed",
			Subtotal: n(3210), Total: n(3210), PaidAmount: n(3500), ChangeGiven: n(290)},
		Items: []sales.SaleItem{
			{ProductName: "Basmati 5kg", UnitAbbr: "pcs", Quantity: n(1), UnitPrice: n(2450), Subtotal: n(2450)},
			{ProductName: "Milk 1L", UnitAbbr: "pcs", Quantity: n(2), UnitPrice: n(380), Subtotal: n(760)},
		},
		Payments: []sales.Payment{{Method: "cash", Amount: n(3500)}},
	}
}

// sampleSlip fills the ?data=1 answer for kind (default sale) and, for a sale,
// sample (default simple). nil = no such sample.
func sampleSlip(kind, sample string, d sales.Detail, opts escpos.Options) func(*SlipData) {
	now := datetime.DateTime(time.Now())
	switch kind {
	case "", "sale":
		switch sample {
		case "", "simple":
			return func(x *SlipData) { x.Kind, x.Sale = "sale", slipSale(d, opts) }
		case "busy":
			return func(x *SlipData) { x.Kind, x.Sale = "sale", busySale(now) }
		case "credit":
			return func(x *SlipData) { x.Kind, x.Sale = "sale", creditSale(now) }
		}
	case "refund":
		return func(x *SlipData) {
			x.Kind, x.Refund = "refund", &SlipRefund{ReceiptNo: "S-00231", Date: now, Customer: "Nimal Perera", Reason: "Wrong size",
				Items:  []SlipRefundItem{{Name: "Samsung Charger 25W", Quantity: "1", UnitAbbr: "pc", Refund: "2205.00"}},
				Refund: "1205.00", CreditReduction: "1000.00", RemainingBalance: "3380.00"}
		}
	case "money":
		return func(x *SlipData) {
			x.Kind, x.Money = "money", &SlipMoney{Title: "Withdrawal", ReceiptNo: "CR-000018", Date: now, From: "Till 1", To: "Owner",
				Party: "Sunil", By: "Kamal", Amount: "15000.00", Note: "Weekly draw"}
		}
	case "debt":
		return func(x *SlipData) {
			x.Kind, x.Debt = "debt", &SlipDebt{ReceiptNo: "DP-000002", Date: now, Customer: "Nimal Perera", Phone: "0771234567",
				Amount: "2000.00", Method: "Cash", BalanceBefore: "6380.00", BalanceAfter: "4380.00", Cashier: "Kamal"}
		}
	case "warranty":
		return func(x *SlipData) {
			x.Kind, x.Warranty = "warranty", &SlipWarranty{Date: datetime.Date(time.Now()), Customer: "Nimal Perera",
				Product: "Samsung Charger 25W", OldSerial: "SC-88231", NewSerial: "SC-90412",
				WarrantyUntil: datetime.Date(time.Now().AddDate(1, 0, 0)), WarrantyLeft: "12 months left"}
		}
	case "test":
		return func(x *SlipData) { x.Kind, x.Test = "test", &SlipTest{Printed: now} }
	}
	return nil
}

func sampleItem(name, qty, unit, price, gross, disc, label, sub string) SlipItem {
	return SlipItem{Name: name, Quantity: qty, UnitAbbr: unit, UnitPrice: price, NetUnitPrice: price, Gross: gross,
		Discount: disc, DiscountLabel: label, Subtotal: sub, Serials: []string{}, Members: []SlipGroupMember{}}
}

func busySale(date string) *SlipSale {
	pack := sampleItem("Panadol 500mg", "14", "tab", "20.00", "280.00", "0.00", "", "280.00")
	pack.PackLabel = "1 Card + 2"
	charger := sampleItem("Samsung Charger 25W", "1", "pc", "2450.00", "2450.00", "245.00", " (10%)", "2205.00")
	charger.NetUnitPrice = "2205.00"
	charger.Warranty = "12-month warranty until " + datetime.Date(time.Now().AddDate(1, 0, 0))
	charger.Serials = []string{"SN: SC-88231"}
	deal := sampleItem("Breakfast Deal", "1", "", "0.00", "1300.00", "100.00", "", "1200.00")
	deal.Group, deal.Sets, deal.SetGross, deal.SetNet = true, "2", "650.00", "600.00"
	deal.Members = []SlipGroupMember{{Name: "Bread", Quantity: "2", Serials: []string{}}, {Name: "Milk 1L", Quantity: "2", Serials: []string{}}}
	payout := sampleItem("Reload cash out", "1", "pc", "0.00", "-500.00", "0.00", "", "-500.00")
	payout.Payout = true
	return &SlipSale{ReceiptNo: "S-00231", Date: date, Cashier: "Kamal",
		Items:    []SlipItem{pack, charger, deal, payout, sampleItem("Rice Samba 5kg", "1", "pc", "3080.00", "3080.00", "0.00", "", "3080.00")},
		Subtotal: "6765.00", ItemSavings: "345.00", BillSaving: "0.00", Tax: "0.00", Given: "500.00", Total: "6265.00", YouSaved: "345.00",
		Payments:    []SlipPayment{{Method: "Cash", Amount: "5000.00"}, {Method: "Card", Amount: "1500.00"}},
		FromAdvance: "0.00", Due: "0.00", Balance: "0.00", Change: "234.50", OnAccount: "0.00", ChangeKept: "0.50"}
}

func creditSale(date string) *SlipSale {
	return &SlipSale{ReceiptNo: "S-00233", Date: date, Cashier: "Kamal", Customer: "Nimal Perera",
		Items:    []SlipItem{sampleItem("Rice Samba 5kg", "1", "pc", "2580.00", "2580.00", "0.00", "", "2580.00")},
		Subtotal: "2580.00", ItemSavings: "0.00", BillSaving: "0.00", Tax: "0.00", Given: "0.00", Total: "2580.00", YouSaved: "0.00",
		Payments: []SlipPayment{{Method: "Cash", Amount: "1000.00"}}, Credit: true,
		FromAdvance: "200.00", Due: "1380.00", Balance: "4380.00", Change: "0.00", OnAccount: "0.25", ChangeKept: "0.00"}
}
