package conversions

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// Folding the two-bag fixture (bag → 25 loose) opens both bags into the loose
// product with their expiry, gives it a "Bag ×25" pack at the bag's price, moves
// the bag's barcode onto that pack and retires the bag and the conversion.
func TestFoldTwinIntoPack(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	f := bagAndLoose(t, ctx, conn)
	_, err := f.tx.ExecContext(ctx, `UPDATE products SET barcode = 'TEST-FOLD-MAIN' WHERE id = $1`, f.bag)
	must(t, err)
	_, err = f.tx.ExecContext(ctx, `INSERT INTO product_barcodes (product_id, barcode, source) VALUES ($1, 'TEST-FOLD-EXTRA', 'form')`, f.bag)
	must(t, err)
	_, err = f.tx.ExecContext(ctx, `UPDATE products SET unit_id = (SELECT id FROM units WHERE NOT allow_decimal ORDER BY id LIMIT 1) WHERE id = $1`, f.loose)
	must(t, err)

	packID, err := FoldTx(ctx, f.tx, f.convID, "Bag", f.user)
	must(t, err)

	lots, mirror := looseLots(t, ctx, f)
	if len(lots) != 2 || !mirror.Equal(d("50")) || lots[0].Expiry == nil {
		t.Fatalf("loose stock %s in %d lots: %+v", mirror, len(lots), lots)
	}
	var pack struct {
		Size    decimal.Decimal `db:"size"`
		Selling decimal.Decimal `db:"selling_price"`
	}
	must(t, f.tx.GetContext(ctx, &pack, `SELECT size, selling_price FROM product_packs WHERE id = $1 AND product_id = $2`, packID, f.loose))
	if !pack.Size.Equal(d("25")) || !pack.Selling.Equal(d("3000")) {
		t.Fatalf("pack %+v", pack)
	}
	var onPack int
	must(t, f.tx.GetContext(ctx, &onPack, `SELECT count(*) FROM product_barcodes WHERE pack_id = $1 AND product_id = $2 AND barcode IN ('TEST-FOLD-MAIN', 'TEST-FOLD-EXTRA')`, packID, f.loose))
	var bagActive, convActive bool
	must(t, f.tx.GetContext(ctx, &bagActive, `SELECT is_active FROM products WHERE id = $1`, f.bag))
	must(t, f.tx.GetContext(ctx, &convActive, `SELECT is_active FROM product_conversions WHERE id = $1`, f.convID))
	if onPack != 2 || bagActive || convActive {
		t.Fatalf("codes on pack %d, bag active %v, conversion active %v", onPack, bagActive, convActive)
	}
	// A second fold of the same (now off) conversion is refused.
	if _, err := FoldTx(ctx, f.tx, f.convID, "Bag", f.user); err == nil {
		t.Fatal("folded twice")
	}
}
