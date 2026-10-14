package repairs

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// The plugin's own JSON, mirrored by hand in web/src/plugins/repairs/types.ts.
// Money and quantities are decimal strings, dates YYYY-MM-DD, times ISO 8601.

type PartDTO struct {
	ID           int64  `json:"id"`
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	Qty          string `json:"qty"`
	Price        string `json:"price"`       // today's selling price, each
	OldDiscount  string `json:"oldDiscount"` // a karots-pos row's discount ("10%" / "80.00 each"), "" for new rows
	WarrantyDays int    `json:"warrantyDays"`
}

type ChargeDTO struct {
	ID     int64  `json:"id"`
	Label  string `json:"label"`
	Amount string `json:"amount"`
}

type DepositDTO struct {
	PaymentID int64  `json:"paymentId"`
	Amount    string `json:"amount"`
	ReceiptNo string `json:"receiptNo"`
	At        string `json:"at"`
}

type PayoutDTO struct {
	ExpenseID int64  `json:"expenseId"`
	Amount    string `json:"amount"`
	Note      string `json:"note"`
	Date      string `json:"date"`
}

// RowDTO is a job in a list.
type RowDTO struct {
	ID            int64  `json:"id"`
	TicketNo      string `json:"ticketNo"`
	TicketCode    string `json:"ticketCode"`
	CustomerName  string `json:"customerName"`
	CustomerPhone string `json:"customerPhone"`
	RepairType    string `json:"repairType"`
	DeviceModel   string `json:"deviceModel"`
	Status        string `json:"status"`
	Due           string `json:"due"`
	DueUrgent     bool   `json:"dueUrgent"`
	CreatedAt     string `json:"createdAt"`
	CollectedAt   string `json:"collectedAt"`
}

type JobDTO struct {
	RowDTO
	CustomerID    *int64       `json:"customerId"`
	RepairedBy    string       `json:"repairedBy"`
	Fault         string       `json:"fault"`
	Notes         string       `json:"notes"`
	PromisedDate  string       `json:"promisedDate"`
	Urgent        bool         `json:"urgent"`
	WarrantyDays  int          `json:"warrantyDays"`
	WarrantyUntil string       `json:"warrantyUntil"`
	InWarranty    bool         `json:"inWarranty"`
	SaleID        *int64       `json:"saleId"`
	SaleReceipt   string       `json:"saleReceipt"` // the pickup sale's receipt no
	ReworkOf      *int64       `json:"reworkOf"`
	ReadyAt       string       `json:"readyAt"`
	CancelledAt   string       `json:"cancelledAt"`
	Parts         []PartDTO    `json:"parts"`
	Charges       []ChargeDTO  `json:"charges"`
	Estimate      string       `json:"estimate"`
	Deposits      []DepositDTO `json:"deposits"`
	DepositTotal  string       `json:"depositTotal"`
	LegacyHeld    string       `json:"legacyHeld"`  // a karots-pos deposit still held (cash)
	Outstanding   string       `json:"outstanding"` // the customer's balance now (negative = advance)
	RepairerPaid  string       `json:"repairerPaid"`
	Payouts       []PayoutDTO  `json:"payouts"`
	WarrantyMode  string       `json:"warrantyMode"`
}

type JobRequest struct {
	CustomerID   int64  `json:"customerId"`
	RepairType   string `json:"repairType"`
	DeviceModel  string `json:"deviceModel"`
	RepairedBy   string `json:"repairedBy"`
	Fault        string `json:"fault"`
	Notes        string `json:"notes"`
	PromisedDate string `json:"promisedDate"`
	Urgent       bool   `json:"urgent"`
	WarrantyDays *int   `json:"warrantyDays"` // nil = the shop's default
}

