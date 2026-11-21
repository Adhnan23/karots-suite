package battle

import (
	"fmt"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/repairs"
)

// REP-1: an open job's edit and urgent date change it (the date change in the
// audit log); the lists find it by status and by its ticket; its slip prints the
// ticket and, once collected, the pickup sale.
func TestREP1_EditListAndSlip(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	openTill(t, s)
	j := job(t, s, 30)
	base := fmt.Sprintf("/api/v1/x/repairs/jobs/%d", j.ID)
	s.Must(t, s.Cashier.Do("PUT", base, map[string]any{"customerId": *j.CustomerID, "deviceModel": "Galaxy A52", "fault": "battery",
		"promisedDate": "2030-01-10", "warrantyDays": 60}, nil))
	s.Must(t, s.Cashier.Do("POST", base+"/urgent", map[string]any{"urgent": true, "promisedDate": "2030-01-05"}, nil))
	s.Must(t, s.Cashier.Do("POST", base+"/status", map[string]string{"status": "ready"}, nil))
	g := get(t, s, j.ID)
	if g.DeviceModel != "Galaxy A52" || g.WarrantyDays != 60 || !g.Urgent || g.PromisedDate != "2030-01-05" || g.Status != "ready" || g.ReadyAt == "" {
		t.Errorf("REP-1: after edits %+v", g)
	}
	var moved int
	s.Must(t, s.DB.Get(&moved, `SELECT count(*) FROM audit_log WHERE entity = 'repair' AND entity_id = $1 AND detail LIKE 'moved pickup date 2030-01-10 → 2030-01-05%'`, fmt.Sprint(j.ID)))
	if moved != 1 {
		t.Errorf("REP-1: %d audit rows for the moved date, want 1", moved)
	}
	inList := func(q string) bool {
		var rows []repairs.RowDTO
		s.Must(t, s.Cashier.Do("GET", "/api/v1/x/repairs/jobs?"+q, nil, &rows))
		for _, r := range rows {
			if r.ID == j.ID {
				return true
			}
		}
		return false
	}
	if !inList("show=ready") || !inList("show=open") || inList("show=collected") || !inList("q="+g.TicketCode) || !inList("show=all&preset=today") {
		t.Error("REP-1: the lists don't find the ready job where they should")
	}
	labour(t, s, j.ID, "400")
	r := collect(t, s, j.ID)
	var slip struct{ Lines []struct{ Text string } }
	s.Must(t, s.Cashier.Do("POST", base+"/print?preview=1&size=58", nil, &slip))
	var text []string
	for _, l := range slip.Lines {
		text = append(text, l.Text)
	}
	var receipt string
	s.Must(t, s.DB.Get(&receipt, `SELECT receipt_no FROM sales WHERE id = $1`, r.ID))
	if all := strings.Join(text, "\n"); !strings.Contains(all, g.TicketNo) || !strings.Contains(all, receipt) {
		t.Errorf("slip:\n%s\nwant the ticket %s and the sale %s", all, g.TicketNo, receipt)
	}
	var cfg struct{ WarrantyMode string }
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/repairs/config", nil, &cfg))
	var lab struct{ ProductID int64 }
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/repairs/labour", nil, &lab))
	if cfg.WarrantyMode == "" || lab.ProductID == 0 {
		t.Errorf("config %+v, labour %d", cfg, lab.ProductID)
	}
}
