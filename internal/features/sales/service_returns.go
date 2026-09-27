package sales

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// returnedLotPrice is the price to stamp on the lot that returned goods re-enter
// stock in. It carries the sold price back ONLY for a line that was rung from a
// picked lot — an old bottle sold at the old price goes back on the shelf at the
// old price, and can be picked again at the till.
//
// Every other line returns zero, leaving the lot following the product. That is
// deliberate: a wholesale line's unit price is a customer-type price, and
// stamping it here would put the goods back on the shelf at the wholesale rate.
func returnedLotPrice(it SaleItem) decimal.Decimal {
	if it.BatchID == nil {
		return decimal.Zero
	}
	return it.UnitPrice
}

// ReturnReceipt loads the most recent return on a sale for a printed refund slip.
func (s *Service) ReturnReceipt(ctx context.Context, saleID int64) (*ReturnReceipt, error) {
	rr, err := s.repo.LatestReturn(ctx, saleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("return")
		}
		return nil, apperr.Internal("failed to load return", err)
	}
	return rr, nil
}

// Return reverses a completed sale: every line is restocked and audited, any
// credit balance the sale created is removed from the customer, and the sale is
// marked 'returned' (which also excludes it from revenue in reports). One tx.
func (s *Service) Return(ctx context.Context, id int64, userID int64) (*Detail, error) {
	var detail *Detail
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		saleRepo := NewRepository(tx)
		stkRepo := stock.NewRepository(tx)
		custRepo := customers.NewRepository(tx)

		sale, err := saleRepo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("sale")
			}
			return apperr.Internal("failed to load sale", err)
		}
		if sale.Status == "returned" {
			return apperr.Conflict("this sale has already been returned")
		}
		// A partial return already restocked some lines AND reduced the customer's
		// credit by that portion. A full return here would remove the full original
		// owed again, over-crediting the customer — so funnel the remainder through
		// line-level returns (PartialReturn), which is credit-correct.
		if sale.Status == "partially_returned" {
			return apperr.Conflict("this sale was already partially returned — use line returns for the rest")
		}

		items, err := saleRepo.Items(ctx, id)
		if err != nil {
			return apperr.Internal("failed to load sale items", err)
		}
		// Service lines (recharge/airtime) can't be returned. If the sale still has
		// one outstanding, block the whole-sale return rather than silently dropping
		// it — the cashier can line-return the other items.
		for _, it := range items {
			if it.IsService && it.Quantity.Sub(it.ReturnedQty).IsPositive() {
				return apperr.Conflict("this sale includes a recharge item, which can't be returned")
			}
		}
		ref := "sale"
		for _, it := range items {
			remaining := it.Quantity.Sub(it.ReturnedQty)
			if !remaining.IsPositive() {
				continue
			}
			if err := stkRepo.Increment(ctx, it.ProductID, remaining); err != nil {
				return apperr.Internal("failed to restock", err)
			}
			// Put it back where it came from when we know; only open a return lot
			// when we do not, so returns stop fragmenting the batch list.
			back, berr := stkRepo.RestockLot(ctx, it.BatchID, it.ProductID, remaining)
			if berr != nil {
				return apperr.Internal("failed to restock batch", berr)
			}
			if !back {
				if _, err := stkRepo.InsertBatch(ctx, stock.NewBatch{
					ProductID: it.ProductID, Quantity: remaining, CostPrice: it.CostPrice,
					SellingPrice: returnedLotPrice(it), Source: "return",
				}); err != nil {
					return apperr.Internal("failed to restock batch", err)
				}
			}
			if err := stkRepo.InsertMovement(ctx, stock.MovementInput{
				ProductID: it.ProductID, Type: stock.MoveReturn, Quantity: remaining,
				ReferenceID: &id, ReferenceType: &ref, UserID: userID,
				Note: strPtr("return of " + sale.ReceiptNo),
			}); err != nil {
				return apperr.Internal("failed to record return movement", err)
			}
			if err := saleRepo.MarkItemFullyReturned(ctx, it.ID); err != nil {
				return apperr.Internal("failed to mark line returned", err)
			}
		}

		// Remove any credit this sale created from the customer's balance.
		if sale.CustomerID != nil {
			if owed := sale.Total.Sub(sale.PaidAmount); owed.IsPositive() {
				if err := custRepo.AddBalance(ctx, *sale.CustomerID, owed.Neg()); err != nil {
					return apperr.Internal("failed to adjust customer balance", err)
				}
			}
		}

		if err := saleRepo.UpdateStatus(ctx, id, "returned"); err != nil {
			return apperr.Internal("failed to update sale status", err)
		}
		d, err := s.loadDetail(ctx, saleRepo, id)
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

