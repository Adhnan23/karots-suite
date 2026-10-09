// Package customers manages registered customers, especially credit ("vade")
// buyers. Walk-in sales leave customer_id NULL.
package customers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// RecordPayment reduces a customer's outstanding credit balance and logs the
// repayment (balance decrement + ledger row in one transaction). createdBy is
// the acting user (0 = unknown/system).
func (s *Service) RecordPayment(ctx context.Context, id int64, in PaymentInput, createdBy int64) error {
	return db.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		_, err := s.RecordPaymentTx(ctx, tx, id, in, createdBy)
		return err
	})
}

// PaymentResult summarises a recorded credit repayment for the cash mirror.
type PaymentResult struct {
	PaymentID     int64
	Amount        decimal.Decimal
	Method        string
	ReceiptNo     string
	BalanceBefore decimal.Decimal
	BalanceAfter  decimal.Decimal
}

// RecordPaymentTx records a credit repayment over an existing transaction, so the
// caller can book the matching cash intake (cashflow.MoveTx) atomically. Returns
// the payment id, parsed amount and normalized method.
func (s *Service) RecordPaymentTx(ctx context.Context, tx *sqlx.Tx, id int64, in PaymentInput, createdBy int64) (*PaymentResult, error) {
	amt, err := money.Parse(in.Amount)
	if err != nil || !amt.IsPositive() {
		return nil, apperr.Validation("payment amount must be greater than zero")
	}
	method := strings.TrimSpace(in.Method)
	if method == "" {
		method = "cash"
	}
	// A repayment must land somewhere: cash in a drawer or cash locker, card/online
	// in the picked card/bank account. Without one the debt would shrink with no
	// money anywhere, so refuse it.
	switch method {
	case "cash":
	case "card", "online":
		if in.AccountID <= 0 {
			return nil, apperr.Validation("choose the card or bank account this payment goes into")
		}
	default:
		return nil, apperr.Validation("payment method must be cash, card or online")
	}
	r := NewRepository(tx)
	cust, err := r.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("customer")
		}
		return nil, apperr.Internal("failed to load customer", err)
	}
	before := cust.OutstandingBalance
	after := before.Sub(amt)
	if in.ApplyToOpening {
		// "Old" first: pay the old debt down, then let any overflow settle the
		// linked balance and — beyond that — leave the customer in credit (an
		// advance). No cap: it simply waterfalls.
		openingPay := amt
		if openingPay.GreaterThan(cust.OpeningUnlinked) {
			openingPay = cust.OpeningUnlinked
		}
		if openingPay.IsNegative() {
			openingPay = decimal.Zero
		}
		if openingPay.IsPositive() {
			if err := r.PayOpening(ctx, id, openingPay); err != nil {
				return nil, apperr.Internal("failed to record payment", err)
			}
		}
		if rest := amt.Sub(openingPay); rest.IsPositive() {
			if err := r.AddBalance(ctx, id, rest.Neg()); err != nil {
				return nil, apperr.Internal("failed to record payment", err)
			}
		}
	} else if err := r.AddBalance(ctx, id, amt.Neg()); err != nil {
		return nil, apperr.Internal("failed to record payment", err)
	}
	var cb *int64
	if createdBy > 0 {
		cb = &createdBy
	}
	var payID int64
	var receiptNo string
	err = tx.QueryRowxContext(ctx, `
		INSERT INTO customer_payments
			(customer_id, amount, method, reference, note, created_by,
			 receipt_no, balance_before, balance_after)
		VALUES ($1,$2,$3,$4,$5,$6,
			'DP-' || lpad(nextval('debt_receipt_seq')::text, 6, '0'), $7, $8)
		RETURNING id, receipt_no`,
		id, amt, method, in.Reference, in.Note, cb, before, after,
	).Scan(&payID, &receiptNo)
	if err != nil {
		return nil, apperr.Internal("failed to record payment", err)
	}
	return &PaymentResult{
		PaymentID: payID, Amount: amt, Method: method,
		ReceiptNo: receiptNo, BalanceBefore: before, BalanceAfter: after,
	}, nil
}

