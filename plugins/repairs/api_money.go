package repairs

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/expenses"
	"karots-suite/internal/features/payout"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// respond answers with the job as the panels show it.
func (h handlers) respond(c echo.Context, id int64) error {
	ctx := c.Request().Context()
	d, err := h.p.store.GetJob(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.p.store.Config(ctx)
	if err != nil {
		return err
	}
	out := JobDTO{RowDTO: rowDTO(d.Job, time.Now()),
		CustomerID: d.Job.CustomerID, RepairedBy: d.Job.RepairedBy, Fault: d.Job.Fault, Notes: d.Job.Notes,
		PromisedDate: ymd(d.Job.PromisedDate), Urgent: d.Job.Urgent, WarrantyDays: d.Job.WarrantyDays,
		WarrantyUntil: ymd(d.Job.WarrantyUntil), InWarranty: inWarranty(d.Job, time.Now()), SaleID: d.Job.SaleID,
		ReworkOf: d.Job.ReworkOf, ReadyAt: iso(d.Job.ReadyAt), CancelledAt: iso(d.Job.CancelledAt),
		Parts: []PartDTO{}, Charges: []ChargeDTO{}, Deposits: []DepositDTO{}, Payouts: []PayoutDTO{},
		Estimate: dec2(estimate(d)), DepositTotal: dec2(deposits(d)), LegacyHeld: dec2(legacyHeld(d)),
		RepairerPaid: dec2(repairerPaid(d)), Outstanding: "0.00", WarrantyMode: cfg.WarrantyMode}
	for _, p := range d.Parts {
		old := ""
		if p.DiscountValue.IsPositive() {
			old = p.DiscountValue.String() + "%"
			if p.DiscountType != "percent" {
				old = dec2(p.DiscountValue) + " each"
			}
		}
		out.Parts = append(out.Parts, PartDTO{ID: p.ID, ProductID: p.ProductID, Name: p.ProductName, Qty: p.Qty.String(),
			Price: dec2(p.Price), OldDiscount: old, WarrantyDays: p.WarrantyDays})
	}
	for _, ch := range d.Charges {
		out.Charges = append(out.Charges, ChargeDTO{ID: ch.ID, Label: ch.Label, Amount: dec2(ch.Amount)})
	}
	for _, x := range d.Deposits {
		out.Deposits = append(out.Deposits, DepositDTO{PaymentID: x.PaymentID, Amount: dec2(x.Amount), ReceiptNo: x.ReceiptNo, At: iso(&x.CreatedAt)})
	}
	for _, x := range d.Payouts {
		out.Payouts = append(out.Payouts, PayoutDTO{ExpenseID: x.ExpenseID, Amount: dec2(x.Amount), Note: x.Note, Date: ymd(&x.Date)})
	}
	if d.Job.CustomerID != nil {
		if cust, err := customers.NewService(h.p.core.DB).Get(ctx, *d.Job.CustomerID); err == nil {
			out.Outstanding = dec2(cust.OutstandingBalance)
		}
	}
	if d.Job.SaleID != nil {
		if s, err := h.p.core.Sales.Get(ctx, *d.Job.SaleID); err == nil {
			out.SaleReceipt = s.Sale.ReceiptNo
		}
	}
	return c.JSON(http.StatusOK, out)
}

func rowDTO(j Job, now time.Time) RowDTO {
	due, urgent := dueLabel(j, now)
	return RowDTO{ID: j.ID, TicketNo: j.TicketNo, TicketCode: j.TicketCode, CustomerName: j.CustomerName,
		CustomerPhone: j.CustomerPhone, RepairType: j.RepairType, DeviceModel: j.DeviceModel, Status: j.Status,
		Due: due, DueUrgent: urgent, CreatedAt: iso(&j.CreatedAt), CollectedAt: iso(j.CollectedAt)}
}

// BillLine is one line Put on the bill adds to the cart.
type BillLine struct {
	Kind      string `json:"kind"` // part | labour
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`  // labour: the line's name
	Qty       string `json:"qty"`   // part
	Price     string `json:"price"` // labour
	// Discount is a karots-pos row's frozen discount, or 100% on a free re-repair.
	Discount     string `json:"discount"`
	DiscountType string `json:"discountType"`
}

type BillDTO struct {
	CustomerID *int64     `json:"customerId"`
	Lines      []BillLine `json:"lines"`
	Free       bool       `json:"free"`
	// OnAccount is the most a deposit can pay: the job's deposits, capped by the
	// customer's advance now. The till caps it again by what's due.
	OnAccount  string `json:"onAccount"`
	Deposits   string `json:"deposits"`
	LegacyHeld string `json:"legacyHeld"`
}

// bill is what Put on the bill adds: the job's parts at the cart's prices, its
// labour as service lines, and the deposit to pre-fill.
func (h handlers) bill(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	if len(d.Parts) == 0 && len(d.Charges) == 0 {
		return apperr.Validation("add a part or labour to the job first")
	}
	ctx := c.Request().Context()
	labour, err := h.p.store.LabourID(ctx)
	if err != nil {
		return err
	}
	free := d.Job.ReworkOf != nil
	out := BillDTO{CustomerID: d.Job.CustomerID, Free: free, Lines: []BillLine{}, Deposits: dec2(deposits(d)),
		LegacyHeld: dec2(legacyHeld(d)), OnAccount: "0.00"}
	for _, p := range d.Parts {
		l := BillLine{Kind: "part", ProductID: p.ProductID, Name: p.ProductName, Qty: p.Qty.String()}
		switch {
		case free:
			l.Discount, l.DiscountType = "100", "percent"
		case p.DiscountValue.IsPositive():
			l.Discount, l.DiscountType = p.DiscountValue.String(), "fixed"
			if p.DiscountType == "percent" {
				l.DiscountType = "percent"
			}
		}
		out.Lines = append(out.Lines, l)
	}
	for _, ch := range d.Charges {
		price := ch.Amount
		if free {
			price = decimal.Zero
		}
		out.Lines = append(out.Lines, BillLine{Kind: "labour", ProductID: labour, Name: ch.Label + " — " + d.Job.TicketNo, Price: dec2(price)})
	}
	if d.Job.CustomerID != nil && !free {
		if cust, err := customers.NewService(h.p.core.DB).Get(ctx, *d.Job.CustomerID); err == nil {
			out.OnAccount = dec2(onAccountPrefill(deposits(d), cust.OutstandingBalance, estimate(d)))
		}
	}
	return c.JSON(http.StatusOK, out)
}

// linkDeposit ties a core customer payment (taken in the core credit form) to
// the job as its deposit.
func (h handlers) linkDeposit(c echo.Context) error {
	d, err := h.open(c)
	if err != nil {
		return err
	}
	var req struct {
		PaymentID int64 `json:"paymentId"`
	}
	if err := c.Bind(&req); err != nil || req.PaymentID <= 0 {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.p.store.LinkDeposit(c.Request().Context(), d.Job.ID, req.PaymentID); err != nil {
		return err
	}
	h.audit(c, audit.ActionPayment, d.Job.ID, "deposit taken for "+d.Job.TicketNo)
	return h.respond(c, d.Job.ID)
}

// rework opens a free warranty re-repair of a collected job, linked to it.
func (h handlers) rework(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	o, err := h.p.store.GetJob(ctx, id)
	if err != nil {
		return err
	}
	if !inWarranty(o.Job, time.Now()) {
		return apperr.Validation("only a collected repair still in warranty can be redone free")
	}
	nid, err := h.p.store.CreateJob(ctx, JobInput{
		CustomerID: o.Job.CustomerID, CustomerName: o.Job.CustomerName, CustomerPhone: o.Job.CustomerPhone,
		RepairType: o.Job.RepairType, DeviceModel: o.Job.DeviceModel, RepairedBy: o.Job.RepairedBy,
		Fault: "Warranty re-repair of " + o.Job.TicketNo, ReworkOf: &o.Job.ID, CreatedBy: middleware.CurrentUserID(c),
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, nid, "warranty re-repair of "+o.Job.TicketNo)
	return h.respond(c, nid)
}

// refundHeld hands a karots-pos held deposit back from this drawer — a cancelled
// job's, or what was left after pickup. One tx with the ledger row.
func (h handlers) refundHeld(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	d, err := h.p.store.GetJob(ctx, id)
	if err != nil {
		return err
	}
	if d.Job.Status != "cancelled" && d.Job.Status != "collected" {
		return apperr.Validation("a held deposit is handed back once the repair is cancelled or collected")
	}
	uid := middleware.CurrentUserID(c)
	var rec *cashflow.Receipt
	err = appdb.WithTx(ctx, h.p.core.DB, func(tx *sqlx.Tx) error {
		held, err := legacyHeldTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !held.IsPositive() {
			return apperr.Validation("no deposit is held for this repair")
		}
		if rec, err = h.p.core.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.Till(uid), To: cashflow.External(),
			Amount: held, Reason: "Repair " + d.Job.TicketNo + " deposit refund", ReceiptKind: "refund",
			Party: d.Job.CustomerName, ActorID: uid}); err != nil {
			return err
		}
		return addPayment(ctx, tx, id, held, "refund", uid)
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionPayment, id, "deposit refunded "+dec2(rec.Amount)+": "+d.Job.TicketNo)
	return c.JSON(http.StatusOK, map[string]any{"receiptId": rec.ID, "amount": dec2(rec.Amount)})
}

// payout pays the outside repairer: a core expense (category Repairs, the
// labour service) out of a cash place or by cheques on the shop's bank places
// (core's payout), linked to the job — all in one tx.
func (h handlers) payout(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	var req struct {
		Amount  string              `json:"amount"`
		Source  string              `json:"source"` // locker:<id> | till:<userId>
		Method  string              `json:"method"` // "cheque" = paid by Cheques
		Cheques []cheques.LineInput `json:"cheques"`
		Note    string              `json:"note"`
	}
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	amt, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || !amt.IsPositive() {
		return apperr.Validation("enter the amount paid")
	}
	src := payout.Source{Method: req.Method}
	if src.IsCheque() {
		src.Cheques, _, err = cheques.ParseLines(req.Cheques, "out")
	} else {
		src.Loc, err = parseSource(req.Source)
	}
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	d, err := h.p.store.GetJob(ctx, id)
	if err != nil {
		return err
	}
	if d.Job.Status == "cancelled" {
		return apperr.Validation("this repair was cancelled")
	}
	labour, err := h.p.store.LabourID(ctx)
	if err != nil {
		return err
	}
	who := d.Job.RepairedBy
	if who == "" {
		who = "the repairer"
	}
	note := "Repair " + d.Job.TicketNo + " — paid " + who
	if n := strings.TrimSpace(req.Note); n != "" {
		note += " (" + n + ")"
	}
	uid := middleware.CurrentUserID(c)
	err = appdb.WithTx(ctx, h.p.core.DB, func(tx *sqlx.Tx) error {
		eid, _, err := payout.NewService(h.p.core.DB, h.p.core.Cashflow).ExpenseTx(ctx, tx, expenses.CreateInput{Category: "Repairs",
			Amount: dec2(amt), Description: &note, ExpenseDate: time.Now().Format("2006-01-02"), ServiceProductID: labour}, src, uid)
		if err != nil {
			return err
		}
		return linkPayout(ctx, tx, id, eid)
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionPayment, id, note)
	return h.respond(c, id)
}

// parseSource is a money place from the core picker: a locker or an open till.
func parseSource(v string) (cashflow.Location, error) {
	kind, raw, _ := strings.Cut(strings.TrimSpace(v), ":")
	id, err := strconv.ParseInt(raw, 10, 64)
	switch {
	case err != nil || id <= 0:
	case kind == "locker":
		return cashflow.Locker(id), nil
	case kind == "till":
		return cashflow.Till(id), nil
	}
	return cashflow.Location{}, apperr.Validation("pick where the money comes from")
}
