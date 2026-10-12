package documents

import (
	"net/url"
	"os"
	"testing"

	appdb "karots-suite/internal/db"
	coremigrations "karots-suite/migrations"
	"karots-suite/plugins/documents/migrations"

	"github.com/pressly/goose/v3"
	"github.com/shopspring/decimal"
)

// A karots-pos shop's Print & Copy data (its migrations stop at 00005 or 00006) restores,
// and 00007 splits what a job uses: a fractional any-size consumable (toner)
// becomes a fractional recipe line, any-size paper stays paper and loses the
// recipe copy karots-pos 00004 made of it, size paper is untouched, and old jobs
// keep their figures with no options (shown as earlier jobs).
func TestKarotsPosDocumentsRestore(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	const name = "documents_restore_db"
	admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	if _, err := admin.Exec(`CREATE DATABASE ` + name); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) //nolint:errcheck
	u, _ := url.Parse(dsn)
	u.Path = "/" + name
	db, err := appdb.Connect(u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(appdb.RunMigrations(db.DB, coremigrations.FS))

	// Up to karots-pos's last documents migration, as an old backup has it.
	goose.SetBaseFS(migrations.FS)
	goose.SetTableName("goose_db_version_documents")
	must(goose.SetDialect("postgres"))
	err = goose.UpTo(db.DB, ".", 5)
	goose.SetBaseFS(nil)
	goose.SetTableName("goose_db_version")
	must(err)

	must(db.Get(new(int64), `INSERT INTO categories (name) VALUES ('Old shop') RETURNING id`))
	item := func(name string, service bool) int64 {
		var id int64
		must(db.Get(&id, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price, is_service)
			VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 1, 0, $2) RETURNING id`, name, service))
		return id
	}
	copyP, a4, film, toner := item("Photocopy", true), item("A4 paper", false), item("Any paper", false), item("Toner", false)
	var sid int64
	must(db.Get(&sid, `INSERT INTO doc_service (name, kind, category, product_id) VALUES ('Photocopy', 'metered', 'copy', $1) RETURNING id`, copyP))
	_, err = db.Exec(`INSERT INTO doc_price (service_id, size, color, double_side, min_qty, unit_price) VALUES
		($1, 'A4', false, false, 1, 10), ($1, 'A4', false, false, 100, 7), ($1, NULL, false, true, 1, 15)`, sid)
	must(err)
	_, err = db.Exec(`INSERT INTO doc_consumable (service_id, size, product_id, qty_per_unit) VALUES
		($1, 'A4', $2, 1), ($1, NULL, $3, 1), ($1, NULL, $4, 0.0002)`, sid, a4, film, toner)
	must(err)
	// What karots-pos 00004 did to the NULL-size rows (it ran before they existed here).
	_, err = db.Exec(`INSERT INTO product_recipes (product_id, component_product_id, qty_per_unit, whole_units) VALUES
		($1, $2, 1, true), ($1, $3, 0.0002, true)`, copyP, film, toner)
	must(err)
	_, err = db.Exec(`INSERT INTO doc_job (service_id, description, qty, unit_price, line_total, consumable_cost, kind)
		VALUES ($1, 'Photocopy A4 B&W 1-side ×20', 20, 10, 200, 20, 'sale')`, sid)
	must(err)
	_, err = db.Exec(`INSERT INTO doc_payout (amount, source, note) VALUES (500, 'external', 'CV design')`)
	must(err)

	must(appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_documents")) // → 00006 (karots-pos fix), 00007

	st := NewStore(db)
	paper, err := st.Paper(t.Context(), sid)
	must(err)
	if len(paper) != 2 {
		t.Fatalf("paper rows %+v, want the A4 paper and the any-size paper", paper)
	}
	var recipe []struct {
		ID    int64           `db:"component_product_id"`
		Qty   decimal.Decimal `db:"qty_per_unit"`
		Whole bool            `db:"whole_units"`
	}
	must(db.Select(&recipe, `SELECT component_product_id, qty_per_unit, whole_units FROM product_recipes WHERE product_id = $1`, copyP))
	if len(recipe) != 1 || recipe[0].ID != toner || recipe[0].Whole || !recipe[0].Qty.Equal(decimal.RequireFromString("0.0002")) {
		t.Fatalf("recipe %+v, want only the toner, fractional", recipe)
	}

	// A 1-copy A4 job now takes one sheet and a sliver of toner — not a cartridge.
	sv, err := st.Service(t.Context(), sid)
	must(err)
	q, err := st.Quote(t.Context(), sv, JobOptions{Size: "A4", Copies: 1})
	must(err)
	got := map[int64]string{}
	for _, c := range q.Components {
		got[c.ProductID] = c.Qty.String()
	}
	if len(got) != 2 || got[a4] != "1" || got[toner] != "0.0002" {
		t.Fatalf("1-copy job uses %v", got)
	}
	// Any-size 2-side (the NULL-size price) on Legal: the any-size paper, half the sheets.
	q, err = st.Quote(t.Context(), sv, JobOptions{Size: "Legal", Double: true, Copies: 5})
	must(err)
	if q.Sheets != 3 || q.Components[0].ProductID != film || !q.Components[0].Qty.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("legal 2-side quote %+v", q)
	}

	var job struct {
		Size   *string         `db:"size"`
		Linked *int64          `db:"sale_item_id"`
		Total  decimal.Decimal `db:"line_total"`
	}
	must(db.Get(&job, `SELECT size, sale_item_id, line_total FROM doc_job LIMIT 1`))
	if job.Size != nil || job.Linked != nil || !job.Total.Equal(decimal.NewFromInt(200)) {
		t.Errorf("old job %+v", job)
	}
}
