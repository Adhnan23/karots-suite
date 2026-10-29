package api

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/printing"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

const printFailed = "Receipt didn't print — check the printer is on and connected"

// slipSettings loads settings for a slip print, honouring ?size=58|80 for this
// one job. It overrides a COPY, never the
// settings the rest of the request shares.
func (h *handlers) slipSettings(c echo.Context) (*settings.Settings, error) {
	cfg, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return nil, err
	}
	eff := *cfg
	if sz := c.QueryParam("size"); sz == "58" || sz == "80" {
		eff.ReceiptWidth = sz
	}
	return &eff, nil
}

// receiptQueue is the print target for the signed-in cashier: their own counter
// printer when set on their user record, else the shop-wide receipt printer
// ("" = OS default; "tcp://host:9100" = network).
func (h *handlers) receiptQueue(c echo.Context, cfg *settings.Settings) string {
	if uid := middleware.CurrentUserID(c); uid != 0 {
		if u, err := h.d.Auth.GetUser(c.Request().Context(), uid); err == nil && strings.TrimSpace(u.ReceiptPrinter) != "" {
			return u.ReceiptPrinter
		}
	}
	return cfg.ReceiptPrinter
}

// previewSlip answers ?preview=1 with the slip as the paper shows it, instead of
// printing. Call it with the slip's bytes BEFORE the drawer kick is folded in and
// before the printer lookup: a preview never opens the drawer and needs no printer.
func previewSlip(c echo.Context, cfg *settings.Settings, payload []byte) (bool, error) {
	if c.QueryParam("preview") != "1" {
		return false, nil
	}
	p := SlipPreview{Width: "80", Columns: escpos.Columns(cfg.ReceiptWidth), Dots: receiptimg.PrinterDots(cfg.ReceiptWidth)}
	if cfg.ReceiptWidth == "58" {
		p.Width = "58"
	}
	for _, l := range escpos.Preview(payload) {
		p.Lines = append(p.Lines, SlipLine{Text: l.Text, Align: l.Align, Bold: l.Bold, Wide: l.Wide, Tall: l.Tall, Small: l.Small, Image: l.Image, ImageDots: l.ImageDots})
	}
	return true, c.JSON(http.StatusOK, p)
}

