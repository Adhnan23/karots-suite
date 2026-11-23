package alternatives

import (
	"context"
	"reflect"
	"strings"
	"testing"
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
