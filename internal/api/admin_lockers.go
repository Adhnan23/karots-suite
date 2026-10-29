package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerAdminLockers(g *echo.Group) {
	g.GET("/lockers", h.AdminLockers)
	g.POST("/lockers", h.AdminLockerCreate)
	g.GET("/lockers/:id", h.AdminLocker)
	g.PUT("/lockers/:id", h.AdminLockerUpdate)
	g.DELETE("/lockers/:id", h.AdminLockerArchive)
	g.POST("/lockers/:id/activate", h.AdminLockerReactivate)
	g.GET("/lockers/:id/ledger", h.AdminLockerLedger)
	g.PUT("/settings/till-cash", h.AdminTillCash)
}

func adminLocker(l lockers.Locker, defaultID int64) AdminLocker {
	return AdminLocker{ID: l.ID, Name: l.Name, Kind: l.Kind, Balance: dec2(l.Balance), Overdraft: l.AllowNegative,
		CashierAccess: l.CashierAccess, Active: l.IsActive, TillDefault: l.ID == defaultID}
}

// tillCash reads the till's cash-move settings (0 = no default locker).
func (h *handlers) tillCash(c echo.Context) (TillCashRequest, error) {
	cfg, err := h.d.Settings.Get(c.Request().Context())
	if err != nil {
		return TillCashRequest{}, err
	}
	out := TillCashRequest{AllowUntracked: cfg.AllowUntrackedCash}
	if cfg.DefaultLockerID != nil {
		out.DefaultLockerID = *cfg.DefaultLockerID
	}
	return out, nil
}

// AdminLockers lists lockers with live balances;
// disabled=1 adds the archived ones. Total covers the active lockers only.
func (h *handlers) AdminLockers(c echo.Context) error {
	ls, err := h.t.lockers.List(c.Request().Context(), c.QueryParam("disabled") != "1")
	if err != nil {
		return err
	}
	tc, err := h.tillCash(c)
	if err != nil {
		return err
	}
	given, err := h.cheques().AfterCheques(c.Request().Context())
	if err != nil {
		return err
	}
	out := AdminLockerList{Items: make([]AdminLocker, 0, len(ls)), TillCash: tc}
	total := decimal.Zero
	for _, l := range ls {
		if l.IsActive && l.Kind != lockers.KindCheque { // uncleared cheques aren't money yet
			total = total.Add(l.Balance)
		}
		row := adminLocker(l, tc.DefaultLockerID)
		if l.Kind == lockers.KindBank {
			row.AfterCheques = dec2(l.Balance.Sub(given[l.ID]))
		}
		out.Items = append(out.Items, row)
	}
	out.Total = dec2(total)
	return c.JSON(http.StatusOK, out)
}

func bindLocker(c echo.Context) (LockerRequest, error) {
	var in LockerRequest
	if err := c.Bind(&in); err != nil {
		return in, apperr.BadRequest("invalid locker")
	}
	in.OpeningBalance = strings.NewReplacer(",", "", " ", "").Replace(in.OpeningBalance)
	return in, nil
}

// AdminLockerCreate adds a locker; a non-zero opening balance becomes its first
// ledger row (open_balance) in the same transaction.
func (h *handlers) AdminLockerCreate(c echo.Context) error {
	in, err := bindLocker(c)
	if err != nil {
		return err
	}
	l, err := h.t.lockers.Create(c.Request().Context(), lockers.CreateInput{Name: in.Name, Kind: in.Kind,
		AllowNegative: onOff(in.Overdraft), OpeningBalance: in.OpeningBalance, CashierAccess: onOff(in.CashierAccess)}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "locker", strconv.FormatInt(l.ID, 10), "created locker "+l.Name)
	return c.JSON(http.StatusCreated, adminLocker(*l, 0))
}

// AdminLockerUpdate edits name, kind, overdraft and cashier access. The balance
// only ever changes through money moves; archive has its own route.
func (h *handlers) AdminLockerUpdate(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	in, err := bindLocker(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	cur, err := h.t.lockers.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := h.t.lockers.Update(ctx, id, lockers.UpdateInput{Name: in.Name, Kind: in.Kind, AllowNegative: onOff(in.Overdraft),
		IsActive: onOff(cur.IsActive), CashierAccess: onOff(in.CashierAccess)}); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "locker", strconv.FormatInt(id, 10), "updated locker "+strings.TrimSpace(in.Name))
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) setLockerActive(c echo.Context, active bool) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.lockers.SetActive(c.Request().Context(), id, active); err != nil {
		return err
	}
	detail := "archived locker"
	if active {
		detail = "reactivated locker"
	}
	h.audit(c, audit.ActionUpdate, "locker", strconv.FormatInt(id, 10), detail)
	return c.NoContent(http.StatusNoContent)
}

// AdminLockerArchive takes a locker out of the pickers and the total; its
// balance and history stay.
func (h *handlers) AdminLockerArchive(c echo.Context) error    { return h.setLockerActive(c, false) }
func (h *handlers) AdminLockerReactivate(c echo.Context) error { return h.setLockerActive(c, true) }

// AdminLocker is a ledger page's header: the locker and its net change over
// the whole range (SQL, not the loaded page).
func (h *handlers) AdminLocker(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	l, err := h.t.lockers.Get(ctx, id)
	if err != nil {
		return err
	}
	net, err := h.t.lockers.NetChange(ctx, id, from, to)
	if err != nil {
		return err
	}
	tc, err := h.tillCash(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, AdminLockerView{Locker: adminLocker(*l, tc.DefaultLockerID), Net: dec2(net)})
}

// AdminLockerLedger pages one locker's movements, newest first, each with the
// CR- receipt its move wrote.
func (h *handlers) AdminLockerLedger(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	from, to, err := listRange(c)
	if err != nil {
		return err
	}
	rows, err := h.t.lockers.Ledger(c.Request().Context(), id, lockers.LedgerFilter{From: from, To: to, Limit: counterPage, Offset: listOffset(c)})
	if err != nil {
		return err
	}
	out := make([]LockerLedgerRow, 0, len(rows))
	for _, e := range rows {
		r := LockerLedgerRow{ID: e.ID, At: e.CreatedAt.Format(time.RFC3339), Kind: e.Kind, Note: e.Note, Delta: dec2(e.BalanceDelta), ReceiptNo: strOr(e.ReceiptNo)}
		if e.ReceiptID != nil {
			r.ReceiptID = *e.ReceiptID
		}
		out = append(out, r)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminTillCash sets the till's default locker (0 = none) and whether cash may
// leave the drawer untracked — only those two settings.
func (h *handlers) AdminTillCash(c echo.Context) error {
	var in TillCashRequest
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid settings")
	}
	ctx := c.Request().Context()
	var def *int64
	if in.DefaultLockerID > 0 {
		l, err := h.t.lockers.Get(ctx, in.DefaultLockerID)
		if err != nil {
			return err
		}
		if !l.IsActive {
			return apperr.Validation("that locker is archived")
		}
		if !lockers.HoldsCash(l.Kind) {
			return apperr.Validation("the till's default locker must hold cash — " + l.Name + " is a bank or card account")
		}
		def = &in.DefaultLockerID
	}
	if err := h.d.Settings.SetTillCash(ctx, def, in.AllowUntracked); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "settings", "till-cash", "till cash settings")
	return c.NoContent(http.StatusNoContent)
}
