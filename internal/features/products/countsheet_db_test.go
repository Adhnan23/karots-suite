package products

import (
	"context"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
)

// TestCountSheet proves the category-subtree filter, the supplier filter, and
// the category-path grouping key all work: a parent category filter must pull
// in a child's products too (with the full path), a child filter must return
// only its own, and a supplier filter must narrow to that supplier alone.
func TestCountSheet(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()

	var unitID, supplierID, parentID, childID, prodParent, prodChild int64
	must(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, conn.GetContext(ctx, &supplierID, `
		INSERT INTO suppliers (name) VALUES ('TEST countsheet supplier') RETURNING id`))
	must(t, conn.GetContext(ctx, &parentID, `
		INSERT INTO categories (name) VALUES ('TEST countsheet parent') RETURNING id`))
	must(t, conn.GetContext(ctx, &childID, `
		INSERT INTO categories (name, parent_id) VALUES ('TEST countsheet child', $1) RETURNING id`, parentID))
	must(t, conn.GetContext(ctx, &prodParent, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, preferred_supplier_id)
		VALUES ('TEST countsheet on parent', $1, $2, 10, 20, $3) RETURNING id`, parentID, unitID, supplierID))
	must(t, conn.GetContext(ctx, &prodChild, `
		INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ('TEST countsheet on child', $1, $2, 10, 20) RETURNING id`, childID, unitID))

	defer func() {
		conn.ExecContext(ctx, `DELETE FROM products WHERE id IN ($1,$2)`, prodParent, prodChild) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM categories WHERE id IN ($1,$2)`, childID, parentID)   //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM suppliers WHERE id = $1`, supplierID)                 //nolint:errcheck
	}()

	repo := NewRepository(conn)

	// Parent filter: pulls in both parent's own product and the child's.
	rows, err := repo.CountSheet(ctx, &parentID, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[int64]string{}
	for _, r := range rows {
		ids[r.ID] = r.CategoryPath
	}
	if _, ok := ids[prodParent]; !ok {
		t.Errorf("parent filter missing the parent-category product")
	}
	if path, ok := ids[prodChild]; !ok {
		t.Errorf("parent filter missing the child-category product")
	} else if path != "TEST countsheet parent > TEST countsheet child" {
		t.Errorf("child product category_path = %q", path)
	}

	// Child filter: only the child's own product.
	rows, err = repo.CountSheet(ctx, &childID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != prodChild {
		t.Errorf("child filter returned %+v, want only prodChild", rows)
	}

	// Supplier filter: only the product with that preferred supplier.
	rows, err = repo.CountSheet(ctx, nil, &supplierID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != prodParent {
		t.Errorf("supplier filter returned %+v, want only prodParent", rows)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
