package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The products sheet carries extra barcodes (";"-separated): import adds them (a
// code on another product is a note, the row still lands), export writes them back.
func TestProductSheetExtraBarcodes(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009593", "manager")
	tag := fmt.Sprintf("TXB%d", time.Now().UnixNano()%1_000_000)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM product_barcodes WHERE barcode LIKE $1`, tag+"%")                                 //nolint:errcheck
		te.db.Exec(`DELETE FROM stock WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, tag+"%") //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE name LIKE $1`, tag+"%")                                            //nolint:errcheck
	})
	sheet := "name,barcode,extra barcodes,selling_price\n" +
		tag + " Tea," + tag + "-M,\"" + tag + "-A; " + tag + "-B;" + f.plainBarcode + "\",250\n"
	res := decode[EntityImportResult](t, te.upload("/api/v1/admin/products/import", "file", "p.csv", []byte(sheet), ck))
	if res.Created != 1 || len(res.Errors) != 0 {
		t.Fatalf("import %+v", res)
	}
	var extras string
	te.db.Get(&extras, `SELECT string_agg(pb.barcode, ';' ORDER BY pb.id) FROM product_barcodes pb JOIN products p ON p.id = pb.product_id WHERE p.name = $1`, tag+" Tea") //nolint:errcheck
	if extras != tag+"-A;"+tag+"-B" {
		t.Fatalf("extras %q", extras)
	}
	if len(res.Notes) == 0 || !strings.Contains(fmt.Sprint(res.Notes), "T plain") {
		t.Fatalf("a code on another product must be a note naming it: %+v", res)
	}
	rec := te.do(http.MethodGet, "/api/v1/admin/products/export?format=csv", nil, ck)
	rows, err := csv.NewReader(strings.NewReader(rec.Body.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	col := -1
	for i, h := range rows[0] {
		if h == "extra_barcodes" {
			col = i
		}
	}
	found := false
	for _, r := range rows[1:] {
		if r[0] == tag+" Tea" {
			found = col >= 0 && r[col] == tag+"-A;"+tag+"-B"
		}
	}
	if !found {
		t.Fatalf("export lost the extras (col %d): %v", col, rows[0])
	}
}
