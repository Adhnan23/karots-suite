package backup

import (
	"context"

	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// AllTables lists the public tables a dump would consider (goose bookkeeping
// excluded), for callers that need to compute buckets before dumping.
func AllTables(ctx context.Context, db *sqlx.DB) ([]string, error) {
	return publicTables(ctx, db)
}

// setupKeepCore is the FK-closed set of CORE tables kept by a "catalogue & setup
// only" export — catalogue, configuration and people, no transaction history.
// It is FK-closed: every table here references only other tables here (or
// nothing), so restoring it into a fresh DB never dangles a foreign key. New
// transactional tables default to EXCLUDED (safe: a seed shouldn't carry them).
var setupKeepCore = map[string]bool{
	"categories": true, "units": true, "products": true,
	"suppliers": true, "customers": true, "users": true,
	"settings": true, "denominations": true, "lockers": true,
	"product_groups": true, "product_group_items": true,
	"product_recipes": true, "product_recipe_costs": true,
}

func groupTableSet(groups []plugin.DataGroup) map[string]bool {
	s := map[string]bool{}
	for _, g := range groups {
		for _, t := range g.Tables {
			s[t] = true
		}
	}
	return s
}

// CoreTables returns the tables owned by core rather than any plugin — every
// public table minus goose bookkeeping minus every plugin group's tables. Order
// follows `all`.
func CoreTables(all []string, groups []plugin.DataGroup) []string {
	owned := groupTableSet(groups)
	out := make([]string, 0, len(all))
	for _, t := range all {
		if skipTable(t) || owned[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

// ExcludeFor returns the table names to exclude from a dump because their plugin
// group was not kept. keepKeys are the group keys the user ticked; every group
// whose key is not kept contributes its tables to the exclusion.
func ExcludeFor(groups []plugin.DataGroup, keepKeys []string) []string {
	keep := map[string]bool{}
	for _, k := range keepKeys {
		keep[k] = true
	}
	var out []string
	for _, g := range groups {
		if !keep[g.Key] {
			out = append(out, g.Tables...)
		}
	}
	return out
}

// SetupZeroColumns are money columns zeroed on a "catalogue & setup only"
// export: customers/suppliers come along as contacts, but their running/opening
// balances would otherwise be phantom dues with no transactions behind them in
// a freshly-seeded shop, so they're reset to 0.
var SetupZeroColumns = map[string][]string{
	"customers": {"outstanding_balance", "opening_balance", "opening_unlinked"},
	"suppliers": {"outstanding_balance", "opening_balance", "opening_unlinked"},
}

// zeroColumns rewrites the named columns of every row to "0" in place. Used to
// strip carried-over balances from a setup-only export.
func zeroColumns(cols []string, rows [][]*string, zeroCols []string) {
	if len(zeroCols) == 0 {
		return
	}
	want := map[string]bool{}
	for _, z := range zeroCols {
		want[z] = true
	}
	var idx []int
	for i, c := range cols {
		if want[c] {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return
	}
	zero := "0"
	for _, row := range rows {
		for _, i := range idx {
			if i < len(row) {
				row[i] = &zero
			}
		}
	}
}

// SetupOnlyExclude returns the core tables to exclude for a "catalogue & setup
// only" export: every core table not in the keep set (i.e. transaction history).
func SetupOnlyExclude(core []string) []string {
	var out []string
	for _, t := range core {
		if !setupKeepCore[t] {
			out = append(out, t)
		}
	}
	return out
}
