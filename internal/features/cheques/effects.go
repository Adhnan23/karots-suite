package cheques

import (
	"context"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/purchases"
	"karots-suite/internal/features/suppliers"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// undo puts back what a bounced / cancelled cheque paid. Received: the money
// leaves Cheques in hand and the customer owes it again (no credit-limit check —
// never stop a sale). Given: nothing leaves the bank; the supplier is owed it
// again (and the bills it paid reopen) or the expense becomes unpaid.
func (s *Service) undo(ctx context.Context, tx *sqlx.Tx, c *Cheque, status string, actor int64) error {
	if c.Direction == "in" {
		inHand, err := lockers.ChequesInHandTx(ctx, tx)
		if err != nil {
			return apperr.Internal("failed to find Cheques in hand", err)
		}
		if _, err := s.cf.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.Locker(inHand), To: cashflow.External(),
			Amount: c.Amount, Reason: "cheque " + c.Number + " " + status, ReceiptKind: "cheque_returned",
			Ref: &cashflow.Ref{Kind: "cheque", ID: c.ID}, Party: deref(c.CustomerName), ActorID: actor, Recorded: true}); err != nil { // never refused
			return err
		}
		if err := customers.NewRepository(tx).AddBalance(ctx, *c.CustomerID, c.Amount); err != nil {
			return apperr.Internal("failed to update customer balance", err)
		}
		return nil
	}
	if c.SupplierID != nil {
		if err := suppliers.NewRepository(tx).AddBalance(ctx, *c.SupplierID, c.Amount); err != nil {
			return apperr.Internal("failed to update supplier balance", err)
		}
		if c.SupplierPaymentID != nil {
			return reopenBills(ctx, tx, *c.SupplierPaymentID, c.Amount)
		}
		return nil
	}
	if c.ExpenseID != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE expenses SET unpaid = unpaid + $1 WHERE id = $2`, c.Amount, *c.ExpenseID); err != nil {
			return apperr.Internal("failed to mark expense unpaid", err)
		}
	}
	return nil
}

// reopenBills takes amount back off the bills supplier payment paymentID paid,
// newest allocation first, never reopening an allocation past its own amount.
func reopenBills(ctx context.Context, tx *sqlx.Tx, paymentID int64, amount decimal.Decimal) error {
	var allocs []struct {
		ID         int64           `db:"id"`
		PurchaseID int64           `db:"purchase_id"`
		Amount     decimal.Decimal `db:"amount"`
		Reopened   decimal.Decimal `db:"reopened"`
	}
	if err := tx.SelectContext(ctx, &allocs, `SELECT id, purchase_id, amount, reopened FROM supplier_payment_allocations
		WHERE payment_id = $1 ORDER BY id DESC FOR UPDATE`, paymentID); err != nil {
		return apperr.Internal("failed to load payment allocations", err)
	}
	prepo := purchases.NewRepository(tx)
	left := amount
	for _, a := range allocs {
		if !left.IsPositive() {
			break
		}
		take := decimal.Min(left, a.Amount.Sub(a.Reopened))
		if !take.IsPositive() {
			continue
		}
		if err := prepo.UnapplyPayment(ctx, a.PurchaseID, take); err != nil {
			return apperr.Internal("failed to reopen bill", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE supplier_payment_allocations SET reopened = reopened + $1 WHERE id = $2`, take, a.ID); err != nil {
			return apperr.Internal("failed to reopen allocation", err)
		}
		left = left.Sub(take)
	}
	return nil
}
