package clearance

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// The plugin's own JSON (core's types.gen.ts is core-only); mirrored by hand in
// web/src/plugins/clearance/types.ts. Money and percents are decimal strings.

type SettingsDTO struct {
	StaleDays        int    `json:"staleDays"`
	DefaultPercent   string `json:"defaultPercent"`
	MinMarginPercent string `json:"minMarginPercent"`
	ExpiryDays       int    `json:"expiryDays"`
	DeadMonths       int    `json:"deadMonths"`
}

type PageRow struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	Unit         string `json:"unit"`
	OnHand       string `json:"onHand"`
	Cost         string `json:"cost"`
	Price        string `json:"price"`
	FloorCost    string `json:"floorCost"`   // the lot's cost when expiring, else the product's
	Reason       string `json:"reason"`      // "stale" | "expiring"
	ReasonLabel  string `json:"reasonLabel"` // "never sold", "75 days no sale", "expires in 3 days"
	Status       string `json:"status"`      // "" | "live" | "snoozed"
	Percent      string `json:"percent"`     // the live markdown, else the safe suggestion
	NewPrice     string `json:"newPrice"`
	SnoozedUntil string `json:"snoozedUntil"` // ISO date, "" unless snoozed
}

type DeadRow struct {
	ProductID    int64  `json:"productId"`
	Name         string `json:"name"`
	Unit         string `json:"unit"`
	MonthsIdle   int    `json:"monthsIdle"`
	LastActivity string `json:"lastActivity"` // ISO date
}

type PageDTO struct {
	Settings SettingsDTO `json:"settings"`
	Rows     []PageRow   `json:"rows"`
	Dead     []DeadRow   `json:"dead"`
}

type handlers struct{ p *Plugin }

func (h handlers) page(c echo.Context) error {
	ctx := c.Request().Context()
	cfg, err := h.p.store.GetSettings(ctx)
	if err != nil {
		return apperr.Internal("load clearance settings", err)
	}
	items, err := h.p.store.Candidates(ctx)
	if err != nil {
		return apperr.Internal("load clearance list", err)
	}
	dead, err := h.p.store.Dead(ctx)
	if err != nil {
		return apperr.Internal("load dead stock", err)
	}
	out := PageDTO{Settings: settingsDTO(cfg), Rows: make([]PageRow, 0, len(items)), Dead: make([]DeadRow, 0, len(dead))}
	for _, it := range items {
		out.Rows = append(out.Rows, pageRow(it, cfg, time.Now()))
	}
	for _, d := range dead {
		out.Dead = append(out.Dead, DeadRow{ProductID: d.ProductID, Name: d.Name, Unit: d.Unit, MonthsIdle: d.MonthsIdle, LastActivity: d.LastActivity.Format("2006-01-02")})
	}
	return c.JSON(http.StatusOK, out)
}

// pageRow decides the reason (expiry wins when a lot is inside the window), the
// floor cost, and the percent to show.
func pageRow(it Candidate, cfg Settings, now time.Time) PageRow {
	r := PageRow{ProductID: it.ProductID, Name: it.Name, Unit: it.Unit, OnHand: it.OnHand.String(),
		Cost: it.Cost.StringFixed(2), Price: it.Price.StringFixed(2), Status: it.Status, Reason: "stale", ReasonLabel: "never sold"}
	if it.DaysSinceSale != nil {
		r.ReasonLabel = strconv.Itoa(*it.DaysSinceSale) + " days no sale"
	}
	floor := it.Cost
	if it.SoonestExpiry != nil {
		e := *it.SoonestExpiry
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, e.Location())
		if d := int(math.Round(e.Sub(today).Hours() / 24)); d <= cfg.ExpiryDays {
			r.Reason = "expiring"
			switch {
			case d <= 0:
				r.ReasonLabel = "expires today"
			case d == 1:
				r.ReasonLabel = "expires in 1 day"
			default:
				r.ReasonLabel = "expires in " + strconv.Itoa(d) + " days"
			}
			if it.LotCost != nil {
				floor = *it.LotCost
			}
		}
	}
	r.FloorCost = floor.StringFixed(2)
	pct := suggestPercent(it.Price, floor, cfg.DefaultPercent, cfg.MinMarginPercent)
	if it.Status == "live" && it.Percent != nil {
		pct = *it.Percent
	}
	r.Percent, r.NewPrice = pct.String(), newPrice(it.Price, pct).StringFixed(2)
	if it.SnoozedUntil != nil {
		r.SnoozedUntil = it.SnoozedUntil.Format("2006-01-02")
	}
	return r
}