func iso(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

func ymd(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func dec2(d decimal.Decimal) string { return d.StringFixed(2) }

func idParam(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid id")
	}
	return id, nil
}

type handlers struct{ p *Plugin }

func (p *Plugin) routes(reg *plugin.Registry) {
	h := handlers{p}
	t := reg.Cashier() // /api/v1/x — anyone signed in: the till's 🔧 panel and the pages
	t.GET("/repairs/config", h.config)
	t.GET("/repairs/suggest", h.suggest)
	t.GET("/repairs/jobs", h.list)
	t.POST("/repairs/jobs", h.create)
	t.GET("/repairs/jobs/:id", h.get)
	t.PUT("/repairs/jobs/:id", h.update)
	t.POST("/repairs/jobs/:id/status", h.status)
	t.POST("/repairs/jobs/:id/urgent", h.urgent)
	t.POST("/repairs/jobs/:id/cancel", h.cancel)
	t.POST("/repairs/jobs/:id/parts", h.addPart)
	t.DELETE("/repairs/jobs/:id/parts/:pid", h.removePart)
	t.POST("/repairs/jobs/:id/charges", h.addCharge)
	t.DELETE("/repairs/jobs/:id/charges/:cid", h.removeCharge)
	t.GET("/repairs/jobs/:id/bill", h.bill)
	t.POST("/repairs/jobs/:id/deposits", h.linkDeposit)
	t.POST("/repairs/jobs/:id/rework", h.rework)
	t.POST("/repairs/jobs/:id/refund-held", h.refundHeld)
	t.POST("/repairs/jobs/:id/print", h.print)
	t.GET("/repairs/labour", h.labour)
	a := reg.Admin() // admin + manager
	a.PUT("/repairs/config", h.setConfig)
	a.POST("/repairs/jobs/:id/payouts", h.payout)
	a.GET("/repairs/report", h.report)
}

func (h handlers) audit(c echo.Context, action string, id int64, detail string) {
	h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, "repair", strconv.FormatInt(id, 10), detail)
}

func (h handlers) config(c echo.Context) error {
	cfg, err := h.p.store.Config(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"warrantyMode": cfg.WarrantyMode, "defaultWarrantyDays": cfg.DefaultWarrantyDays})
}

func (h handlers) setConfig(c echo.Context) error {
	var req struct {
		WarrantyMode        string `json:"warrantyMode"`
		DefaultWarrantyDays int    `json:"defaultWarrantyDays"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.p.store.SetConfig(c.Request().Context(), req.WarrantyMode, req.DefaultWarrantyDays); err != nil {
		return err
	}
	h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), audit.ActionSettings, "repair", "config",
		"repair warranty "+req.WarrantyMode+", default "+strconv.Itoa(req.DefaultWarrantyDays)+" days")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) suggest(c echo.Context) error {
	s, err := h.p.store.Suggestions(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, s)
}

// labour is the labour product the till rings labour and quick fixes on.
func (h handlers) labour(c echo.Context) error {
	id, err := h.p.store.LabourID(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]int64{"productId": id})
}

func (h handlers) jobInput(c echo.Context, req JobRequest) (JobInput, error) {
	ctx := c.Request().Context()
	if req.CustomerID <= 0 {
		return JobInput{}, apperr.Validation("pick the customer — we must be able to call them")
	}
	cust, err := customers.NewService(h.p.core.DB).Get(ctx, req.CustomerID)
	if err != nil {
		return JobInput{}, err
	}
	in := JobInput{
		CustomerID: &req.CustomerID, CustomerName: cust.Name, RepairType: strings.TrimSpace(req.RepairType),
		DeviceModel: strings.TrimSpace(req.DeviceModel), RepairedBy: strings.TrimSpace(req.RepairedBy),
		Fault: strings.TrimSpace(req.Fault), Notes: strings.TrimSpace(req.Notes), Urgent: req.Urgent,
		CreatedBy: middleware.CurrentUserID(c),
	}
	if cust.Phone != nil {
		in.CustomerPhone = *cust.Phone
	}
	if in.DeviceModel == "" && in.RepairType == "" {
		return in, apperr.Validation("say what device it is or what the repair is")
	}
	if s := strings.TrimSpace(req.PromisedDate); s != "" {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return in, apperr.Validation("the promised date is not a date")
		}
		in.PromisedDate = &d
	}
	if req.WarrantyDays != nil {
		if *req.WarrantyDays < 0 || *req.WarrantyDays > 3650 {
			return in, apperr.Validation("warranty days must be 0 to 3650")
		}
		in.WarrantyDays = *req.WarrantyDays
	} else if cfg, err := h.p.store.Config(ctx); err == nil {
		in.WarrantyDays = cfg.DefaultWarrantyDays
	}
	return in, nil
}

func (h handlers) create(c echo.Context) error {
	var req JobRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in, err := h.jobInput(c, req)
	if err != nil {
		return err
	}
	id, err := h.p.store.CreateJob(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, id, "repair opened: "+in.DeviceModel+" for "+in.CustomerName)
	return h.respond(c, id)
}

// open loads a job that may still change (not collected or cancelled).
func (h handlers) open(c echo.Context) (*Detail, error) {
	id, err := idParam(c, "id")
	if err != nil {
		return nil, err
	}
	d, err := h.p.store.GetJob(c.Request().Context(), id)
	if err != nil {
		return nil, err
	}
	if d.Job.Status == "collected" || d.Job.Status == "cancelled" {
		return nil, apperr.Validation("this repair is " + d.Job.Status + " — it can't change now")
	}
	return d, nil
}

func (h handlers) get(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	return h.respond(c, id)
}

func (h handlers) update(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	var req JobRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in, err := h.jobInput(c, req)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	if err := h.p.store.UpdateJob(ctx, d.Job.ID, in); err != nil {
		return err
	}
	h.auditDeadline(c, d.Job, in.PromisedDate)
	return h.respond(c, d.Job.ID)
}

// auditDeadline puts a pickup-date change in the audit log — for "you promised
// it sooner" (karots-pos auditDeadline).
func (h handlers) auditDeadline(c echo.Context, j Job, next *time.Time) {
	if next == nil || (j.PromisedDate != nil && ymd(j.PromisedDate) == ymd(next)) {
		return
	}
	msg := "set pickup date " + ymd(next)
	if j.PromisedDate != nil {
		msg = "moved pickup date " + ymd(j.PromisedDate) + " → " + ymd(next)
	}
	h.audit(c, audit.ActionUpdate, j.ID, msg+" ("+j.TicketNo+")")
}

func (h handlers) status(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	switch req.Status {
	case "received", "in_progress", "ready":
	default:
		return apperr.Validation("invalid status")
	}
	if err := h.p.store.SetStatus(c.Request().Context(), d.Job.ID, req.Status); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, d.Job.ID, d.Job.TicketNo+" "+strings.ReplaceAll(req.Status, "_", " "))
	return h.respond(c, d.Job.ID)
}

func (h handlers) urgent(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	var req struct {
		Urgent       bool   `json:"urgent"`
		PromisedDate string `json:"promisedDate"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	var next *time.Time
	if s := strings.TrimSpace(req.PromisedDate); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return apperr.Validation("the date is not a date")
		}
		next = &t
	}
	if err := h.p.store.SetUrgent(c.Request().Context(), d.Job.ID, req.Urgent, next); err != nil {
		return err
	}
	h.auditDeadline(c, d.Job, next)
	return h.respond(c, d.Job.ID)
}

