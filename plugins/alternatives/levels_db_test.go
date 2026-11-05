package alternatives

import (
	"context"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/alternatives/migrations"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
	"github.com/shopspring/decimal"
)

// The migration keeps a typed level typed; an unset 0 becomes Auto.
func TestMigrationKeepsTyped(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	noErr(t, err)
	t.Cleanup(func() { admin.Close() })
	const name = "alternatives_levels_migration"
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	_, err = admin.Exec(`CREATE DATABASE ` + name)
	noErr(t, err)
	t.Cleanup(func() { admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) }) //nolint:errcheck
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	noErr(t, err)
	t.Cleanup(func() { db.Close() })
	noErr(t, appdb.RunMigrations(db.DB, coremigrations.FS))

	goose.SetBaseFS(migrations.FS)
	goose.SetTableName("goose_db_version_alternatives")
	defer goose.SetBaseFS(nil)
	defer goose.SetTableName("goose_db_version")
	noErr(t, goose.SetDialect("postgres"))
	noErr(t, goose.UpTo(db.DB, ".", 1))
	if _, err := db.Exec(`INSERT INTO alt_groups (id, name, sort_order) VALUES (1, 'G', 0);
		INSERT INTO alt_tiers (id, group_id, name, reorder_level) VALUES (1, 1, 'Typed', 40), (2, 1, 'Unset', 0)`); err != nil {
		t.Fatal(err)
	}
	noErr(t, goose.Up(db.DB, "."))
	var typed, unset bool
	noErr(t, db.Get(&typed, `SELECT level_auto FROM alt_tiers WHERE id = 1`))
	noErr(t, db.Get(&unset, `SELECT level_auto FROM alt_tiers WHERE id = 2`))
	if typed || !unset {
		t.Fatalf("typed auto = %v, unset auto = %v", typed, unset)
	}
	var level int
	noErr(t, db.Get(&level, `SELECT reorder_level FROM alt_tiers WHERE id = 1`))
	if level != 40 {
		t.Fatalf("typed level became %d", level)
	}
}

