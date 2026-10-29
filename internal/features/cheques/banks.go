package cheques

import (
	"context"
	"sort"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
)

// builtInBanks are the Sri Lankan banks offered for a received cheque.
var builtInBanks = []string{
	"Bank of Ceylon", "People's Bank", "Commercial Bank", "Hatton National Bank", "Sampath Bank", "Seylan Bank",
	"Nations Trust Bank", "DFCC Bank", "NDB Bank", "Pan Asia Bank", "Union Bank", "Amana Bank", "Cargills Bank",
	"HSBC", "Standard Chartered", "Regional Development Bank", "National Savings Bank", "Sanasa Development Bank",
}

// Banks is the built-in list plus every bank name used on a cheque before, sorted.
func Banks(ctx context.Context, q appdb.Queryer) ([]string, error) {
	var past []string
	if err := q.SelectContext(ctx, &past, `SELECT DISTINCT bank FROM cheques WHERE bank <> ''`); err != nil {
		return nil, apperr.Internal("failed to list banks", err)
	}
	seen := map[string]bool{}
	out := []string{}
	for _, b := range append(append([]string{}, builtInBanks...), past...) {
		if k := strings.ToLower(strings.TrimSpace(b)); !seen[k] {
			seen[k] = true
			out = append(out, b)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Duplicate reports whether a cheque with this number is already on file: same
// bank for a received one (dir "in"), same bank place for a given one. A warning,
// never a block.
func Duplicate(ctx context.Context, q appdb.Queryer, dir, bank string, lockerID int64, number string) (bool, error) {
	var dup bool
	err := q.GetContext(ctx, &dup, `SELECT EXISTS (SELECT 1 FROM cheques WHERE direction = $1 AND number = $2
		AND CASE WHEN $1 = 'in' THEN lower(bank) = lower($3) ELSE locker_id = $4 END)`,
		dir, strings.TrimSpace(number), strings.TrimSpace(bank), lockerID)
	if err != nil {
		return false, apperr.Internal("failed to check cheque number", err)
	}
	return dup, nil
}
