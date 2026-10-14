package repairs

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/reports"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// list: ?show=open|ready|collected|cancelled|all&q=&preset=&from=&to= (drop-off date).
func (h handlers) list(c echo.Context) error {
	f := ListFilter{Show: c.QueryParam("show"), Q: c.QueryParam("q")}
	if f.Show == "" {
		f.Show = "open"
	}
	if p, fr, to := c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to"); p != "" || fr != "" || to != "" {
		from, until, _, _, err := reports.ResolveRange(p, fr, to)
		if err != nil {
			return apperr.Validation(err.Error())
		}
		f.From, f.To = &from, &until
	}
	jobs, err := h.p.store.List(c.Request().Context(), f)
	if err != nil {
		return err
	}
	now := time.Now()
	out := make([]RowDTO, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, rowDTO(j, now))
	}
	return c.JSON(http.StatusOK, out)
}

type ReportRowDTO struct {
	ID          int64  `json:"id"`
	TicketNo    string `json:"ticketNo"`
	Device      string `json:"device"`
	Customer    string `json:"customer"`
	CollectedAt string `json:"collectedAt"`
	Free        bool   `json:"free"` // a warranty re-repair
	Revenue     string `json:"revenue"`
	Cost        string `json:"cost"`
	Repairer    string `json:"repairer"`
	Profit      string `json:"profit"`
}

type ReportDTO struct {
	From     string         `json:"from"`
	To       string         `json:"to"`
	Rows     []ReportRowDTO `json:"rows"`
	Revenue  string         `json:"revenue"`
	Cost     string         `json:"cost"`
	Repairer string         `json:"repairer"`
	Profit   string         `json:"profit"`
}

// report is profit per collected job in the range (default this month).
func (h handlers) report(c echo.Context) error {
	preset := c.QueryParam("preset")
	if preset == "" && c.QueryParam("from") == "" && c.QueryParam("to") == "" {
		preset = "this-month"
	}
	from, to, fromS, toS, err := reports.ResolveRange(preset, c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return apperr.Validation(err.Error())
	}
	ctx := c.Request().Context()
	labour, err := h.p.store.LabourID(ctx)
	if err != nil {
		return err
	}
	rows, err := h.p.store.Report(ctx, from, to, labour)
	if err != nil {
		return err
	}
	out := ReportDTO{From: fromS, To: toS, Rows: []ReportRowDTO{}}
	var rev, cost, rep decimal.Decimal
	for _, r := range rows {
		p := r.Revenue.Sub(r.Cost).Sub(r.Repairer)
		rev, cost, rep = rev.Add(r.Revenue), cost.Add(r.Cost), rep.Add(r.Repairer)
		out.Rows = append(out.Rows, ReportRowDTO{ID: r.ID, TicketNo: r.TicketNo, Device: r.Device, Customer: r.Customer,
			CollectedAt: iso(&r.CollectedAt), Free: r.ReworkOf != nil, Revenue: dec2(r.Revenue), Cost: dec2(r.Cost),
			Repairer: dec2(r.Repairer), Profit: dec2(p)})
	}
	out.Revenue, out.Cost, out.Repairer, out.Profit = dec2(rev), dec2(cost), dec2(rep), dec2(rev.Sub(cost).Sub(rep))
	return c.JSON(http.StatusOK, out)
}
