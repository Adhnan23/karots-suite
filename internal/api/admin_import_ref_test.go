package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// A shop's product list exported with refs, fixed outside, and imported back:
// each row updates its own product even when the name changed and there is no
// barcode; a ref from another shop or for a product that isn't there is refused.
func TestAdminImportByRef(t *testing.T) {
	te := newTestEnv(t)
	stamp := time.Now().UnixNano() % 1_000_000
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "manager")
	tag := fmt.Sprintf("TREF%d", stamp)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM products WHERE name LIKE $1`, tag+"%")   //nolint:errcheck
		te.db.Exec(`DELETE FROM categories WHERE name LIKE $1`, tag+"%") //nolint:errcheck
	})
	res := decode[EntityImportResult](t, te.upload("/api/v1/admin/products/import", "file", "p.csv",
		[]byte("name,category,cost_price,selling_price\n"+tag+" ACIFREE SYRUP 200ML,"+tag+" Old,80,100\n"), ck))
	if res.Created != 1 {
		t.Fatalf("seed: %+v", res)
	}
	var id int64
	var install string
	te.db.QueryRow(`SELECT id FROM products WHERE name = $1`, tag+" ACIFREE SYRUP 200ML").Scan(&id) //nolint:errcheck
	te.db.QueryRow(`SELECT install_id FROM settings LIMIT 1`).Scan(&install)                        //nolint:errcheck
	ref := fmt.Sprintf("%s-%d", install, id)

	rec := te.do(http.MethodGet, "/api/v1/admin/products/export?ref=1", nil, ck)
	head := strings.SplitN(rec.Body.String(), "\n", 2)[0]
	if !strings.HasSuffix(strings.TrimSpace(head), ",ref") || !strings.Contains(rec.Body.String(), ","+ref+"\n") {
		t.Fatalf("export with ref=1: header %q, no %s in the rows", head, ref)
	}
	if plain := te.do(http.MethodGet, "/api/v1/admin/products/export", nil, ck).Body.String(); strings.Contains(strings.SplitN(plain, "\n", 2)[0], "ref") {
		t.Fatal("a plain export has no ref column")
	}

	sheet := "ref,name,category\n" + ref + "," + tag + " Acifree Syrup 200ml," + tag + " Medicines > " + tag + " Antacids\n"
	res = decode[EntityImportResult](t, te.upload("/api/v1/admin/products/import", "file", "fixed.csv", []byte(sheet), ck))
	var name, price, cat string
	var n int
	te.db.QueryRow(`SELECT p.name, p.selling_price::text, c.name FROM products p JOIN categories c ON c.id = p.category_id WHERE p.id = $1`, id).Scan(&name, &price, &cat) //nolint:errcheck
	te.db.QueryRow(`SELECT count(*) FROM products WHERE name LIKE $1`, tag+"%").Scan(&n)                                                                                   //nolint:errcheck
	if res.Updated != 1 || res.Created != 0 || name != tag+" Acifree Syrup 200ml" || price != "100.00" || cat != tag+" Antacids" || n != 1 {
		t.Fatalf("by ref: %+v → %q %s %q, %d products", res, name, price, cat, n)
	}

	bad := "ref,name\nZZZZ0000-" + fmt.Sprint(id) + "," + tag + " Other shop\n" + install + "-999999999," + tag + " Missing\n" + install + "-x," + tag + " Garbled\n"
	res = decode[EntityImportResult](t, te.upload("/api/v1/admin/products/import", "file", "bad.csv", []byte(bad), ck))
	te.db.QueryRow(`SELECT count(*) FROM products WHERE name LIKE $1`, tag+"%").Scan(&n) //nolint:errcheck
	if res.Created != 0 || res.Updated != 0 || len(res.Errors) != 3 || n != 1 || !strings.Contains(res.Errors[0].Message, "another shop") {
		t.Fatalf("refused refs: %+v, %d products", res, n)
	}
}
