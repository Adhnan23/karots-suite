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

// Return sends back everything still on a sale, in its own tx. See ReturnTx.
func (s *Service) Return(ctx context.Context, id int64, userID int64) (*Detail, error) {
	var detail *Detail
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		d, _, _, err := s.ReturnTx(ctx, tx, id, userID)
		detail = d
		return err
	})
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// ReturnTx is a line return of every remaining line (restocked), so a whole-sale
// return records its return and refund exactly as the line path does — what was
// paid after item and bill discounts — and works after a partial return. Like
// PartialReturnTx it returns the cash-refund portion and the return id, so the
// caller moves that cash out of a named place in the same tx.
func (s *Service) ReturnTx(ctx context.Context, tx *sqlx.Tx, id, userID int64) (*Detail, decimal.Decimal, int64, error) {
	saleRepo := NewRepository(tx)
	sale, err := saleRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, decimal.Zero, 0, apperr.NotFound("sale")
		}
		return nil, decimal.Zero, 0, apperr.Internal("failed to load sale", err)
	}
	if sale.Status == "returned" {
		return nil, decimal.Zero, 0, apperr.Conflict("this sale has already been returned")
	}
	items, err := saleRepo.Items(ctx, id)
	if err != nil {
		return nil, decimal.Zero, 0, apperr.Internal("failed to load sale items", err)
	}
	var lines []ReturnLineInput
	for _, it := range items {
		if !it.ReturnableQty().IsPositive() {
			continue
		}
		lines = append(lines, ReturnLineInput{SaleItemID: it.ID, Quantity: it.ReturnableQty().String()})
	}
	if len(lines) == 0 {
		return nil, decimal.Zero, 0, apperr.Conflict("nothing left on this sale to return")
	}
	return s.PartialReturnTx(ctx, tx, id, PartialReturnInput{Lines: lines}, userID)
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
		factor, err := saleRepo.PaidFactor(ctx, saleID)
		if err != nil {
			return apperr.Internal("failed to value the return", err)
		}
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
			if qty.GreaterThan(it.ReturnableQty()) {
				return apperr.Conflict(fmt.Sprintf("cannot return %s of %s — only %s remain",
					money.Display(qty), it.ProductName, money.Display(it.ReturnableQty())))
			}
			// Refund what the customer PAID for these units (it is read before
			// AddReturnedQty, so ReturnableQty is what was left before this return).
			lineRefund, err := s.lineRefund(ctx, saleRepo, *it, qty, factor)
			if err != nil {
				return err
			}
			ok, err := saleRepo.AddReturnedQty(ctx, it.ID, qty)
			if err != nil {
				return apperr.Internal("failed to update returned qty", err)
			}
			if !ok {
				return apperr.Conflict("return quantity exceeds what was sold")
			}
			totalRefundValue = totalRefundValue.Add(lineRefund)

			// A service line holds no stock, so nothing goes back on a shelf — but
			// what it consumed (the paper of a spoilt copy job, the cup of a spilt
			// coffee) is gone. Returning the line takes its COGS off the P&L with
			// the revenue, so book that cost as a loss instead.
			if it.IsService {
				if loss := it.CostPrice.Mul(qty).Round(2); loss.IsPositive() {
					if err := stkRepo.InsertMovement(ctx, stock.MovementInput{
						ProductID: it.ProductID, Type: stock.MoveDamage, Quantity: decimal.Zero,
						ReferenceID: &returnID, ReferenceType: &ref, UserID: userID,
						Note: strPtr("used by a returned service line (" + sale.ReceiptNo + ")"),
						Cost: loss,
					}); err != nil {
						return apperr.Internal("failed to record the used materials", err)
					}
				}
				if err := saleRepo.InsertSaleReturnItem(ctx, returnID, it.ID, it.ProductID, qty, lineRefund); err != nil {
					return apperr.Internal("failed to record return line", err)
				}
				continue
			}
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
			// Only the debt earlier returns haven't already cleared: two returns
			// on a part-paid credit sale used to each clear up to the whole debt,
			// over-crediting the customer and refunding no cash.
			prior, err := saleRepo.CreditReduced(ctx, saleID)
			if err != nil {
				return apperr.Internal("failed to read earlier returns", err)
			}
			owed := sale.Total.Sub(sale.PaidAmount).Sub(prior)
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

// lineRefund values qty units of a line at what was paid: the line net (item
// discount already off) shared down by the bill discount. The last of a line
// takes exactly what's left of its paid value, so a line returned in parts
// refunds what was paid, to the cent.
func (s *Service) lineRefund(ctx context.Context, repo *Repository, it SaleItem, qty, factor decimal.Decimal) (decimal.Decimal, error) {
	linePaid := it.PaidValue(factor)
	if !qty.Equal(it.ReturnableQty()) {
		return linePaid.Mul(qty).Div(it.Quantity).Round(2), nil
	}
	prior, err := repo.RefundedForItem(ctx, it.ID)
	if err != nil {
		return decimal.Zero, apperr.Internal("failed to read earlier refunds", err)
	}
	return decimal.Max(linePaid.Sub(prior), decimal.Zero), nil
}
