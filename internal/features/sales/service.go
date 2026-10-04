package sales

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/features/warranty"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Service struct {
	db              *sqlx.DB
	repo            *Repository
	drawerKick      func(ctx context.Context) // optional; fires post-commit on a cash sale
	nonCashLeg      NonCashLeg                // optional; books card/online money into an account
	pluginTenderLeg PluginTenderLeg           // optional; books a plugin tender (e.g. wallet) in-tx
}

// WithDrawerKick injects a best-effort action fired AFTER a cash-tendered sale
// commits, to pop the physical cash drawer. nil = no drawer. Returns the service
// for chaining.
func (s *Service) WithDrawerKick(fn func(ctx context.Context)) *Service {
	s.drawerKick = fn
	return s
}

// NonCashPayment is one card/online tender line whose money must be booked into
// a chosen money account (a card- or bank-kind locker), inside the sale's tx.
type NonCashPayment struct {
	AccountID int64           // the picked locker (card for card, bank for online)
	Method    string          // "card" | "online"
	Amount    decimal.Decimal // positive
	SaleID    int64
	Party     string // customer name, or "" for a walk-in
	ActorID   int64
}

// NonCashLeg books the money side (+ CR- receipt) of a non-cash sale tender into
// the chosen account, on the sale's own tx. Injected by the web layer via
// WithNonCashLeg (cashflow.SaleNonCashLeg) so sales never imports cashflow — the
// same DI shape as cashregister.WithLockerLeg. nil = money routing off (the line
// is recorded as a payment only, today's behaviour).
type NonCashLeg func(ctx context.Context, tx *sqlx.Tx, ev NonCashPayment) error

// WithNonCashLeg injects the non-cash money hook. Returns the service for chaining.
func (s *Service) WithNonCashLeg(fn NonCashLeg) *Service {
	s.nonCashLeg = fn
	return s
}

// PluginTender is a tender line whose method is not one core handles (cash, card,
// online, credit) — i.e. a method a plugin registered (e.g. recharge "wallet").
// Meta carries that method's opaque per-line data (core never interprets it —
// wallet puts its device_id there).
type PluginTender struct {
	Method  string
	Amount  decimal.Decimal
	SaleID  int64
	Meta    map[string]string
	ActorID int64
}

// PluginTenderLeg books the money side of a plugin tender on the sale's own tx,
// so the plugin's ledger (e.g. a wallet device float) commits atomically with the
// sale instead of a best-effort call after it. Injected from the composition root
// (main.go), which dispatches to whatever plugin registered the method — so core
// stays free of any specific plugin. nil = plugin tenders recorded only.
type PluginTenderLeg func(ctx context.Context, tx *sqlx.Tx, ev PluginTender) error

// WithPluginTenderLeg injects the plugin-tender hook. Returns the service.
func (s *Service) WithPluginTenderLeg(fn PluginTenderLeg) *Service {
	s.pluginTenderLeg = fn
	return s
}

// coreTender reports whether a payment method is one core settles itself (so it
// is NOT dispatched to the plugin tender hook).
func coreTender(method string) bool {
	switch method {
	case "cash", "card", "online", MethodOnAccount:
		return true
	}
	return false
}

