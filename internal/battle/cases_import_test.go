package battle

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
	"time"

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

func fieldsOf(c struct {
	Line    int
	Product string
	New     bool
	Fields  []struct{ Field, From, To string }
}) map[string][2]string {
	m := map[string][2]string{}
	for _, f := range c.Fields {
		m[f.Field] = [2]string{f.From, f.To}
	}
	return m
}

// IMP-4: the preview lists, per product, exactly the fields that change, old →
// new; an untouched product isn't listed; a product named twice is listed twice;
// a sheet matched by barcode instead of ref finds the same product.
func TestIMP4PreviewListsTheChanges(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.stocked(t, "Imp Rename Me", s.unitPcs, 0, map[string]any{"barcode": "IMP-RN-1"})
	s.stocked(t, "Imp Move Me", s.unitPcs, 0, nil)
	s.stocked(t, "Imp Leave Me", s.unitPcs, 0, nil)
	r := s.importProducts(t, csvOf(
		[]string{"ref", "name", "category", "selling_price"},
		[]string{s.refOf(t, "Imp Rename Me"), "Imp Renamed", "", ""},
		[]string{s.refOf(t, "Imp Move Me"), "Imp Move Me", "Imp A > Imp B", ""},
		[]string{s.refOf(t, "Imp Leave Me"), "Imp Leave Me", "", ""},
		[]string{s.refOf(t, "Imp Move Me"), "Imp Move Me", "", "99"},
	), true)
	if r.Updated != 4 || len(r.Changes) != 3 {
		t.Fatalf("IMP-4: %d updated, changes %+v", r.Updated, r.Changes)
	}
	if f := fieldsOf(r.Changes[0]); len(f) != 1 || f["name"] != [2]string{"Imp Rename Me", "Imp Renamed"} || r.Changes[0].Line != 2 {
		t.Fatalf("IMP-4 rename: %+v", r.Changes[0])
	}
	if f := fieldsOf(r.Changes[1]); len(f) != 1 || f["category"][1] != "Imp A > Imp B" || r.Changes[1].Product != "Imp Move Me" {
		t.Fatalf("IMP-4 move: %+v", r.Changes[1])
	}
	if f := fieldsOf(r.Changes[2]); len(f) != 1 || f["selling_price"][1] != "99" || r.Changes[2].Line != 5 {
		t.Fatalf("IMP-4 second row of a product: %+v", r.Changes[2])
	}
	byBarcode := s.importProducts(t, csvOf([]string{"barcode", "name"}, []string{"IMP-RN-1", "Imp By Barcode"}), true)
	if len(byBarcode.Changes) != 1 || fieldsOf(byBarcode.Changes[0])["name"] != [2]string{"Imp Rename Me", "Imp By Barcode"} {
		t.Fatalf("IMP-4 by barcode: %+v", byBarcode.Changes)
	}
	nw := s.importProducts(t, csvOf([]string{"name", "selling_price"}, []string{"Imp Fresh", "12"}), true)
	if len(nw.Changes) != 1 || !nw.Changes[0].New || nw.Changes[0].Product != "Imp Fresh" {
		t.Fatalf("IMP-4 new: %+v", nw.Changes)
	}
}

// IMP-2: Apply reports exactly what the preview did.
func TestIMP2ApplyIsThePreview(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.stocked(t, "Imp Same", s.unitPcs, 0, nil)
	sheet := csvOf(
		[]string{"ref", "name", "category", "selling_price"},
		[]string{s.refOf(t, "Imp Same"), "Imp Same Renamed", "Imp X", "33"},
		[]string{"", "Imp Added", "Imp X", "5"},
		[]string{"BAD-REF-1", "Imp Refused", "", ""},
	)
	p := s.importProducts(t, sheet, true)
	a := s.importProducts(t, sheet, false)
	a.Applied = false
	if !reflect.DeepEqual(p, a) {
		t.Fatalf("IMP-2: preview %+v\napply %+v", p, a)
	}
}

// IMP-3 (review): a line that fails inside the product write itself — a ref of
// this shop to a product that isn't there — still leaves nothing it made behind
// (nested savepoints roll back to the line's own, not the inner one).
func TestIMP3FailedInsideTheWrite(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.stocked(t, "Imp Anchor", s.unitPcs, 0, nil)
	ref := s.refOf(t, "Imp Anchor")
	missing := ref[:strings.LastIndex(ref, "-")] + "-999999"
	r := s.importProducts(t, csvOf(
		[]string{"ref", "name", "category", "supplier"},
		[]string{missing, "Imp Gone", "Imp Inner Ghost Cat", "Imp Inner Ghost Supplier"},
	), false)
	if r.Skipped != 1 {
		t.Fatalf("result %+v", r)
	}
	var cats, sups int
	s.must(t, s.db.Get(&cats, `SELECT count(*) FROM categories WHERE name = 'Imp Inner Ghost Cat'`))
	s.must(t, s.db.Get(&sups, `SELECT count(*) FROM suppliers WHERE name = 'Imp Inner Ghost Supplier'`))
	if cats != 0 || sups != 0 {
		t.Fatalf("IMP-3: a line that failed inside the write left %d categories and %d suppliers", cats, sups)
	}
}

// IMP-3 (never stop a sale): a product a till is busy with (its row locked) is
// skipped by the import within a moment, never waited on — the till goes first.
func TestIMP3TillGoesFirst(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	busy := s.stocked(t, "Imp Busy", s.unitPcs, 0, nil)
	s.stocked(t, "Imp Free", s.unitPcs, 0, nil)
	till, err := s.db.Beginx()
	s.must(t, err)
	defer till.Rollback() //nolint:errcheck
	till.MustExec(`SELECT 1 FROM products WHERE id = $1 FOR UPDATE`, busy)
	start := time.Now()
	r := s.importProducts(t, csvOf(
		[]string{"ref", "name", "selling_price"},
		[]string{s.refOf(t, "Imp Busy"), "Imp Busy", "77"},
		[]string{s.refOf(t, "Imp Free"), "Imp Free", "78"},
	), false)
	if took := time.Since(start); took > 5*time.Second {
		t.Fatalf("the import waited %v on a till's product", took)
	}
	if r.Updated != 1 || r.Skipped != 1 || len(r.Errors) != 1 || r.Errors[0].Line != 2 {
		t.Fatalf("result %+v", r)
	}
}
