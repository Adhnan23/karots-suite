package battle

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/backup"
	"karots-suite/internal/features/productgroups"
	"karots-suite/internal/ledgercheck/ltest"
)

// The cashier menu sheet on real shops (docs/rules/menu.md).

type menuResult struct {
	Groups, Items, RemovedGroups, RemovedItems int
	Skipped                                    []string
	Applied                                    bool
}

func (s *shop) group(t *testing.T, name, emoji string, parent int64, items ...int64) int64 {
	var g struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/groups", map[string]any{"name": name, "emoji": emoji, "parentId": parent}, &g))
	for _, p := range items {
		s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/groups/%d/items", g.ID), map[string]any{"productId": p}, nil))
	}
	return g.ID
}

func (s *shop) menuSheet(t *testing.T) []byte {
	var b []byte
	s.must(t, s.admin.Do("GET", "/api/v1/admin/data/menu/export?format=csv", nil, &b))
	return b
}

func (s *shop) importMenu(t *testing.T, sheet []byte, dry bool) menuResult {
	var r menuResult
	path := "/api/v1/admin/data/menu/import"
	if dry {
		path += "?dry=1"
	}
	s.must(t, s.admin.upload(path, "menu.csv", sheet, &r))
	return r
}

// menu is the shop's menu as the sheet sees it, line numbers left out.
func (s *shop) menu(t *testing.T) []productgroups.MenuLine {
	ls, err := productgroups.NewRepository(s.db).MenuLines(context.Background())
	s.must(t, err)
	for i := range ls {
		ls[i].Line = 0
	}
	return ls
}

func sheetOf(rows ...[]string) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.Write([]string{"Group", "Emoji", "Active", "Product", "Barcode", "Product emoji"}) //nolint:errcheck
	w.WriteAll(rows)                                                                     //nolint:errcheck
	return b.Bytes()
}

// MENU-1, 2, 3, 7: an exported menu comes back unchanged; an edited sheet
// becomes the whole menu in its order; the preview changed nothing and said
// exactly what Apply then did.
func TestMENU2ReplaceOnARealShop(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	coke := s.stocked(t, "Menu Coke", s.unitPcs, 0, nil)
	sprite := s.stocked(t, "Menu Sprite", s.unitPcs, 0, nil)
	bun := s.stocked(t, "Menu Bun", s.unitPcs, 0, nil)
	drinks := s.group(t, "Drinks", "🥤", 0, coke, sprite)
	s.group(t, "Fizzy", "", drinks, sprite)
	s.group(t, "Bakery", "🥐", 0, bun)

	before := s.menu(t)
	if r := s.importMenu(t, s.menuSheet(t), false); r.RemovedGroups != 0 || r.RemovedItems != 0 || len(r.Skipped) != 0 {
		t.Fatalf("MENU-1: re-importing the export: %+v", r)
	}
	if after := s.menu(t); !reflect.DeepEqual(after, before) {
		t.Fatalf("MENU-1: the round trip changed the menu:\n got %+v\nwant %+v", after, before)
	}

	edited := sheetOf(
		[]string{"Bakery", "🥐", "", "", "", ""},
		[]string{"Bakery", "", "", "Menu Bun", "", ""},
		[]string{"Drinks", "🥤", "", "", "", ""},
		[]string{"Drinks", "", "", "Menu Sprite", "", ""},
		[]string{"Drinks", "", "", "Menu Coke", "", "🧊"},
	)
	untouched := ltest.State(t, s.db)
	preview := s.importMenu(t, edited, true)
	if changed := ltest.Changed(untouched, ltest.State(t, s.db)); len(changed) != 0 {
		t.Fatalf("MENU-3: the preview changed %v", changed)
	}
	applied := s.importMenu(t, edited, false)
	if !applied.Applied || preview.Applied {
		t.Fatalf("applied flags: preview %v, apply %v", preview.Applied, applied.Applied)
	}
	applied.Applied = false
	if !reflect.DeepEqual(preview, applied) {
		t.Fatalf("MENU-3: the preview said %+v, Apply did %+v", preview, applied)
	}
	if applied.Groups != 2 || applied.Items != 3 || applied.RemovedGroups != 1 || applied.RemovedItems != 1 {
		t.Fatalf("result %+v", applied)
	}
	want := []productgroups.MenuLine{
		{Group: "Bakery", Emoji: "🥐"},
		{Group: "Bakery", Product: "Menu Bun"},
		{Group: "Drinks", Emoji: "🥤"},
		{Group: "Drinks", Product: "Menu Sprite"},
		{Group: "Drinks", Product: "Menu Coke", ProductEmoji: "🧊"},
	}
	if got := s.menu(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("MENU-2, 7: the menu after Apply:\n got %+v\nwant %+v", got, want)
	}
}

