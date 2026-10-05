package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/money"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminMoney(g *echo.Group) {
	g.GET("/money-sources", h.AdminMoneySources)
	g.GET("/money-receipts/:id", h.AdminMoneyReceipt)
}

// AdminMoneySources lists active lockers and open tills (karots-pos
// cashLocationChoices) for the admin's pay / collect forms.
func (h *handlers) AdminMoneySources(c echo.Context) error {
	ctx := c.Request().Context()
	ls, err := h.t.lockers.List(ctx, true)
	if err != nil {
		return err
	}
	tills, err := h.d.CashRegister.OpenSessions(ctx)
	if err != nil {
		return err
	}
	out := make([]MoneySource, 0, len(ls)+len(tills))
	for _, l := range ls {
		out = append(out, MoneySource{Value: "locker:" + strconv.FormatInt(l.ID, 10), Label: l.Name, Group: "Lockers", Kind: l.Kind, Balance: dec2(l.Balance)})
	}
	for _, t := range tills {
		out = append(out, MoneySource{Value: "till:" + strconv.FormatInt(t.UserID, 10), Label: "Till — " + t.UserName, Group: "Tills", Kind: "till"})
	}
	return c.JSON(http.StatusOK, out)
}

// parseLocation turns a MoneySource value into a cashflow location; "" is
// outside the shop (karots-pos parseLocation + parsePaymentSource).
func parseLocation(v string) (cashflow.Location, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return cashflow.External(), nil
	}
	kind, raw, _ := strings.Cut(v, ":")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return cashflow.Location{}, apperr.Validation("invalid money source")
	}
	switch kind {
	case "locker":
		return cashflow.Locker(id), nil
	case "till":
		return cashflow.Till(id), nil
	}
	return cashflow.Location{}, apperr.Validation("invalid money source")
}

// parseAmount reads a money amount typed by a person: spaces and thousands
// commas dropped, then a plain decimal above zero with at most 2 dp. what names
// the field in the error ("amount", "fee", …).
func parseAmount(s, what string) (decimal.Decimal, error) {
	s = strings.NewReplacer(",", "", " ", "").Replace(s)
	v, err := money.Parse(s)
	if err != nil || s == "" || strings.ContainsAny(s, "eE") || !v.IsPositive() || !v.Equal(v.Round(2)) {
		return decimal.Zero, apperr.Validation("enter a valid " + what)
	}
	return v, nil
}
