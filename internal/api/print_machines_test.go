package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"karots-suite/internal/features/printmachines"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/printing"
)

func (te *testEnv) raw(method, path string, body any) (*http.Request, *httptest.ResponseRecorder) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req, httptest.NewRecorder()
}

// A cashier PC's helper asks, is approved, and a job from that PC's browser reaches it.
func TestPrintMachineFollowsTheMachine(t *testing.T) {
	te := newTestEnv(t)
	boss := te.login(t, devPhone(), "admin")
	token := strings.Repeat("a", 24) + fmt.Sprintf("%08d", time.Now().UnixNano()%100_000_000)
	t.Cleanup(func() { te.db.MustExec(`DELETE FROM print_machines`) })

	poll := func() PrintAgentPollReply {
		req, rec := te.raw(http.MethodPost, "/api/v1/print-agent/poll", PrintAgentPollRequest{Name: "Laptop", Printers: []string{"XP80"}})
		req.Header.Set(printAgentHeader, token)
		te.e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("poll %d %s", rec.Code, rec.Body)
		}
		return decode[PrintAgentPollReply](t, rec)
	}
	if r := poll(); r.Approved {
		t.Fatal("approved by itself")
	}
	rows := decode[[]PrintMachine](t, te.do(http.MethodGet, "/api/v1/admin/print-machines", nil, boss))
	if len(rows) != 1 || rows[0].Name != "Laptop" || rows[0].Approved || !rows[0].Online || rows[0].Printers[0] != "XP80" {
		t.Fatalf("machines %+v", rows)
	}
	path := fmt.Sprintf("/api/v1/admin/print-machines/%d", rows[0].ID)
	if rec := te.do(http.MethodPut, path, PrintMachineRequest{ReceiptPrinter: "XP80"}, boss); rec.Code != http.StatusNoContent {
		t.Fatalf("save %d %s", rec.Code, rec.Body)
	}

	// That machine's browser prints on it; another PC's, and Settings, don't.
	seen := func(path, addr string) bool {
		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = addr
		c := e.NewContext(req, httptest.NewRecorder())
		c.SetPath(path)
		on := false
		h := &handlers{machines: printmachines.NewService(te.db)}
		h.machinePrinters(func(c echo.Context) error {
			on = settings.HasMachineReceiptPrinter(c.Request().Context())
			return nil
		})(c) //nolint:errcheck
		return on
	}
	if !seen("/api/v1/till/labels", "192.0.2.1:5") || seen("/api/v1/till/labels", "192.0.2.9:5") || seen("/api/v1/admin/settings", "192.0.2.1:5") {
		t.Fatal("the job doesn't follow the machine")
	}

	// The helper takes one job and answers it.
	got := make(chan PrintAgentJob, 1)
	go func() {
		r := poll()
		if len(r.Jobs) != 1 {
			t.Errorf("jobs %+v", r.Jobs)
			close(got)
			return
		}
		got <- r.Jobs[0]
		req, rec := te.raw(http.MethodPost, "/api/v1/print-agent/done", PrintAgentDoneRequest{ID: r.Jobs[0].ID})
		req.Header.Set(printAgentHeader, token)
		te.e.ServeHTTP(rec, req)
	}()
	time.Sleep(200 * time.Millisecond)
	if err := printing.Raw(t.Context(), fmt.Sprintf("machine://%d/XP80", rows[0].ID), []byte("slip")); err != nil {
		t.Fatal(err)
	}
	if j := <-got; j.Queue != "XP80" || string(j.Data) != "slip" {
		t.Fatalf("job %+v", j)
	}

	// Off: it fails at once, the way a printer that is off does.
	te.db.MustExec(`UPDATE print_machines SET last_seen = now() - interval '5 minutes'`)
	if err := printing.Raw(t.Context(), fmt.Sprintf("machine://%d/XP80", rows[0].ID), []byte("slip")); err == nil || !strings.Contains(err.Error(), "isn't connected") {
		t.Fatalf("offline: %v", err)
	}
}
