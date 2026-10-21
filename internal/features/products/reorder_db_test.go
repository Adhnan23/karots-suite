package products

import (
	"context"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
)

// New products start Auto from the typed level; an update keeps the mode unless
// asked, switching to Auto starts from the typed number, and an Auto product's
// level is the nightly run's — the form's number only moves its starting guess.
func TestReorderAutoCreateAndUpdate(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var unitID, catID int64
	must(t, conn.GetContext(ctx, &unitID, `SELECT id FROM units LIMIT 1`))
	must(t, conn.GetContext(ctx, &catID, `INSERT INTO categories (name) VALUES ('TEST reorder cat') RETURNING id`))
	t.Cleanup(func() {
		conn.ExecContext(ctx, `DELETE FROM products WHERE category_id = $1`, catID) //nolint:errcheck
		conn.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, catID)        //nolint:errcheck
		conn.Close()
	})
	svc := NewService(conn)
	p, err := svc.Create(ctx, CreateInput{Name: "TEST reorder pencil", CategoryID: catID, UnitID: unitID, ReorderLevel: 12})
	must(t, err)
	if !p.ReorderAuto || p.ReorderStart != 12 || p.ReorderLevel != 12 {
		t.Fatalf("new product: auto=%v start=%d level=%d, want true 12 12", p.ReorderAuto, p.ReorderStart, p.ReorderLevel)
	}

	// Auto: the form's number moves the starting guess only.
	in := CreateInput{Name: p.Name, CategoryID: catID, UnitID: unitID, ReorderLevel: 30}
	start := 20
	in.ReorderStart = &start
	p, err = svc.Update(ctx, p.ID, in)
	must(t, err)
	if !p.ReorderAuto || p.ReorderStart != 20 || p.ReorderLevel != 12 {
		t.Fatalf("auto update: auto=%v start=%d level=%d, want true 20 12", p.ReorderAuto, p.ReorderStart, p.ReorderLevel)
	}

	// Auto → Fixed: the typed number is the level.
	off := false
	in = CreateInput{Name: p.Name, CategoryID: catID, UnitID: unitID, ReorderLevel: 15, ReorderAuto: &off}
	p, err = svc.Update(ctx, p.ID, in)
	must(t, err)
	if p.ReorderAuto || p.ReorderLevel != 15 {
		t.Fatalf("to fixed: auto=%v level=%d, want false 15", p.ReorderAuto, p.ReorderLevel)
	}

	// Fixed, mode not sent: stays Fixed.
	in = CreateInput{Name: p.Name, CategoryID: catID, UnitID: unitID, ReorderLevel: 18}
	p, err = svc.Update(ctx, p.ID, in)
	must(t, err)
	if p.ReorderAuto || p.ReorderLevel != 18 {
		t.Fatalf("fixed update: auto=%v level=%d, want false 18", p.ReorderAuto, p.ReorderLevel)
	}

	// Fixed → Auto: starts from the typed number.
	on := true
	in = CreateInput{Name: p.Name, CategoryID: catID, UnitID: unitID, ReorderLevel: 18, ReorderAuto: &on}
	p, err = svc.Update(ctx, p.ID, in)
	must(t, err)
	if !p.ReorderAuto || p.ReorderStart != 18 {
		t.Fatalf("to auto: auto=%v start=%d, want true 18", p.ReorderAuto, p.ReorderStart)
	}
}
