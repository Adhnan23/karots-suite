package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// Losses & Recovery: a damage write-off shows in its range (not outside it),
// a supplier credit against it lowers what we owe them and counts as
// recovered, and the same loss can't be recovered twice.
func TestLossesAndRecovery(t *testing.T) {
	te := newTestEnv(t)
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	var cat, unit, user, prod, sup, mv int64
	te.db.Get(&cat, `SELECT id FROM categories LIMIT 1`)                                                                                                            //nolint:errcheck
	te.db.Get(&unit, `SELECT id FROM units LIMIT 1`)                                                                                                                //nolint:errcheck
	te.db.Get(&user, `SELECT id FROM users ORDER BY id LIMIT 1`)                                                                                                    //nolint:errcheck
	te.db.Get(&prod, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price) VALUES ('T ro kettle', $1, $2, 40, 60) RETURNING id`, cat, unit) //nolint:errcheck
	te.db.Get(&sup, `INSERT INTO suppliers (name, outstanding_balance) VALUES ('T ro supplier', 500) RETURNING id`)                                                 //nolint:errcheck
	te.db.Get(&mv, `INSERT INTO stock_movements (product_id, type, quantity, user_id, note, cost, created_at)
		VALUES ($1, 'damage', -2, $2, 'dropped', 80, '2096-01-10T10:00:00+05:30') RETURNING id`, prod, user) //nolint:errcheck
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM loss_recoveries WHERE source_type = 'damage' AND source_id = $1`, mv) //nolint:errcheck
		te.db.Exec(`DELETE FROM stock_movements WHERE id = $1`, mv)                                   //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE id = $1`, prod)                                        //nolint:errcheck
		te.db.Exec(`DELETE FROM suppliers WHERE id = $1`, sup)                                        //nolint:errcheck
	})
	q := "from=2096-01-01&to=2096-01-31"
	lines := decode[[]LossRow](t, te.do(http.MethodGet, "/api/v1/admin/reports/losses/lines?"+q, nil, admin))
	if len(lines) != 1 || lines[0].SourceID != mv || lines[0].Loss != "80.00" || lines[0].Outcome != "" {
		t.Fatalf("lines %+v", lines)
	}
	if other := decode[[]LossRow](t, te.do(http.MethodGet, "/api/v1/admin/reports/losses/lines?from=2096-02-01&to=2096-02-28", nil, admin)); len(other) != 0 {
		t.Fatalf("a January loss showed in February: %+v", other)
	}
	rec := func(outcome string) int {
		return te.do(http.MethodPost, "/api/v1/admin/recovery", RecoveryRequest{SourceType: "damage", SourceID: mv, Outcome: outcome, SupplierID: sup, Amount: "30"}, admin).Code
	}
	if c := rec("bogus"); c != http.StatusUnprocessableEntity {
		t.Fatalf("bad outcome: %d", c)
	}
	if c := rec("paid"); c != http.StatusNoContent {
		t.Fatalf("recover: %d", c)
	}
	var owed string
	te.db.Get(&owed, `SELECT outstanding_balance::text FROM suppliers WHERE id = $1`, sup) //nolint:errcheck
	// The loss row carries its recovery; the summary counts a recovery on the
	// day it was booked (as the P&L's "recovered from suppliers" line does).
	sum := decode[LossesReport](t, te.do(http.MethodGet, "/api/v1/admin/reports/losses?"+q, nil, admin))
	after := decode[[]LossRow](t, te.do(http.MethodGet, "/api/v1/admin/reports/losses/lines?"+q, nil, admin))
	if owed != "470.00" || sum.DamageCost != "80.00" || after[0].Outcome != "paid" || after[0].Recovered != "30.00" {
		t.Fatalf("after recovery: owed %s, %+v, %+v", owed, sum, after)
	}
	if c := rec("paid"); c != http.StatusConflict {
		t.Fatalf("second recovery: %d", c)
	}
}