// cancel closes an open job. Its deposit isn't touched here: the till then
// offers it back (the customer's advance, or a karots-pos held deposit).
func (h handlers) cancel(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	if err := h.p.store.SetStatus(c.Request().Context(), d.Job.ID, "cancelled"); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, d.Job.ID, "repair cancelled: "+d.Job.TicketNo)
	return h.respond(c, d.Job.ID)
}

func (h handlers) addPart(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	var req struct {
		ProductID    int64  `json:"productId"`
		Qty          string `json:"qty"`
		WarrantyDays int    `json:"warrantyDays"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	qty, err := decimal.NewFromString(strings.TrimSpace(req.Qty))
	if err != nil || !qty.IsPositive() {
		return apperr.Validation("quantity must be more than 0")
	}
	ctx := c.Request().Context()
	prod, err := h.p.core.Products.Get(ctx, req.ProductID)
	if err != nil {
		return err
	}
	if prod.IsService {
		return apperr.Validation("a part is a stocked product — add work as labour")
	}
	if req.WarrantyDays < 0 || req.WarrantyDays > 3650 {
		return apperr.Validation("warranty days must be 0 to 3650")
	}
	if err := h.p.store.AddPart(ctx, d.Job.ID, prod.ID, qty, req.WarrantyDays); err != nil {
		return err
	}
	return h.respond(c, d.Job.ID)
}

func (h handlers) removePart(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	pid, err := idParam(c, "pid")
	if err != nil {
		return err
	}
	if err := h.p.store.RemovePart(c.Request().Context(), d.Job.ID, pid); err != nil {
		return err
	}
	return h.respond(c, d.Job.ID)
}

func (h handlers) addCharge(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	var req struct {
		Label  string `json:"label"`
		Amount string `json:"amount"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	amt, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amt.IsNegative() {
		return apperr.Validation("enter the amount")
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = "Labour"
	}
	if err := h.p.store.AddCharge(c.Request().Context(), d.Job.ID, label, amt.Round(2)); err != nil {
		return err
	}
	return h.respond(c, d.Job.ID)
}

func (h handlers) removeCharge(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	cid, err := idParam(c, "cid")
	if err != nil {
		return err
	}
	if err := h.p.store.RemoveCharge(c.Request().Context(), d.Job.ID, cid); err != nil {
		return err
	}
	return h.respond(c, d.Job.ID)
}
