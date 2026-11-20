package battle

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck"
)

// A clean day of warranty swaps and recoveries breaks no rule; each corruption
// below breaks the rule it names (the auditor sees what it is meant to see).
func TestRecoveryRulesSeeCorruption(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	ctx := context.Background()
	ph, plain := s.prod["serial"], s.prod["plain"]
	s.must(t, sqlxExec(ctx, s.db, fmt.Sprintf(`UPDATE products SET warranty_months = 6 WHERE id = %d`, plain)))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": ph, "newQuantity": "2"}, nil))
	s.open(t, s.till1)

	// A serial phone sold and swapped; three plain items sold and one swapped by receipt.
	s.sell(t, s.till1, "0", []line{{"productId": ph, "quantity": "1", "serials": []string{"IMEI-L1"}}}, cash("15000"))
	var unit int64
	s.must(t, s.db.GetContext(ctx, &unit, `SELECT id FROM warranty_units WHERE serial_no = 'IMEI-L1'`))
	s.must(t, s.till1.Do("POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": unit, "newSerial": "IMEI-L1-B", "reason": "dead"}, nil))
	rs := s.sell(t, s.till1, "0", []line{{"productId": plain, "quantity": "3"}}, cash("300"))
	s.must(t, s.till1.Do("POST", "/api/v1/till/warranty/receipt-replace", map[string]any{"saleId": rs.ID, "productId": plain, "reason": "cracked"}, nil))

	// A supplier's delivery with a bill left open, then three write-offs of it.
	var sup struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/suppliers", map[string]string{"name": "Loss Wholesale"}, &sup))
	s.must(t, s.admin.Do("POST", "/api/v1/admin/purchases/receive", map[string]any{"supplierId": sup.ID, "lines": []map[string]any{
		{"productId": s.prod["odd"], "quantity": "10", "costPrice": "50"}}}, nil))
	damage := func(qty string) int64 {
		s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/damage", map[string]any{"productId": s.prod["odd"], "quantity": qty, "reason": "damage"}, nil))
		var mv int64
		s.must(t, s.db.GetContext(ctx, &mv, `SELECT id FROM stock_movements WHERE product_id = $1 AND type = 'damage' ORDER BY id DESC LIMIT 1`, s.prod["odd"]))
		return mv
	}
	recover := func(body map[string]any) int64 {
		s.must(t, s.admin.Do("POST", "/api/v1/admin/recovery", body, nil))
		var id int64
		s.must(t, s.db.GetContext(ctx, &id, `SELECT MAX(id) FROM loss_recoveries`))
		return id
	}
	repl := recover(map[string]any{"sourceType": "damage", "sourceId": damage("2"), "outcome": "replacement", "supplierId": sup.ID,
		"expiry": "2030-01-01", "extraPerUnit": "5"})
	paid := recover(map[string]any{"sourceType": "damage", "sourceId": damage("1"), "outcome": "paid", "supplierId": sup.ID, "amount": "800"})
	woff := recover(map[string]any{"sourceType": "damage", "sourceId": damage("1"), "outcome": "written_off"})
	recover(map[string]any{"sourceType": "warranty", "sourceId": unit, "outcome": "written_off"})
	s.audit(t)

	var claim, wmove int64
	s.must(t, s.db.GetContext(ctx, &claim, `SELECT id FROM warranty_claims WHERE unit_id = $1`, unit))
	s.must(t, s.db.GetContext(ctx, &wmove, `SELECT id FROM stock_movements WHERE type = 'warranty_replacement' AND reference_id = $1`, claim))
	cases := []struct{ rule, breakIt, mend string }{
		{"REC-1", `UPDATE loss_recoveries SET loss_value = loss_value + 1 WHERE id = WOFF`, `UPDATE loss_recoveries SET loss_value = loss_value - 1 WHERE id = WOFF`},
		{"REC-1", `UPDATE loss_recoveries SET quantity = quantity + 1 WHERE id = WOFF`, `UPDATE loss_recoveries SET quantity = quantity - 1 WHERE id = WOFF`},
		{"REC-5", `UPDATE loss_recoveries SET recovered_value = 1 WHERE id = WOFF`, `UPDATE loss_recoveries SET recovered_value = 0 WHERE id = WOFF`},
		{"REC-5", `UPDATE loss_recoveries SET recovered_value = recovered_value - 1 WHERE id = REPL`, `UPDATE loss_recoveries SET recovered_value = recovered_value + 1 WHERE id = REPL`},
		{"REC-5", `UPDATE loss_recoveries SET supplier_id = NULL WHERE id = PAID`, `UPDATE loss_recoveries SET supplier_id = SUP WHERE id = PAID`},
		{"REC-5", `UPDATE loss_recoveries SET credit_unapplied = recovered_value + 1 WHERE id = PAID`,
			`UPDATE loss_recoveries SET credit_unapplied = UNAPPLIED WHERE id = PAID`},
		{"REC-2", `UPDATE stock_movements SET quantity = quantity + 1 WHERE type = 'recovery' AND reference_id = REPL`,
			`UPDATE stock_movements SET quantity = quantity - 1 WHERE type = 'recovery' AND reference_id = REPL`},
		{"REC-3", `UPDATE loss_recovery_bills SET amount = amount + 10000 WHERE recovery_id = PAID`,
			`UPDATE loss_recovery_bills SET amount = amount - 10000 WHERE recovery_id = PAID`},
		{"REC-3", `UPDATE loss_recoveries SET old_debt = 5 WHERE id = WOFF`, `UPDATE loss_recoveries SET old_debt = NULL WHERE id = WOFF`},
		{"WAR-3", `UPDATE stock_movements SET reference_id = NULL WHERE id = WMOVE`, `UPDATE stock_movements SET reference_id = CLAIM WHERE id = WMOVE`},
		{"WAR-3", `UPDATE stock_movements SET quantity = -2 WHERE id = WMOVE`, `UPDATE stock_movements SET quantity = -1 WHERE id = WMOVE`},
		{"WAR-2", `UPDATE warranty_units SET warranty_until = warranty_until + 1 WHERE serial_no = 'IMEI-L1-B'`,
			`UPDATE warranty_units SET warranty_until = warranty_until - 1 WHERE serial_no = 'IMEI-L1-B'`},
		{"WAR-4", `UPDATE sale_items SET quantity = 0 WHERE sale_id = RSALE`, `UPDATE sale_items SET quantity = 3 WHERE sale_id = RSALE`},
		{"WAR-4", `UPDATE sales SET status = 'void' WHERE id = RSALE`, `UPDATE sales SET status = 'STATUS' WHERE id = RSALE`},
	}
	var status, unapplied string
	s.must(t, s.db.GetContext(ctx, &status, `SELECT status FROM sales WHERE id = $1`, rs.ID))
	s.must(t, s.db.GetContext(ctx, &unapplied, `SELECT credit_unapplied FROM loss_recoveries WHERE id = $1`, paid))
	ids := strings.NewReplacer("WOFF", fmt.Sprint(woff), "REPL", fmt.Sprint(repl), "PAID", fmt.Sprint(paid), "SUP", fmt.Sprint(sup.ID),
		"WMOVE", fmt.Sprint(wmove), "CLAIM", fmt.Sprint(claim), "RSALE", fmt.Sprint(rs.ID), "STATUS", status, "UNAPPLIED", unapplied)
	for _, c := range cases {
		if _, err := s.db.ExecContext(ctx, ids.Replace(c.breakIt)); err != nil {
			t.Fatalf("%s: %v", c.rule, err)
		}
		vs, err := ledgercheck.Run(ctx, s.db)
		if _, merr := s.db.ExecContext(ctx, ids.Replace(c.mend)); merr != nil {
			t.Fatalf("%s mend: %v", c.rule, merr)
		}
		if err != nil {
			t.Fatal(err)
		}
		var rules []string
		for _, v := range vs {
			rules = append(rules, v.Rule)
		}
		if !slices.Contains(rules, c.rule) {
			t.Errorf("%s: corruption not seen (got %v)", c.rule, rules)
		}
	}
	s.audit(t)
}
