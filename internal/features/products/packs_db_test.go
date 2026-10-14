package products

import (
	"context"
	"testing"

	"karots-suite/internal/apperr"
)

func TestPacksDB(t *testing.T) {
	conn, a, b := barcodeFixture(t)
	ctx := context.Background()
	s := NewService(conn)
	var pcs int64
	must(t, conn.GetContext(ctx, &pcs, `SELECT id FROM units WHERE NOT allow_decimal ORDER BY id LIMIT 1`))
	_, err := conn.ExecContext(ctx, `UPDATE products SET unit_id = $1 WHERE id IN ($2, $3)`, pcs, a, b)
	must(t, err)

	// A product's own main code moves onto the pack (it was really the card's).
	card, err := s.SavePack(ctx, a, PackInput{Name: "Card", Size: "12", SellingPrice: "250", Barcode: "TEST-B-MAIN"})
	must(t, err)
	if card.Size.String() != "12" || card.SellingPrice.String() != "250" || len(card.Barcodes) != 1 {
		t.Fatalf("card = %+v", card)
	}
	var main *string
	must(t, conn.GetContext(ctx, &main, `SELECT barcode FROM products WHERE id = $1`, a))
	if main != nil {
		t.Fatalf("main barcode still %v", *main)
	}
	// The code still finds the product (till scan) and resolves the pack.
	p, err := s.GetByBarcode(ctx, "TEST-B-MAIN")
	must(t, err)
	if p.ID != a {
		t.Fatalf("scan found %d", p.ID)
	}
	pid, kid, err := PackForBarcode(ctx, conn, "TEST-B-MAIN")
	must(t, err)
	if pid != a || kid != card.ID {
		t.Fatalf("pack for barcode = %d/%d", pid, kid)
	}
	// Pack codes are not listed as the product's extras.
	rows, err := s.Barcodes(ctx, a)
	must(t, err)
	for _, r := range rows {
		if r.Barcode == "TEST-B-MAIN" {
			t.Fatal("pack code listed as a product code")
		}
	}
	// Another product's code is refused; so is a same-name pack.
	_, err = conn.ExecContext(ctx, `UPDATE products SET barcode = 'TEST-B-OTHER' WHERE id = $1`, b)
	must(t, err)
	if _, err := s.SavePack(ctx, a, PackInput{Name: "Box", Size: "120", Barcode: "TEST-B-OTHER"}); !isCode(err, "CONFLICT") {
		t.Fatalf("other's code: %v", err)
	}
	if _, err := s.SavePack(ctx, a, PackInput{Name: "card", Size: "10"}); !isCode(err, "CONFLICT") {
		t.Fatalf("same name: %v", err)
	}
	if _, err := s.SavePack(ctx, a, PackInput{Name: "Half", Size: "1"}); !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("size 1: %v", err)
	}
	// A pack code can't become a product's main barcode either.
	if _, err := conn.ExecContext(ctx, `UPDATE products SET barcode = 'TEST-B-MAIN' WHERE id = $1`, b); err == nil {
		t.Fatal("pack code taken as another product's main")
	}
	// Merge: the duplicate's packs follow; a same-name pack hands over its codes.
	_, err = s.SavePack(ctx, b, PackInput{Name: "CARD", Size: "12", Barcode: "TEST-B-CARD2"})
	must(t, err)
	_, err = s.SavePack(ctx, b, PackInput{Name: "Box", Size: "120"})
	must(t, err)
	tx := conn.MustBegin()
	must(t, mergeTables(ctx, tx, b, a))
	must(t, tx.Commit())
	ks, err := s.Packs(ctx, a)
	must(t, err)
	if len(ks) != 2 || ks[0].Name != "Card" || len(ks[0].Barcodes) != 2 || ks[1].Name != "Box" {
		t.Fatalf("after merge = %+v", ks)
	}
	// Delete takes the pack's codes with it.
	must(t, s.DeletePack(ctx, a, ks[0].ID))
	if pid, _, _ := PackForBarcode(ctx, conn, "TEST-B-CARD2"); pid != 0 {
		t.Fatal("pack code survived its pack")
	}
	conn.ExecContext(ctx, `DELETE FROM product_packs WHERE product_id IN ($1, $2)`, a, b) //nolint:errcheck
}

func isCode(err error, code string) bool {
	ae, ok := apperr.As(err)
	return ok && ae.Code == code
}

// Rename used to write a products.updated_at column that never existed, so every
// rename failed (karots-pos recharge carriers swallowed the error).
func TestRenameDB(t *testing.T) {
	conn, a, _ := barcodeFixture(t)
	ctx := context.Background()
	must(t, NewService(conn).Rename(ctx, a, "TEST bc renamed"))
	var name string
	must(t, conn.GetContext(ctx, &name, `SELECT name FROM products WHERE id = $1`, a))
	if name != "TEST bc renamed" {
		t.Fatalf("name %q", name)
	}
}