// tenderPaidCash reports whether a sale's payment split includes a positive cash
// line — the condition for popping the physical drawer. Card/online/wallet/credit
// move no drawer cash.
func tenderPaidCash(ps []PaymentInput) bool {
	for _, p := range ps {
		if p.Method == "cash" {
			if v, err := money.Parse(p.Amount); err == nil && v.IsPositive() {
				return true
			}
		}
	}
	return false
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

var hundred = decimal.NewFromInt(100)

// normDiscountType defaults a blank discount type to "fixed".
func normDiscountType(t string) string {
	if t == "percent" {
		return "percent"
	}
	return "fixed"
}

// resolveBillDiscount turns the bill-level discount (a flat fixed amount, or a
// percentage of base) into a concrete amount, clamped to [0, base].
func resolveBillDiscount(dtype string, value, base decimal.Decimal) decimal.Decimal {
	amt := value
	if dtype == "percent" {
		amt = base.Mul(value).Div(hundred).Round(2)
	}
	return clampDiscount(amt, base)
}

// resolveItemDiscount turns a per-item discount into a line amount. A fixed
// value is PER UNIT and multiplies by quantity (Rs 5 off × 3 = Rs 15); a percent
// is taken off the line gross (equivalent to % of the unit price). Clamped to
// [0, lineGross].
func resolveItemDiscount(dtype string, value, lineGross, qty decimal.Decimal) decimal.Decimal {
	amt := value.Mul(qty).Round(2)
	if dtype == "percent" {
		amt = lineGross.Mul(value).Div(hundred).Round(2)
	}
	return clampDiscount(amt, lineGross)
}

func clampDiscount(amt, base decimal.Decimal) decimal.Decimal {
	if amt.IsNegative() {
		return decimal.Zero
	}
	if amt.GreaterThan(base) {
		return base
	}
	return amt
}

// Create writes a sale atomically. The whole computation — pricing, stock
// guard, audit, customer credit — happens inside one transaction so a failure
// at any step rolls everything back.
func (s *Service) Create(ctx context.Context, in CreateInput, cashierID int64) (*Detail, error) {
	// Bill-level discount: the cashier enters a value that is either a fixed
	// amount or a percentage (resolved against the pre-tax net, after item
	// discounts, once the lines are totalled below).
	billDiscValue, err := money.Parse(in.Discount)
	if err != nil || billDiscValue.IsNegative() {
		return nil, apperr.Validation("discount must be a non-negative amount")
	}
	billDiscType := normDiscountType(in.DiscountType)

	var detail *Detail
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		saleRepo := NewRepository(tx)
		stkRepo := stock.NewRepository(tx)
		custRepo := customers.NewRepository(tx)
		warrRepo := warranty.NewRepository(tx)

		// Price every line, take stock, and collect the deferred records; the
		// phases below need the sale id, so they run once it exists.
		var sl saleLines
		if err := s.buildSaleLines(ctx, tx, in, cashierID, &sl); err != nil {
			return err
		}
		lines := sl.lines
		subtotal := sl.subtotal
		taxTotal := sl.taxTotal
		itemDiscTotal := sl.itemDiscTotal
		serviceProducts := sl.serviceProducts
		componentMoves := sl.componentMoves
		serialBatches := sl.serialBatches
		// Resolve the bill discount against the pre-tax net (after item discounts).
		billDiscount := resolveBillDiscount(billDiscType, billDiscValue, subtotal.Sub(itemDiscTotal))
		discount := itemDiscTotal.Add(billDiscount)
		total := subtotal.Sub(discount).Add(taxTotal).Round(2)
		if total.IsNegative() {
			return apperr.Validation("bill discount exceeds the sale total")
		}

		methods := make([]string, 0, len(in.Payments))
		amounts := make([]decimal.Decimal, 0, len(in.Payments))
		for _, p := range in.Payments {
			amt, err := money.Parse(p.Amount)
			if err != nil || amt.IsNegative() {
				return apperr.Validation("payment amount is invalid")
			}
			methods = append(methods, p.Method)
			amounts = append(amounts, amt)
		}
		tender := SplitTender(methods, amounts)

		// The customer is only loaded when something is going on their account,
		// so an ordinary cash sale still costs no extra query.
		available := decimal.Zero
		var cust *customers.Customer
		if tender.OnAccount.IsPositive() && in.CustomerID != nil {
			c, err := custRepo.FindByID(ctx, *in.CustomerID)
			if err != nil {
				return apperr.Validation("selected customer not found")
			}
			cust = c
			available = c.AvailableCredit()
		}
		if err := CheckTender(tender, total, in.CustomerID != nil, available, in.AllowOverLimit); err != nil {
			return err
		}

		// Status follows the on-account line alone. It used to follow
		// underpayment, which converted a sale to credit silently — the debt was
		// then recorded as an ordinary retail sale and never showed in the
		// receipts list.
		status := "completed"
		change := tender.Paid.Add(tender.OnAccount).Sub(total)
		roundingToAccount := decimal.Zero
		var balanceAfter *decimal.Decimal
		if tender.OnAccount.IsPositive() {
			if err := custRepo.AddBalance(ctx, cust.ID, tender.OnAccount); err != nil {
				return apperr.Internal("failed to update customer balance", err)
			}
			after := cust.OutstandingBalance.Add(tender.OnAccount)
			balanceAfter = &after
			status = "credit"
			change = decimal.Zero
		} else if change.IsPositive() && in.ChangeGiven != "" {
			// Cashier recorded the actual change handed back. It may be less than
			// the computed change when the customer leaves the difference; the kept
			// surplus stays in the drawer (change_given now holds the real cash out,
			// so close reconciles) and is either booked to an account customer's
			// balance as an advance or simply kept as a rounding gain.
			given, err := money.Parse(in.ChangeGiven)
			if err != nil || given.IsNegative() {
				return apperr.Validation("change given is invalid")
			}
			if given.GreaterThan(change) {
				given = change // never hand back more than is due
			}
			kept := change.Sub(given)
			change = given
			if in.OnAccountRounding && kept.IsPositive() && in.CustomerID != nil {
				if _, err := custRepo.FindByID(ctx, *in.CustomerID); err != nil {
					return apperr.Validation("selected customer not found")
				}
				// Negative delta = advance: reduces their outstanding balance (or
				// becomes store credit when they owe nothing).
				if err := custRepo.AddBalance(ctx, *in.CustomerID, kept.Neg()); err != nil {
					return apperr.Internal("failed to credit customer account", err)
				}
				roundingToAccount = kept
			}
		}

		receiptNo, err := saleRepo.NextReceiptNo(ctx)
		if err != nil {
			return apperr.Internal("failed to allocate receipt number", err)
		}

		saleID, err := saleRepo.InsertSale(ctx, saleRow{
			ReceiptNo:     receiptNo,
			CustomerID:    in.CustomerID,
			SaleType:      in.SaleType,
			Subtotal:      subtotal,
			Discount:      discount,
			DiscountType:  billDiscType,
			DiscountValue: billDiscValue,
			Tax:           taxTotal,
			Total:         total,
			// Money actually received — the on-account part is a debt, not a payment.
			PaidAmount:           tender.Paid,
			ChangeGiven:          change,
			RoundingToAccount:    roundingToAccount,
			CustomerBalanceAfter: balanceAfter,
			Status:               status,
			CashierID:            cashierID,
			Notes:                in.Notes,
		})
		if err != nil {
			return apperr.Internal("failed to save sale", err)
		}

		for i := range lines {
			lines[i].SaleID = saleID
			if err := saleRepo.InsertItem(ctx, saleID, lines[i]); err != nil {
				return apperr.Internal("failed to save sale item", err)
			}
			if serviceProducts[lines[i].ProductID] {
				// Service line: no inventory of its own, but record a movement for
				// each consumable it drew down (e.g. paper for a document job).
				refType := "sale"
				for _, comp := range componentMoves[i] {
					if err := stkRepo.InsertMovement(ctx, stock.MovementInput{
						ProductID:     comp.ProductID,
						Type:          stock.MoveSale,
						Quantity:      comp.Qty.Neg(),
						ReferenceID:   &saleID,
						ReferenceType: &refType,
						UserID:        cashierID,
					}); err != nil {
						return apperr.Internal("failed to record stock movement", err)
					}
				}
				continue
			}
			neg := lines[i].Quantity.Neg()
			refType := "sale"
			if err := stkRepo.InsertMovement(ctx, stock.MovementInput{
				ProductID:     lines[i].ProductID,
				Type:          stock.MoveSale,
				Quantity:      neg,
				ReferenceID:   &saleID,
				ReferenceType: &refType,
				UserID:        cashierID,
			}); err != nil {
				return apperr.Internal("failed to record stock movement", err)
			}
		}

		// Record captured serials as warranty units, now that the sale id exists.
		if len(serialBatches) > 0 {
			now := time.Now()
			for _, sb := range serialBatches {
				for _, sn := range sb.serials {
					if _, err := warrRepo.InsertUnit(ctx, warranty.NewUnit{
						ProductID:      sb.productID,
						SerialNo:       sn,
						SaleID:         &saleID,
						CustomerID:     in.CustomerID,
						SoldAt:         now,
						WarrantyMonths: sb.months,
						WarrantyUntil:  warranty.Until(now, sb.months),
						Source:         "sale",
					}); err != nil {
						return apperr.Internal("failed to record warranty serial", err)
					}
				}
			}
		}

		// Best-effort party label for a non-cash tender's money receipt; the sale
		// ref links it either way, so no extra query when the customer isn't loaded.
		saleParty := ""
		if cust != nil {
			saleParty = cust.Name
		}
		for _, p := range in.Payments {
			amt, err := money.Parse(p.Amount)
			if err != nil {
				return apperr.Validation("payment amount is invalid")
			}
			// Skip blank/zero tender lines (e.g. an unused method in a multi-tender
			// form); negative amounts were already rejected above.
			if !amt.IsPositive() {
				continue
			}
			if err := saleRepo.InsertPayment(ctx, saleID, p.Method, amt, p.Reference); err != nil {
				return apperr.Internal("failed to save payment", err)
			}
			// Card/online money lands in a chosen account (card- or bank-kind locker)
			// so non-cash takings are tracked and reconcilable. Cash stays in the
			// drawer (computed, not moved); credit is a debt, not money. When no
			// account was picked (none configured) the line is recorded only — the
			// pre-existing behaviour — so nothing breaks for shops not set up yet.
			if s.nonCashLeg != nil && p.AccountID > 0 && (p.Method == "card" || p.Method == "online") {
				if err := s.nonCashLeg(ctx, tx, NonCashPayment{
					AccountID: p.AccountID,
					Method:    p.Method,
					Amount:    amt,
					SaleID:    saleID,
					Party:     saleParty,
					ActorID:   cashierID,
				}); err != nil {
					return err
				}
			}
			// A plugin tender (e.g. recharge "wallet") settles its own ledger inside
			// this tx so it can't silently miss like a best-effort post-sale call.
			// Dispatched generically — core doesn't know which plugin handles it.
			if s.pluginTenderLeg != nil && !coreTender(p.Method) {
				if err := s.pluginTenderLeg(ctx, tx, PluginTender{
					Method:  p.Method,
					Amount:  amt,
					SaleID:  saleID,
					Meta:    p.Meta,
					ActorID: cashierID,
				}); err != nil {
					return err
				}
			}
		}

		d, err := s.loadDetail(ctx, saleRepo, saleID)
		if err != nil {
			return err
		}
		detail = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Post-commit: pop the physical drawer when the sale actually took cash.
	// Skipped when the till will fold the pulse into the receipt print (a single
	// USB job — see DeferDrawerKick); the drawer still pops, just in that one job.
	if s.drawerKick != nil && tenderPaidCash(in.Payments) && !in.DeferDrawerKick {
		s.drawerKick(ctx)
	}
	return detail, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Detail, error) {
	return s.loadDetail(ctx, s.repo, id)
}

func (s *Service) loadDetail(ctx context.Context, repo *Repository, id int64) (*Detail, error) {
	sale, err := repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("sale")
		}
		return nil, apperr.Internal("failed to load sale", err)
	}
	items, err := repo.Items(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to load sale items", err)
	}
	payments, err := repo.Payments(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to load payments", err)
	}
	factor, err := repo.PaidFactor(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to load sale", err)
	}
	credited, err := repo.CreditReduced(ctx, id)
	if err != nil {
		return nil, apperr.Internal("failed to load sale", err)
	}
	return &Detail{Sale: *sale, Items: items, Payments: payments, PaidFactor: factor, CreditReduced: credited}, nil
}

func (s *Service) List(ctx context.Context, f ListFilter) ([]Sale, error) {
	rows, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, apperr.Internal("failed to list sales", err)
	}
	return rows, nil
}
