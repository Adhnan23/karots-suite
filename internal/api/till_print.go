package api

import (
	"bytes"
	"log"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/printing"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
)

const printFailed = "Receipt didn't print — check the printer is on and connected"

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
	cfg, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	target := h.receiptQueue(c, cfg)
	if strings.TrimSpace(target) == "" {
		return apperr.ConflictCode("NO_PRINTER", "No receipt printer configured")
	}
	slip := moneySlip(cfg, *rec, receiptimg.SlipOptions(ctx, cfg, nil))
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
