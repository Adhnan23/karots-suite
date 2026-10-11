package alternatives

import (
	"context"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/plugins/alternatives/migrations"

	"github.com/jmoiron/sqlx"
)

// testStore migrates alternatives onto the dev DB. Groups a test makes carry a
// per-test tag in their name and are removed afterwards.
func testStore(t *testing.T) (*Store, *sqlx.DB, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_alternatives"); err != nil {
		t.Fatal(err)
	}
	tag := fmt.Sprintf("za%d", time.Now().UnixNano())
	t.Cleanup(func() { db.Exec(`DELETE FROM alt_groups WHERE name LIKE '%' || $1 || '%'`, tag) }) //nolint:errcheck
	return NewStore(db), db, tag
}

// product makes a product of the test's own with that stock.
func product(t *testing.T, db *sqlx.DB, name, qty string, service, active bool) int64 {
	t.Helper()
	var pid int64
	if err := db.Get(&pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, wholesale_price, is_service, is_active)
		VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 100, 90, $2, $3) RETURNING id`,
		name, service, active); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO stock (product_id, quantity) VALUES ($1, $2)
		ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, pid, qty); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec(`DELETE FROM alt_members WHERE product_id = $1`, pid) //nolint:errcheck
		db.Exec(`DELETE FROM stock WHERE product_id = $1`, pid)       //nolint:errcheck
		db.Exec(`DELETE FROM products WHERE id = $1`, pid)            //nolint:errcheck
	})
	return pid
}

// ok fails the test (by panic) on an error and returns the value.
func ok[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func code(err error) string {
	if e, ok := apperr.As(err); ok {
		return e.Code
	}
	return ""
}

func TestGroupsTiersAndMembers(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	gid := ok(s.CreateGroup(ctx, "USB 32GB "+tag))
	g := ok(s.Group(ctx, gid))
	if len(g.Tiers) != 1 || g.Tiers[0].Name != "" {
		t.Fatalf("a new group has one blank tier: %+v", g.Tiers)
	}
	blank := g.Tiers[0].ID
	if _, err := s.CreateTier(ctx, gid, TierInput{Name: "Cheap"}); err == nil {
		t.Fatal("a second tier needs the blank one named first")
	}
	noErr(t, s.UpdateTier(ctx, blank, TierInput{Name: "Genuine", ReorderLevel: 5}))
	cheap := ok(s.CreateTier(ctx, gid, TierInput{Name: "Cheap", ReorderLevel: 3}))
	if err := s.UpdateTier(ctx, cheap, TierInput{Name: ""}); err == nil {
		t.Fatal("with two tiers a blank name is refused")
	}

	king := product(t, db, "Kingston "+tag, "2", false, true)
	sand := product(t, db, "SanDisk "+tag, "4", false, true)
	off := product(t, db, "Old brand "+tag, "50", false, false)
	svc := product(t, db, "Service "+tag, "0", true, true)
	noname := product(t, db, "Noname "+tag, "1", false, true)
	for _, p := range []int64{king, sand} {
		ok(s.AddMember(ctx, blank, p, false))
	}
	ok(s.AddMember(ctx, cheap, noname, false))
	if _, err := s.AddMember(ctx, blank, off, false); err == nil {
		t.Fatal("a disabled product can't be added")
	}
	if _, err := s.AddMember(ctx, blank, svc, false); err == nil {
		t.Fatal("a service can't be added")
	}
	db.MustExec(`INSERT INTO alt_members (product_id, tier_id) VALUES ($1, $2)`, off, blank) // old data: a member disabled later

	g = ok(s.Group(ctx, gid))
	if g.Products != 3 || g.Tiers[0].Total.String() != "6" || g.Tiers[0].Low || !g.Tiers[1].Low {
		t.Fatalf("totals count active members only; Genuine 6 > 5, Cheap 1 ≤ 3: %+v", g)
	}
	if n := len(g.Tiers[0].Members); n != 3 || g.Tiers[0].Members[2].Active {
		t.Fatalf("a disabled member still shows, greyed, last: %+v", g.Tiers[0].Members)
	}

	// Another group: adding asks before moving.
	g2 := ok(s.CreateGroup(ctx, "Other "+tag))
	t2 := ok(s.Group(ctx, g2)).Tiers[0].ID
	if _, err := s.AddMember(ctx, t2, king, false); code(err) != "IN_OTHER_GROUP" {
		t.Fatalf("want IN_OTHER_GROUP, got %v", err)
	}
	if moved := ok(s.AddMember(ctx, t2, king, true)); !moved {
		t.Fatal("move=true moves it")
	}
	ok(s.AddMember(ctx, blank, king, true))

	// Coverage: Genuine is above its level (covered); Cheap is low.
	cov := ok(s.CoverageFor(ctx, []int64{king, noname, off}))
	if !cov[king].Covered || cov[king].Tier != "Genuine" || !cov[noname].Low || cov[noname].Covered {
		t.Fatalf("coverage: %+v", cov)
	}
	if _, ok := cov[off]; ok {
		t.Fatal("a disabled member has no coverage")
	}
	low := ok(s.LowTierMemberIDs(ctx))
	if !slices.Contains(low, noname) || slices.Contains(low, king) {
		t.Fatalf("low tier members: %v", low)
	}

	// Search: the group name, a member's name and a tier's name find the whole live group.
	for _, q := range []string{"USB 32GB " + tag, "Kingston " + tag} {
		ids := ok(s.MatchProductIDs(ctx, q))
		if !slices.Contains(ids, sand) || !slices.Contains(ids, noname) || slices.Contains(ids, off) {
			t.Fatalf("%q found %v", q, ids)
		}
	}
	b := ok(s.BadgesFor(ctx, []int64{king, noname}))
	if b[king][0] != "⇄ Genuine" || b[noname][0] != "⇄ Cheap" {
		t.Fatalf("badges: %v", b)
	}
	if b := ok(s.BadgesFor(ctx, []int64{})); len(b) != 0 {
		t.Fatal("no ids, no badges")
	}

	// Alternatives: best tier first, then most stock; never itself, never out of stock.
	db.MustExec(`UPDATE stock SET quantity = 0 WHERE product_id = $1`, king)
	alts := ok(s.Alternatives(ctx, king, 3))
	if len(alts) != 2 || alts[0].ProductID != sand || alts[1].ProductID != noname {
		t.Fatalf("alternatives: %+v", alts)
	}
	noErr(t, s.MoveTier(ctx, cheap, true))
	if alts = ok(s.Alternatives(ctx, king, 3)); alts[0].ProductID != noname {
		t.Fatalf("Cheap moved up ranks first: %+v", alts)
	}
	if alts = ok(s.Alternatives(ctx, off, 3)); len(alts) != 0 {
		t.Fatal("a disabled member offers nothing")
	}
	noErr(t, s.SetGroupActive(ctx, gid, false))
	if ids := ok(s.MatchProductIDs(ctx, "USB 32GB "+tag)); len(ids) != 0 {
		t.Fatal("a disabled group isn't searched")
	}

	// Tiers: an empty one can go, never one with products or the last one.
	if err := s.DeleteTier(ctx, cheap); err == nil {
		t.Fatal("a tier with products can't be deleted")
	}
	noErr(t, s.RemoveMember(ctx, noname))
	noErr(t, s.DeleteTier(ctx, cheap))
	if err := s.DeleteTier(ctx, t2); err == nil {
		t.Fatal("a group's last tier stays")
	}
	gid, tid, err := s.TierOf(ctx, sand)
	noErr(t, err)
	if tid != blank || gid == 0 {
		t.Fatalf("TierOf: %d %d", gid, tid)
	}
	noErr(t, s.DeleteGroup(ctx, gid))
	if _, tid, _ = s.TierOf(ctx, sand); tid != 0 {
		t.Fatal("deleting the group drops its memberships")
	}
}