func strPtr(s string) *string { return &s }

// ReturnLineInput is one line of a partial return: how much of a specific sale
// item to send back.
type ReturnLineInput struct {
	SaleItemID int64  `json:"sale_item_id" validate:"required,gt=0"`
	Quantity   string `json:"quantity"     validate:"required"`
	// Disposition decides what happens to the returned goods: "restock" (default)
	// puts them back into sellable stock; "damage" writes them off as a loss so
	// faulty items never re-enter inventory. The customer is refunded either way.
	Disposition string `json:"disposition"`
}

type PartialReturnInput struct {
	Reason *string           `json:"reason"`
	Lines  []ReturnLineInput `json:"lines" validate:"required,min=1,dive"`
}

// PartialReturn sends back specific quantities of specific lines. It restocks
// (into a return batch), reduces the customer's credit for the credit portion
// and treats the rest as a cash refund, records the return, and moves the sale
// to 'partially_returned' (or 'returned' when nothing is left). One tx.
// The returned decimal is the cash-refund portion (refund value minus any credit
// reduction), so the caller can post it to the cashier's drawer ledger.
func (s *Service) PartialReturn(ctx context.Context, saleID int64, in PartialReturnInput, userID int64) (*Detail, decimal.Decimal, error) {
	var detail *Detail
	cashRefund := decimal.Zero
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		d, refund, _, err := s.PartialReturnTx(ctx, tx, saleID, in, userID)
		if err != nil {
			return err
		}
		detail, cashRefund = d, refund
		return nil
	})
	if err != nil {
		return nil, decimal.Zero, err
	}
	return detail, cashRefund, nil
}

