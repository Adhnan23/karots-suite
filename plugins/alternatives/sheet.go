package alternatives

import (
	"context"

	appdb "karots-suite/internal/db"
)

// The Alternatives sheet (ALT-9…18): every group as one row per member, out to a
// spreadsheet and back in as the whole set, previewed first.

// SheetLine is one row of the sheet. Line is its row number in an imported file
// (the header is 1); an exported line leaves it 0.
type SheetLine struct {
	Line                          int
	Group, Tier, Product, Barcode string
}

// SheetLines is the whole set as the sheet holds it (ALT-9): every group, on or
// off, in page order; its tiers in order; their members by name, a disabled one
// too (ALT-18). An empty tier or group has no line.
func (s *Store) SheetLines(ctx context.Context) ([]SheetLine, error) {
	var out []SheetLine
	err := appdb.On(ctx, s.db).SelectContext(ctx, &out, `
		SELECT g.name AS "group", t.name AS tier, p.name AS product, COALESCE(p.barcode, '') AS barcode
		FROM alt_members m
		JOIN alt_tiers t ON t.id = m.tier_id
		JOIN alt_groups g ON g.id = t.group_id
		JOIN products p ON p.id = m.product_id
		ORDER BY g.sort_order, lower(g.name), g.id, t.sort_order, t.id, lower(p.name), p.id`)
	return out, err
}
