package api

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// Messy sheets: synonyms in any case, "1,250.00", a blank line, a bad row in
// the middle, an Excel number cell; re-importing updates, never duplicates.
func TestAdminImports(t *testing.T) {
	te := newTestEnv(t)
	stamp := time.Now().UnixNano() % 1_000_000
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "manager")
	tag := fmt.Sprintf("TIMP%d", stamp)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, tag+"%") //nolint:errcheck
		te.db.Exec(`DELETE FROM stock_batches WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, tag+"%")   //nolint:errcheck
		te.db.Exec(`DELETE FROM stock WHERE product_id IN (SELECT id FROM products WHERE name LIKE $1)`, tag+"%")           //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE name LIKE $1`, tag+"%")                                                      //nolint:errcheck
		te.db.Exec(`DELETE FROM customers WHERE name LIKE $1`, tag+"%")                                                     //nolint:errcheck
		te.db.Exec(`DELETE FROM suppliers WHERE name LIKE $1`, tag+"%")                                                     //nolint:errcheck
		te.db.Exec(`DELETE FROM categories WHERE name LIKE $1`, tag+"%")                                                    //nolint:errcheck
	})
	bc := fmt.Sprintf("99%011d", stamp)
	sheet := "Product,Code,Category Path,Cost,PRICE,Opening,Supplier\n" +
		tag + " Rice," + bc + "," + tag + " Dry > " + tag + " Grains,\"1,000.00\",\"Rs. 1,250.00\",5," + tag + " Sup\n" +
		"\n" +
		tag + " Bad,,,abc,10,,\n" +
		",,,,,,\n" +
		tag + " Soap,,,40,55,,\n"
	rec := te.upload("/api/v1/admin/products/import", "file", "p.csv", []byte(sheet), ck)
	res := decode[EntityImportResult](t, rec)
	if rec.Code != http.StatusOK || res.Created != 2 || res.Skipped != 1 || len(res.Errors) != 1 || res.Errors[0].Line != 4 ||
		!strings.Contains(res.Errors[0].Message, "cost price") {
		t.Fatalf("products: %d %+v", rec.Code, res)
	}
	var price, qty, cat string
	te.db.QueryRow(`SELECT p.selling_price::text, s.quantity::numeric(10,0)::text, c.name FROM products p JOIN stock s ON s.product_id=p.id
		JOIN categories c ON c.id=p.category_id WHERE p.barcode=$1`, bc).Scan(&price, &qty, &cat) //nolint:errcheck
	if price != "1250.00" || qty != "5" || cat != tag+" Grains" {
		t.Fatalf("rice: price %s qty %s category %s", price, qty, cat)
	}

	// The same rice again from Excel, price as a number cell: updates, stock not doubled.
	f := excelize.NewFile()
	for i, v := range []string{"name", "barcode", "selling_price", "opening_qty"} {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellStr("Sheet1", cell, v)
	}
	f.SetCellStr("Sheet1", "A2", tag+" Rice")
	f.SetCellStr("Sheet1", "B2", bc)
	f.SetCellFloat("Sheet1", "C2", 1300, 2, 64)
	f.SetCellInt("Sheet1", "D2", 5)
	var x bytes.Buffer
	f.Write(&x) //nolint:errcheck
	res = decode[EntityImportResult](t, te.upload("/api/v1/admin/products/import", "file", "p.xlsx", x.Bytes(), ck))
	te.db.QueryRow(`SELECT p.selling_price::text, s.quantity::numeric(10,0)::text FROM products p JOIN stock s ON s.product_id=p.id WHERE p.barcode=$1`, bc).Scan(&price, &qty) //nolint:errcheck
	if res.Updated != 1 || res.Created != 0 || price != "1300.00" || qty != "5" || len(res.Notes) != 1 {
		t.Fatalf("re-import: %+v price %s qty %s", res, price, qty)
	}

	// Export round-trips through the template's columns.
	rec = te.do(http.MethodGet, "/api/v1/admin/products/export", nil, ck)
	if !strings.HasPrefix(rec.Body.String(), strings.Join(productColumns, ",")) || !strings.Contains(rec.Body.String(), tag+" Dry > "+tag+" Grains") {
		for _, l := range strings.Split(rec.Body.String(), "\n") {
			if strings.Contains(l, tag) {
				t.Log(l)
			}
		}
		t.Fatal("export missing the category path")
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/customers/import/template?format=xlsx", nil, ck); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("template: %d", rec.Code)
	}

	cust := "Customer Name,Mobile,Credit Limit,Balance\n" + tag + " Nimal,077 900 " + fmt.Sprintf("%04d", stamp%10000) + ",\"5,000\",\"1,200.50\"\n" + tag + " Neg,,,-5\n"
	res = decode[EntityImportResult](t, te.upload("/api/v1/admin/customers/import", "file", "c.csv", []byte(cust), ck))
	var owed string
	te.db.Get(&owed, `SELECT outstanding_balance::text FROM customers WHERE name=$1`, tag+" Nimal") //nolint:errcheck
	if res.Created != 1 || len(res.Errors) != 1 || owed != "1200.50" {
		t.Fatalf("customers: %+v owed %s", res, owed)
	}
	// A sheet of just names and phones leaves the credit limit alone.
	phone := fmt.Sprintf("077900%04d", stamp%10000)
	res = decode[EntityImportResult](t, te.upload("/api/v1/admin/customers/import", "file", "c2.csv", []byte("name,phone\n"+tag+" Nimal,"+phone+"\n"), ck))
	var limit string
	te.db.Get(&limit, `SELECT credit_limit::text FROM customers WHERE name=$1`, tag+" Nimal") //nolint:errcheck
	if res.Updated != 1 || limit != "5000.00" {
		t.Fatalf("customer partial re-import: %+v limit %s", res, limit)
	}
	sup := "Vendor,Terms,Payable\n" + tag + " Sup,30,\"2,500\"\n" + tag + " New,x,\n"
	res = decode[EntityImportResult](t, te.upload("/api/v1/admin/suppliers/import", "file", "s.csv", []byte(sup), ck))
	if res.Updated != 1 || len(res.Errors) != 1 || len(res.Notes) != 1 {
		t.Fatalf("suppliers: %+v", res)
	}
}