// MENU-5: one shop's menu fits another whose products have other ids — found
// by barcode (even renamed), then by name; a product it hasn't got is listed.
func TestMENU5AnotherShopsMenu(t *testing.T) {
	t.Parallel()
	a, b := newShop(t), newShop(t)
	x := a.stocked(t, "Menu X", a.unitPcs, 0, map[string]any{"barcode": "MENU-X-1"})
	y := a.stocked(t, "Menu Y", a.unitPcs, 0, nil)
	w := a.stocked(t, "Menu W", a.unitPcs, 0, nil)
	a.group(t, "Shelf", "", 0, x, y, w)

	by := b.stocked(t, "Menu Y", b.unitPcs, 0, nil)
	b.stocked(t, "Padding so ids differ", b.unitPcs, 0, nil)
	bx := b.stocked(t, "Menu X renamed here", b.unitPcs, 0, map[string]any{"barcode": "MENU-X-1"})
	r := b.importMenu(t, a.menuSheet(t), false)
	if r.Items != 2 || len(r.Skipped) != 1 || !strings.Contains(r.Skipped[0], "no product with this barcode or name") {
		t.Fatalf("MENU-5: %+v", r)
	}
	var got []int64
	b.must(t, b.db.Select(&got, `SELECT product_id FROM product_group_items ORDER BY sort_order`))
	if !slices.Equal(got, []int64{bx, by}) {
		t.Fatalf("MENU-5: shop B's items %v, want %v", got, []int64{bx, by})
	}
}

// MENU-12: a menu import changes only the menu; the auditor agrees.
func TestMENU12NothingElseMoves(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	s.group(t, "Old", "", 0, s.prod["plain"])
	before := ltest.State(t, s.db)
	s.importMenu(t, sheetOf([]string{"New", "⭐", "", "", "", ""}), false)
	for _, tb := range ltest.Changed(before, ltest.State(t, s.db)) {
		if tb != "product_groups" && tb != "product_group_items" {
			t.Errorf("MENU-12: a menu import changed %s", tb)
		}
	}
	s.audit(t)
}

// MENU-13: a merged product's place moves to the product it merged into; a
// disabled one stays in the menu and survives a round trip.
func TestMENU13MergeAndDisable(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	twin := s.stocked(t, "Menu Twin", s.unitPcs, 0, nil)
	keep := s.stocked(t, "Menu Keep", s.unitPcs, 0, nil)
	off := s.stocked(t, "Menu Off", s.unitPcs, 0, nil)
	s.group(t, "Shelf", "", 0, twin, off)
	s.must(t, s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", twin, keep), nil, nil))
	s.must(t, s.admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", off), nil, nil))
	want := []productgroups.MenuLine{{Group: "Shelf"}, {Group: "Shelf", Product: "Menu Keep"}, {Group: "Shelf", Product: "Menu Off"}}
	if got := s.menu(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("MENU-13 after merge + disable:\n got %+v\nwant %+v", got, want)
	}
	if r := s.importMenu(t, s.menuSheet(t), false); len(r.Skipped) != 0 {
		t.Fatalf("MENU-13: the round trip skipped %v", r.Skipped)
	}
	if got := s.menu(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("MENU-13 after the round trip:\n got %+v\nwant %+v", got, want)
	}
}

// MENU-14: a setup-only backup carries the menu and brings it back.
func TestMENU14SetupBackupCarriesMenu(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	g := s.group(t, "Drinks", "🥤", 0, s.prod["plain"])
	s.group(t, "Cold", "🧊", g)
	want := s.menu(t)
	var raw []byte
	s.must(t, s.admin.Do("GET", "/api/v1/admin/data/backup?setupOnly=1", nil, &raw))
	cp := ltest.NewDB(t)
	if _, err := backup.Restore(context.Background(), cp, bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	got, err := productgroups.NewRepository(cp).MenuLines(context.Background())
	s.must(t, err)
	for i := range got {
		got[i].Line = 0
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MENU-14: restored menu\n got %+v\nwant %+v", got, want)
	}
}
