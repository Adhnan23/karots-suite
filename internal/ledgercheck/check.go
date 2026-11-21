// Package ledgercheck is the auditor: it reads the database and reports every
// row that breaks a rule of the rule book (docs/rules). Read-only — it only
// SELECTs — so it can run after any test, in the Demo Room, or at a shop.
package ledgercheck

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Violation is one row breaking one rule.
type Violation struct {
	Rule   string // rule id, e.g. "RET-4"
	Ref    string // what broke it, e.g. "sale 12"
	Detail string // the numbers that disagree
}

// check is one rule's query: it returns (ref, detail) for every row that
// breaks the rule, and nothing when the data keeps it.
type check struct {
	rule string
	sql  string
}

var checks []check

// Add registers a check from outside core — a plugin's own rules (docs/rules/plugins),
// from its init(). sql returns (ref, detail) text for every row breaking rule.
func Add(rule, sql string) { checks = append(checks, check{rule, sql}) }

// Run runs every check and returns all violations, in rule order.
func Run(ctx context.Context, q sqlx.QueryerContext) ([]Violation, error) {
	var out []Violation
	for _, c := range checks {
		rows, err := q.QueryxContext(ctx, c.sql)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.rule, err)
		}
		for rows.Next() {
			v := Violation{Rule: c.rule}
			if err := rows.Scan(&v.Ref, &v.Detail); err != nil {
				rows.Close()
				return nil, fmt.Errorf("%s: %w", c.rule, err)
			}
			out = append(out, v)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.rule, err)
		}
	}
	return out, nil
}
