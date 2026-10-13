package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestLooseVersion(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009660", "admin")
	var kg int64
	if err := te.db.Get(&kg, `SELECT id FROM units WHERE id <> 1 ORDER BY id LIMIT 1`); err != nil {
		t.Fatal(err)
	}
	// The plain product's one lot: 10 bags expiring in a month, batch "R-7".
	exp := time.Now().AddDate(0, 1, 0).Truncate(24 * time.Hour)
	te.db.MustExec(`UPDATE stock_batches SET expiry_date = $2, batch_no = 'R-7' WHERE product_id = $1`, f.plainID, exp)
	name := fmt.Sprintf("T loose %d", time.Now().UnixNano())
	url := "/api/v1/admin/products/" + strconv.FormatInt(f.plainID, 10) + "/loose"
	count := func() (n int) {
		te.db.Get(&n, `SELECT COUNT(*) FROM products WHERE name = $1`, name) //nolint:errcheck
		return n
	}

	// Focus 2: every refusal leaves nothing behind — including a run short of stock.
	for label, req := range map[string]LooseVersionRequest{
		"blank name":  {Name: " ", UnitID: kg, Ratio: "25"},
		"same unit":   {Name: name, UnitID: 1, Ratio: "25"},
		"zero ratio":  {Name: name, UnitID: kg, Ratio: "0"},
		"short stock": {Name: name, UnitID: kg, Ratio: "25", ConvertNow: "11"},
	} {
		rec := te.do(http.MethodPost, url, req, ck)
		if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusConflict {
			t.Errorf("%s: %d %s, want a 422/409 refusal", label, rec.Code, rec.Body)
		}
		if n := count(); n != 0 {
			t.Fatalf("%s left %d product(s) behind", label, n)
		}
	}
	var convs int
	te.db.Get(&convs, `SELECT COUNT(*) FROM product_conversions WHERE from_product_id = $1`, f.plainID) //nolint:errcheck
	if convs != 0 {
		t.Fatalf("refusals left %d conversion(s)", convs)
	}

	rec := te.do(http.MethodPost, url, LooseVersionRequest{Name: name, UnitID: kg, Ratio: "25", SellingPrice: "4", ConvertNow: "1"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("make loose: %d %s", rec.Code, rec.Body)
	}
	got := decode[LooseVersionResult](t, rec)
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM conversion_runs WHERE conversion_id = $1`, got.ConversionID)             //nolint:errcheck
		te.db.Exec(`DELETE FROM product_conversions WHERE id = $1`, got.ConversionID)                    //nolint:errcheck
		te.db.Exec(`DELETE FROM stock_movements WHERE product_id IN ($1, $2)`, got.ProductID, f.plainID) //nolint:errcheck
		te.db.Exec(`DELETE FROM stock_batches WHERE product_id = $1`, got.ProductID)                     //nolint:errcheck
		te.db.Exec(`DELETE FROM products WHERE id = $1`, got.ProductID)                                  //nolint:errcheck
	})
	var p struct {
		Unit  int64   `db:"unit_id"`
		Cat   int64   `db:"category_id"`
		Cost  float64 `db:"cost_price"`
		Sell  float64 `db:"selling_price"`
		Stock float64 `db:"quantity"`
	}
	if err := te.db.Get(&p, `SELECT p.unit_id, p.category_id, p.cost_price, p.selling_price, s.quantity
		FROM products p JOIN stock s ON s.product_id = p.id WHERE p.id = $1`, got.ProductID); err != nil {
		t.Fatal(err)
	}
	var srcCat int64
	var srcStock float64
	te.db.Get(&srcCat, `SELECT category_id FROM products WHERE id = $1`, f.plainID)     //nolint:errcheck
	te.db.Get(&srcStock, `SELECT quantity FROM stock WHERE product_id = $1`, f.plainID) //nolint:errcheck
	if p.Unit != kg || p.Cat != srcCat || p.Cost != 2 || p.Sell != 4 || p.Stock != 25 || srcStock != 9 {
		t.Fatalf("loose product %+v, source stock %v; want kg, same category, cost 2, sell 4, 25 in, 9 bags left", p, srcStock)
	}
	var lot struct {
		Expiry  *time.Time `db:"expiry_date"`
		BatchNo *string    `db:"batch_no"`
	}
	te.db.Get(&lot, `SELECT expiry_date, batch_no FROM stock_batches WHERE product_id = $1`, got.ProductID) //nolint:errcheck
	if lot.Expiry == nil || !lot.Expiry.Equal(exp) || lot.BatchNo == nil || *lot.BatchNo != "R-7" {
		t.Fatalf("loose lot %+v, want the bag's expiry and batch R-7", lot)
	}
}