// TillPrintMoneyReceipt prints a CR- slip (till open/close/withdraw/deposit).
// ?kick=1 (fresh withdraw/deposit in auto-print mode) folds the drawer pulse into
// this one job; reprints never pass it.
func (h *handlers) TillPrintMoneyReceipt(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	rec, err := h.t.receipts.Get(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.slipSettings(c)
	if err != nil {
		return err
	}
	opts := receiptimg.SlipOptions(ctx, cfg, nil)
	if ok, err := slipData(c, cfg, opts, func(d *SlipData) { d.Kind, d.Money = "money", slipMoney(*rec) }); ok {
		return err
	}
	slip := moneySlip(cfg, *rec, opts)
	if ok, err := previewSlip(c, cfg, slip); ok {
		return err
	}
	if job, ok, err := slipImage(c, cfg); err != nil {
		return err
	} else if ok {
		slip = job
	}
	target := h.receiptQueue(c, cfg)
	if strings.TrimSpace(target) == "" {
		return apperr.ConflictCode("NO_PRINTER", "No receipt printer configured")
	}
	if c.QueryParam("kick") == "1" {
		slip = append(escpos.DrawerKick(*cfg), slip...)
	}
	if err := printing.Raw(ctx, target, slip); err != nil {
		log.Printf("money slip %d: print: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}

// moneySlip renders a money receipt as ESC/POS with the shared receipt header and
// footer. Ported from karots-pos web/admin_money_receipts.go buildReceiptSlip.
func moneySlip(cfg *settings.Settings, r cashflow.Receipt, opts escpos.Options) []byte {
	w := escpos.Columns(cfg.ReceiptWidth)
	var b bytes.Buffer
	escpos.Init(&b)
	escpos.Header(&b, *cfg, opts)
	escpos.Title(&b, *cfg, receiptKindLabel(r.Kind), w)
	escpos.Left(&b)
	escpos.Divider(&b, *cfg, w)
	escpos.Line(&b, escpos.LeftRight("Receipt:", r.ReceiptNo, w))
	escpos.Line(&b, escpos.LeftRight("Date:", r.CreatedAt.Format("2006-01-02 15:04"), w))
	escpos.Line(&b, escpos.LeftRight("From:", escpos.ASCII(r.FromLabel), w))
	escpos.Line(&b, escpos.LeftRight("To:", escpos.ASCII(r.ToLabel), w))
	if strings.TrimSpace(r.Party) != "" {
		escpos.Line(&b, escpos.LeftRight("Party:", escpos.ASCII(r.Party), w))
	}
	if r.CreatedByName != nil && *r.CreatedByName != "" {
		escpos.Line(&b, escpos.LeftRight("By:", escpos.ASCII(*r.CreatedByName), w))
	}
	escpos.Divider(&b, *cfg, w)
	escpos.Emphasis(&b, true)
	escpos.Line(&b, escpos.LeftRight("Amount", money.Format(cfg.CurrencySymbol, r.Amount), w))
	escpos.Emphasis(&b, false)
	if strings.TrimSpace(r.Note) != "" {
		for _, ln := range escpos.Wrap(escpos.ASCII("Note: "+r.Note), w) {
			escpos.Line(&b, ln)
		}
	}
	escpos.Divider(&b, *cfg, w)
	escpos.Center(&b)
	escpos.Line(&b, "Signature: ____________________")
	escpos.Line(&b, "")
	escpos.Line(&b, "")
	escpos.Footer(&b, *cfg)
	return b.Bytes()
}

// receiptKindLabel is the human label for a core receipt kind. Plugin kinds fall
// through to their raw name (M7 can add a label hook).
func receiptKindLabel(k string) string {
	switch k {
	case "transfer":
		return "Transfer"
	case "payment":
		return "Payment"
	case "intake":
		return "Intake"
	case "supplier_payment":
		return "Supplier payment"
	case "supplier_refund":
		return "Supplier refund"
	case "customer_payment":
		return "Customer payment"
	case "expense":
		return "Expense"
	case "expense_void":
		return "Expense voided"
	case "cheque_returned":
		return "Cheque returned"
	case "refund":
		return "Refund"
	case "capital":
		return "Capital"
	case "bank_charge":
		return "Bank charge"
	case "interest":
		return "Interest"
	case "adjust":
		return "Adjustment"
	case "deposit":
		return "Deposit"
	case "withdrawal":
		return "Withdrawal"
	}
	return k
}

// TillPrintSale sends a sale receipt to the thermal printer as ESC/POS. ?kick=1
// (the till's auto-print of a fresh cash sale) folds the drawer pulse into THIS
// job — one USB job, no inter-job gap. Reprints never pass kick.
func (h *handlers) TillPrintSale(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	detail, err := h.d.Sales.Get(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.slipSettings(c)
	if err != nil {
		return err
	}
	opts := receiptimg.SlipOptions(ctx, cfg, nil)
	opts.Serials = h.saleSerials(ctx, id)
	opts.CustomerDue = h.customerDue(ctx, detail)
	if ok, err := slipData(c, cfg, opts, func(d *SlipData) { d.Kind, d.Sale = "sale", slipSale(*detail, opts) }); ok {
		return err
	}
	payload := escpos.Document(*detail, *cfg, opts)
	if ok, err := previewSlip(c, cfg, payload); ok {
		return err
	}
	if job, ok, err := slipImage(c, cfg); err != nil {
		return err
	} else if ok {
		payload = job
	}
	if c.QueryParam("kick") == "1" && paidCash(detail) {
		payload = append(escpos.DrawerKick(*cfg), payload...)
	}
	if err := escpos.Send(ctx, h.receiptQueue(c, cfg), payload); err != nil {
		log.Printf("sale %d: print: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}

func paidCash(d *sales.Detail) bool {
	for _, p := range d.Payments {
		if p.Method == "cash" && p.Amount.IsPositive() {
			return true
		}
	}
	return false
}

// customerDue is the customer's outstanding balance for the "Total due" line on
// credit receipts. Best-effort: zero on any failure.
func (h *handlers) customerDue(ctx context.Context, d *sales.Detail) decimal.Decimal {
	if d.Sale.CustomerID == nil {
		return decimal.Zero
	}
	cust, err := h.t.customers.Get(ctx, *d.Sale.CustomerID)
	if err != nil {
		return decimal.Zero
	}
	return cust.OutstandingBalance
}

// saleSerials labels the serial-tracked units on the receipt ("SN: … (wty …)").
func (h *handlers) saleSerials(ctx context.Context, saleID int64) map[int64][]string {
	units, err := h.t.warranty.UnitsForSale(ctx, saleID)
	if err != nil || len(units) == 0 {
		return nil
	}
	m := make(map[int64][]string, len(units))
	for _, u := range units {
		label := "SN: " + u.SerialNo
		if u.WarrantyMonths > 0 {
			label += " (wty " + datetime.Date(u.WarrantyUntil) + ")"
		}
		m[u.ProductID] = append(m[u.ProductID], label)
	}
	return m
}
