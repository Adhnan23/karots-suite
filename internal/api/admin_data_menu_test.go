package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The cashier menu sheet's routes (docs/rules/menu.md). Apply is proven on
// battle shops (internal/battle/cases_menu_test.go): here it would wipe the
// menu every other test in this package shares.
func TestAdminMenuSheet(t *testing.T) {
	te := newTestEnv(t)
	ph := func(n int64) string { return fmt.Sprintf("07%08d", (time.Now().UnixNano()+n)%100_000_000) }

	// MENU-11: only an admin.
	mgr := te.login(t, ph(1), "manager")
	if rec := te.do(http.MethodGet, "/api/v1/admin/data/menu/export", nil, mgr); rec.Code != http.StatusForbidden {
		t.Fatalf("manager export: %d", rec.Code)
	}
	if rec := te.upload("/api/v1/admin/data/menu/import?dry=1", "file", "m.csv", []byte("Group\nA\n"), mgr); rec.Code != http.StatusForbidden {
		t.Fatalf("manager import: %d", rec.Code)
	}
	ck := te.login(t, ph(2), "admin")

	rec := te.do(http.MethodGet, "/api/v1/admin/data/menu/export?format=csv", nil, ck)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Disposition"), "cashier-menu.csv") {
		t.Fatalf("export: %d %q", rec.Code, rec.Header().Get("Content-Disposition"))
	}
	if first := strings.SplitN(strings.TrimPrefix(rec.Body.String(), "\ufeff"), "\n", 2)[0]; strings.TrimSpace(first) != "Group,Emoji,Active,Product,Barcode,Product emoji" {
		t.Fatalf("export header %q", first)
	}

	// MENU-6: a sheet with no menu lines is refused, never "remove everything".
	for _, body := range []string{"Group,Product\n", "Group,Product\n,\n , \n"} {
		rec := te.upload("/api/v1/admin/data/menu/import?dry=1", "file", "m.csv", []byte(body), ck)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "the sheet has no menu lines") {
			t.Fatalf("empty sheet %q: %d %s", body, rec.Code, rec.Body.String())
		}
	}

	// MENU-3: the preview changes nothing. Headers in any case and order (AI
	// rewrites them), a blank line in the middle, a product that isn't here.
	count := func() (n int) {
		te.db.Get(&n, `SELECT count(*) FROM product_groups`) //nolint:errcheck
		return
	}
	before := count()
	sheet := "PRODUCT,group,Item Emoji\n,Menu sheet test A,\n,,\nNo such product here,Menu sheet test A > B,🥤\n"
	rec = te.upload("/api/v1/admin/data/menu/import?dry=1", "file", "m.csv", []byte(sheet), ck)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	var res MenuImportResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Applied || res.Groups != 1 || res.Items != 0 || len(res.Skipped) != 1 || !strings.HasPrefix(res.Skipped[0], "line 4: ") {
		t.Fatalf("preview result %+v", res)
	}
	if after := count(); after != before {
		t.Fatalf("the preview changed the menu: %d groups → %d", before, after)
	}
}

// MENU-16: a group's name can't hold ">" — the sheet reads it as a path, so
// "Soft > Hard" would come back as two groups.
func TestGroupNameRefusesPathSeparator(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	if rec := te.do(http.MethodPost, "/api/v1/admin/groups", GroupRequest{Name: "Soft > Hard"}, ck); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("create with '>': %d %s", rec.Code, rec.Body.String())
	}
	rec := te.do(http.MethodPost, "/api/v1/admin/groups", GroupRequest{Name: "Menu sep test"}, ck)
	var g Option
	json.Unmarshal(rec.Body.Bytes(), &g) //nolint:errcheck
	t.Cleanup(func() { te.db.Exec(`DELETE FROM product_groups WHERE id = $1`, g.ID) }) //nolint:errcheck
	if rec := te.do(http.MethodPut, fmt.Sprintf("/api/v1/admin/groups/%d", g.ID), GroupRequest{Name: "A>B"}, ck); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
		t.Fatalf("rename with '>': %d %s", rec.Code, rec.Body.String())
	}
}
