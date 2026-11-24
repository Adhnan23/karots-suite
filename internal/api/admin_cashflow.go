package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/reports"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminCashflow(g *echo.Group) {
	g.GET("/cashflow", h.AdminCashflow)
	g.GET("/cashflow/ledger", h.AdminCashflowLedger)
}

// cashflowRange is the page's period; nothing chosen = today.
func cashflowRange(c echo.Context) (time.Time, time.Time, string, string, error) {
	preset := c.QueryParam("preset")
	if preset == "" && c.QueryParam("from") == "" {
		preset = "today"
	}
	from, to, fs, ts, err := reports.ResolveRange(preset, c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return from, to, fs, ts, apperr.Validation(err.Error())
	}
	return from, to, fs, ts, nil
}

// AdminCashflow: where the money is now, how it moved over the range, and what
// the business is worth.
func (h *handlers) AdminCashflow(c echo.Context) error {
	ctx := c.Request().Context()
	from, to, fs, ts, err := cashflowRange(c)
	if err != nil {
		return err
	}
	out := AdminCashflow{From: fs, To: ts, Places: []CashPlace{}, Plugins: []CashPlaceGroup{}}
	cash, inHand := decimal.Zero, decimal.Zero
	ls, err := h.t.lockers.List(ctx, true)
	if err != nil {
		return err
	}
	for _, l := range ls {
		out.Places = append(out.Places, CashPlace{Value: "locker:" + strconv.FormatInt(l.ID, 10), Name: l.Name, Kind: l.Kind, Balance: dec2(l.Balance)})
		if l.Kind == lockers.KindCheque { // uncleared cheques are shown, but are not cash
			inHand = inHand.Add(l.Balance)
		} else {
			cash = cash.Add(l.Balance)
		}
	}
	tills, err := h.d.CashRegister.OpenSessions(ctx)
	if err != nil {
		return err
	}
	for _, t := range tills {
		bal := t.OpeningCash
		if sum, err := h.d.CashRegister.Summary(ctx, t.UserID); err == nil {
			bal = sum.Expected
		}
		out.Places = append(out.Places, CashPlace{Value: "till:" + strconv.FormatInt(t.UserID, 10), Name: "Till — " + t.UserName, Kind: "till", Balance: dec2(bal)})
		cash = cash.Add(bal)
	}
	// Plugin money (e.g. carrier float) is the shop's, but not cash on hand.
	pluginTotal := decimal.Zero
	for _, g := range plugin.CollectMoneyLocations(ctx) {
		pg := CashPlaceGroup{Group: g.Group, Total: dec2(g.Total), Items: []CashPlace{}}
		for _, it := range g.Items {
			pg.Items = append(pg.Items, CashPlace{Name: it.Name, Kind: "plugin", Balance: dec2(it.Balance)})
		}
		out.Plugins = append(out.Plugins, pg)
		pluginTotal = pluginTotal.Add(g.Total)
	}
	out.CashOnHand = dec2(cash)

	tot, err := cashflow.RangeTotals(ctx, h.d.DB, from, to)
	if err != nil {
		return err
	}
	out.TillSales, out.In, out.Openings, out.Out, out.Moved, out.Net, out.Rows = dec2(tot.TillSales), dec2(tot.In), dec2(tot.Openings), dec2(tot.Out), dec2(tot.Moved), dec2(tot.Net), tot.Rows

	// Net position: what the shop has and is owed, less what it owes. Customers
	// and suppliers sit on both sides, so dues and advances are summed apart.
	var stock, recv, adv, pay, credit, written decimal.Decimal
	for _, q := range []struct {
		dst *decimal.Decimal
		sql string
	}{
		{&stock, `SELECT COALESCE(SUM(qty_remaining * cost_price),0) FROM stock_batches`},
		{&recv, `SELECT COALESCE(SUM(GREATEST(outstanding_balance,0)),0) FROM customers`},
		{&adv, `SELECT COALESCE(SUM(GREATEST(-outstanding_balance,0)),0) FROM customers`},
		{&pay, `SELECT COALESCE(SUM(GREATEST(outstanding_balance,0)),0) FROM suppliers`},
		{&credit, `SELECT COALESCE(SUM(GREATEST(-outstanding_balance,0)),0) FROM suppliers`},
		// Cheques the shop wrote that the bank hasn't paid yet: still in the bank balance.
		{&written, `SELECT COALESCE(SUM(amount),0) FROM cheques WHERE direction = 'out' AND status = 'pending'`},
	} {
		if err := h.d.DB.GetContext(ctx, q.dst, q.sql); err != nil {
			return apperr.Internal("failed to compute net position", err)
		}
	}
	line := func(label string, v decimal.Decimal, sign int) NetLine { return NetLine{Label: label, Amount: dec2(v), Sign: sign} }
	out.NetLines = []NetLine{line("Cash on hand", cash, 1)}
	if inHand.IsPositive() || written.IsPositive() {
		out.NetLines = append(out.NetLines, line("Cheques in hand", inHand, 1))
	}
	out.NetLines = append(out.NetLines, line("Stock at cost", stock, 1), line("Customers owe you", recv, 1), line("Suppliers hold for you", credit, 1))
	if len(out.Plugins) > 0 {
		out.NetLines = append(out.NetLines, line("Plugin money", pluginTotal, 1))
	}
	out.NetLines = append(out.NetLines, line("You owe suppliers", pay, -1), line("Customers paid ahead", adv, -1))
	if written.IsPositive() {
		out.NetLines = append(out.NetLines, line("Your cheques not cashed yet", written, -1))
	}
	out.NetPosition = dec2(cash.Add(inHand).Add(stock).Add(recv).Add(credit).Add(pluginTotal).Sub(pay).Sub(adv).Sub(written))
	return c.JSON(http.StatusOK, out)
}

// AdminCashflowLedger pages the unified ledger (50 at a time; ?all=1 = the
// whole range, for the print), filtered by ?place= and ?kind=.
func (h *handlers) AdminCashflowLedger(c echo.Context) error {
	from, to, _, _, err := cashflowRange(c)
	if err != nil {
		return err
	}
	f := cashflow.LedgerFilter{Place: strings.TrimSpace(c.QueryParam("place")), Kind: strings.TrimSpace(c.QueryParam("kind")), Limit: counterPage, Offset: listOffset(c)}
	if c.QueryParam("all") == "1" {
		f.Limit, f.Offset = 0, 0
	}
	rows, err := cashflow.LedgerPage(c.Request().Context(), h.d.DB, from, to, f)
	if err != nil {
		return err
	}
	out := make([]CashflowRow, 0, len(rows))
	for _, r := range rows {
		row := CashflowRow{At: r.CreatedAt.Format(time.RFC3339), Place: r.Place, Location: r.Location, Kind: r.Kind, Delta: dec2(r.Delta), Note: r.Note, Moved: r.Internal, ReceiptNo: strOr(r.ReceiptNo)}
		if r.ReceiptID != nil {
			row.ReceiptID = *r.ReceiptID
		}
		out = append(out, row)
	}
	return c.JSON(http.StatusOK, out)
}
