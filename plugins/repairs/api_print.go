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

// slipData is ?data=1: the job slip's print data for its React template
// (web/src/plugins/repairs/JobSlip.tsx), each value the one repairSlip prints.
type slipData struct {
	Kind     string          `json:"kind"` // "repairs.job"
	Width    string          `json:"width"`
	Dots     int             `json:"dots"`
	Columns  int             `json:"columns"`
	Currency string          `json:"currency"`
	Shop     receiptimg.Shop `json:"shop"`
	Job      slipJob         `json:"job"`
}

type slipJob struct {
	TicketNo      string     `json:"ticketNo"`
	Received      string     `json:"received"`
	Customer      string     `json:"customer"`
	Device        string     `json:"device"` // "type - model", "" = none
	Fault         string     `json:"fault"`
	Open          bool       `json:"open"`
	ReadyBy       string     `json:"readyBy"`       // open jobs only
	WarrantyUntil string     `json:"warrantyUntil"` // collected, job warranty mode only
	Parts         []slipPart `json:"parts"`
	Charges       []string   `json:"charges"`
	Estimate      string     `json:"estimate"` // open jobs
	Deposit       string     `json:"deposit"`  // open jobs
	PaidOn        string     `json:"paidOn"`   // the sale's receipt no once collected
}

type slipPart struct {
	Name string `json:"name"` // with "(30d warranty)" in parts warranty mode
	Qty  string `json:"qty"`
}

func jobSlipData(cfg settings.Settings, d *Detail, saleNo string, opts escpos.Options, mode string) slipData {
	out := slipData{Kind: "repairs.job", Width: "80", Dots: receiptimg.PrinterDots(cfg.ReceiptWidth), Columns: escpos.Columns(cfg.ReceiptWidth),
		Currency: cfg.CurrencySymbol, Shop: receiptimg.ShopFor(&cfg, opts)}
	if cfg.ReceiptWidth == "58" {
		out.Width = "58"
	}
	if out.Currency == "" {
		out.Currency = "Rs."
	}
	j := &out.Job
	j.TicketNo, j.Received, j.Customer, j.Fault = d.Job.TicketNo, datetime.DateTime(d.Job.CreatedAt), d.Job.CustomerName, d.Job.Fault
	j.Device = strings.Trim(strings.TrimSpace(d.Job.RepairType+" - "+d.Job.DeviceModel), " -")
	j.Open = d.Job.Status != "collected" && d.Job.Status != "cancelled"
	if j.Open && d.Job.PromisedDate != nil {
		j.ReadyBy = datetime.Date(*d.Job.PromisedDate)
	}
	if !j.Open && mode != "parts" && d.Job.WarrantyUntil != nil {
		j.WarrantyUntil = datetime.Date(*d.Job.WarrantyUntil)
	}
	for _, p := range d.Parts {
		name := p.ProductName
		if mode == "parts" && p.WarrantyDays > 0 {
			name += " (" + strconv.Itoa(p.WarrantyDays) + "d warranty)"
		}
		j.Parts = append(j.Parts, slipPart{Name: name, Qty: p.Qty.String()})
	}
	for _, ch := range d.Charges {
		j.Charges = append(j.Charges, ch.Label)
	}
	if j.Open {
		j.Estimate, j.Deposit = estimate(d).StringFixed(2), deposits(d).Add(legacyHeld(d)).StringFixed(2)
	} else {
		j.PaidOn = saleNo
	}
	return out
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
// job, ?preview=1 answers the core slip preview shape instead, ?data=1 the
// template's data).
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
	opts := receiptimg.SlipOptions(ctx, &cfg, nil)
	if c.QueryParam("data") == "1" {
		return c.JSON(http.StatusOK, jobSlipData(cfg, d, saleNo, opts, rc.WarrantyMode))
	}
	slip := repairSlip(cfg, d, saleNo, opts, rc.WarrantyMode)
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
	// Image mode: the SPA sends the slip as a picture at the printer's dots.
	if strings.HasPrefix(c.Request().Header.Get("Content-Type"), "image/png") {
		dots := receiptimg.PrinterDots(cfg.ReceiptWidth)
		img, err := escpos.ReadImage(c.Request().Body, dots)
		if err != nil {
			return &apperr.AppError{Code: "BAD_IMAGE", Message: "That slip picture can't be printed", Status: 422, Err: err}
		}
		if slip, err = escpos.ImageJob(img, dots); err != nil {
			return err
		}
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
