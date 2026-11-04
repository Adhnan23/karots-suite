package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"karots-suite/internal/apperr"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/printmachines"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
)

func (h *handlers) registerPrintMachines(v1 *echo.Group) {
	// The helper on a cashier PC: its own token, no person signed in.
	v1.POST("/print-agent/poll", h.PrintAgentPoll)
	v1.POST("/print-agent/done", h.PrintAgentDone)
	g := v1.Group("/admin/print-machines", append(append([]echo.MiddlewareFunc{}, h.protected...), middleware.RequireRole(auth.RoleAdmin, auth.RoleManager))...)
	g.GET("", h.AdminPrintMachines)
	g.PUT("/:id", h.AdminPrintMachineSave)
	g.DELETE("/:id", h.AdminPrintMachineRemove)
	g.POST("/:id/test", h.AdminPrintMachineTest)
}

// socketIP is the caller's real address (a header can't move a job to another PC).
func socketIP(c echo.Context) string {
	host, _, err := net.SplitHostPort(c.Request().RemoteAddr)
	if err != nil {
		return c.Request().RemoteAddr
	}
	return host
}

// machinePrinters points this request at the printers of the machine it came
// from. Settings is left out: that page shows and saves the shop's own printers.
func (h *handlers) machinePrinters(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if strings.HasPrefix(c.Path(), "/api/v1/admin/settings") {
			return next(c)
		}
		ctx := c.Request().Context()
		if receipt, label := h.machines.Targets(ctx, socketIP(c)); receipt != "" || label != "" {
			c.SetRequest(c.Request().WithContext(settings.WithMachinePrinters(ctx, receipt, label)))
		}
		return next(c)
	}
}

func (h *handlers) PrintAgentPoll(c echo.Context) error {
	var req PrintAgentPollRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	approved, jobs, err := h.machines.Poll(c.Request().Context(), c.Request().Header.Get(printAgentHeader), socketIP(c), req.Name, req.Printers)
	if err != nil {
		return err
	}
	out := PrintAgentPollReply{Approved: approved, Jobs: []PrintAgentJob{}}
	for _, j := range jobs {
		out.Jobs = append(out.Jobs, PrintAgentJob{ID: j.ID, Queue: j.Queue, Data: j.Data})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) PrintAgentDone(c echo.Context) error {
	var req PrintAgentDoneRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.machines.Done(c.Request().Context(), c.Request().Header.Get(printAgentHeader), req.ID, req.Error); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// printAgentHeader carries a helper's token.
const printAgentHeader = "X-Ks-Print-Agent"

func (h *handlers) AdminPrintMachines(c echo.Context) error {
	rows, err := h.machines.List(c.Request().Context())
	if err != nil {
		return err
	}
	now := time.Now()
	out := make([]PrintMachine, 0, len(rows))
	for _, m := range rows {
		printers := []string{}
		json.Unmarshal(m.Printers, &printers) //nolint:errcheck // written by Poll
		out = append(out, PrintMachine{ID: m.ID, Name: m.Name, Address: m.IP, Printers: printers, Approved: m.Approved,
			Online: m.Online(now), ReceiptPrinter: m.ReceiptPrinter, LabelPrinter: m.LabelPrinter, LastSeen: m.LastSeen.Format(time.RFC3339)})
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) AdminPrintMachineSave(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req PrintMachineRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.machines.Save(c.Request().Context(), id, strings.TrimSpace(req.ReceiptPrinter), strings.TrimSpace(req.LabelPrinter)); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "settings", "", "set a machine's printers")
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) AdminPrintMachineRemove(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.machines.Remove(c.Request().Context(), id); err != nil {
		return err
	}
	h.audit(c, audit.ActionSettings, "settings", "", "removed a printing machine")
	return c.NoContent(http.StatusNoContent)
}

// AdminPrintMachineTest prints a test slip on a machine's receipt printer, as the
// shop's own test does: in Image mode the page sends the picture, and the drawer opens.
func (h *handlers) AdminPrintMachineTest(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	rows, err := h.machines.List(ctx)
	if err != nil {
		return err
	}
	for _, m := range rows {
		if m.ID != id {
			continue
		}
		if !m.Approved || m.ReceiptPrinter == "" {
			return apperr.Validation("pick and save this machine's receipt printer first")
		}
		s, err := h.d.Settings.Get(ctx)
		if err != nil {
			return err
		}
		doc := escpos.TestDocument(*s)
		if job, ok, err := slipImage(c, s); err != nil {
			return err
		} else if ok {
			doc = job
		}
		s.ReceiptPrinter = printmachines.Target(m.ID, m.ReceiptPrinter)
		if err := escpos.Send(ctx, s.ReceiptPrinter, doc); err != nil {
			return apperr.Validation("the test slip didn't print: " + err.Error())
		}
		escpos.KickDrawer(ctx, *s)
		return c.NoContent(http.StatusNoContent)
	}
	return apperr.NotFound("machine")
}