// RefundAdvanceTx hands back part of a customer's advance (negative balance) as
// cash: the balance moves up toward zero and the customer ledger records a
// negative "refund" payment. It gets no DP- number — the caller's CR- cash
// receipt is the proof. Refunding more than the advance is rejected.
func (s *Service) RefundAdvanceTx(ctx context.Context, tx *sqlx.Tx, id int64, amount string, createdBy int64) (*PaymentResult, error) {
	amt, err := money.Parse(amount)
	if err != nil || !amt.IsPositive() {
		return nil, apperr.Validation("refund amount must be greater than zero")
	}
	r := NewRepository(tx)
	cust, err := r.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("customer")
		}
		return nil, apperr.Internal("failed to load customer", err)
	}
	before := cust.OutstandingBalance
	if amt.GreaterThan(before.Neg()) {
		return nil, apperr.Validation("refund is more than the customer's advance")
	}
	if err := r.AddBalance(ctx, id, amt); err != nil {
		return nil, apperr.Internal("failed to record refund", err)
	}
	var cb *int64
	if createdBy > 0 {
		cb = &createdBy
	}
	var payID int64
	after := before.Add(amt)
	if err := tx.QueryRowxContext(ctx, `
		INSERT INTO customer_payments (customer_id, amount, method, created_by, balance_before, balance_after)
		VALUES ($1,$2,'refund',$3,$4,$5) RETURNING id`,
		id, amt.Neg(), cb, before, after,
	).Scan(&payID); err != nil {
		return nil, apperr.Internal("failed to record refund", err)
	}
	return &PaymentResult{PaymentID: payID, Amount: amt, Method: "refund", BalanceBefore: before, BalanceAfter: after}, nil
}

// DebtReceipt is one credit-payment receipt as shown in the Credit receipts tab
// and rendered on the printable debt slip: the payment joined to its customer
// (name/phone/limit) and the cashier who took it.
type DebtReceipt struct {
	ID            int64            `db:"id"`
	ReceiptNo     string           `db:"receipt_no"`
	CreatedAt     time.Time        `db:"created_at"`
	CustomerName  string           `db:"customer_name"`
	CustomerPhone *string          `db:"customer_phone"`
	Amount        decimal.Decimal  `db:"amount"`
	Method        string           `db:"method"`
	BalanceBefore *decimal.Decimal `db:"balance_before"`
	BalanceAfter  *decimal.Decimal `db:"balance_after"`
	CreditLimit   decimal.Decimal  `db:"credit_limit"`
	CashierName   *string          `db:"cashier_name"`
}

// DebtFilter narrows the credit-payment list: Query matches the receipt number
// or the customer name/phone; From/To bound the created_at range (To exclusive,
// nil = unbounded).
type DebtFilter struct {
	Query string
	From  *time.Time
	To     *time.Time
	Limit  int
	Offset int // rows to skip (a list's later pages); used only with Limit
}

const debtSelect = `
	SELECT cp.id, cp.receipt_no, cp.created_at, c.name AS customer_name, c.phone AS customer_phone,
	       cp.amount, cp.method, cp.balance_before, cp.balance_after, c.credit_limit,
	       u.name AS cashier_name
	FROM customer_payments cp
	JOIN customers c ON c.id = cp.customer_id
	LEFT JOIN users u ON u.id = cp.created_by`

// ListPayments returns credit-payment receipts newest-first for the Credit tab.
func (s *Service) ListPayments(ctx context.Context, f DebtFilter) ([]DebtReceipt, error) {
	q := debtSelect + ` WHERE cp.receipt_no IS NOT NULL`
	args := []any{}
	n := 1
	if t := strings.TrimSpace(f.Query); t != "" {
		q += fmt.Sprintf(` AND (cp.receipt_no ILIKE $%d OR c.name ILIKE $%d OR c.phone ILIKE $%d)`, n, n, n)
		args = append(args, "%"+t+"%")
		n++
	}
	if f.From != nil {
		q += fmt.Sprintf(` AND cp.created_at >= $%d`, n)
		args = append(args, *f.From)
		n++
	}
	if f.To != nil {
		q += fmt.Sprintf(` AND cp.created_at < $%d`, n)
		args = append(args, *f.To)
		n++
	}
	q += ` ORDER BY cp.id DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, n, n+1)
		args = append(args, f.Limit, max(f.Offset, 0))
	}
	var rows []DebtReceipt
	if err := s.db.SelectContext(ctx, &rows, q, args...); err != nil {
		return nil, apperr.Internal("failed to list credit payments", err)
	}
	return rows, nil
}

// GetPayment loads one credit-payment receipt for the View page / reprint.
func (s *Service) GetPayment(ctx context.Context, id int64) (*DebtReceipt, error) {
	var r DebtReceipt
	if err := s.db.GetContext(ctx, &r, debtSelect+` WHERE cp.id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("payment")
		}
		return nil, apperr.Internal("failed to load credit payment", err)
	}
	return &r, nil
}
