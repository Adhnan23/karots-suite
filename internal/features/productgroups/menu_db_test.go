package productgroups_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	pg "karots-suite/internal/features/productgroups"
	"karots-suite/internal/features/products"
	"karots-suite/internal/ledgercheck/ltest"

	"github.com/jmoiron/sqlx"
)

type menuShop struct {
	db   *sqlx.DB
	repo *pg.Repository
	ids  map[string]int64 // product name → id
	find pg.FindProduct
}

// newMenuShop is a throwaway shop with a few products (one disabled).
func newMenuShop(t *testing.T) *menuShop {
	t.Helper()
	db := ltest.NewDB(t)
	ctx := context.Background()
	var unit, cat int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(db.GetContext(ctx, &unit, `SELECT id FROM units LIMIT 1`))
	must(db.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ('Menu') RETURNING id`))
	s := &menuShop{db: db, repo: pg.NewRepository(db), ids: map[string]int64{}}
	for _, n := range []string{"Coke", "Sprite", "Bun", "Cake", "Old Tea"} {
		var id int64
		must(db.GetContext(ctx, &id, `INSERT INTO products (name, category_id, unit_id, is_active) VALUES ($1, $2, $3, $4) RETURNING id`,
			n, cat, unit, n != "Old Tea"))
		s.ids[n] = id
	}
	s.find = products.NewRepository(db).FindForMenu
	return s
}

func (s *menuShop) rebuild(t *testing.T, lines []pg.MenuLine, find pg.FindProduct) (pg.MenuResult, error) {
	t.Helper()
	tx, err := s.db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	res, err := pg.Rebuild(context.Background(), tx, lines, find)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return res, err
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return res, nil
}

func (s *menuShop) lines(t *testing.T) []pg.MenuLine {
	t.Helper()
	ls, err := s.repo.MenuLines(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := range ls {
		ls[i].Line = 0
	}
	return ls
}

func emoji(s string) *string { return &s }

// sample builds Drinks 🥤 (Coke 🥤, Sprite) > Fizzy (hidden, Old Tea), Bakery (Bun, Cake 🎂) by hand.
func (s *menuShop) sample(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	drinks, err := s.repo.Create(ctx, pg.CreateInput{Name: "Drinks", Emoji: emoji("🥤")})
	if err != nil {
		t.Fatal(err)
	}
	fizzy, _ := s.repo.Create(ctx, pg.CreateInput{Name: "Fizzy", ParentID: &drinks})
	bakery, _ := s.repo.Create(ctx, pg.CreateInput{Name: "Bakery"})
	s.db.MustExec(`UPDATE product_groups SET is_active = false WHERE id = $1`, fizzy)
	for _, l := range []struct {
		g    int64
		name string
		e    *string
	}{{drinks, "Coke", emoji("🥤")}, {drinks, "Sprite", nil}, {fizzy, "Old Tea", nil}, {bakery, "Bun", nil}, {bakery, "Cake", emoji("🎂")}} {
		if err := s.repo.LinkProduct(ctx, l.g, s.ids[l.name], l.e); err != nil {
			t.Fatal(err)
		}
	}
}

// MENU-1, 7, 8: menu → lines → rebuild → the same lines (a hidden group, a
// disabled product, a sub-group, order and emoji all survive).
func TestRebuildRoundTrip(t *testing.T) {
	s := newMenuShop(t)
	s.sample(t)
	before := s.lines(t)
	want := []pg.MenuLine{
		{Group: "Drinks", Emoji: "🥤"},
		{Group: "Drinks", Product: "Coke", ProductEmoji: "🥤"},
		{Group: "Drinks", Product: "Sprite"},
		{Group: "Drinks > Fizzy", Active: "no"},
		{Group: "Drinks > Fizzy", Product: "Old Tea"},
		{Group: "Bakery"},
		{Group: "Bakery", Product: "Bun"},
		{Group: "Bakery", Product: "Cake", ProductEmoji: "🎂"},
	}
	if !reflect.DeepEqual(before, want) {
		t.Fatalf("export:\n got %+v\nwant %+v", before, want)
	}
	res, err := s.rebuild(t, before, s.find)
	if err != nil {
		t.Fatal(err)
	}
	if res.Groups != 3 || res.Items != 5 || res.RemovedGroups != 0 || res.RemovedItems != 0 || len(res.Skipped) != 0 {
		t.Fatalf("result %+v", res)
	}
	if after := s.lines(t); !reflect.DeepEqual(after, before) {
		t.Fatalf("round trip:\n got %+v\nwant %+v", after, before)
	}
}

// MENU-2: what the sheet doesn't name is gone.
func TestRebuildReplacesWholeMenu(t *testing.T) {
	s := newMenuShop(t)
	s.sample(t)
	res, err := s.rebuild(t, []pg.MenuLine{
		{Line: 2, Group: "Bakery", Product: "Bun"},
		{Line: 3, Group: "New", Product: "Coke"},
	}, s.find)
	if err != nil {
		t.Fatal(err)
	}
	got := s.lines(t)
	want := []pg.MenuLine{{Group: "Bakery"}, {Group: "Bakery", Product: "Bun"}, {Group: "New"}, {Group: "New", Product: "Coke"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("menu:\n got %+v\nwant %+v", got, want)
	}
	// gone: Drinks, Drinks > Fizzy; items gone: Coke, Sprite (Drinks), Old Tea, Cake
	if res.Groups != 2 || res.Items != 2 || res.RemovedGroups != 2 || res.RemovedItems != 4 {
		t.Fatalf("result %+v", res)
	}
}

// MENU-15: a group named only on product lines is made with every level of its
// path, in first-appearance order; "A>B" and "A > B" are one group.
func TestRebuildMakesGroupsFromProductRows(t *testing.T) {
	s := newMenuShop(t)
	if _, err := s.rebuild(t, []pg.MenuLine{
		{Line: 2, Group: "A > B > C", Product: "Coke"},
		{Line: 3, Group: "A>B", Product: "Bun"},
		{Line: 4, Group: " Z ", Product: "Cake"},
		{Line: 5, Group: "A >  B", Product: "Sprite"},
	}, s.find); err != nil {
		t.Fatal(err)
	}
	got := s.lines(t)
	want := []pg.MenuLine{
		{Group: "A"},
		{Group: "A > B"},
		{Group: "A > B", Product: "Bun"},
		{Group: "A > B", Product: "Sprite"},
		{Group: "A > B > C"},
		{Group: "A > B > C", Product: "Coke"},
		{Group: "Z"},
		{Group: "Z", Product: "Cake"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("menu:\n got %+v\nwant %+v", got, want)
	}
}

// MENU-5, 9, 10: missing products, a product twice in one group, over-long
// cells — skipped and listed by line; the twice-named product is one item with
// the first line's emoji; blank lines are no lines at all.
func TestRebuildSkips(t *testing.T) {
	s := newMenuShop(t)
	res, err := s.rebuild(t, []pg.MenuLine{
		{Line: 2, Group: "Drinks", Product: "Coke", ProductEmoji: "🥤"},
		{Line: 3, Group: "Drinks", Product: "Nope"},
		{Line: 4, Group: "Drinks", Product: "coke", ProductEmoji: "🧃"},
		{Line: 5},
		{Line: 6, Group: strings.Repeat("x", 81)},
		{Line: 7, Group: "Bakery", Emoji: strings.Repeat("🎂", 17)},
		{Line: 8, Group: "Bakery", Product: "Bun", ProductEmoji: strings.Repeat("y", 17)},
	}, s.find)
	if err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, k := range res.Skipped {
		lines = append(lines, k.Line)
		if k.Reason == "" {
			t.Errorf("line %d skipped without a reason", k.Line)
		}
	}
	if !reflect.DeepEqual(lines, []int{3, 6, 7, 8}) {
		t.Fatalf("skipped lines %v (%+v)", lines, res.Skipped)
	}
	got := s.lines(t)
	want := []pg.MenuLine{{Group: "Drinks"}, {Group: "Drinks", Product: "Coke", ProductEmoji: "🥤"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("menu:\n got %+v\nwant %+v", got, want)
	}
}

// MENU-4: a failure part way leaves the menu as it was.
func TestRebuildAllOrNothing(t *testing.T) {
	s := newMenuShop(t)
	s.sample(t)
	before := s.lines(t)
	n := 0
	boom := errors.New("boom")
	failing := func(ctx context.Context, barcode, name string) (int64, string, error) {
		if n++; n == 3 {
			return 0, "", boom
		}
		return s.find(ctx, barcode, name)
	}
	_, err := s.rebuild(t, []pg.MenuLine{
		{Line: 2, Group: "New", Product: "Coke"},
		{Line: 3, Group: "New", Product: "Bun"},
		{Line: 4, Group: "New", Product: "Cake"},
	}, failing)
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
	if after := s.lines(t); !reflect.DeepEqual(after, before) {
		t.Fatalf("menu changed:\n got %+v\nwant %+v", after, before)
	}
}
