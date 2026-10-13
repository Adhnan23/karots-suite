package alternatives

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

// The plugin's own JSON (core's types.gen.ts is core-only); mirrored by hand in
// web/src/plugins/alternatives/types.ts. Stock and money are decimal strings.

type MemberDTO struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Stock     string `json:"stock"`
	Active    bool   `json:"active"`
	Unit      string `json:"unit"`
}

type TierDTO struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	ReorderLevel int         `json:"reorderLevel"`
	Active       bool        `json:"active"`
	Total        string      `json:"total"`
	Low          bool        `json:"low"`
	Members      []MemberDTO `json:"members"`
}

type GroupDTO struct {
	ID       int64     `json:"id"`
	Name     string    `json:"name"`
	Active   bool      `json:"active"`
	Products int       `json:"products"`
	Tiers    []TierDTO `json:"tiers"`
}

type AltDTO struct {
	ProductID      int64  `json:"productId"`
	Name           string `json:"name"`
	Tier           string `json:"tier"`
	Stock          string `json:"stock"`
	SellingPrice   string `json:"sellingPrice"`
	WholesalePrice string `json:"wholesalePrice"`
	// Unit is the alternative's unit (abbreviation); SameUnit says whether it
	// matches the short product's. A different unit can't keep the line's
	// quantity (1 bag ≠ 1 kg), so the till asks how many.
	Unit     string `json:"unit"`
	SameUnit bool   `json:"sameUnit"`
}

type MembershipDTO struct {
	GroupID int64  `json:"groupId"`
	TierID  int64  `json:"tierId"`
	Group   string `json:"group"`
	Tier    string `json:"tier"`
}

type NameRequest struct {
	Name string `json:"name"`
}

type TierRequest struct {
	Name         string `json:"name"`
	ReorderLevel int    `json:"reorderLevel"`
}

type ActiveRequest struct {
	Active bool `json:"active"`
}

type MoveRequest struct {
	Up bool `json:"up"`
}

type MemberRequest struct {
	ProductID int64 `json:"productId"`
	Move      bool  `json:"move"`
}

func groupDTO(g Group) GroupDTO {
	out := GroupDTO{ID: g.ID, Name: g.Name, Active: g.IsActive, Products: g.Products, Tiers: make([]TierDTO, 0, len(g.Tiers))}
	for _, t := range g.Tiers {
		td := TierDTO{ID: t.ID, Name: t.Name, ReorderLevel: t.ReorderLevel, Active: t.IsActive, Total: t.Total.String(), Low: t.Low,
			Members: make([]MemberDTO, 0, len(t.Members))}
		for _, m := range t.Members {
			td.Members = append(td.Members, MemberDTO{ProductID: m.ProductID, Name: m.Name, Stock: m.Stock.String(), Active: m.Active, Unit: m.Unit})
		}
		out.Tiers = append(out.Tiers, td)
	}
	return out
}

type handlers struct{ p *Plugin }

func itoa(n int) string { return strconv.Itoa(n) }

func param(c echo.Context, name string) (int64, error) {
	v, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		return 0, apperr.BadRequest("invalid id")
	}
	return v, nil
}

func bind[T any](c echo.Context) (T, error) {
	var v T
	if err := c.Bind(&v); err != nil {
		return v, apperr.BadRequest("invalid body")
	}
	return v, nil
}

func (h handlers) audit(c echo.Context, action, entity string, id int64, detail string) {
	if h.p.core.Audit != nil {
		h.p.core.Audit.Record(c.Request().Context(), middleware.CurrentUserID(c), action, entity, strconv.FormatInt(id, 10), detail)
	}
}

func (h handlers) list(c echo.Context) error {
	gs, err := h.p.store.Groups(c.Request().Context(), c.QueryParam("disabled") == "1")
	if err != nil {
		return err
	}
	out := make([]GroupDTO, 0, len(gs))
	for _, g := range gs {
		out = append(out, groupDTO(g))
	}
	return c.JSON(http.StatusOK, out)
}

func (h handlers) sendGroup(c echo.Context, id int64) error {
	g, err := h.p.store.Group(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, groupDTO(g))
}

func (h handlers) group(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	return h.sendGroup(c, id)
}

func (h handlers) createGroup(c echo.Context) error {
	req, err := bind[NameRequest](c)
	if err != nil {
		return err
	}
	id, err := h.p.store.CreateGroup(c.Request().Context(), req.Name)
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "alt_group", id, "created group "+strings.TrimSpace(req.Name))
	return h.sendGroup(c, id)
}

func (h handlers) renameGroup(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	req, err := bind[NameRequest](c)
	if err != nil {
		return err
	}
	if err := h.p.store.RenameGroup(c.Request().Context(), id, req.Name); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "alt_group", id, "renamed group to "+strings.TrimSpace(req.Name))
	return h.sendGroup(c, id)
}

func onOff(on bool) string {
	if on {
		return "enabled"
	}
	return "disabled"
}

func (h handlers) groupActive(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	req, err := bind[ActiveRequest](c)
	if err != nil {
		return err
	}
	if err := h.p.store.SetGroupActive(c.Request().Context(), id, req.Active); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "alt_group", id, "group "+onOff(req.Active))
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) deleteGroup(c echo.Context) error {
	id, err := param(c, "id")
	if err != nil {
		return err
	}
	if err := h.p.store.DeleteGroup(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionDelete, "alt_group", id, "deleted group")
	return c.NoContent(http.StatusNoContent)
}

