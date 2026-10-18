package recharge

import (
	"context"
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// Setup is the Carriers page: every carrier with its rates, devices and packages.
type Setup struct {
	Carriers []TillCarrier `json:"carriers"`
	Devices  []Device      `json:"devices"`
	Packages []Package     `json:"packages"`
}

func (h handlers) setup(c echo.Context) error {
	ctx := c.Request().Context()
	out := Setup{Carriers: []TillCarrier{}}
	cs, err := h.p.store.Carriers(ctx, true)
	if err != nil {
		return err
	}
	for _, cr := range cs {
		rs, err := h.p.store.Rates(ctx, h.p.store.db, cr.ID)
		if err != nil {
			return err
		}
		out.Carriers = append(out.Carriers, TillCarrier{Carrier: cr, Rates: rs})
	}
	if out.Devices, err = h.p.store.Devices(ctx, true); err != nil {
		return err
	}
	if out.Packages, err = h.p.store.Packages(ctx, true); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

type nameInput struct {
	Name string `json:"name"`
}

type activeInput struct {
	On bool `json:"on"`
}

func (h handlers) createCarrier(c echo.Context) error {
	in, err := bind[nameInput](c)
	if err != nil {
		return err
	}
	id, err := h.p.store.CreateCarrier(c.Request().Context(), in.Name)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "recharge_carrier", id, in.Name)
	return c.JSON(http.StatusCreated, map[string]int64{"id": id})
}

func (h handlers) renameCarrier(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[nameInput](c)
	if err != nil {
		return err
	}
	if err := h.p.store.RenameCarrier(c.Request().Context(), id, in.Name); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "recharge_carrier", id, "renamed "+in.Name)
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) carrierSupplier(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[struct {
		SupplierID int64 `json:"supplierId"`
	}](c)
	if err != nil {
		return err
	}
	if err := h.p.store.SetCarrierSupplier(c.Request().Context(), id, in.SupplierID); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "recharge_carrier", id, "float dealer set")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) carrierActive(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[activeInput](c)
	if err != nil {
		return err
	}
	if err := h.p.store.SetCarrierActive(c.Request().Context(), id, in.On); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) setRates(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[[]Rate](c)
	if err != nil {
		return err
	}
	if err := h.p.store.SetRates(c.Request().Context(), id, in); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "recharge_carrier", id, "commission rates")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) createDevice(c echo.Context) error {
	in, err := bind[DeviceInput](c)
	if err != nil {
		return err
	}
	id, err := h.p.store.CreateDevice(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "recharge_device", id, in.Label+" (float "+in.Opening.StringFixed(2)+")")
	return c.JSON(http.StatusCreated, map[string]int64{"id": id})
}

func (h handlers) updateDevice(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[DeviceInput](c)
	if err != nil {
		return err
	}
	if err := h.p.store.UpdateDevice(c.Request().Context(), id, in); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) deviceActive(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[activeInput](c)
	if err != nil {
		return err
	}
	if err := h.p.store.SetDeviceActive(c.Request().Context(), id, in.On); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) createPackage(c echo.Context) error {
	in, err := bind[PackageInput](c)
	if err != nil {
		return err
	}
	p, created, err := h.p.store.CreatePackage(c.Request().Context(), in, 0)
	if err != nil {
		return err
	}
	if !created {
		return apperr.Conflict("that carrier already has " + p.Name)
	}
	return c.JSON(http.StatusCreated, p)
}

func (h handlers) updatePackage(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	in, err := bind[PackageInput](c)
	if err != nil {
		return err
	}
	if err := h.p.store.UpdatePackage(c.Request().Context(), id, in); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) reviewPackage(c echo.Context) error {
	id, err := idParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.p.store.ReviewPackage(c.Request().Context(), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// FloatPage is the admin Float page: balances, what the counts suggest, earnings.
type FloatPage struct {
	Floats      []Float         `json:"floats"`
	Total       decimal.Decimal `json:"total"`
	Suggestions []Suggestion    `json:"suggestions"`
	Today       decimal.Decimal `json:"today"`
	Month       decimal.Decimal `json:"month"`
	NewPackages int             `json:"newPackages"` // added at the till, not reviewed yet
}

func (h handlers) float(c echo.Context) error {
	ctx := c.Request().Context()
	out := FloatPage{}
	var err error
	if out.Floats, err = h.p.store.Floats(ctx, h.p.store.db, false); err != nil {
		return err
	}
	for _, f := range out.Floats {
		out.Total = out.Total.Add(f.Balance)
	}
	if out.Suggestions, err = h.p.store.Suggestions(ctx); err != nil {
		return err
	}
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if out.Today, err = Earnings(ctx, h.p.store.db, day, now.Add(time.Minute)); err != nil {
		return err
	}
	if out.Month, err = Earnings(ctx, h.p.store.db, day.AddDate(0, 0, 1-day.Day()), now.Add(time.Minute)); err != nil {
		return err
	}
	err = h.p.store.db.GetContext(ctx, &out.NewPackages, `SELECT count(*) FROM recharge_packages WHERE is_active AND reviewed_at IS NULL`)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) adminCounts(c echo.Context) error {
	in, err := bind[struct {
		Counts []CountInput `json:"counts"`
	}](c)
	if err != nil {
		return err
	}
	res, err := h.p.store.Count(c.Request().Context(), in.Counts, nil, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, res)
}

// Suggestions: per carrier, the last 30 days' counts against its reloads
// (suggestRate) — offered on the Float page, never applied by itself.
func (s *Store) Suggestions(ctx context.Context) ([]Suggestion, error) {
	var rows []struct {
		CarrierID int64           `db:"carrier_id"`
		Carrier   string          `db:"carrier"`
		Current   decimal.Decimal `db:"current"`
		Reloads   decimal.Decimal `db:"reloads"`
	}
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT c.id AS carrier_id, c.name AS carrier, COALESCE(r.percent, 0) AS current,
		       COALESCE((SELECT SUM(amount) FROM recharge_transactions t WHERE t.carrier_id = c.id AND t.type = 'reload'
		                 AND t.package_id IS NULL AND t.created_at > now() - interval '30 days'), 0) AS reloads
		FROM recharge_carriers c LEFT JOIN recharge_rates r ON r.carrier_id = c.id AND r.kind = 'reload'
		WHERE c.is_active ORDER BY c.name`); err != nil {
		return nil, err
	}
	out := []Suggestion{}
	for _, r := range rows {
		var diffs []decimal.Decimal
		if err := s.db.SelectContext(ctx, &diffs, `SELECT t.float_delta FROM recharge_transactions t
			WHERE t.carrier_id = $1 AND t.type = 'count' AND t.created_at > now() - interval '30 days' ORDER BY t.created_at`, r.CarrierID); err != nil {
			return nil, err
		}
		if next, ok := suggestRate(r.Current, diffs, r.Reloads); ok {
			sum := decimal.Zero
			for _, d := range diffs {
				sum = sum.Add(d)
			}
			out = append(out, Suggestion{CarrierID: r.CarrierID, Carrier: r.Carrier, Current: r.Current, Suggested: next,
				Counts: len(diffs), Extra: sum, Reloads: r.Reloads})
		}
	}
	return out, nil
}
