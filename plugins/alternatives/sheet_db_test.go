package alternatives

import (
	"context"
	"reflect"
	"strings"
	"testing"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/products"

	"github.com/jmoiron/sqlx"
)

// mine keeps only the lines of this test's own groups (the package's database is shared).
func mine(lines []SheetLine, tag string) []SheetLine {
	var out []SheetLine
	for _, l := range lines {
		if strings.Contains(l.Group, tag) {
			l.Line = 0
			out = append(out, l)
		}
	}
	return out
}

// ALT-9 / ALT-18: the export lists every group (a turned-off one too) in page order,
// tiers in order, members by name, a disabled member included, with the main barcode.
func TestSheetLinesOrder_ALT9(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	b := product(t, db, tag+" Bakers 1kg", "5", false, true)
	gen := product(t, db, tag+" Generic 32GB", "5", false, true)
	san := product(t, db, tag+" SanDisk 32GB", "5", false, true)
	off := product(t, db, tag+" Old 32GB", "0", false, true)
	noErrExec(t, db, `UPDATE products SET barcode = $2 WHERE id = $1`, san, tag+"-111")

	usb := ok(s.CreateGroup(ctx, tag+" USB 32GB"))
	g := ok(s.Group(ctx, usb))
	noErr(t, s.UpdateTier(ctx, g.Tiers[0].ID, TierInput{Name: "Genuine"}))
	cheap := ok(s.CreateTier(ctx, usb, TierInput{Name: "Cheap"}))
	ok(s.AddMember(ctx, g.Tiers[0].ID, san, false))
	ok(s.AddMember(ctx, g.Tiers[0].ID, off, false))
	ok(s.AddMember(ctx, cheap, gen, false))
	noErrExec(t, db, `UPDATE products SET is_active = false WHERE id = $1`, off)
	rice := groupTier(t, s, tag+" Rice 1kg", b, a)
	_ = rice
	noErrExec(t, db, `UPDATE alt_groups SET sort_order = 1 WHERE id = $1`, usb)
	riceID := ok(s.Memberships(ctx, []int64{a}))[a].GroupID
	noErr(t, s.SetGroupActive(ctx, riceID, false))

	got := mine(ok(s.SheetLines(ctx)), tag)
	want := []SheetLine{
		{Group: tag + " Rice 1kg", Product: tag + " Anchor 1kg"},
		{Group: tag + " Rice 1kg", Product: tag + " Bakers 1kg"},
		{Group: tag + " USB 32GB", Tier: "Genuine", Product: tag + " Old 32GB"},
		{Group: tag + " USB 32GB", Tier: "Genuine", Product: tag + " SanDisk 32GB", Barcode: tag + "-111"},
		{Group: tag + " USB 32GB", Tier: "Cheap", Product: tag + " Generic 32GB"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
}

// rebuild runs Rebuild in its own transaction with core's product finder (MENU-5's).
func rebuild(t *testing.T, db *sqlx.DB, lines []SheetLine) (SheetResult, error) {
	t.Helper()
	var res SheetResult
	err := appdb.WithTx(context.Background(), db, func(tx *sqlx.Tx) error {
		var err error
		res, err = Rebuild(context.Background(), tx, lines, products.NewRepository(db).FindForMenu)
		return err
	})
	return res, err
}

func numbered(lines []SheetLine) []SheetLine {
	for i := range lines {
		lines[i].Line = i + 2
	}
	return lines
}

type tierState struct {
	Level       int `db:"reorder_level"`
	Auto, Split bool
	Active      bool `db:"is_active"`
}

func tierOfState(t *testing.T, db *sqlx.DB, id int64) tierState {
	t.Helper()
	var st tierState
	noErr(t, db.Get(&st, `SELECT reorder_level, level_auto AS auto, split_auto AS split, is_active FROM alt_tiers WHERE id = $1`, id))
	return st
}

func shares(t *testing.T, db *sqlx.DB, tierID int64) map[int64]int {
	t.Helper()
	var rows []struct {
		P int64 `db:"product_id"`
		S int   `db:"s"`
	}
	noErr(t, db.Select(&rows, `SELECT product_id, COALESCE(split_share, 0) AS s FROM alt_members WHERE tier_id = $1`, tierID))
	out := map[int64]int{}
	for _, r := range rows {
		out[r.P] = r.S
	}
	return out
}

// ALT-9: export, import unchanged, export again: the same set, the same ids, a typed
// level, a hand split and a turned-off group all kept.
func TestSheetRoundTrip_ALT9(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	b := product(t, db, tag+" Bakers 1kg", "5", false, true)
	c := product(t, db, tag+" Cable 1m", "5", false, true)
	d := product(t, db, tag+" Cord 1m", "5", false, true)
	rice := groupTier(t, s, tag+" Rice 1kg", a, b)
	noErr(t, s.UpdateTier(ctx, rice, TierInput{ReorderLevel: 7}))
	noErr(t, s.SetSplit(ctx, rice, false, map[int64]int{a: 60, b: 40}))
	cable := groupTier(t, s, tag+" Cable 1m", c, d)
	cableGroup := ok(s.Memberships(ctx, []int64{c}))[c].GroupID
	noErr(t, s.SetGroupActive(ctx, cableGroup, false))

	before := ok(s.SheetLines(ctx))
	res, err := rebuild(t, db, numbered(append([]SheetLine(nil), before...)))
	noErr(t, err)
	if res.Groups != len(ok(s.Groups(ctx, true))) || res.Members != len(before) || len(res.Skipped) != 0 || res.GroupsRemoved != 0 {
		t.Fatalf("result %+v", res)
	}
	if after := ok(s.SheetLines(ctx)); !reflect.DeepEqual(mine(after, tag), mine(before, tag)) {
		t.Fatalf("after %+v\nbefore %+v", mine(after, tag), mine(before, tag))
	}
	if st := tierOfState(t, db, rice); st.Level != 7 || st.Auto || st.Split {
		t.Fatalf("rice tier %+v", st)
	}
	if sh := shares(t, db, rice); sh[a] != 60 || sh[b] != 40 {
		t.Fatalf("shares %v", sh)
	}
	if m := ok(s.Memberships(ctx, []int64{c}))[c]; m.TierID != cable || m.GroupID != cableGroup {
		t.Fatalf("cable moved: %+v", m)
	}
	var on bool
	noErr(t, db.Get(&on, `SELECT is_active FROM alt_groups WHERE id = $1`, cableGroup))
	if on {
		t.Fatal("the turned-off group came back on")
	}
}

// ALT-10: what the sheet doesn't name is gone.
func TestSheetWhole_ALT10(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	b := product(t, db, tag+" Bakers 1kg", "5", false, true)
	x := product(t, db, tag+" Extra 1kg", "5", false, true)
	c := product(t, db, tag+" Cable 1m", "5", false, true)
	d := product(t, db, tag+" Cord 1m", "5", false, true)
	groupTier(t, s, tag+" Rice 1kg", a, b, x)
	groupTier(t, s, tag+" Cable 1m", c, d)

	res, err := rebuild(t, db, numbered([]SheetLine{
		{Group: tag + " Rice 1kg", Product: tag + " Anchor 1kg"},
		{Group: tag + " Rice 1kg", Product: tag + " Bakers 1kg"},
	}))
	noErr(t, err)
	if res.GroupsRemoved != 1 || !reflect.DeepEqual(res.Removed, []string{tag + " Cable 1m"}) || res.TakenOut != 3 || res.Members != 2 {
		t.Fatalf("result %+v", res)
	}
	if got := mine(ok(s.SheetLines(ctx)), tag); len(got) != 2 {
		t.Fatalf("left %+v", got)
	}
	if m := ok(s.Memberships(ctx, []int64{x, c})); len(m) != 0 {
		t.Fatalf("still grouped %+v", m)
	}
}

// ALT-13: lines that can't be placed are skipped and listed; the rest go in.
func TestSheetFind_ALT13(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	product(t, db, tag+" Bakers 1kg", "5", false, true)
	product(t, db, tag+" Delivery", "0", true, true)
	product(t, db, tag+" Old 1kg", "0", false, false)
	product(t, db, tag+" Twin", "1", false, true)
	product(t, db, tag+" Twin", "1", false, true)
	g := tag + " Rice 1kg"
	res, err := rebuild(t, db, numbered([]SheetLine{
		{Group: g, Product: tag + " Anchor 1kg"},
		{Group: g, Product: tag + " anchor   1KG"}, // the same member twice: one member, no skip
		{Group: g, Product: tag + " Bakers 1kg"},
		{Group: g, Product: tag + " Nowhere"},
		{Group: g, Product: tag + " Delivery"},
		{Group: g, Product: tag + " Old 1kg"},
		{Group: g, Product: tag + " Twin"},
		{Product: tag + " Bakers 1kg"},
		{Group: tag + " Other", Product: tag + " Anchor 1kg"},
	}))
	noErr(t, err)
	var lines []int
	for _, k := range res.Skipped {
		lines = append(lines, k.Line)
	}
	if !reflect.DeepEqual(lines, []int{5, 6, 7, 8, 9, 10}) {
		t.Fatalf("skipped %+v", res.Skipped)
	}
	if r := res.Skipped[5].Reason; !strings.Contains(r, "already in "+g+" on line 2") {
		t.Fatalf("two groups: %q", r)
	}
	if r := res.Skipped[1].Reason; !strings.Contains(r, "service") {
		t.Fatalf("service: %q", r)
	}
	if r := res.Skipped[2].Reason; !strings.Contains(r, "disabled") {
		t.Fatalf("disabled: %q", r)
	}
	if r := res.Skipped[4].Reason; !strings.Contains(r, "group") {
		t.Fatalf("no group: %q", r)
	}
	if res.Members != 2 || res.Groups != 1 || ok(s.Memberships(ctx, []int64{a}))[a].Group != g {
		t.Fatalf("result %+v", res)
	}
}

// ALT-14: no product lines is refused, never "remove every group".
func TestSheetEmpty_ALT14(t *testing.T) {
	s, db, tag := testStore(t)
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	b := product(t, db, tag+" Bakers 1kg", "5", false, true)
	groupTier(t, s, tag+" Rice 1kg", a, b)
	if _, err := rebuild(t, db, nil); err == nil {
		t.Fatal("an empty sheet was taken")
	}
	if got := mine(ok(s.SheetLines(context.Background())), tag); len(got) != 2 {
		t.Fatalf("groups touched: %+v", got)
	}
}

// ALT-15: matching ignores case and spacing and keeps ids and settings; a hand split
// that lost a member is rescaled (ALT-5); a new tier starts Auto.
func TestSheetKeeps_ALT15(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	a := product(t, db, tag+" A 32GB", "5", false, true)
	b := product(t, db, tag+" B 32GB", "5", false, true)
	c := product(t, db, tag+" C 32GB", "5", false, true)
	tier := groupTier(t, s, tag+" USB 32GB", a, b, c)
	gid := ok(s.Memberships(ctx, []int64{a}))[a].GroupID
	noErr(t, s.SetSplit(ctx, tier, false, map[int64]int{a: 50, b: 30, c: 20}))

	name := strings.ToLower(tag) + "  usb   32gb"
	res, err := rebuild(t, db, numbered([]SheetLine{
		{Group: name, Product: tag + " A 32GB"},
		{Group: name, Product: tag + " B 32GB"},
		{Group: name, Tier: "Cheap", Product: tag + " C 32GB"},
	}))
	noErr(t, err)
	if res.GroupsAdded != 0 || res.Moved != 1 || res.Placed != 0 {
		t.Fatalf("result %+v", res)
	}
	m := ok(s.Memberships(ctx, []int64{a, c}))
	if m[a].GroupID != gid || m[a].TierID != tier || m[a].Group != name {
		t.Fatalf("kept %+v", m[a])
	}
	if sh := shares(t, db, tier); sh[a]+sh[b] != 100 || sh[a] <= sh[b] {
		t.Fatalf("rescaled %v", sh)
	}
	if st := tierOfState(t, db, m[c].TierID); !st.Auto || !st.Split || !st.Active {
		t.Fatalf("new tier %+v", st)
	}
}

// ALT-12: two imports at once never mix: one waits, the last one wins whole.
func TestSheetTwoAtOnce_ALT12(t *testing.T) {
	s, db, tag := testStore(t)
	for _, n := range []string{"A", "B", "C", "D"} {
		product(t, db, tag+" "+n, "1", false, true)
	}
	one := numbered([]SheetLine{{Group: tag + " One", Product: tag + " A"}, {Group: tag + " One", Product: tag + " B"}})
	two := numbered([]SheetLine{{Group: tag + " Two", Product: tag + " C"}, {Group: tag + " Two", Product: tag + " D"}})
	errs := make(chan error, 2)
	for _, l := range [][]SheetLine{one, two} {
		go func() { _, err := rebuild(t, db, l); errs <- err }()
	}
	noErr(t, <-errs)
	noErr(t, <-errs)
	got := mine(ok(s.SheetLines(context.Background())), tag)
	if len(got) != 2 || got[0].Group != got[1].Group {
		t.Fatalf("mixed: %+v", got)
	}
}

// ALT-14: a sheet none of whose lines matches a product here (another shop's) is refused too.
func TestSheetNothingMatches_ALT14(t *testing.T) {
	s, db, tag := testStore(t)
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	b := product(t, db, tag+" Bakers 1kg", "5", false, true)
	groupTier(t, s, tag+" Rice 1kg", a, b)
	if _, err := rebuild(t, db, numbered([]SheetLine{{Group: tag + " X", Product: tag + " Nowhere"}})); err == nil {
		t.Fatal("a sheet matching nothing was taken")
	}
	if got := mine(ok(s.SheetLines(context.Background())), tag); len(got) != 2 {
		t.Fatalf("groups touched: %+v", got)
	}
}
