package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/plugin"
)

// fakeSheet is a stand-in plugin's sheet columns: "Fake Grade" holds a value per
// product; import records what it was handed. Registered once (hooks are global).
var (
	fakeSheetOnce sync.Once
	fakeSheetMu   sync.Mutex
	fakeGrades    = map[int64]string{}
)

func useFakeSheet() {
	fakeSheetOnce.Do(func() {
		plugin.NewRegistry(plugin.Core{}, plugin.NewMux(), nil).AddProductSheetColumns(plugin.ProductSheetColumns{
			Key: "fake", Label: "Fake fields",
			Columns: func(context.Context) ([]string, error) { return []string{"Fake Grade"}, nil },
			Export: func(_ context.Context, ids []int64) (map[int64][]string, error) {
				fakeSheetMu.Lock()
				defer fakeSheetMu.Unlock()
				out := map[int64][]string{}
				for _, id := range ids {
					if g, ok := fakeGrades[id]; ok {
						out[id] = []string{g}
					}
				}
				return out, nil
			},
			Import: func(_ context.Context, productID, _ int64, cells map[string]string) (string, error) {
				fakeSheetMu.Lock()
				defer fakeSheetMu.Unlock()
				if cells["Fake Grade"] == "bad" {
					return "Fake Grade: not a grade", nil
				}
				fakeGrades[productID] = cells["Fake Grade"]
				return "", nil
			},
		})
	})
}

func TestProductSheetOptions(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009534", "manager")
	tag := fmt.Sprintf("TSHX%d", time.Now().UnixNano()%1_000_000)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM products WHERE name LIKE $1`, tag+"%")   //nolint:errcheck
		te.db.Exec(`DELETE FROM categories WHERE name LIKE $1`, tag+"%") //nolint:errcheck
	})
	export := func(qs string) string { return te.do(http.MethodGet, "/api/v1/admin/products/export"+qs, nil, ck).Body.String() }
	header := func(body string) string { return strings.SplitN(body, "\n", 2)[0] }
	line := func(body, name string) string {
		for _, l := range strings.Split(body, "\n") {
			if strings.HasPrefix(l, name+",") {
				return l
			}
		}
		return ""
	}
	imp := func(qs, csv string) EntityImportResult {
		return decode[EntityImportResult](t, te.upload("/api/v1/admin/products/import"+qs, "file", "p.csv", []byte(csv), ck))
	}

	// Core only: no extras offered, defaults export exactly the core columns.
	if l := decode[[]SheetExtra](t, te.do(http.MethodGet, "/api/v1/admin/products/sheet-extras", nil, ck)); len(l) != 0 {
		t.Fatalf("extras before any plugin: %+v", l)
	}
	if h := header(export("")); h != strings.Join(productColumns, ",") {
		t.Fatalf("default header changed: %s", h)
	}

	// Skip disabled off: a disabled product comes with active=no, and a re-import keeps it disabled.
	imp("", "name,category,selling_price\n"+tag+" Off,"+tag+" Cat,10\n"+tag+" On,"+tag+" Cat,12\n")
	te.db.Exec(`UPDATE products SET is_active=false WHERE name=$1`, tag+" Off") //nolint:errcheck
	if line(export(""), tag+" Off") != "" {
		t.Fatal("a disabled product in the default export")
	}
	all := export("?disabled=1")
	if !strings.HasSuffix(header(all), ",active") || !strings.HasSuffix(line(all, tag+" Off"), ",no") || !strings.HasSuffix(line(all, tag+" On"), ",yes") {
		t.Fatalf("disabled export: %s / %s / %s", header(all), line(all, tag+" Off"), line(all, tag+" On"))
	}
	res := imp("", header(all)+"\n"+line(all, tag+" Off")+"\n")
	var n int
	var active bool
	te.db.Get(&n, `SELECT count(*) FROM products WHERE name=$1`, tag+" Off")              //nolint:errcheck
	te.db.Get(&active, `SELECT is_active FROM products WHERE name=$1 LIMIT 1`, tag+" Off") //nolint:errcheck
	if res.Updated != 1 || n != 1 || active {
		t.Fatalf("re-import of a disabled row: %+v, %d rows, active %v", res, n, active)
	}
	imp("", "name,active\n"+tag+" Off,yes\n")
	te.db.Get(&active, `SELECT is_active FROM products WHERE name=$1`, tag+" Off") //nolint:errcheck
	if !active {
		t.Fatal("active=yes did not re-enable")
	}

	// A plugin's sheet columns, only when asked for.
	useFakeSheet()
	if l := decode[[]SheetExtra](t, te.do(http.MethodGet, "/api/v1/admin/products/sheet-extras", nil, ck)); len(l) != 1 || l[0].Key != "fake" || l[0].Label != "Fake fields" {
		t.Fatalf("extras: %+v", l)
	}
	if h := header(export("")); h != strings.Join(productColumns, ",") {
		t.Fatalf("extras leaked into the default export: %s", h)
	}
	if h := header(te.do(http.MethodGet, "/api/v1/admin/products/import/template?extras=fake", nil, ck).Body.String()); !strings.HasSuffix(h, ",Fake Grade") {
		t.Fatalf("template with extras: %s", h)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/products/export?extras=nope", nil, ck); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown extra: %d", rec.Code)
	}
	// Ignored without the toggle; imported (with its note) with it.
	imp("", "name,Fake Grade\n"+tag+" On,A\n")
	var onID int64
	te.db.Get(&onID, `SELECT id FROM products WHERE name=$1`, tag+" On") //nolint:errcheck
	fakeSheetMu.Lock()
	_, leaked := fakeGrades[onID]
	fakeSheetMu.Unlock()
	if leaked {
		t.Fatal("custom column imported without the toggle")
	}
	res = imp("?extras=fake", "name,Fake Grade\n"+tag+" On,A\n"+tag+" Off,bad\n")
	if res.Updated != 2 || len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "Line 3: Fake Grade: not a grade") {
		t.Fatalf("import with extras: %+v", res)
	}
	if l := line(export("?extras=fake"), tag+" On"); !strings.HasSuffix(l, ",A") {
		t.Fatalf("export with extras: %s", l)
	}
}

func TestProductSheetNameTaken(t *testing.T) {
	for name, want := range map[string]bool{
		"Barcode": true, " price ": true, "SKU": true, "Selling Price": true, "name local": true, "Active": true,
		"Model no": false, "Grade": false, "Size": false,
	} {
		if got := ProductSheetNameTaken(name); got != want {
			t.Errorf("%q: got %v, want %v", name, got, want)
		}
	}
}
