package productplus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Fields move one place up or down; the ends stay put; a missing field is not found.
func TestMoveField(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, l := range []string{"A", "B", "C"} {
		f.field(t, Field{Label: l, Type: "text"})
	}
	order := func() []int64 {
		fs, err := f.s.Fields(ctx, true)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]int64, len(fs))
		for i, x := range fs {
			ids[i] = x.ID
		}
		return ids
	}
	o := order()
	if err := f.s.MoveField(ctx, o[0], false); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[0] != o[1] || got[1] != o[0] {
		t.Errorf("first moved down: %v from %v", got, o)
	}
	if err := f.s.MoveField(ctx, o[0], true); err != nil {
		t.Fatal(err)
	}
	if got := order(); !slices.Equal(got, o) {
		t.Errorf("moved back up: %v, want %v", got, o)
	}
	last := o[len(o)-1]
	if err := f.s.MoveField(ctx, last, false); err != nil || !slices.Equal(order(), o) {
		t.Errorf("the last moved down: %v %v", err, order())
	}
	if err := f.s.MoveField(ctx, o[0], true); err != nil || !slices.Equal(order(), o) {
		t.Errorf("the first moved up: %v %v", err, order())
	}
	if err := f.s.MoveField(ctx, -1, true); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("a missing field: %v", err)
	}
}

// A label becomes its key: lowercase letters and digits, spaces / dashes / underscores
// as one "_", anything else dropped; the same label again gets _2, _3.
func TestKeys(t *testing.T) {
	for in, want := range map[string]string{" Shelf Life-2_x ": "shelf_life_2_x", "AZaz09": "azaz09", "€!": "field", "a@b`{": "ab"} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
	f := setup(t)
	var keys []string
	for range 3 {
		keys = append(keys, f.field(t, Field{Label: "Twin", Type: "text"}).Key)
	}
	base := keys[0]
	if keys[1] != base+"_2" || keys[2] != base+"_3" {
		t.Errorf("keys %v", keys)
	}
	if truncate("abcd", 4) != "abcd" || truncate("abcde", 4) != "abcd…" {
		t.Errorf("truncate at the edge: %q %q", truncate("abcd", 4), truncate("abcde", 4))
	}
}

// The products sheet carries each field's value, yes / no for a yes/no, blank where unset.
func TestSheetExport(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p1 := f.product(t, "S1", f.parent)
	p2 := f.product(t, "S2", f.parent)
	yes := f.field(t, Field{Label: "Boxed", Type: "bool"})
	note := f.field(t, Field{Label: "Note", Type: "text"})
	f.set(t, p1, yes, "yes")
	f.set(t, p1, note, "blue")
	if out, err := f.s.SheetExport(ctx, nil); err != nil || out != nil {
		t.Errorf("no products: %v %v", out, err)
	}
	cols, _ := f.s.SheetColumns(ctx)
	out, err := f.s.SheetExport(ctx, []int64{p1, p2})
	if err != nil {
		t.Fatal(err)
	}
	cell := func(pid int64, fl Field) string {
		i := slices.Index(cols, fl.Label)
		if i < 0 || out[pid] == nil {
			return "?"
		}
		return out[pid][i]
	}
	if cell(p1, yes) != "yes" || cell(p1, note) != "blue" || cell(p2, yes) != "no" || cell(p2, note) != "" {
		t.Errorf("sheet: p1 %q %q, p2 %q %q", cell(p1, yes), cell(p1, note), cell(p2, yes), cell(p2, note))
	}
}

// A type change rewrites values to the new type's form (text "yes" → a yes/no "1"), and
// a change to a dropdown counts what its options lose.
func TestTypeChangeRewrites(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	p1 := f.product(t, "T1", f.parent)
	p2 := f.product(t, "T2", f.parent)
	note := f.field(t, Field{Label: "Colour", Type: "text"})
	f.set(t, p1, note, "Red")
	f.set(t, p2, note, "Blue")
	if im, err := f.s.Impact(ctx, note.ID, "select", []string{"Red"}); err != nil || im.Lossy != 1 {
		t.Errorf("to a dropdown of Red only: %+v %v", im, err)
	}
	f.set(t, p1, note, "yes")
	n := note
	n.Type = "bool"
	if err := f.s.UpdateField(ctx, n, nil, UpdateOpts{ConfirmLoss: true}); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.stored(p1, note); v != "1" {
		t.Errorf("text yes became %q", v)
	}
}

