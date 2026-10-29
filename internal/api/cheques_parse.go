package api

import (
	"context"
	"strconv"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/cheques"
	"karots-suite/internal/features/customers"

	"github.com/jmoiron/sqlx"
)

// chequeLines turns a form's cheque lines into cheques (dir "in" received,
// "out" given) and their total.
func chequeLines(in []ChequeLine, dir string) ([]cheques.Line, string, error) {
	raw := make([]cheques.LineInput, 0, len(in))
	for _, l := range in {
		raw = append(raw, cheques.LineInput{Bank: l.Bank, LockerID: l.LockerID, Number: l.Number, Amount: l.Amount, Date: l.Date, Note: l.Note})
	}
	lines, total, err := cheques.ParseLines(raw, dir)
	if err != nil {
		return nil, "", err
	}
	return lines, total.StringFixed(2), nil
}

// customerChequePay records a customer payment made with cheques: one payment
// row (method cheque) for their total, then each cheque received against it,
// its money into Cheques in hand. More than they owe becomes an advance, as
// with cash. in carries the reference / note / old-debt choice.
func (h *handlers) customerChequePay(ctx context.Context, custID int64, party string, lines []ChequeLine, in customers.PaymentInput, userID int64) (*customers.PaymentResult, error) {
	ls, total, err := chequeLines(lines, "in")
	if err != nil {
		return nil, err
	}
	in.Method, in.Amount = "cheque", total
	var res *customers.PaymentResult
	err = appdb.WithTx(ctx, h.d.DB, func(tx *sqlx.Tx) error {
		var err error
		if res, err = h.t.customers.RecordPaymentTx(ctx, tx, custID, in, userID); err != nil {
			return err
		}
		return h.takeChequesTx(ctx, tx, custID, res.PaymentID, party, ls, userID)
	})
	return res, err
}

// takeChequesTx receives each cheque of a customer payment, in the caller's tx.
func (h *handlers) takeChequesTx(ctx context.Context, tx *sqlx.Tx, custID, custPaymentID int64, party string, lines []cheques.Line, userID int64) error {
	for _, l := range lines {
		if _, err := cheques.ReceiveTx(ctx, tx, h.d.Cashflow, l, cheques.Link{CustomerID: custID, CustomerPaymentID: &custPaymentID, Party: party}, userID); err != nil {
			return err
		}
	}
	return nil
}

func chequeCount(n int) string {
	if n == 1 {
		return "1 cheque"
	}
	return strconv.Itoa(n) + " cheques"
}
