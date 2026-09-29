package api

import (
	"net/http"
	"strings"
	"time"

	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/customers"

	"github.com/labstack/echo/v4"
)

// CounterMoneyReceipts lists CR- money receipts shop-wide (karots-pos ReceiptsCash).
func (h *handlers) CounterMoneyReceipts(c echo.Context) error {
	rows, err := h.t.receipts.List(c.Request().Context(), cashflow.ReceiptFilter{Query: strings.TrimSpace(c.QueryParam("q")), Limit: 50})
	if err != nil {
		return err
	}
	out := make([]MoneyReceiptRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, MoneyReceiptRow{ID: r.ID, ReceiptNo: r.ReceiptNo, Kind: r.Kind, KindLabel: receiptKindLabel(r.Kind),
			FromLabel: r.FromLabel, ToLabel: r.ToLabel, Party: r.Party, Amount: dec2(r.Amount), Note: r.Note,
			CreatedAt: r.CreatedAt.Format(time.RFC3339), CreatedBy: strOr(r.CreatedByName)})
	}
	return c.JSON(http.StatusOK, out)
}

// CounterDebtReceipts lists credit repayments (karots-pos ReceiptsCredit).
func (h *handlers) CounterDebtReceipts(c echo.Context) error {
	rows, err := h.t.customers.ListPayments(c.Request().Context(), customers.DebtFilter{Query: strings.TrimSpace(c.QueryParam("q")), Limit: 50})
	if err != nil {
		return err
	}
	out := make([]DebtReceiptRow, 0, len(rows))
	for _, r := range rows {
		row := DebtReceiptRow{ID: r.ID, ReceiptNo: r.ReceiptNo, CreatedAt: r.CreatedAt.Format(time.RFC3339), CustomerName: r.CustomerName,
			CustomerPhone: strOr(r.CustomerPhone), Amount: dec2(r.Amount), Method: methodOr(r.Method), CashierName: strOr(r.CashierName)}
		if r.BalanceBefore != nil {
			row.BalanceBefore = dec2(*r.BalanceBefore)
		}
		if r.BalanceAfter != nil {
			row.BalanceAfter = dec2(*r.BalanceAfter)
		}
		out = append(out, row)
	}
	return c.JSON(http.StatusOK, out)
}
