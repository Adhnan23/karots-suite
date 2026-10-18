package api

import (
	"net/http"

	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// paymentTender is the plugin tender a credit repayment may use (OnPayments).
func paymentTender(method string) (plugin.TenderMethod, bool) {
	for _, t := range plugin.TenderMethods() {
		if t.Value == method && t.OnPayments() {
			return t, true
		}
	}
	return plugin.TenderMethod{}, false
}

// AdminTenderMethods lists the plugin tenders a customer's repayment can use.
func (h *handlers) AdminTenderMethods(c echo.Context) error {
	out := []TenderMethod{}
	for _, t := range plugin.TenderMethods() {
		if t.OnPayments() {
			out = append(out, TenderMethod{Value: t.Value, Label: t.Label, OnPayments: true})
		}
	}
	return c.JSON(http.StatusOK, out)
}