func settingsDTO(s Settings) SettingsDTO {
	return SettingsDTO{StaleDays: s.StaleDays, DefaultPercent: s.DefaultPercent.String(), MinMarginPercent: s.MinMarginPercent.String(), ExpiryDays: s.ExpiryDays, DeadMonths: s.DeadMonths}
}

func (h handlers) saveSettings(c echo.Context) error {
	var in SettingsDTO
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid settings")
	}
	dp, err1 := decimal.NewFromString(in.DefaultPercent)
	mm, err2 := decimal.NewFromString(in.MinMarginPercent)
	switch {
	case in.StaleDays < 1 || in.ExpiryDays < 1 || in.DeadMonths < 1:
		return apperr.Validation("days and months must be at least 1")
	case err1 != nil || !dp.IsPositive() || dp.GreaterThanOrEqual(decimal.NewFromInt(100)):
		return apperr.Validation("the suggested markdown must be more than 0% and under 100%")
	case err2 != nil || mm.IsNegative():
		return apperr.Validation("the minimum margin can't be negative")
	}
	s := Settings{StaleDays: in.StaleDays, DefaultPercent: dp, MinMarginPercent: mm, ExpiryDays: in.ExpiryDays, DeadMonths: in.DeadMonths}
	if err := h.p.store.SaveSettings(c.Request().Context(), s); err != nil {
		return apperr.Internal("save clearance settings", err)
	}
	h.audit(c, audit.ActionSettings, "clearance", "", "rules")
	return c.JSON(http.StatusOK, settingsDTO(s))
}

func (h handlers) approve(c echo.Context) error {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil {
		return apperr.Validation("invalid product")
	}
	var in struct {
		Percent string `json:"percent"`
	}
	if err := c.Bind(&in); err != nil {
		return apperr.Validation("invalid markdown")
	}
	pct, err := decimal.NewFromString(in.Percent)
	if err != nil || !pct.IsPositive() || pct.GreaterThanOrEqual(decimal.NewFromInt(100)) {
		return apperr.Validation("the markdown must be more than 0% and under 100%")
	}
	if err := h.p.store.Approve(c.Request().Context(), pid, pct, middleware.CurrentUserID(c)); err != nil {
		return apperr.Internal("approve markdown", err)
	}
	h.audit(c, audit.ActionUpdate, "clearance_markdown", strconv.FormatInt(pid, 10), "approved -"+pct.String()+"%")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) snooze(c echo.Context) error {
	return h.byProduct(c, "snoozed", func(pid int64) error { return h.p.store.Snooze(c.Request().Context(), pid, middleware.CurrentUserID(c)) })
}

func (h handlers) unsnooze(c echo.Context) error {
	return h.byProduct(c, "unsnoozed", func(pid int64) error { return h.p.store.Unsnooze(c.Request().Context(), pid) })
}

func (h handlers) byProduct(c echo.Context, what string, fn func(int64) error) error {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
	if err != nil {
		return apperr.Validation("invalid product")
	}
	if err := fn(pid); err != nil {
		return apperr.Internal("clearance "+what, err)
	}
	h.audit(c, audit.ActionUpdate, "clearance_markdown", strconv.FormatInt(pid, 10), what)
	return c.NoContent(http.StatusNoContent)
}

// disable retires dead products through core's product disable (reversible from
// Products → show disabled), one or every product shown.
func (h handlers) disable(c echo.Context) error {
	var in struct {
		ProductIDs []int64 `json:"productIds"`
	}
	if err := c.Bind(&in); err != nil || len(in.ProductIDs) == 0 || len(in.ProductIDs) > 500 {
		return apperr.Validation("pick between 1 and 500 products to disable")
	}
	n := 0
	for _, id := range in.ProductIDs {
		if err := h.p.core.Products.Delete(c.Request().Context(), id); err != nil {
			return err
		}
		n++
		h.audit(c, audit.ActionDelete, "product", strconv.FormatInt(id, 10), "disabled as dead stock (clearance)")
	}
	return c.JSON(http.StatusOK, map[string]int{"disabled": n})
}

func (h handlers) audit(c echo.Context, action, entity, id, detail string) {
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, entity, id, detail)
	}
}
