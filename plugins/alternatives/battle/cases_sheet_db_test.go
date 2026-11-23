package battle

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/alternatives"
)

// The sheet (ALT-9…18). These tests are NOT parallel: an import replaces every
// group, and Go runs serial tests before any t.Parallel one starts.

const sheetURL = "/api/v1/x/alternatives/sheet"

func export(t *testing.T, c *ltest.Client) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Do("GET", sheetURL+"?format=csv", nil, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func importSheet(c *ltest.Client, csv string, dry bool) (alternatives.SheetImportDTO, error) {
	var r alternatives.SheetImportDTO
	q := ""
	if dry {
		q = "?dry=1"
	}
	err := c.Upload(sheetURL+"/import"+q, "alternatives.csv", []byte(csv), &r)
	return r, err
}

func manager(t *testing.T, s *ltest.Shop) *ltest.Client {
	t.Helper()
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/users", map[string]any{"name": "Sheet Manager", "phone": "0770000099", "role": "manager", "pin": "4321"}, nil))
	s.Must(t, func() error { _, err := s.DB.Exec(`UPDATE users SET must_change_pin = false WHERE phone = '0770000099'`); return err }())
	c, err := ltest.NewClient(s.H, "0770000099", "4321", "sheet-manager")
	s.Must(t, err)
	return c
}

// ALT-9, ALT-17, ALT-18: export → import those bytes → export: the same sheet; a
// disabled member stays; nothing outside Alternatives moves; the auditor agrees.
func TestALT9_SheetRoundTrip(t *testing.T) {
	s := ltest.Shared(t).Shop
	gid, t1 := group(t, s, "Sheet USB 32GB")
	t2 := tier(t, s, gid, "Cheap")
	a := s.Product(t, "Sheet SanDisk 32GB", "1800", "5", nil)
	b := s.Product(t, "Sheet Kingston 32GB", "1750", "0", nil)
	g := s.Product(t, "Sheet Generic 32GB", "900", "5", nil)
	s.Must(t, join(s, t1, a, false))
	s.Must(t, join(s, t1, b, false))
	s.Must(t, join(s, t2, g, false))
	s.Must(t, func() error { _, err := s.DB.Exec(`UPDATE products SET is_active = false WHERE id = $1`, b); return err }())

	var empty int // an empty group has no line, so it doesn't come back (ALT-9)
	s.Must(t, s.DB.Get(&empty, `SELECT count(*) FROM alt_groups g WHERE NOT EXISTS (
		SELECT 1 FROM alt_tiers t JOIN alt_members m ON m.tier_id = t.id WHERE t.group_id = g.id)`))
	before := ltest.State(t, s.DB)
	first := export(t, s.Admin)
	r, err := importSheet(s.Admin, string(first), false)
	s.Must(t, err)
	if len(r.Skipped) != 0 || r.GroupsRemoved != empty || r.Moved+r.Placed+r.TakenOut != 0 {
		t.Errorf("ALT-9: an unchanged sheet changed things: %+v", r)
	}
	if again := export(t, s.Admin); !bytes.Equal(again, first) {
		t.Errorf("ALT-9: round trip differs:\n%s\n---\n%s", first, again)
	}
	if !strings.Contains(string(first), "Sheet Kingston 32GB") {
		t.Errorf("ALT-18: the disabled member isn't exported:\n%s", first)
	}
	ltest.Untouched(t, before, ltest.State(t, s.DB), append(ltest.MoneyAndStock, "products"))
	s.Audit(t)
}

// ALT-10, ALT-11, ALT-13: the preview writes nothing and reports what Apply then does;
// a dropped group goes; an unknown product is skipped with its line.
func TestALT10_SheetWholeSetWithPreview(t *testing.T) {
	s := ltest.Shared(t).Shop
	_, keep := group(t, s, "Sheet Rice 1kg")
	_, gone := group(t, s, "Sheet Cable 1m")
	a := s.Product(t, "Sheet Anchor rice 1kg", "300", "5", nil)
	b := s.Product(t, "Sheet Bakers rice 1kg", "310", "5", nil)
	c := s.Product(t, "Sheet Cable A 1m", "500", "5", nil)
	s.Product(t, "Sheet Cable B 1m", "500", "5", nil)
	s.Must(t, join(s, keep, a, false))
	s.Must(t, join(s, gone, c, false))
	csv := "Group,Tier,Product,Barcode\n" +
		"Sheet Rice 1kg,Standard,Sheet Anchor rice 1kg,\n" +
		"Sheet Rice 1kg,Standard,Sheet Bakers rice 1kg,\n" +
		"Sheet Rice 1kg,Standard,Sheet Nowhere,\n"
	snap := export(t, s.Admin)
	pre, err := importSheet(s.Admin, csv, true)
	s.Must(t, err)
	if !bytes.Equal(export(t, s.Admin), snap) {
		t.Errorf("ALT-11: the preview wrote")
	}
	if len(pre.Skipped) != 1 || !strings.HasPrefix(pre.Skipped[0], "line 4:") {
		t.Errorf("ALT-13: skipped %v", pre.Skipped)
	}
	done, err := importSheet(s.Admin, csv, false)
	s.Must(t, err)
	pre.Applied = true
	if fmt.Sprint(done) != fmt.Sprint(pre) {
		t.Errorf("ALT-11: apply %+v, preview %+v", done, pre)
	}
	if !contains(done.Removed, "Sheet Cable 1m") {
		t.Errorf("ALT-10: removed %v", done.Removed)
	}
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM alt_members WHERE product_id = ANY(ARRAY[$1, $2]::bigint[])`, b, c))
	if n != 1 {
		t.Errorf("ALT-10: %d of Bakers (in) and Cable A (out) grouped, want 1", n)
	}
	s.Audit(t)
}

// ALT-16: a manager exports but can't import.
func TestALT16_SheetImportIsAdminOnly(t *testing.T) {
	s := ltest.Shared(t).Shop
	m := manager(t, s)
	export(t, m)
	_, err := importSheet(m, "Group,Tier,Product,Barcode\nX,,Y,\n", true)
	if se, ok := err.(*ltest.StatusError); !ok || se.Code != 403 {
		t.Errorf("ALT-16: a manager's import got %v", err)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
