package api

import (
	"context"
	"net/http"
	"testing"
)

func TestTillCopy(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	ctx := context.Background()
	f := te.seedTill(t)
	cashierID, managerID := te.seedUser(t, devPhone(), "cashier"), te.seedUser(t, devPhone(), "manager")
	adminID := te.seedUser(t, devPhone(), "admin")
	sysPhone := devPhone()
	sysID := te.seedUser(t, sysPhone, "admin")
	te.db.MustExec(`UPDATE users SET is_system = true WHERE id = $1`, sysID)
	te.db.MustExec(`INSERT INTO product_packs (product_id, name, size) VALUES ($1, 'Box', 5)`, f.plainID)
	te.db.MustExec(`UPDATE stock SET quantity = 5 WHERE product_id = $1`, f.plainID)
	var custID int64
	if err := te.db.Get(&custID, `INSERT INTO customers (name, credit_limit, outstanding_balance) VALUES ('T copy cust', 500, 50) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { te.db.Exec(`DELETE FROM customers WHERE id = $1`, custID) }) //nolint:errcheck
	ck := te.signIn(t, "dev-copy", "", te.phoneOf(t, cashierID))
	te.on("dev-copy", http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	t.Cleanup(func() { te.on("dev-copy", http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck) })
	te.on("dev-copy", http.MethodPost, "/api/v1/till/held", HoldRequest{Label: "T copy held", SaleType: "retail", Cart: `{"lines":[]}`, ItemCount: 1, Total: "100"}, ck)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM held_sales WHERE label = 'T copy held'`) }) //nolint:errcheck

	c, err := BuildTillCopy(ctx, te.db)
	if err != nil {
		t.Fatal(err)
	}
	people := map[int64]CopyPerson{}
	for _, p := range c.People {
		if p.Role != "cashier" && p.Role != "manager" {
			t.Fatalf("a %s in the copy: %+v", p.Role, p)
		}
		people[p.ID] = p
	}
	if people[cashierID].PinHash == "" || people[managerID].PinHash == "" {
		t.Fatalf("cashier/manager missing or without a PIN hash: %+v", people)
	}
	if _, ok := people[adminID]; ok {
		t.Fatal("admin in the copy")
	}
	if _, ok := people[sysID]; ok {
		t.Fatal("system user in the copy")
	}
	var prod *CopyProduct
	for i := range c.Products {
		if c.Products[i].ID == f.plainID {
			prod = &c.Products[i]
		}
	}
	if prod == nil || len(prod.Packs) != 1 || prod.StockQty != "5" || prod.Barcode != f.plainBarcode {
		t.Fatalf("product %+v", prod)
	}
	var cust *CopyCustomer
	for i := range c.Customers {
		if c.Customers[i].ID == custID {
			cust = &c.Customers[i]
		}
	}
	if cust == nil || cust.Balance != "50.00" || cust.CreditLimit != "500.00" {
		t.Fatalf("customer %+v", cust)
	}
	var sess *CopySession
	for i := range c.Sessions {
		if c.Sessions[i].UserID == cashierID {
			sess = &c.Sessions[i]
		}
	}
	if sess == nil || sess.Expected != sess.OpeningCash || sess.OpeningCash != "1000.00" {
		t.Fatalf("session %+v", sess)
	}
	held := false
	for _, h := range c.Held {
		held = held || (h.UserID == cashierID && h.Label == "T copy held")
	}
	if !held || c.Version == "" || c.Symbol == "" {
		t.Fatalf("held %v version %q symbol %q", held, c.Version, c.Symbol)
	}
}

func TestTillCopyVersionChanges(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	ctx := context.Background()
	f := te.seedTill(t)
	ph := devPhone()
	te.seedUser(t, ph, "cashier")
	ck := te.signIn(t, "dev-ver", "", ph)
	te.on("dev-ver", http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	t.Cleanup(func() { te.on("dev-ver", http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck) })
	v1, err := TillCopyVersion(ctx, te.db)
	if err != nil || v1 == "" {
		t.Fatal(v1, err)
	}
	if v, _ := TillCopyVersion(ctx, te.db); v != v1 {
		t.Fatal("version moved with nothing changed")
	}
	if r := te.on("dev-ver", http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "100"}), ck); r.Code != http.StatusCreated && r.Code != http.StatusOK {
		t.Fatalf("sale %d %s", r.Code, r.Body)
	}
	v2, _ := TillCopyVersion(ctx, te.db)
	if v2 == v1 {
		t.Fatal("version unchanged after a sale")
	}
	var custID int64
	te.db.Get(&custID, `SELECT id FROM customers ORDER BY id LIMIT 1`) //nolint:errcheck
	te.db.MustExec(`UPDATE customers SET name = name || '' , credit_limit = credit_limit + 1 WHERE id = $1`, custID)
	t.Cleanup(func() { te.db.Exec(`UPDATE customers SET credit_limit = credit_limit - 1 WHERE id = $1`, custID) }) //nolint:errcheck
	if v3, _ := TillCopyVersion(ctx, te.db); v3 == v2 {
		t.Fatal("version unchanged after a customer edit")
	}
}

// phoneOf is a seeded user's phone.
func (te *testEnv) phoneOf(t *testing.T, id int64) string {
	t.Helper()
	var p string
	if err := te.db.Get(&p, `SELECT phone FROM users WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return p
}
