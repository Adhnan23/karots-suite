package api

import (
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// slipData answers ?data=1 with the slip's print data for its React template,
// instead of printing. It sits beside previewSlip: same cfg copy (so ?size=
// holds), same opts, and no kick or printer lookup. fill sets the kind and record.
func slipData(c echo.Context, cfg *settings.Settings, opts escpos.Options, fill func(*SlipData)) (bool, error) {
	if c.QueryParam("data") != "1" {
		return false, nil
	}
	d := SlipData{Width: "80", Dots: receiptimg.PrinterDots(cfg.ReceiptWidth), Columns: escpos.Columns(cfg.ReceiptWidth),
		Currency: cfg.CurrencySymbol, Shop: SlipShop(receiptimg.ShopFor(cfg, opts))}
	if cfg.ReceiptWidth == "58" {
		d.Width = "58"
	}
	if d.Currency == "" {
		d.Currency = "Rs."
	}
	fill(&d)
	return true, c.JSON(http.StatusOK, d)
}

func amt(d decimal.Decimal) string { return d.StringFixed(2) }

// slipSale is escpos.Document's numbers: the same item order, groups, payouts,
// discount split, credit block and rounding.
func slipSale(d sales.Detail, opts escpos.Options) *SlipSale {
	s := &SlipSale{ReceiptNo: d.Sale.ReceiptNo, Date: datetime.DateTime(d.Sale.CreatedAt), Cashier: d.Sale.CashierName,
		Customer: strOr(d.Sale.CustomerName)}
	itemDisc, given := decimal.Zero, decimal.Zero
	done := map[int]bool{}
	for _, it := range d.Items {
		if it.Subtotal.IsNegative() {
			given = given.Sub(it.Subtotal)
		}
		if it.LineGroup == nil || it.LineGroupLabel == nil || it.LineGroupSets == nil {
			s.Items = append(s.Items, slipItem(it, d.Sale.CreatedAt, opts))
			if !it.Subtotal.IsNegative() {
				itemDisc = itemDisc.Add(it.Discount)
			}
			continue
		}
		if done[*it.LineGroup] {
			continue
		}
		done[*it.LineGroup] = true
		g := SlipItem{Group: true, Name: *it.LineGroupLabel, Sets: it.LineGroupSets.String(), Itemised: it.LineGroupItemised}
		gross, disc, net := decimal.Zero, decimal.Zero, decimal.Zero
		for _, m := range d.Items {
			if m.LineGroup == nil || *m.LineGroup != *it.LineGroup {
				continue
			}
			gross, disc, net = gross.Add(m.Gross()), disc.Add(m.Discount), net.Add(m.Subtotal)
			g.Members = append(g.Members, SlipGroupMember{Name: m.Label(), Quantity: m.Quantity.String(), Subtotal: amt(m.Subtotal), Discount: amt(m.Discount),
				Warranty: warrantyText(m, d.Sale.CreatedAt), Serials: opts.Serials[m.ProductID]})
		}
		g.Gross, g.Discount, g.Subtotal = amt(gross), amt(disc), amt(net)
		g.SetGross, g.SetNet = amt(gross.Div(*it.LineGroupSets).Round(2)), amt(net.Div(*it.LineGroupSets).Round(2))
		s.Items = append(s.Items, g)
		itemDisc = itemDisc.Add(disc)
	}
	s.Subtotal = amt(d.Sale.Subtotal.Add(given))
	s.ItemSavings = amt(itemDisc)
	s.BillSaving = amt(d.Sale.Discount.Sub(itemDisc))
	s.BillSavingLabel = escpos.DiscSuffix(d.Sale.DiscountType, d.Sale.DiscountValue)
	s.Tax, s.Given, s.Total, s.YouSaved = amt(d.Sale.Tax), amt(given), amt(d.Sale.Total), amt(d.Sale.Discount)
	for _, p := range d.Payments {
		s.Payments = append(s.Payments, SlipPayment{Method: escpos.PaymentLabel(p), Amount: amt(p.Amount)})
	}
	if cs, ok := d.Sale.Credit(opts.CustomerDue); ok {
		s.Credit = true
		s.FromAdvance, s.Due, s.Balance = amt(cs.FromAdvance), amt(cs.Due.Sub(cs.FromAdvance)), amt(cs.Balance)
	}
	s.Change = amt(d.Sale.ChangeGiven)
	kept := d.Sale.PaidAmount.Sub(d.Sale.ChangeGiven).Sub(d.Sale.Total)
	s.OnAccount, s.ChangeKept = "0.00", "0.00"
	if kept.IsPositive() {
		s.OnAccount, s.ChangeKept = amt(d.Sale.RoundingToAccount), amt(kept.Sub(d.Sale.RoundingToAccount))
	}
	return s
}

func slipItem(it sales.SaleItem, soldAt time.Time, opts escpos.Options) SlipItem {
	name := it.ProductName
	if it.Description != nil && *it.Description != "" {
		name = *it.Description
	}
	return SlipItem{Name: name, Payout: it.Subtotal.IsNegative(), Quantity: it.Quantity.String(), UnitAbbr: it.UnitAbbr,
		UnitPrice: amt(it.UnitPrice), NetUnitPrice: amt(it.NetUnitPrice()), PackLabel: strOr(it.PackLabel),
		Gross: amt(it.Gross()), Discount: amt(it.Discount), DiscountLabel: escpos.DiscSuffix(it.DiscountType, it.DiscountValue),
		Subtotal: amt(it.Subtotal), Warranty: warrantyText(it, soldAt), Serials: opts.Serials[it.ProductID]}
}

func warrantyText(it sales.SaleItem, soldAt time.Time) string {
	if until := it.WarrantyUntil(soldAt); !until.IsZero() {
		return strconv.Itoa(it.WarrantyMonths) + "-month warranty until " + datetime.Date(until)
	}
	return ""
}

func slipRefund(rr sales.ReturnReceipt) *SlipRefund {
	r := &SlipRefund{ReceiptNo: rr.ReceiptNo, Date: datetime.DateTime(rr.CreatedAt), Customer: strOr(rr.CustomerName),
		Reason: strOr(rr.Reason), Refund: amt(rr.Refund), CreditReduction: amt(rr.CreditReduction)}
	if rr.RemainingBalance != nil {
		r.RemainingBalance = amt(*rr.RemainingBalance)
	}
	for _, it := range rr.Items {
		r.Items = append(r.Items, SlipRefundItem{Name: it.ProductName, Quantity: it.Quantity.String(), UnitAbbr: it.UnitAbbr, Refund: amt(it.Refund)})
	}
	return r
}

func slipMoney(r cashflow.Receipt) *SlipMoney {
	return &SlipMoney{Title: receiptKindLabel(r.Kind), ReceiptNo: r.ReceiptNo, Date: r.CreatedAt.Format("2006-01-02 15:04"),
		From: r.FromLabel, To: r.ToLabel, Party: r.Party, By: strOr(r.CreatedByName), Amount: amt(r.Amount), Note: r.Note}
}

func slipDebt(s escpos.DebtSlip) *SlipDebt {
	d := &SlipDebt{ReceiptNo: s.ReceiptNo, Date: s.Date, Customer: s.CustomerName, Phone: s.CustomerPhone, Amount: amt(s.Amount),
		Method: s.Method, Cashier: s.CashierName, Cheques: []SlipPayment{}}
	for _, c := range s.Cheques {
		d.Cheques = append(d.Cheques, SlipPayment{Method: escpos.ChequeText(c.Number, c.Bank, c.Date), Amount: amt(c.Amount)})
	}
	if s.BalanceBefore != nil && s.BalanceAfter != nil {
		d.BalanceBefore, d.BalanceAfter = amt(*s.BalanceBefore), amt(*s.BalanceAfter)
	}
	return d
}

func slipWarranty(s escpos.WarrantySlip) *SlipWarranty {
	return &SlipWarranty{Date: datetime.Date(time.Now()), Customer: s.CustomerName, Product: s.ProductName, OldSerial: s.OldSerial,
		NewSerial: s.NewSerial, WarrantyUntil: s.WarrantyUntil, WarrantyLeft: s.WarrantyLeft}
}
