package products

import (
	"context"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
)

// A product is found by its local (Sinhala / Tamil) name too. Each word may
// match either name, so "test-l සහල්" finds the rice; a product with no local
// name is still found by its English one.
func TestSearchFindsLocalName(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var unitID, catID, rice, plain int64
	must(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, conn.GetContext(ctx, &catID, `INSERT INTO categories (name) VALUES ('TEST local cat') RETURNING id`))
	must(t, conn.GetContext(ctx, &rice, `INSERT INTO products (name, name_local, category_id, unit_id) VALUES ('TEST-L Samba Rice 1kg', 'සම්බා සහල්', $1, $2) RETURNING id`, catID, unitID))
	must(t, conn.GetContext(ctx, &plain, `INSERT INTO products (name, category_id, unit_id) VALUES ('TEST-L Plain', $1, $2) RETURNING id`, catID, unitID))
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM products WHERE id IN ($1, $2)`, rice, plain) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, catID)            //nolint:errcheck
		conn.Close()
	})
	found := func(q string) map[int64]bool {
		rows, err := NewRepository(conn).List(ctx, ListQuery{Search: q, Limit: 10, Page: 1})
		if err != nil {
			t.Fatalf("search %q: %v", q, err)
		}
		m := map[int64]bool{}
		for _, r := range rows {
			m[r.ID] = true
		}
		return m
	}
	if !found("සම්බා")[rice] {
		t.Error("local name alone must find the product")
	}
	if !found("test-l සහල්")[rice] {
		t.Error("one word per name must find the product")
	}
	if !found("test-l plain")[plain] {
		t.Error("no local name must not hide a product")
	}
	if m := found("සම්බා plain"); m[rice] || m[plain] {
		t.Error("every word must match")
	}
}
