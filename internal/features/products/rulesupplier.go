package products

import (
	"context"

	"karots-suite/internal/db"

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

// FillRuleSuppliers sets the rule supplier on rows with no own supplier; a rule
// naming a missing or disabled supplier gives none.
func (s *Service) FillRuleSuppliers(ctx context.Context, rows []Product) error {
	if RuleSupplierFor == nil {
		return nil
	}
	var ids []int64
	for _, p := range rows {
		if p.PreferredSupplierID == nil {
			ids = append(ids, p.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	rules, err := RuleSupplierFor(ctx, ids)
	if err != nil || len(rules) == 0 {
		return err
	}
	sups := make([]int64, 0, len(rules))
	for _, r := range rules {
		sups = append(sups, r.SupplierID)
	}
	var names []struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	if err := s.db.SelectContext(ctx, &names, `SELECT id, name FROM suppliers WHERE id = ANY($1) AND is_active`, pq.Array(sups)); err != nil {
		return err
	}
	name := map[int64]string{}
	for _, n := range names {
		name[n.ID] = n.Name
	}
	for i := range rows {
		r, ok := rules[rows[i].ID]
		if !ok || rows[i].PreferredSupplierID != nil || name[r.SupplierID] == "" {
			continue
		}
		id := r.SupplierID
		rows[i].RuleSupplierID, rows[i].RuleSupplierName, rows[i].RuleSupplierWhy = &id, name[id], r.Why
	}
	return nil
}