// A dropdown choice later taken off the list stays on a product saved unchanged, and
// the form marks it removed; a new value must be on the list.
func TestRemovedChoiceKept(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	pid := f.product(t, "R1", f.parent)
	size := f.field(t, Field{Label: "Size", Type: "select", Options: []string{"S", "M"}})
	f.set(t, pid, size, "S")
	size.Options = []string{"M"}
	if err := f.s.UpdateField(ctx, size, nil, UpdateOpts{}); err != nil {
		t.Fatal(err)
	}
	rows, _ := f.s.FormFields(ctx, pid, f.parent, "form")
	for _, r := range rows {
		if r.Field.ID == size.ID && (!r.Removed || r.Value != "S") {
			t.Errorf("form row %+v", r)
		}
	}
	if err := f.s.SaveProduct(ctx, pid, f.parent, "form", map[string]string{size.Key: "S"}); err != nil {
		t.Errorf("saved unchanged: %v", err)
	}
	if err := f.s.SaveProduct(ctx, pid, f.parent, "form", map[string]string{size.Key: "L"}); err == nil {
		t.Errorf("a value off the list was taken")
	}
}

// A field's default is checked like a value: a yes/no default becomes "1", a bad number is refused.
func TestAPIDefaultsAndDisabled(t *testing.T) {
	f := setup(t)
	_, do := testAPI(t, f)
	rec := do("POST", "/fields", fmt.Sprintf(`{"label":"%s Boxed","type":"bool","defaultValue":"yes"}`, f.tag))
	if rec.Code != 201 {
		t.Fatalf("bool with a default: %d %s", rec.Code, rec.Body)
	}
	made := decode[FieldDTO](t, rec)
	if made.DefaultValue != "1" {
		t.Errorf("bool default stored %q", made.DefaultValue)
	}
	if rec := do("POST", "/fields", fmt.Sprintf(`{"label":"%s Weight","type":"number","defaultValue":"heavy"}`, f.tag)); rec.Code != 422 {
		t.Errorf("a number default that isn't one: %d", rec.Code)
	}
	if rec := do("POST", "/fields", fmt.Sprintf(`{"label":"%s Flag","type":"bool","defaultValue":"maybe"}`, f.tag)); rec.Code != 422 {
		t.Errorf("a yes/no default that isn't one: %d", rec.Code)
	}
	if rec := do("POST", fmt.Sprintf("/fields/%d/active", made.ID), `{"active":false}`); rec.Code != 204 {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	has := func(q string) bool {
		for _, x := range decode[[]FieldDTO](t, do("GET", "/fields"+q, "")) {
			if x.ID == made.ID {
				return true
			}
		}
		return false
	}
	if rec := do("PUT", "/fields/999999999", fmt.Sprintf(`{"label":"%s Gone","type":"text"}`, f.tag)); rec.Code != 404 {
		t.Errorf("editing a missing field: %d", rec.Code)
	}
	if rec := do("POST", "/fields/999999999/move", `{"dir":"up"}`); rec.Code != 404 {
		t.Errorf("moving a missing field: %d", rec.Code)
	}
	if rec := do("POST", "/fields", fmt.Sprintf(`{"label":"%s Plain","type":"text"}`, f.tag)); !strings.Contains(rec.Body.String(), `"options":[]`) {
		t.Errorf("a text field's options should be [] for the screens: %s", rec.Body)
	}
	if has("") || !has("?disabled=1") {
		t.Errorf("a disabled field: listed %v, with disabled=1 %v", has(""), has("?disabled=1"))
	}
}

// The grid pages by 50.
func TestAPIGridPages(t *testing.T) {
	f := setup(t)
	_, do := testAPI(t, f)
	fd := f.field(t, Field{Label: "Page", Type: "text"})
	for i := range 51 {
		f.product(t, fmt.Sprintf("G%02d", i), f.other)
	}
	g := decode[GridDTO](t, do("GET", fmt.Sprintf("/grid?fields=%d&categoryId=%d&page=2", fd.ID, f.other), ""))
	if g.Total != 51 || len(g.Rows) != 1 {
		t.Errorf("page 2: %d rows of %d", len(g.Rows), g.Total)
	}
}
