package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/datetime"
	"karots-suite/internal/features/activity"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

const activityPage = 100

func (h *handlers) registerAdminActivity(g *echo.Group) {
	a := g.Group("/activity", middleware.RequireRole(auth.RoleAdmin))
	a.GET("", h.AdminActivity)
	a.GET("/filters", h.AdminActivityFilters)
}

// activityFilter reads Activity's filters: range (nothing = this month),
// ?user=, ?source=, ?q=, and the ?before= cursor ("when|source|id").
func activityFilter(c echo.Context) (activity.Filter, error) {
	from, to, _, _, err := reportRange(c)
	if err != nil {
		return activity.Filter{}, err
	}
	f := activity.Filter{From: &from, To: &to, Source: strings.TrimSpace(c.QueryParam("source")), Query: strings.TrimSpace(c.QueryParam("q"))}
	if id, err := strconv.ParseInt(c.QueryParam("user"), 10, 64); err == nil && id > 0 {
		f.UserID = &id
	}
	if parts := strings.SplitN(c.QueryParam("before"), "|", 3); len(parts) == 3 {
		at, terr := time.Parse(time.RFC3339Nano, parts[0])
		id, ierr := strconv.ParseInt(parts[2], 10, 64)
		if terr == nil && ierr == nil {
			f.Before = &activity.Cursor{At: at, Source: parts[1], ID: id}
		}
	}
	return f, nil
}

// activityRows is the core trails plus every plugin contributor for f, in the
// view's order, f.Limit of them (0 = all). A contributor that errors is
// skipped, never failing the page.
func (h *handlers) activityRows(c echo.Context, f activity.Filter) ([]activity.Row, error) {
	ctx := c.Request().Context()
	rows, err := activity.NewService(h.d.DB).List(ctx, f)
	if err != nil {
		return nil, err
	}
	for _, cb := range plugin.ActivityContributors() {
		if f.Source != "" && f.Source != "all" && f.Source != cb.Source {
			continue
		}
		pr, err := cb.List(ctx, f)
		if err != nil {
			continue
		}
		for _, r := range pr {
			if f.Before == nil || f.Before.After(r) {
				rows = append(rows, r)
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return activity.Less(rows[i], rows[j]) })
	if f.Limit > 0 && len(rows) > f.Limit {
		rows = rows[:f.Limit]
	}
	return rows, nil
}

func activityAmount(r activity.Row) string {
	if r.Amount.IsZero() {
		return ""
	}
	return dec2(r.Amount)
}

// AdminActivity is the unified who-did-what view:
// audit, stock, cash drawer and lockers, plus plugin trails; never the system
// user. Pages 100 by cursor; ?format= downloads every match. Admins only.
func (h *handlers) AdminActivity(c echo.Context) error {
	f, err := activityFilter(c)
	if err != nil {
		return err
	}
	if wantsSheet(c) {
		f.Before = nil
		rows, err := h.activityRows(c, f)
		if err != nil {
			return err
		}
		sheet := make([][]string, 0, len(rows))
		for _, r := range rows {
			sheet = append(sheet, []string{datetime.DateTime(r.When), r.UserName, r.Source, r.Action, r.Detail, activityAmount(r)})
		}
		return writeSheet(c, "activity_"+datetime.Date(time.Now()), []string{"When", "Who", "Source", "Action", "Detail", "Amount"}, sheet)
	}
	f.Limit = activityPage
	rows, err := h.activityRows(c, f)
	if err != nil {
		return err
	}
	out := ActivityPage{Rows: make([]ActivityRow, 0, len(rows))}
	for _, r := range rows {
		out.Rows = append(out.Rows, ActivityRow{When: r.When.Format(time.RFC3339), User: r.UserName, Source: r.Source, Action: r.Action, Detail: r.Detail, Amount: activityAmount(r)})
	}
	if len(rows) == activityPage {
		last := rows[len(rows)-1]
		out.Next = last.When.Format(time.RFC3339Nano) + "|" + last.Source + "|" + strconv.FormatInt(last.ID, 10)
	}
	return c.JSON(http.StatusOK, out)
}

// AdminActivityFilters is the Who and Source choices.
func (h *handlers) AdminActivityFilters(c echo.Context) error {
	users, err := activity.NewService(h.d.DB).Users(c.Request().Context())
	if err != nil {
		return err
	}
	out := ActivityFilters{Users: make([]CashierOption, 0, len(users)), Sources: []ActivityOption{
		{ID: activity.SourceAudit, Label: "Changes"}, {ID: activity.SourceStock, Label: "Stock"},
		{ID: activity.SourceCash, Label: "Cash drawer"}, {ID: activity.SourceLocker, Label: "Lockers"},
	}}
	for _, u := range users {
		out.Users = append(out.Users, CashierOption{ID: u.ID, Name: u.Name})
	}
	for _, cb := range plugin.ActivityContributors() {
		out.Sources = append(out.Sources, ActivityOption{ID: cb.Source, Label: strings.ToUpper(cb.Source[:1]) + cb.Source[1:]})
	}
	return c.JSON(http.StatusOK, out)
}
