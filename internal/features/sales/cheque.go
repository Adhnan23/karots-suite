package sales

import (
	"context"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// ChequeTaken is one cheque tender line of a completed sale, handed to the
// cheque leg inside the sale's tx.
type ChequeTaken struct {
	SaleID, PaymentID, CustomerID int64
	Party                         string // the customer's name
	Amount                        decimal.Decimal
	Cheque                        ChequeInput
	ActorID                       int64
}

// ChequeLeg records a cheque taken at the till and books its money into
// Cheques in hand, on the sale's own tx. Injected from the composition root
// (sales never imports the cheques package — it imports cashflow, which
// imports sales). nil = cheque tenders are refused.
type ChequeLeg func(ctx context.Context, tx *sqlx.Tx, ev ChequeTaken) error

// WithChequeLeg injects the cheque seam. Returns the service.
func (s *Service) WithChequeLeg(fn ChequeLeg) *Service {
	s.chequeLeg = fn
	return s
}

// chequeCheck: a cheque needs its details and someone to chase if it bounces.
// Reports whether the sale takes one.
func chequeCheck(in CreateInput) (bool, error) {
	hasCheque := false
	for _, p := range in.Payments {
		if p.Method != MethodCheque {
			continue
		}
		if amt, perr := money.Parse(p.Amount); perr != nil || !amt.IsPositive() {
			continue
		}
		hasCheque = true
		if in.CustomerID == nil {
			return false, apperr.Validation("pick the customer who gave the cheque")
		}
		if p.Cheque == nil || strings.TrimSpace(p.Cheque.Number) == "" || strings.TrimSpace(p.Cheque.Date) == "" {
			return false, apperr.Validation("enter the cheque number and date")
		}
		if strings.TrimSpace(p.Cheque.Bank) == "" {
			return false, apperr.Validation("enter the cheque's bank")
		}
	}
	return hasCheque, nil
}