func setLevel(t *testing.T, s *Store, pid int64, level int) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE products SET reorder_level = $2 WHERE id = $1`, pid, level); err != nil {
		t.Fatal(err)
	}
}

func groupTier(t *testing.T, s *Store, name string, ids ...int64) int64 {
	t.Helper()
	ctx := context.Background()
	gid, err := s.CreateGroup(ctx, name)
	noErr(t, err)
	g, err := s.Group(ctx, gid)
	noErr(t, err)
	for _, id := range ids {
		_, err := s.AddMember(ctx, g.Tiers[0].ID, id, false)
		noErr(t, err)
	}
	return g.Tiers[0].ID
}

func TestAutoLevelSumsMembers(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a, b := product(t, db, "A"+sfx, "1", false, true), product(t, db, "B"+sfx, "1", false, true)
	off := product(t, db, "Off"+sfx, "1", false, true)
	setLevel(t, s, a, 10)
	setLevel(t, s, b, 20)
	setLevel(t, s, off, 500)
	tid := groupTier(t, s, "Sum "+sfx, a, b, off)
	if _, err := db.Exec(`UPDATE products SET is_active = false WHERE id = $1`, off); err != nil {
		t.Fatal(err)
	}
	tier, err := s.tier(ctx, tid)
	noErr(t, err)
	if !tier.LevelAuto || tier.ReorderLevel != 30 {
		t.Fatalf("%+v", tier)
	}
	empty := groupTier(t, s, "Empty "+sfx)
	tier, err = s.tier(ctx, empty)
	noErr(t, err)
	if tier.ReorderLevel != 0 || low(tier).Low {
		t.Fatalf("an empty group has no level, and isn't low: %+v", tier)
	}
}

func TestTypedLevelOverrides(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a := product(t, db, "A"+sfx, "1", false, true)
	setLevel(t, s, a, 10)
	tid := groupTier(t, s, "Typed "+sfx, a)
	noErr(t, s.UpdateTier(ctx, tid, TierInput{ReorderLevel: 40}))
	tier, _ := s.tier(ctx, tid)
	if tier.LevelAuto || tier.ReorderLevel != 40 {
		t.Fatalf("typed: %+v", tier)
	}
	noErr(t, s.UpdateTier(ctx, tid, TierInput{ReorderLevel: 0}))
	tier, _ = s.tier(ctx, tid)
	if tier.LevelAuto || tier.ReorderLevel != 0 || low(tier).Low {
		t.Fatalf("a typed 0 means don't track: %+v", tier)
	}
	cov, err := s.CoverageFor(ctx, []int64{a})
	noErr(t, err)
	if cov[a].Covered || cov[a].Low {
		t.Fatalf("untracked is never covered: %+v", cov[a])
	}
	auto := true
	noErr(t, s.UpdateTier(ctx, tid, TierInput{ReorderLevel: 7, LevelAuto: &auto}))
	tier, _ = s.tier(ctx, tid)
	if !tier.LevelAuto || tier.ReorderLevel != 10 {
		t.Fatalf("back to Auto: %+v", tier)
	}
}

func TestNewTierDefault(t *testing.T) {
	s, _, sfx := testStore(t)
	ctx := context.Background()
	gid, err := s.CreateGroup(ctx, "New "+sfx)
	noErr(t, err)
	g, _ := s.Group(ctx, gid)
	noErr(t, s.UpdateTier(ctx, g.Tiers[0].ID, TierInput{Name: "First"}))
	typed, err := s.CreateTier(ctx, gid, TierInput{Name: "Typed", ReorderLevel: 9})
	noErr(t, err)
	blank, err := s.CreateTier(ctx, gid, TierInput{Name: "Blank"})
	noErr(t, err)
	if x, _ := s.tier(ctx, typed); x.LevelAuto || x.ReorderLevel != 9 {
		t.Fatalf("%+v", x)
	}
	if x, _ := s.tier(ctx, blank); !x.LevelAuto {
		t.Fatalf("%+v", x)
	}
}

func TestLowMembersUsesAuto(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a, b := product(t, db, "A"+sfx, "3", false, true), product(t, db, "B"+sfx, "4", false, true)
	setLevel(t, s, a, 10)
	setLevel(t, s, b, 10)
	groupTier(t, s, "Low "+sfx, a, b)
	ids, err := s.LowTierMemberIDs(ctx)
	noErr(t, err)
	if !slices.Contains(ids, a) || !slices.Contains(ids, b) {
		t.Fatalf("7 left against a learned 20: %v", ids)
	}
	cov, err := s.CoverageFor(ctx, []int64{a, b})
	noErr(t, err)
	// 2 × 20 − 7 = 33, nothing sold: an even split that adds up exactly.
	if !cov[a].Low || !cov[a].Need.Add(cov[b].Need).Equal(decimal.NewFromInt(33)) {
		t.Fatalf("%+v %+v", cov[a], cov[b])
	}
}

func TestManualSplitAndCheck(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a, b := product(t, db, "A"+sfx, "0", false, true), product(t, db, "B"+sfx, "0", false, true)
	setLevel(t, s, a, 11)
	setLevel(t, s, b, 11)
	tid := groupTier(t, s, "Hand "+sfx, a, b)
	if err := s.SetSplit(ctx, tid, false, map[int64]int{a: 50, b: 40}); err == nil {
		t.Fatal("90% was accepted")
	}
	if err := s.SetSplit(ctx, tid, false, map[int64]int{a: 50, 999999: 50}); err == nil {
		t.Fatal("a product outside the tier was accepted")
	}
	if err := s.SetSplit(ctx, tid, false, map[int64]int{a: 150, b: -50}); err == nil {
		t.Fatal("a share over 100 was accepted")
	}
	noErr(t, s.SetSplit(ctx, tid, false, map[int64]int{a: 75, b: 25}))
	cov, err := s.CoverageFor(ctx, []int64{a, b})
	noErr(t, err)
	if !cov[a].Need.Equal(decimal.NewFromInt(33)) || !cov[b].Need.Equal(decimal.NewFromInt(11)) {
		t.Fatalf("2 × 22 − 0 = 44 by 75/25: %s %s", cov[a].Need, cov[b].Need)
	}
	noErr(t, s.SetSplit(ctx, tid, true, nil))
	tier, _ := s.tier(ctx, tid)
	if !tier.SplitAuto {
		t.Fatal("back to automatic")
	}
	// Disabled members don't take a share of the order.
	if _, err := db.Exec(`UPDATE products SET is_active = false WHERE id = $1`, b); err != nil {
		t.Fatal(err)
	}
	cov, err = s.CoverageFor(ctx, []int64{a})
	noErr(t, err)
	if !cov[a].Need.Equal(decimal.NewFromInt(22)) {
		t.Fatalf("the one left takes it all: %s", cov[a].Need)
	}
}

// A backup from before Auto levels restores without these columns: its typed level
// must stay typed, so the column's default can't be Auto.
func TestOldRowsStayTyped(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a := product(t, db, "A"+sfx, "1", false, true)
	setLevel(t, s, a, 3)
	gid, err := s.CreateGroup(ctx, "Old "+sfx)
	noErr(t, err)
	var tier int64
	noErr(t, db.Get(&tier, `INSERT INTO alt_tiers (group_id, name, reorder_level) VALUES ($1, 'Restored', 50) RETURNING id`, gid))
	got, err := s.tier(ctx, tier)
	noErr(t, err)
	if got.LevelAuto || got.ReorderLevel != 50 {
		t.Fatalf("%+v", got)
	}
	g, _ := s.Group(ctx, gid)
	if !g.Tiers[0].LevelAuto {
		t.Fatalf("a new group still starts on Auto: %+v", g.Tiers[0])
	}
}

// A brand made from another (loose from a bag) never shows on Reorder: it takes no share.
func TestDerivedMemberTakesNoShare(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	a, b := product(t, db, "A"+sfx, "0", false, true), product(t, db, "B"+sfx, "0", false, true)
	src := product(t, db, "Bag"+sfx, "5", false, true)
	setLevel(t, s, a, 10)
	setLevel(t, s, b, 10)
	groupTier(t, s, "Derived "+sfx, a, b)
	noErrExec(t, db, `INSERT INTO product_conversions (from_product_id, to_product_id, ratio) VALUES ($1, $2, 25)`, src, b)
	cov, err := s.CoverageFor(ctx, []int64{a, b})
	noErr(t, err)
	if !cov[a].Need.Equal(decimal.NewFromInt(40)) || !cov[b].Need.IsZero() {
		t.Fatalf("%s %s", cov[a].Need, cov[b].Need)
	}
}

func TestSameNamedGroupsAreSeparateBlocks(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	p := &Plugin{store: s}
	a, b := product(t, db, "A"+sfx, "0", false, true), product(t, db, "B"+sfx, "0", false, true)
	setLevel(t, s, a, 5)
	setLevel(t, s, b, 5)
	groupTier(t, s, "Twin name "+sfx, a)
	groupTier(t, s, "Twin name "+sfx, b)
	notes, err := p.reorderNotes(ctx, []int64{a, b})
	noErr(t, err)
	if notes[a].BlockKey == "" || notes[a].BlockKey == notes[b].BlockKey || notes[a].Block != notes[b].Block {
		t.Fatalf("%+v %+v", notes[a], notes[b])
	}
}

func noErrExec(t *testing.T, db *sqlx.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatal(err)
	}
}

// Split by hand, and another brand outsells its share: the low-stock block says so, as
// well as the group page. Nothing changes by itself.
func TestLowBlockSaysWhenABrandOutsellsItsShare(t *testing.T) {
	s, db, sfx := testStore(t)
	ctx := context.Background()
	p := &Plugin{store: s}
	a, b := product(t, db, "Fast"+sfx, "0", false, true), product(t, db, "Slow"+sfx, "0", false, true)
	setLevel(t, s, a, 10)
	setLevel(t, s, b, 10)
	tid := groupTier(t, s, "Sells "+sfx, a, b)
	var sale, user int64
	noErr(t, db.Get(&user, `SELECT id FROM users ORDER BY id LIMIT 1`))
	noErr(t, db.Get(&sale, `INSERT INTO sales (receipt_no, cashier_id) VALUES ($1, $2) RETURNING id`, sfx[len(sfx)-12:], user))
	t.Cleanup(func() { db.Exec(`DELETE FROM sales WHERE id = $1`, sale) }) //nolint:errcheck
	noErrExec(t, db, `INSERT INTO sale_items (sale_id, product_id, quantity, unit_price, subtotal) VALUES ($1, $2, 30, 1, 30)`, sale, a)
	line := func() string {
		t.Helper()
		notes, err := p.reorderNotes(ctx, []int64{a, b})
		noErr(t, err)
		return notes[a].BlockLine
	}
	if got := line(); strings.Contains(got, "selling faster") {
		t.Fatalf("an automatic split follows sales already: %q", got)
	}
	noErr(t, s.SetSplit(ctx, tid, false, map[int64]int{a: 50, b: 50}))
	if got := line(); !strings.Contains(got, "Fast"+sfx+" is selling faster than its share") {
		t.Fatalf("%q", got)
	}
	cov, err := s.CoverageFor(ctx, []int64{a})
	noErr(t, err)
	if !cov[a].Need.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("the shop's shares still decide: %s", cov[a].Need)
	}
}
