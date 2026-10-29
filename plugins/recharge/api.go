package recharge

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/payout"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/supplierpay"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

type handlers struct{ p *Plugin }

func (p *Plugin) routes(reg *plugin.Registry) {
	h := handlers{p}
	t := reg.Cashier() // /api/v1/x — anyone signed in: the till's 📲 menu, drawer counts, Returns
	t.GET("/recharge/till", h.till)
	t.POST("/recharge/packages/quick", h.quickPackage)
	t.POST("/recharge/counts/drawer", h.drawerCounts)
	t.GET("/recharge/sale-lines", h.saleLines)
	t.GET("/recharge/refill/sources", h.counterSources, middleware.RequireSupplierAccess())
	t.POST("/recharge/refill/counter", h.counterRefill, middleware.RequireSupplierAccess())
	a := reg.Admin() // admin + manager
	a.GET("/recharge/setup", h.setup)
	a.POST("/recharge/carriers", h.createCarrier)
	a.PUT("/recharge/carriers/:id", h.renameCarrier)
	a.POST("/recharge/carriers/:id/active", h.carrierActive)
	a.PUT("/recharge/carriers/:id/supplier", h.carrierSupplier)
	a.PUT("/recharge/carriers/:id/rates", h.setRates)
	a.POST("/recharge/devices", h.createDevice)
	a.PUT("/recharge/devices/:id", h.updateDevice)
	a.POST("/recharge/devices/:id/active", h.deviceActive)
	a.POST("/recharge/packages", h.createPackage)
	a.PUT("/recharge/packages/:id", h.updatePackage)
	a.POST("/recharge/packages/:id/review", h.reviewPackage)
	a.GET("/recharge/float", h.float)
	a.POST("/recharge/counts", h.adminCounts)
	a.POST("/recharge/refill", h.adminRefill)
	a.GET("/recharge/ledger", h.ledger)
	a.GET("/recharge/report", h.report)
}

func idParam(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid " + name)
	}
	return id, nil
}

func bind[T any](c echo.Context) (T, error) {
	var v T
	if err := c.Bind(&v); err != nil {
		return v, apperr.BadRequest("invalid request body")
	}
	return v, nil
}

func (h handlers) audit(c echo.Context, action, entity string, id int64, detail string) {
	h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, entity, strconv.FormatInt(id, 10), detail)
}

// Bank is a bank locker a cashier may pay a bill from / take a cash out into.
type Bank struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name"`
	Balance decimal.Decimal `json:"balance"`
}

// TillData is everything the till's 📲 menu needs in one call.
type TillData struct {
	Carriers  []TillCarrier    `json:"carriers"`
	Floats    []Float          `json:"floats"`
	Packages  []Package        `json:"packages"`
	Banks     []Bank           `json:"banks"`
	BillTypes []string         `json:"billTypes"`
	Bills     []RecentBill     `json:"bills"`    // bill + account number pairs used before (suggestions)
	Products  map[string]int64 `json:"products"` // billpay | deposit | cashout | charge
}

// RecentBill is a bill type and account number paid before, newest first.
type RecentBill struct {
	BillType string `db:"bill_type" json:"billType"`
	Ref      string `db:"ref"       json:"ref"`
}

// TillCarrier is a carrier with its rates (the till shows the charge it offers).
type TillCarrier struct {
	Carrier
	Rates []Rate `json:"rates"`
}

