package repairs

import (
	"bytes"
	"log"
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/datetime"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/printing"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
)

// repairSlip is the job on the receipt printer: a claim ticket while it's open
// (what was left, the promised date, the estimate and deposits), the work done
// and its warranty once collected (the money is on the sale's receipt).
func repairSlip(cfg settings.Settings, d *Detail, saleNo string, opts escpos.Options, mode string) []byte {
	w := escpos.Columns(cfg.ReceiptWidth)
	sym := cfg.CurrencySymbol
	if sym == "" {
		sym = "Rs."
	}
	var b bytes.Buffer
	escpos.Init(&b)
	escpos.Header(&b, cfg, opts)
	escpos.Title(&b, cfg, "REPAIR", w)
	escpos.Left(&b)
	escpos.Divider(&b, cfg, w)
	escpos.Emphasis(&b, true)
	escpos.Line(&b, escpos.LeftRight("Ticket:", d.Job.TicketNo, w))
	escpos.Emphasis(&b, false)
	escpos.Line(&b, escpos.LeftRight("Received:", datetime.DateTime(d.Job.CreatedAt), w))
	if d.Job.CustomerName != "" {
		escpos.Line(&b, escpos.LeftRight("Customer:", escpos.ASCII(d.Job.CustomerName), w))
	}
	if dev := strings.TrimSpace(d.Job.RepairType + " - " + d.Job.DeviceModel); dev != "-" {
		for _, ln := range escpos.Wrap(escpos.ASCII(strings.Trim(dev, " -")), w) {
			escpos.Line(&b, ln)
		}
	}
	if d.Job.Fault != "" {
		for _, ln := range escpos.Wrap(escpos.ASCII("Fault: "+d.Job.Fault), w) {
			escpos.Line(&b, ln)
		}
	}
	open := d.Job.Status != "collected" && d.Job.Status != "cancelled"
	if open && d.Job.PromisedDate != nil {
		escpos.Line(&b, escpos.LeftRight("Ready by:", datetime.Date(*d.Job.PromisedDate), w))
	}
	if !open && mode != "parts" && d.Job.WarrantyUntil != nil {
		escpos.Line(&b, escpos.LeftRight("Warranty until:", datetime.Date(*d.Job.WarrantyUntil), w))
	}
	if len(d.Parts)+len(d.Charges) > 0 {
		escpos.Divider(&b, cfg, w)
		for _, p := range d.Parts {
			name := p.ProductName
			if mode == "parts" && p.WarrantyDays > 0 {
				name += " (" + strconv.Itoa(p.WarrantyDays) + "d warranty)"
			}
			escpos.Line(&b, escpos.LeftRight(escpos.ASCII(name), "x"+money.Display(p.Qty), w))
		}
		for _, ch := range d.Charges {
			escpos.Line(&b, escpos.ASCII(ch.Label))
		}
	}
	escpos.Divider(&b, cfg, w)
	if open {
		if est := estimate(d); est.IsPositive() {
			escpos.Line(&b, escpos.LeftRight("Estimate", money.Format(sym, est), w))
		}
		if dep := deposits(d).Add(legacyHeld(d)); dep.IsPositive() {
			escpos.Emphasis(&b, true)
			escpos.Line(&b, escpos.LeftRight("Deposit paid", money.Format(sym, dep), w))
			escpos.Emphasis(&b, false)
		}
	} else if saleNo != "" {
		escpos.Line(&b, escpos.LeftRight("Paid on receipt", saleNo, w))
	}
	escpos.Center(&b)
	if open {
		escpos.Line(&b, "Bring this slip to collect.")
	}
	escpos.Footer(&b, cfg)
	return b.Bytes()
}

type slipLine struct {
	Text      string `json:"text"`
	Align     string `json:"align"`
	Bold      bool   `json:"bold"`
	Wide      bool   `json:"wide"`
	Tall      bool   `json:"tall"`
	Small     bool   `json:"small"`
	Image     string `json:"image,omitempty"`
	ImageDots int    `json:"imageDots,omitempty"`
}

// print sends the slip to this cashier's receipt printer (?size=58|80 for one
// job, ?preview=1 answers the core slip preview shape instead).
func (h handlers) print(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	d, err := h.p.store.GetJob(ctx, id)
	if err != nil {
		return err
	}
	cfg0, err := h.p.core.Settings.Get(ctx)
	if err != nil {
		return err
	}
	cfg := *cfg0
	if sz := c.QueryParam("size"); sz == "58" || sz == "80" {
		cfg.ReceiptWidth = sz
	}
	rc, err := h.p.store.Config(ctx)
	if err != nil {
		return err
	}
	saleNo := ""
	if d.Job.SaleID != nil {
		if s, err := h.p.core.Sales.Get(ctx, *d.Job.SaleID); err == nil {
			saleNo = s.Sale.ReceiptNo
		}
	}
	slip := repairSlip(cfg, d, saleNo, receiptimg.SlipOptions(ctx, &cfg, nil), rc.WarrantyMode)
	if c.QueryParam("preview") == "1" {
		out := struct {
			Width   string     `json:"width"`
			Columns int        `json:"columns"`
			Dots    int        `json:"dots"`
			Lines   []slipLine `json:"lines"`
		}{Width: "80", Columns: escpos.Columns(cfg.ReceiptWidth), Dots: receiptimg.PrinterDots(cfg.ReceiptWidth)}
		if cfg.ReceiptWidth == "58" {
			out.Width = "58"
		}
		for _, l := range escpos.Preview(slip) {
			out.Lines = append(out.Lines, slipLine(l))
		}
		return c.JSON(http.StatusOK, out)
	}
	target := cfg.ReceiptPrinter
	var own string
	if err := h.p.core.DB.GetContext(ctx, &own, `SELECT COALESCE(receipt_printer, '') FROM users WHERE id = $1`, middleware.CurrentUserID(c)); err == nil && strings.TrimSpace(own) != "" {
		target = own
	}
	if strings.TrimSpace(target) == "" {
		return apperr.ConflictCode("NO_PRINTER", "No receipt printer configured")
	}
	if err := printing.Raw(ctx, target, slip); err != nil {
		log.Printf("repair slip %d: print: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", "Slip didn't print — check the printer is on and connected")
	}
	return c.NoContent(http.StatusNoContent)
}
