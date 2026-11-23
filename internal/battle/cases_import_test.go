package battle

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// The products sheet's preview and all-or-nothing import (docs/rules/stock.md, IMP).

type importResult struct {
	Created, Updated, Skipped int
	Errors                    []struct{ Line int; Message string }
	Notes                     []string
	Applied                   bool
	Changes                   []struct {
		Line    int
		Product string
		New     bool
		Fields  []struct{ Field, From, To string }
	}
}

func csvOf(rows ...[]string) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.WriteAll(rows) //nolint:errcheck
	return b.Bytes()
}

func (s *shop) importProducts(t *testing.T, sheet []byte, dry bool) importResult {
	t.Helper()
	var r importResult
	path := "/api/v1/admin/products/import"
	if dry {
		path += "?dry=1"
	}
	s.must(t, s.admin.upload(path, "products.csv", sheet, &r))
	return r
}

// refOf is a product's ref as its own export writes it (Include ref).
func (s *shop) refOf(t *testing.T, name string) string {
	t.Helper()
	var b []byte
	s.must(t, s.admin.Do("GET", "/api/v1/admin/products/export?format=csv&ref=1", nil, &b))
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(b, []byte("\ufeff")))).ReadAll()
	s.must(t, err)
	ref, nm := -1, -1
	for i, h := range rows[0] {
		switch strings.ToLower(h) {
		case "ref":
			ref = i
		case "name":
			nm = i
		}
	}
	for _, r := range rows[1:] {
		if r[nm] == name {
			return r[ref]
		}
	}
	t.Fatalf("no export row for %q", name)
	return ""
}

// IMP-1: the preview runs the real import and changes nothing anywhere — not
// the renamed product, the new one, its new category and supplier, its extra
// barcode or its opening stock.
func TestIMP1PreviewChangesNothing(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.stocked(t, "Imp Old Name", s.unitPcs, 0, nil)
	ref := s.refOf(t, "Imp Old Name")
	sheet := csvOf(
		[]string{"ref", "name", "category", "supplier", "selling_price"},
		[]string{ref, "Imp New Name", "Imp Cat > Imp Sub", "Imp Supplier", "55"},
	)
	sheet2 := csvOf(
		[]string{"name", "category", "supplier", "barcode", "extra_barcodes", "opening_qty", "cost_price", "selling_price"},
		[]string{"Imp Brand New", "Imp Cat 2", "Imp Supplier 2", "IMP-111", "IMP-222", "5", "10", "20"},
	)
	before := ltest.State(t, s.db)
	for _, sh := range [][]byte{sheet, sheet2} {
		r := s.importProducts(t, sh, true)
		if r.Applied || r.Created+r.Updated != 1 || r.Skipped != 0 {
			t.Fatalf("preview %+v", r)
		}
	}
	if changed := ltest.Changed(before, ltest.State(t, s.db)); len(changed) != 0 {
		t.Fatalf("IMP-1: the preview changed %v", changed)
	}
	if r := s.importProducts(t, sheet, false); !r.Applied || r.Updated != 1 {
		t.Fatalf("apply %+v", r)
	}
}

// IMP-3: a line that fails after it made a category and a supplier leaves
// neither behind; the good lines land.
func TestIMP3FailedLineLeavesNothing(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	r := s.importProducts(t, csvOf(
		[]string{"ref", "name", "category", "supplier", "selling_price"},
		[]string{"", "Imp Good", "Imp Fine Cat", "", "10"},
		[]string{"DEADBEEF-1", "Imp Bad", "Imp Ghost Cat", "Imp Ghost Supplier", "10"},
	), false)
	if r.Created != 1 || r.Skipped != 1 || len(r.Errors) != 1 || r.Errors[0].Line != 3 {
		t.Fatalf("result %+v", r)
	}
	var cats, sups int
	s.must(t, s.db.Get(&cats, `SELECT count(*) FROM categories WHERE name = 'Imp Ghost Cat'`))
	s.must(t, s.db.Get(&sups, `SELECT count(*) FROM suppliers WHERE name = 'Imp Ghost Supplier'`))
	if cats != 0 || sups != 0 {
		t.Fatalf("IMP-3: the failed line left %d categories and %d suppliers", cats, sups)
	}
	s.audit(t)
}
