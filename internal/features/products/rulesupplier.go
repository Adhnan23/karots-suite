package products

import (
	"context"

	"karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// RuleSupplierOf is the supplier a plugin's rule gives a product, and why
// ("Brand: Bosch").
type RuleSupplierOf struct {
	SupplierID int64
	Why        string
}

// RuleSupplierFor and RuleSupplierProducts are a plugin's supplier rules (SUP-12),
// set by the web layer from plugin.RuleSuppliers; nil = no rules. For answers for
// products with no own supplier; Products lists the products a supplier gets by
// rule (own suppliers are applied here, not by the plugin).
var (
	RuleSupplierFor      func(ctx context.Context, productIDs []int64) (map[int64]RuleSupplierOf, error)
	RuleSupplierProducts func(ctx context.Context, supplierID int64) ([]int64, error)
)

// SupplierMoveHook, when set, moves plugins' rows for a supplier along when its
// products move to another (SUP-11), inside the move's transaction.
var SupplierMoveHook func(ctx context.Context, tx *sqlx.Tx, fromID, toID int64) error

// ruleProductIDs is the products supplierID gets by rule; none when there are no
// rules, no supplier filter, or the supplier is disabled (it counts as none).
func ruleProductIDs(ctx context.Context, q db.Queryer, supplierID *int64) ([]int64, error) {
	if RuleSupplierProducts == nil || supplierID == nil {
		return nil, nil
	}
	var active bool
	if err := q.GetContext(ctx, &active, `SELECT COALESCE((SELECT is_active FROM suppliers WHERE id = $1), false)`, *supplierID); err != nil || !active {
		return nil, err
	}
	return RuleSupplierProducts(ctx, *supplierID)
}

// RuleSuppliersFor is the rule supplier of each of ids that has no own supplier
// (active suppliers only, names filled). nil map when there are no rules.
func RuleSuppliersFor(ctx context.Context, q db.Queryer, ids []int64) (map[int64]RuledSupplier, error) {
	if RuleSupplierFor == nil || len(ids) == 0 {
		return nil, nil
	}
	var bare []int64
	if err := q.SelectContext(ctx, &bare, `SELECT id FROM products WHERE id = ANY($1) AND preferred_supplier_id IS NULL`, pq.Array(ids)); err != nil || len(bare) == 0 {
		return nil, err
	}
	rules, err := RuleSupplierFor(ctx, bare)
	if err != nil || len(rules) == 0 {
		return nil, err
	}
	sups := make([]int64, 0, len(rules))
	for _, r := range rules {
		sups = append(sups, r.SupplierID)
	}
	var names []struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	if err := q.SelectContext(ctx, &names, `SELECT id, name FROM suppliers WHERE id = ANY($1) AND is_active`, pq.Array(sups)); err != nil {
		return nil, err
	}
	name := map[int64]string{}
	for _, n := range names {
		name[n.ID] = n.Name
	}
	out := map[int64]RuledSupplier{}
	for id, r := range rules {
		if n, ok := name[r.SupplierID]; ok {
			out[id] = RuledSupplier{ID: r.SupplierID, Name: n, Why: r.Why}
		}
	}
	return out, nil
}

// RuledSupplier is a product's rule supplier, resolved.
type RuledSupplier struct {
	ID        int64
	Name, Why string
}

// FillRuleSuppliers sets the rule supplier on rows with no own supplier; a rule
// naming a missing or disabled supplier gives none.
func (s *Service) FillRuleSuppliers(ctx context.Context, rows []Product) error {
	ids := make([]int64, 0, len(rows))
	for _, p := range rows {
		if p.PreferredSupplierID == nil {
			ids = append(ids, p.ID)
		}
	}
	ruled, err := RuleSuppliersFor(ctx, s.db, ids)
	if err != nil {
		return err
	}
	for i := range rows {
		if r, ok := ruled[rows[i].ID]; ok && rows[i].PreferredSupplierID == nil {
			id := r.ID
			rows[i].RuleSupplierID, rows[i].RuleSupplierName, rows[i].RuleSupplierWhy = &id, r.Name, r.Why
		}
	}
	return nil
}

// CountByRuleSupplier counts the products supplierID gets by rule (no own
// supplier, not merged away) — the supplier page's "by rule" count.
func (s *Service) CountByRuleSupplier(ctx context.Context, supplierID int64) (int, error) {
	ids, err := ruleProductIDs(ctx, s.db, &supplierID)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	var n int
	err = s.db.GetContext(ctx, &n, `SELECT count(*) FROM products WHERE id = ANY($1) AND preferred_supplier_id IS NULL AND merged_into_id IS NULL`, pq.Array(ids))
	return n, err
}