func (h handlers) createTier(c echo.Context) error {
	gid, err := param(c, "id")
	if err != nil {
		return err
	}
	req, err := bind[TierRequest](c)
	if err != nil {
		return err
	}
	tid, err := h.p.store.CreateTier(c.Request().Context(), gid, TierInput(req))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "alt_tier", tid, "added tier "+strings.TrimSpace(req.Name))
	return h.sendGroup(c, gid)
}

// tierEdit runs a change to one tier, audits it and answers with its group.
func (h handlers) tierEdit(c echo.Context, action, detail string, f func(tid int64) error) error {
	tid, err := param(c, "tid")
	if err != nil {
		return err
	}
	t, err := h.p.store.tier(c.Request().Context(), tid)
	if err != nil {
		return err
	}
	if err := f(tid); err != nil {
		return err
	}
	if detail != "" { // "" = f audited its own entity
		h.audit(c, action, "alt_tier", tid, detail)
	}
	if action == audit.ActionDelete {
		return c.NoContent(http.StatusNoContent)
	}
	return h.sendGroup(c, t.GroupID)
}

func (h handlers) updateTier(c echo.Context) error {
	req, err := bind[TierRequest](c)
	if err != nil {
		return err
	}
	return h.tierEdit(c, audit.ActionUpdate, "updated tier "+strings.TrimSpace(req.Name), func(tid int64) error {
		return h.p.store.UpdateTier(c.Request().Context(), tid, TierInput(req))
	})
}

func (h handlers) tierActive(c echo.Context) error {
	req, err := bind[ActiveRequest](c)
	if err != nil {
		return err
	}
	return h.tierEdit(c, audit.ActionUpdate, "tier "+onOff(req.Active), func(tid int64) error {
		return h.p.store.SetTierActive(c.Request().Context(), tid, req.Active)
	})
}

func (h handlers) moveTier(c echo.Context) error {
	req, err := bind[MoveRequest](c)
	if err != nil {
		return err
	}
	return h.tierEdit(c, audit.ActionUpdate, "moved tier", func(tid int64) error {
		return h.p.store.MoveTier(c.Request().Context(), tid, req.Up)
	})
}

func (h handlers) deleteTier(c echo.Context) error {
	return h.tierEdit(c, audit.ActionDelete, "deleted tier", func(tid int64) error {
		return h.p.store.DeleteTier(c.Request().Context(), tid)
	})
}

func (h handlers) addMember(c echo.Context) error {
	req, err := bind[MemberRequest](c)
	if err != nil {
		return err
	}
	var moved bool
	return h.tierEdit(c, audit.ActionUpdate, "", func(tid int64) error {
		moved, err = h.p.store.AddMember(c.Request().Context(), tid, req.ProductID, req.Move)
		if err == nil {
			msg := "product " + strconv.FormatInt(req.ProductID, 10) + " added"
			if moved {
				msg = "product " + strconv.FormatInt(req.ProductID, 10) + " moved here"
			}
			h.audit(c, audit.ActionUpdate, "alt_member", req.ProductID, msg+" (tier "+strconv.FormatInt(tid, 10)+")")
		}
		return err
	})
}

func (h handlers) removeMember(c echo.Context) error {
	pid, err := param(c, "pid")
	if err != nil {
		return err
	}
	if err := h.p.store.RemoveMember(c.Request().Context(), pid); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "alt_member", pid, "removed from group")
	return c.NoContent(http.StatusNoContent)
}

// membership: ?ids=1,2,3 → where each grouped product sits; no ids = all (picker hints).
func (h handlers) membership(c echo.Context) error {
	var ids []int64
	for _, s := range strings.Split(c.QueryParam("ids"), ",") {
		if v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
			ids = append(ids, v)
		}
	}
	ms, err := h.p.store.Memberships(c.Request().Context(), ids)
	if err != nil {
		return err
	}
	out := make(map[string]MembershipDTO, len(ms))
	for id, m := range ms {
		out[strconv.FormatInt(id, 10)] = MembershipDTO{GroupID: m.GroupID, TierID: m.TierID, Group: m.Group, Tier: m.Tier}
	}
	return c.JSON(http.StatusOK, out)
}

// product: the product form's section reads where the product sits.
func (h handlers) product(c echo.Context) error {
	pid, err := param(c, "pid")
	if err != nil {
		return err
	}
	gid, tid, err := h.p.store.TierOf(c.Request().Context(), pid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, MembershipDTO{GroupID: gid, TierID: tid})
}

// till: a product's in-stock alternatives for the swap chip.
func (h handlers) till(c echo.Context) error {
	pid, err := param(c, "pid")
	if err != nil {
		return err
	}
	alts, err := h.p.store.Alternatives(c.Request().Context(), pid, 3)
	if err != nil {
		return err
	}
	out := make([]AltDTO, 0, len(alts))
	for _, a := range alts {
		out = append(out, AltDTO{ProductID: a.ProductID, Name: a.Name, Tier: a.Tier, Stock: a.Stock.String(),
			SellingPrice: a.SellingPrice.StringFixed(2), WholesalePrice: a.WholesalePrice.StringFixed(2),
			Unit: a.Unit, SameUnit: a.SameUnit})
	}
	return c.JSON(http.StatusOK, out)
}