// PartialReturnTx performs a partial return over an existing transaction, so a
// caller can book the cash refund (cashflow.MoveTx) atomically in the same tx.
// Returns the refreshed sale detail, the cash-refund amount and the return id.
func (s *Service) PartialReturnTx(ctx context.Context, tx *sqlx.Tx, saleID int64, in PartialReturnInput, userID int64) (*Detail, decimal.Decimal, int64, error) {
	cashRefund := decimal.Zero
	var detail *Detail
	var returnID int64
	err := func() error {
		saleRepo := NewRepository(tx)
		stkRepo := stock.NewRepository(tx)
		custRepo := customers.NewRepository(tx)

		sale, err := saleRepo.FindByID(ctx, saleID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.NotFound("sale")
			}
			return apperr.Internal("failed to load sale", err)
		}
		if sale.Status == "returned" {
			return apperr.Conflict("this sale has already been fully returned")
		}

		ref := "sale"
		totalRefundValue := decimal.Zero
		returnID, err = saleRepo.InsertSaleReturn(ctx, saleID, userID, decimal.Zero, decimal.Zero, in.Reason)
		if err != nil {
			return apperr.Internal("failed to open return", err)
		}

		for _, ln := range in.Lines {
			qty, err := money.Parse(ln.Quantity)
			if err != nil || !qty.IsPositive() {
				return apperr.Validation("return quantity must be greater than zero")
			}
			it, err := saleRepo.FindItem(ctx, saleID, ln.SaleItemID)
			if err != nil {
				return apperr.Validation("sale line not found on this sale")
			}
			if it.IsService {
				return apperr.Conflict(fmt.Sprintf("%s is a recharge item and can't be returned", it.ProductName))
			}
			if qty.GreaterThan(it.ReturnableQty()) {
				return apperr.Conflict(fmt.Sprintf("cannot return %s of %s — only %s remain",
					money.Display(qty), it.ProductName, money.Display(it.ReturnableQty())))
			}
			ok, err := saleRepo.AddReturnedQty(ctx, it.ID, qty)
			if err != nil {
				return apperr.Internal("failed to update returned qty", err)
			}
			if !ok {
				return apperr.Conflict("return quantity exceeds what was sold")
			}
			// per-unit refund value = line net / line qty
			unitValue := it.Subtotal.Div(it.Quantity).Round(2)
			lineRefund := unitValue.Mul(qty).Round(2)
			totalRefundValue = totalRefundValue.Add(lineRefund)

			if err := stkRepo.Increment(ctx, it.ProductID, qty); err != nil {
				return apperr.Internal("failed to restock", err)
			}
			back, berr := stkRepo.RestockLot(ctx, it.BatchID, it.ProductID, qty)
			if berr != nil {
				return apperr.Internal("failed to restock batch", berr)
			}
			if !back {
				if _, err := stkRepo.InsertBatch(ctx, stock.NewBatch{
					ProductID: it.ProductID, Quantity: qty, CostPrice: it.CostPrice,
					SellingPrice: returnedLotPrice(*it), Source: "return",
				}); err != nil {
					return apperr.Internal("failed to restock batch", err)
				}
			}
			if err := stkRepo.InsertMovement(ctx, stock.MovementInput{
				ProductID: it.ProductID, Type: stock.MoveReturn, Quantity: qty,
				ReferenceID: &returnID, ReferenceType: &ref, UserID: userID,
				Note: strPtr("return of " + sale.ReceiptNo),
			}); err != nil {
				return apperr.Internal("failed to record return movement", err)
			}
			if err := saleRepo.InsertSaleReturnItem(ctx, returnID, it.ID, it.ProductID, qty, lineRefund); err != nil {
				return apperr.Internal("failed to record return line", err)
			}

			// "Send to damage": the cashier judged these goods faulty, so don't
			// leave them in sellable stock. They came back in just above (return
			// movement + batch); now write them straight back out through the same
			// path as the Damage screen — a damage movement valued at FEFO cost.
			// Net sellable stock is unchanged and BOTH the returns and the
			// damage/loss reports stay accurate. The refund is unaffected.
			if strings.EqualFold(strings.TrimSpace(ln.Disposition), "damage") {
				ok, err := stkRepo.DecrementGuarded(ctx, it.ProductID, qty)
				if err != nil {
					return apperr.Internal("failed to write off damaged return", err)
				}
				if !ok {
					return apperr.Conflict("not enough stock to write off the damaged return")
				}
				cost, err := stkRepo.DepleteFEFO(ctx, it.ProductID, qty)
				if err != nil {
					return apperr.Internal("failed to deplete batches", err)
				}
				if err := stkRepo.InsertMovement(ctx, stock.MovementInput{
					ProductID: it.ProductID, Type: stock.MoveDamage, Quantity: qty.Neg(),
					ReferenceID: &returnID, ReferenceType: &ref, UserID: userID,
					Note: strPtr("damaged on return of " + sale.ReceiptNo),
					Cost: cost.Mul(qty),
				}); err != nil {
					return apperr.Internal("failed to record damage movement", err)
				}
			}
		}

		// Split the returned value: credit portion reduces the customer balance,
		// the remainder is a cash refund.
		creditReduction := decimal.Zero
		if sale.CustomerID != nil {
			owed := sale.Total.Sub(sale.PaidAmount)
			if owed.IsPositive() {
				creditReduction = decimal.Min(totalRefundValue, owed)
				if err := custRepo.AddBalance(ctx, *sale.CustomerID, creditReduction.Neg()); err != nil {
					return apperr.Internal("failed to adjust customer balance", err)
				}
			}
		}
		refund := totalRefundValue.Sub(creditReduction)
		cashRefund = refund
		if err := saleRepo.SetReturnTotals(ctx, returnID, refund, creditReduction); err != nil {
			return apperr.Internal("failed to finalize return", err)
		}

		// Recompute sale status.
		outstanding, err := saleRepo.OutstandingItems(ctx, saleID)
		if err != nil {
			return apperr.Internal("failed to recompute status", err)
		}
		newStatus := "partially_returned"
		if outstanding == 0 {
			newStatus = "returned"
		}
		if err := saleRepo.UpdateStatus(ctx, saleID, newStatus); err != nil {
			return apperr.Internal("failed to update sale status", err)
		}

		d, err := s.loadDetail(ctx, saleRepo, saleID)
		if err != nil {
			return err
		}
		detail = d
		return nil
	}()
	if err != nil {
		return nil, decimal.Zero, 0, err
	}
	return detail, cashRefund, returnID, nil
}
