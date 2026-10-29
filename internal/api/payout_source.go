package api

import (
	"context"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/payout"

	"github.com/labstack/echo/v4"
)

func (h *handlers) payout() *payout.Service { return payout.NewService(h.d.DB, h.d.Cashflow) }

// adminGiven is where an admin form's given money comes from: its cheques
// (method cheque), else the source / method rules of adminMoneyLoc.
func (h *handlers) adminGiven(ctx context.Context, method, source string, lines []ChequeLine) (payout.Source, error) {
	if method == payout.MethodCheque {
		ls, _, err := chequeLines(lines, "out")
		return payout.Source{Method: method, Cheques: ls}, err
	}
	m, ok := normSupplierMethod(method)
	if !ok {
		return payout.Source{}, apperr.Validation("invalid payment method")
	}
	loc, err := h.adminMoneyLoc(ctx, m, source)
	return payout.Source{Loc: loc, Method: m}, err
}

// counterGiven is adminGiven at the counter: cheques only on bank places the
// cashier may use, else supplierMoneyLoc's rules.
func (h *handlers) counterGiven(c echo.Context, method string, lockerID int64, lines []ChequeLine) (payout.Source, error) {
	if method == payout.MethodCheque {
		ls, _, err := chequeLines(lines, "out")
		if err != nil {
			return payout.Source{}, err
		}
		for _, l := range ls {
			if _, err := h.counterSource(c, l.LockerID); err != nil {
				return payout.Source{}, apperr.Forbidden("you can't write cheques on that bank account")
			}
		}
		return payout.Source{Method: method, Cheques: ls}, nil
	}
	m, ok := normSupplierMethod(method)
	if !ok {
		return payout.Source{}, apperr.Validation("invalid payment method")
	}
	loc, err := h.supplierMoneyLoc(c, m, lockerID)
	return payout.Source{Loc: loc, Method: m}, err
}

// paidHow is the audit wording for how given money went.
func paidHow(src payout.Source) string {
	if src.IsCheque() {
		return chequeCount(len(src.Cheques))
	}
	return src.Method
}
