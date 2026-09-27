package purchases

import (
	"context"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/features/suppliers"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// CreateTx records a delivery that had no prior order: a purchase already in
// received state, with stock, batches, movements and pricing applied, and the
// full total added to the supplier's balance.
//
// Paying is deliberately not part of this. It happens in the web layer, which
// composes CreateTx with supplierpay.PayTx and cashflow.MoveTx in one
// transaction — this package cannot import either (supplierpay imports it), and
// the field that used to live here marked invoices paid without moving a cent.
func CreateTx(ctx context.Context, tx *sqlx.Tx, in CreateInput, userID int64) (*Detail, error) {
	billValue, err := money.Parse(in.Discount)
	if err != nil || billValue.IsNegative() {
		return nil, apperr.Validation("discount must be a non-negative amount")
	}
	billType := normDiscountType(in.DiscountType)
	var dueDate *time.Time
	if in.DueDate != "" {
		d, err := time.Parse("2006-01-02", in.DueDate)
		if err != nil {
			return nil, apperr.Validation("due date must be YYYY-MM-DD")
		}
		dueDate = &d
	}

	repo := NewRepository(tx)
	stk := stock.NewRepository(tx)
	sup := suppliers.NewRepository(tx)

	lines, subtotal, err := parseLines(in.Items)
	if err != nil {
		return nil, err
	}

	discount := resolveBillDiscount(billType, billValue, subtotal)
	total := subtotal.Sub(discount)

	purchaseID, err := repo.InsertPurchase(ctx, purchaseRow{
		SupplierID: in.SupplierID, InvoiceNo: in.InvoiceNo, Status: receivedStatus(decimal.Zero, total),
		Subtotal: subtotal, Discount: discount, DiscountType: billType, DiscountValue: billValue,
		Total: total, Paid: decimal.Zero,
		DueDate: dueDate, ReceivedBy: userID, Notes: in.Notes,
	})
	if err != nil {
		return nil, mapErr(err)
	}

	if err := applyReceivedLines(ctx, repo, stk, sup, purchaseID, in.SupplierID, lines, total, userID); err != nil {
		return nil, err
	}
	if err := applySettlement(ctx, tx, in.SupplierID, purchaseID); err != nil {
		return nil, err
	}

	return loadDetailTx(ctx, repo, purchaseID)
}

// Create is CreateTx in its own transaction.
func (s *Service) Create(ctx context.Context, in CreateInput, userID int64) (*Detail, error) {
	var detail *Detail
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		d, err := CreateTx(ctx, tx, in, userID)
		if err != nil {
			return err
		}
		detail = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// receivedStatus picks paid/partial/received from the amount paid vs the total.
func receivedStatus(paid, total decimal.Decimal) string {
	switch {
	case paid.GreaterThanOrEqual(total):
		return "paid"
	case paid.IsPositive():
		return "partial"
	default:
		return "received"
	}
}

// parseLines validates item inputs into purchase lines and returns their subtotal.
func parseLines(items []ItemInput) ([]PurchaseItem, decimal.Decimal, error) {
	subtotal := decimal.Zero
	lines := make([]PurchaseItem, 0, len(items))
	for _, it := range items {
		qty, err := money.Parse(it.Quantity)
		if err != nil || !qty.IsPositive() {
			return nil, decimal.Zero, apperr.Validation("quantity must be greater than zero")
		}
		free := decimal.Zero
		if strings.TrimSpace(it.FreeQty) != "" {
			free, err = money.Parse(it.FreeQty)
			if err != nil || free.IsNegative() {
				return nil, decimal.Zero, apperr.Validation("free quantity is invalid")
			}
		}
		cost, err := money.Parse(it.CostPrice)
		if err != nil || cost.IsNegative() {
			return nil, decimal.Zero, apperr.Validation("cost price is invalid")
		}
		selling, err := money.Parse(it.SellingPrice)
		if err != nil || selling.IsNegative() {
			return nil, decimal.Zero, apperr.Validation("selling price is invalid")
		}
		var expiry *time.Time
		if it.ExpiryDate != "" {
			e, err := time.Parse("2006-01-02", it.ExpiryDate)
			if err != nil {
				return nil, decimal.Zero, apperr.Validation("expiry date must be YYYY-MM-DD")
			}
			expiry = &e
		}
		discVal := decimal.Zero
		if strings.TrimSpace(it.Discount) != "" {
			discVal, err = money.Parse(it.Discount)
			if err != nil || discVal.IsNegative() {
				return nil, decimal.Zero, apperr.Validation("item discount is invalid")
			}
		}
		discType := normDiscountType(it.DiscountType)
		// Free units are deliberately absent from the payable: the gross is paid
		// qty × cost only, so nothing about a freebie reaches the supplier balance.
		// A per-item discount then comes off that gross, and the line net is what
		// is owed for the line.
		lineGross := qty.Mul(cost).Round(2)
		disc := resolveItemDiscount(discType, discVal, lineGross, qty)
		lineNet := lineGross.Sub(disc)
		subtotal = subtotal.Add(lineNet)
		lines = append(lines, PurchaseItem{
			ProductID: it.ProductID, Quantity: qty, FreeQty: free, CostPrice: cost,
			SellingPrice: selling, ExpiryDate: expiry,
			Discount: disc, DiscountType: discType, DiscountValue: discVal,
			Subtotal: lineNet,
		})
	}
	return lines, subtotal, nil
}

// insertDraftLines saves planned lines on a draft (ordered_qty = quantity, no
// inventory effect).
func insertDraftLines(ctx context.Context, repo *Repository, purchaseID int64, lines []PurchaseItem) error {
	for _, ln := range lines {
		oq := ln.Quantity
		ln.OrderedQty = &oq
		if err := repo.InsertItem(ctx, purchaseID, ln); err != nil {
			return apperr.Internal("failed to save draft line", err)
		}
	}
	return nil
}

// createDraftTx inserts one draft purchase (status='draft', no stock/payable
// effects) within the given transaction and returns its id.
func createDraftTx(ctx context.Context, tx *sqlx.Tx, in CreateInput, userID int64) (int64, error) {
	repo := NewRepository(tx)
	billValue, err := money.Parse(in.Discount)
	if err != nil || billValue.IsNegative() {
		billValue = decimal.Zero
	}
	billType := normDiscountType(in.DiscountType)
	dueDate, err := parseDate(in.DueDate)
	if err != nil {
		return 0, err
	}
	expected, err := parseDate(in.ExpectedDate)
	if err != nil {
		return 0, err
	}
	lines, subtotal, err := parseLines(in.Items)
	if err != nil {
		return 0, err
	}
	discount := resolveBillDiscount(billType, billValue, subtotal)
	total := subtotal.Sub(discount)
	id, err := repo.InsertPurchase(ctx, purchaseRow{
		SupplierID: in.SupplierID, InvoiceNo: in.InvoiceNo, Status: "draft",
		Subtotal: subtotal, Discount: discount, DiscountType: billType, DiscountValue: billValue,
		Total: total, Paid: decimal.Zero,
		DueDate: dueDate, ExpectedDate: expected, ReceivedBy: userID, Notes: in.Notes,
	})
	if err != nil {
		return 0, mapErr(err)
	}
	if err := insertDraftLines(ctx, repo, id, lines); err != nil {
		return 0, err
	}
	// Ordering from a supplier is itself a signal of who supplies an item, so
	// backfill the default supplier here too (only when none is set yet).
	for _, ln := range lines {
		if err := repo.SetPreferredSupplierIfEmpty(ctx, ln.ProductID, in.SupplierID); err != nil {
			return 0, apperr.Internal("failed to set preferred supplier", err)
		}
	}
	return id, nil
}

// CreateDraft saves a single Purchase Order as a draft (no inventory effect).
func (s *Service) CreateDraft(ctx context.Context, in CreateInput, userID int64) (*Detail, error) {
	var detail *Detail
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		id, err := createDraftTx(ctx, tx, in, userID)
		if err != nil {
			return err
		}
		d, err := s.loadDetail(ctx, NewRepository(tx), id)
		if err != nil {
			return err
		}
		detail = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// ReorderLineInput is one product+supplier+qty picked on the low-stock report.
type ReorderLineInput struct {
	ProductID  int64  `json:"product_id"`
	SupplierID int64  `json:"supplier_id"`
	Quantity   string `json:"quantity"`
	CostPrice  string `json:"cost_price"`
}

// ReorderPOInput is the payload the low-stock PO builder posts.
type ReorderPOInput struct {
	Lines []ReorderLineInput `json:"lines"`
}

// CreateDraftsFromReorder groups the picked lines by supplier and creates one
// draft Purchase Order per supplier, all in a single transaction. Returns the
// new draft IDs in supplier-first-seen order (for the print step).
func (s *Service) CreateDraftsFromReorder(ctx context.Context, in ReorderPOInput, userID int64) ([]int64, error) {
	if len(in.Lines) == 0 {
		return nil, apperr.Validation("select at least one product to order")
	}
	order := make([]int64, 0)
	bySup := make(map[int64][]ItemInput)
	for _, l := range in.Lines {
		if l.SupplierID <= 0 {
			return nil, apperr.Validation("each ordered line needs a supplier")
		}
		if l.ProductID <= 0 {
			return nil, apperr.Validation("invalid product on an ordered line")
		}
		if _, ok := bySup[l.SupplierID]; !ok {
			order = append(order, l.SupplierID)
		}
		cost := l.CostPrice
		if cost == "" {
			cost = "0"
		}
		bySup[l.SupplierID] = append(bySup[l.SupplierID], ItemInput{
			ProductID: l.ProductID, Quantity: l.Quantity, CostPrice: cost,
		})
	}
	ids := make([]int64, 0, len(order))
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		for _, supID := range order {
			id, err := createDraftTx(ctx, tx, CreateInput{
				SupplierID: supID, Discount: "0", Items: bySup[supID],
			}, userID)
			if err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// UpdateDraft replaces a draft's lines and header. Only drafts are editable.
func (s *Service) UpdateDraft(ctx context.Context, id int64, in CreateInput, userID int64) (*Detail, error) {
	billValue, err := money.Parse(in.Discount)
	if err != nil || billValue.IsNegative() {
		billValue = decimal.Zero
	}
	billType := normDiscountType(in.DiscountType)
	dueDate, err := parseDate(in.DueDate)
	if err != nil {
		return nil, err
	}
	expected, err := parseDate(in.ExpectedDate)
	if err != nil {
		return nil, err
	}
	var detail *Detail
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		repo := NewRepository(tx)
		cur, err := repo.FindByID(ctx, id)
		if err != nil {
			return notFoundOr(err)
		}
		if cur.Status != "draft" {
			return apperr.Validation("only draft purchase orders can be edited")
		}
		lines, subtotal, err := parseLines(in.Items)
		if err != nil {
			return err
		}
		discount := resolveBillDiscount(billType, billValue, subtotal)
		total := subtotal.Sub(discount)
		if err := repo.DeleteItems(ctx, id); err != nil {
			return apperr.Internal("failed to clear draft lines", err)
		}
		if err := insertDraftLines(ctx, repo, id, lines); err != nil {
			return err
		}
		if err := repo.UpdateHeader(ctx, id, purchaseRow{
			InvoiceNo: in.InvoiceNo, Status: "draft", Subtotal: subtotal,
			Discount: discount, DiscountType: billType, DiscountValue: billValue,
			Total: total, Paid: decimal.Zero,
			DueDate: dueDate, ExpectedDate: expected, ReceivedBy: userID, Notes: in.Notes,
		}); err != nil {
			return apperr.Internal("failed to update draft", err)
		}
		d, err := s.loadDetail(ctx, repo, id)
		if err != nil {
			return err
		}
		detail = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}
