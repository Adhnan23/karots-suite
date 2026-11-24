package products

import (
	"context"
	"testing"
)

// REV-1: Items to review lists core's flagged products and every product a plugin adds, once each, with the
// plugin's notes beside it; the count agrees. REV-2: Mark reviewed clears core's flag and asks the plugins to clear.
func TestReviewSourceAddsNotesAndClears(t *testing.T) {
	conn := ppTestDB(t)
	defer conn.Close()
	ctx := context.Background()
	svc := NewService(conn)
	all, err := NewRepository(conn).List(ctx, ListQuery{Page: 1, Limit: 2})
	if err != nil || len(all) < 2 {
		t.Skip("needs two products")
	}
	flagged, pluginOnly := all[0].ID, all[1].ID
	conn.MustExec(`UPDATE products SET needs_review = true WHERE id = $1`, flagged)
	defer conn.MustExec(`UPDATE products SET needs_review = false WHERE id = ANY($1)`, []int64{flagged, pluginOnly})
	before, _ := svc.CountNeedsReview(ctx)

	notes := map[int64][]ReviewNote{
		flagged:    {{Plugin: "test", Text: "also the plugin's"}},
		pluginOnly: {{Plugin: "test", Text: "From the test"}, {Plugin: "other", Text: "Second note"}},
	}
	var cleared []int64
	ReviewSource = func(context.Context) (map[int64][]ReviewNote, error) { return notes, nil }
	ReviewClear = func(_ context.Context, id int64) error { cleared = append(cleared, id); delete(notes, id); return nil }
	defer func() { ReviewSource, ReviewClear = nil, nil }()

	rows, err := svc.NeedsReview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]int{}
	for _, r := range rows {
		seen[r.ID]++
		if want := notes[r.ID]; len(r.ReviewNotes) != len(want) {
			t.Fatalf("product %d notes %+v, want %+v", r.ID, r.ReviewNotes, want)
		}
	}
	if seen[flagged] != 1 || seen[pluginOnly] != 1 {
		t.Fatalf("each product once: %v", seen)
	}
	if n, _ := svc.CountNeedsReview(ctx); n != before+1 {
		t.Fatalf("count %d, want %d (the plugin's product added, the flagged one not twice)", n, before+1)
	}

	if err := svc.MarkReviewed(ctx, pluginOnly); err != nil {
		t.Fatal(err)
	}
	if len(cleared) != 1 || cleared[0] != pluginOnly {
		t.Fatalf("plugins asked to clear %v", cleared)
	}
	if n, _ := svc.CountNeedsReview(ctx); n != before {
		t.Fatalf("after review count %d, want %d", n, before)
	}
}