func (h handlers) till(c echo.Context) error {
	ctx := c.Request().Context()
	out := TillData{Carriers: []TillCarrier{}, Banks: []Bank{}, BillTypes: []string{}, Bills: []RecentBill{}}
	cs, err := h.p.store.Carriers(ctx, false)
	if err != nil {
		return err
	}
	for _, cr := range cs {
		rs, err := h.p.store.Rates(ctx, h.p.store.db, cr.ID)
		if err != nil {
			return err
		}
		out.Carriers = append(out.Carriers, TillCarrier{Carrier: cr, Rates: rs})
	}
	if out.Floats, err = h.p.store.Floats(ctx, h.p.store.db, false); err != nil {
		return err
	}
	if out.Packages, err = h.p.store.Packages(ctx, false); err != nil {
		return err
	}
	ls, err := h.p.core.Lockers.ListForCashier(ctx)
	if err != nil {
		return err
	}
	for _, l := range ls {
		if l.Kind == "bank" {
			out.Banks = append(out.Banks, Bank{ID: l.ID, Name: l.Name, Balance: l.Balance})
		}
	}
	if err := h.p.store.db.SelectContext(ctx, &out.BillTypes, `SELECT bill_type FROM (
		SELECT bill_type, max(created_at) AS at FROM (
			SELECT bill_type, created_at FROM recharge_bill_tx WHERE COALESCE(bill_type, '') <> ''
			UNION ALL SELECT bill_type, created_at FROM recharge_transactions WHERE COALESCE(bill_type, '') <> '') x
		GROUP BY bill_type) y ORDER BY at DESC LIMIT 30`); err != nil {
		return err
	}
	if err := h.p.store.db.SelectContext(ctx, &out.Bills, `SELECT bill_type, ref FROM (
		SELECT COALESCE(bill_type, '') AS bill_type, reference AS ref, max(created_at) AS at FROM (
			SELECT bill_type, reference, created_at FROM recharge_bill_tx WHERE COALESCE(reference, '') <> '' AND type = 'billpay'
			UNION ALL SELECT bill_type, reference, created_at FROM recharge_transactions WHERE COALESCE(reference, '') <> '' AND type = 'billpay') x
		GROUP BY 1, 2) y ORDER BY at DESC LIMIT 200`); err != nil {
		return err
	}
	if out.Products, err = h.p.store.ProductIDs(ctx); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

// quickPackage: a new package turns up mid-rush — any cashier adds it with the
// commission the carrier's app shows, and it goes straight on the cart.
func (h handlers) quickPackage(c echo.Context) error {
	in, err := bind[PackageInput](c)
	if err != nil {
		return err
	}
	if in.Commission == nil {
		return apperr.Validation("enter the commission the carrier's app shows")
	}
	in.FloatCost = nil
	p, created, err := h.p.store.CreatePackage(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	if created {
		h.audit(c, audit.ActionCreate, "recharge_package", p.ID, "added at the till: "+p.Name+" "+p.Price.StringFixed(2))
	}
	return c.JSON(http.StatusOK, map[string]any{"package": p, "created": created})
}

// drawerCounts: the float counts typed in the till's Open / Close dialog.
func (h handlers) drawerCounts(c echo.Context) error {
	in, err := bind[struct {
		Counts []CountInput `json:"counts"`
	}](c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	uid := middleware.CurrentUserID(c)
	var sess *int64
	if s, err := h.p.core.CashRegister.Current(ctx, uid); err == nil && s != nil {
		sess = &s.ID
	}
	res, err := h.p.store.Count(ctx, in.Counts, sess, uid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res)
}

// SaleLine is a recharge line on a sale, for the Returns field ("how much came back?").
type SaleLine struct {
	SaleItemID int64           `db:"sale_item_id" json:"saleItemId"`
	Kind       string          `db:"kind"         json:"kind"`  // reload | billpay | deposit | bank
	Label      string          `db:"label"        json:"label"` // where the float / money was
	Left       decimal.Decimal `db:"left_amount"  json:"left"`  // float (or bank money) that left for the whole line
}

func (h handlers) saleLines(c echo.Context) error {
	sale, err := strconv.ParseInt(c.QueryParam("sale"), 10, 64)
	if err != nil || sale <= 0 {
		return apperr.BadRequest("invalid sale")
	}
	out := []SaleLine{}
	err = h.p.store.db.SelectContext(c.Request().Context(), &out, `
		SELECT t.sale_item_id, t.type AS kind, c.name || COALESCE(' · ' || d.label, '') AS label, -t.float_delta AS left_amount
		FROM recharge_transactions t JOIN recharge_carriers c ON c.id = t.carrier_id LEFT JOIN recharge_devices d ON d.id = t.device_id
		WHERE t.sale_id = $1 AND t.parent_id IS NULL AND t.type IN ('reload','billpay','deposit')
		UNION ALL
		SELECT sale_item_id, 'bank', bank_name, amount FROM recharge_bill_tx WHERE sale_id = $1 AND type = 'billpay'`, sale)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

// Source is a place cash for buying float can come from.
type Source struct {
	Value   string          `json:"value"` // till:<user> | locker:<id>
	Label   string          `json:"label"`
	Balance decimal.Decimal `json:"balance"`
}

// counterSources: the cashier's own drawer and the cash lockers opened to cashiers.
func (h handlers) counterSources(c echo.Context) error {
	ctx := c.Request().Context()
	uid := middleware.CurrentUserID(c)
	out := []Source{{Value: "till:" + strconv.FormatInt(uid, 10), Label: "My drawer"}}
	ls, err := h.p.core.Lockers.ListForCashier(ctx)
	if err != nil {
		return err
	}
	for _, l := range ls {
		if l.Kind != "bank" && l.Kind != "card" {
			out = append(out, Source{Value: "locker:" + strconv.FormatInt(l.ID, 10), Label: l.Name, Balance: l.Balance})
		}
	}
	return c.JSON(http.StatusOK, out)
}

// RefillInput buys float from a dealer (a core supplier): a supplier bill for
// Amount, on Credit or paid now from Source (cash place or bank) or — Method
// "cheque" — by the Cheques written on the shop's bank places; the device's
// float up by Amount, plus any bonus the dealer gave on it. Reference is the
// dealer's invoice number.
type RefillInput struct {
	DeviceID   int64               `json:"deviceId"`
	SupplierID int64               `json:"supplierId"`
	Amount     decimal.Decimal     `json:"amount"`
	Bonus      decimal.Decimal     `json:"bonus"`
	Credit     bool                `json:"credit"`
	Source     string              `json:"source"`
	Method     string              `json:"method"`
	Cheques    []cheques.LineInput `json:"cheques"`
	Reference  string              `json:"reference"`
}

// chequesFit: cheques for a refill pay only what the dealer's advance left
// owed on its bill — none when the advance covers it all.
func chequesFit(lines []cheques.Line, owe decimal.Decimal) error {
	if lines == nil {
		return nil
	}
	if !owe.IsPositive() {
		return apperr.Validation("the money they hold of yours covers this bill — choose Paid now, no cheque needed")
	}
	sum := decimal.Zero
	for _, l := range lines {
		sum = sum.Add(l.Amount)
	}
	if !sum.Equal(owe) {
		return apperr.Validation("they hold some of your money, so only " + money.Display(owe) + " is still owed — write the cheque for that")
	}
	return nil
}

// chequeLines is the cheques a refill paid by cheque was given with (none for
// a refill on credit or paid from a place).
func chequeLines(in RefillInput) ([]cheques.Line, error) {
	if in.Credit || in.Method != payout.MethodCheque {
		return nil, nil
	}
	ls, _, err := cheques.ParseLines(in.Cheques, "out")
	return ls, err
}

func parseSource(v string) (cashflow.Location, error) {
	kind, raw, _ := strings.Cut(strings.TrimSpace(v), ":")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return cashflow.Location{}, apperr.Validation("pick where the money comes from")
	}
	switch kind {
	case "locker":
		return cashflow.Locker(id), nil
	case "till":
		return cashflow.Till(id), nil
	}
	return cashflow.Location{}, apperr.Validation("pick where the money comes from")
}

// counterRefill: only the cashier's own drawer or a cashier locker (a forged
// source is refused before any money moves).
func (h handlers) counterRefill(c echo.Context) error {
	in, err := bind[RefillInput](c)
	if err != nil {
		return err
	}
	if in.Credit {
		return h.refill(c, in, cashflow.External(), nil)
	}
	ctx := c.Request().Context()
	lines, err := chequeLines(in)
	if err != nil {
		return err
	}
	if lines != nil {
		// Only cheques on bank places the cashier may use.
		ls, err := h.p.core.Lockers.ListForCashier(ctx)
		if err != nil {
			return err
		}
		for _, cl := range lines {
			ok := false
			for _, l := range ls {
				ok = ok || (l.ID == cl.LockerID && l.Kind == "bank")
			}
			if !ok {
				return apperr.Forbidden("you can't write a cheque on that account")
			}
		}
		return h.refill(c, in, cashflow.External(), lines)
	}
	uid := middleware.CurrentUserID(c)
	src, err := parseSource(in.Source)
	if err != nil {
		return err
	}
	ok := src.Kind == cashflow.KindTill && src.ID == uid
	if src.Kind == cashflow.KindLocker {
		ls, err := h.p.core.Lockers.ListForCashier(ctx)
		if err != nil {
			return err
		}
		for _, l := range ls {
			ok = ok || (l.ID == src.ID && l.Kind != "bank" && l.Kind != "card")
		}
	}
	if !ok {
		return apperr.Forbidden("you can't take cash from there")
	}
	return h.refill(c, in, src, nil)
}

func (h handlers) adminRefill(c echo.Context) error {
	in, err := bind[RefillInput](c)
	if err != nil {
		return err
	}
	if in.Credit {
		return h.refill(c, in, cashflow.External(), nil)
	}
	lines, err := chequeLines(in)
	if err != nil {
		return err
	}
	if lines != nil {
		return h.refill(c, in, cashflow.External(), lines)
	}
	src, err := parseSource(in.Source)
	if err != nil {
		return err
	}
	return h.refill(c, in, src, nil)
}

// refill books the float as a core supplier bill and puts it on the device in
// one tx. Paid now = core's supplier payment on that bill and the money leaving
// src (as the counter's Pay) or the cheques written for it (lines; the bank pays
// when they clear); on credit = the bill alone, owed like any delivery.
// Buying float is cash turned into float — no expense (karots-pos booked one,
// which the P&L counted as a loss).
func (h handlers) refill(c echo.Context, in RefillInput, src cashflow.Location, lines []cheques.Line) error {
	if !in.Amount.IsPositive() || in.Bonus.IsNegative() {
		return apperr.Validation("enter the amount of float bought")
	}
	if in.SupplierID <= 0 {
		return apperr.Validation("pick who sold the float")
	}
	ctx := c.Request().Context()
	uid := middleware.CurrentUserID(c)
	method := "cash"
	if src.Kind == cashflow.KindLocker {
		l, err := h.p.core.Lockers.Get(ctx, src.ID)
		if err != nil {
			return err
		}
		if l.Kind == "bank" {
			method = "online"
		}
	}
	if !in.Credit && lines == nil {
		if err := h.p.core.Cashflow.RequirePlace(ctx, src, method); err != nil {
			return err
		}
	}
	var rec *cashflow.Receipt
	err := appdb.WithTx(ctx, h.p.core.DB, func(tx *sqlx.Tx) error {
		dv, err := h.p.store.Device(ctx, tx, in.DeviceID)
		if err != nil {
			return err
		}
		sup, err := suppliers.NewRepository(tx).FindByID(ctx, in.SupplierID)
		if err != nil {
			return apperr.NotFound("supplier")
		}
		ref := strings.TrimSpace(in.Reference)
		bill, err := purchases.CreateBillTx(ctx, tx, purchases.BillInput{SupplierID: sup.ID, Amount: in.Amount, InvoiceNo: ref,
			Notes: "Reload float · " + dv.Carrier + " · " + dv.Label}, uid)
		if err != nil {
			return err
		}
		pid := bill.Purchase.ID
		note := "on credit · bill #" + strconv.FormatInt(pid, 10)
		// An advance the dealer holds may already have settled it; pay the rest.
		owe := bill.Purchase.Balance()
		if err := chequesFit(lines, owe); err != nil {
			return err
		}
		if !in.Credit && owe.IsPositive() {
			ps := payout.Source{Loc: src, Method: method, Cheques: lines}
			if lines != nil {
				ps.Method = payout.MethodCheque
			}
			if rec, err = payout.NewService(h.p.core.DB, h.p.core.Cashflow).SupplierTx(ctx, tx, sup.ID, sup.Name, supplierpay.PayInput{Method: method,
				Reference: ref, Note: "reload float", Allocations: []supplierpay.Alloc{{PurchaseID: pid, Amount: owe}}}, ps, uid); err != nil {
				return err
			}
			note = "paid · bill #" + strconv.FormatInt(pid, 10)
			if lines != nil {
				note += " · by cheque"
			} else if rec != nil {
				note += " · money receipt " + rec.ReceiptNo
			}
		}
		_, err = recordWithCommission(ctx, tx, TxInput{CarrierID: dv.CarrierID, DeviceID: dv.ID, Type: "refill", Amount: in.Amount,
			FloatDelta: in.Amount, PurchaseID: &pid, Reference: ref, Note: note, CreatedBy: uid}, in.Bonus.Round(2))
		return err
	})
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "recharge_refill", in.DeviceID, "float bought "+in.Amount.StringFixed(2))
	var receiptID int64
	if rec != nil {
		receiptID = rec.ID
		if src.Kind == cashflow.KindTill {
			h.kick(ctx)
		}
	}
	return c.JSON(http.StatusCreated, map[string]any{"receiptId": receiptID})
}

// kick pops the drawer (best-effort, off the request — core's drawer kicker).
func (h handlers) kick(ctx context.Context) {
	bg := context.WithoutCancel(ctx)
	go func() {
		c, cancel := context.WithTimeout(bg, 20*time.Second)
		defer cancel()
		if cfg, err := h.p.core.Settings.Get(c); err == nil && cfg != nil {
			escpos.KickDrawer(c, *cfg)
		}
	}()
}
