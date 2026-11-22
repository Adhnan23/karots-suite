package battle

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"karots-suite/internal/backup"
	"karots-suite/internal/config"
	"karots-suite/internal/ledgercheck/ltest"
)

// BAK-2: a shop at rest comes back row for row.
func TestBAK2_RoundTrip(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	r := s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "2"}}, cash("200"))
	s.sale(t, s.till1, r.ID)
	ltest.RoundTrip(t, s.db)
}

// BAK-1: a backup taken while two tills sell is one moment — it restores and
// the rule book holds in it.
func TestBAK1_BackupWhileSelling(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.open(t, s.till2)
	var stop atomic.Bool
	var wg sync.WaitGroup
	for _, c := range []*client{s.till1, s.till2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				var r saleResult
				if err := c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0",
					"discountType": "fixed", "items": []line{{"productId": s.prod["plain"], "quantity": "1"},
						{"productId": s.prod["odd"], "quantity": "3"}}, "payments": []pay{cash("200")}}, &r); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	for range 8 {
		ltest.Restored(t, s.db)
	}
	stop.Store(true)
	wg.Wait()
}

// BAK-6: a restore from the Data page while a till sells loses nothing it
// didn't mean to: every sale the till was told went through is in the safety
// copy or in the restored shop.
func TestBAK6_SaleAtOnceWithRestore(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("battle: skipped with -short")
	}
	dir := t.TempDir()
	db, dsn := ltest.NewDBWithDSN(t)
	s := newShopCfg(t, db, dsn, func(c *config.Config) { c.BackupDir = dir })
	s.open(t, s.till1)
	b, err := backup.WriteSnapshot(context.Background(), s.db, dir, time.Now().Add(-time.Hour), "")
	s.must(t, err)

	var ok sync.Map // clientRef → told it went through
	var stop atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; !stop.Load(); i++ {
			ref := fmt.Sprintf("bak6-%d", i)
			if s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0",
				"discountType": "fixed", "clientRef": ref, "items": []line{{"productId": s.prod["plain"], "quantity": "1"}},
				"payments": []pay{cash("100")}}, nil) == nil {
				ok.Store(ref, true)
			}
		}
	}()
	time.Sleep(300 * time.Millisecond)
	var res struct{ SafetyCopy string }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/data/backups/"+filepath.Base(b)+"/restore", nil, &res))
	time.Sleep(300 * time.Millisecond)
	stop.Store(true)
	wg.Wait()

	refs := map[string]bool{}
	var now []string
	s.must(t, s.db.Select(&now, `SELECT client_ref FROM sales WHERE client_ref IS NOT NULL`))
	for _, r := range now {
		refs[r] = true
	}
	f, err := os.Open(filepath.Join(dir, res.SafetyCopy))
	s.must(t, err)
	defer f.Close()
	arc, err := backup.ReadArchive(f)
	s.must(t, err)
	for _, tb := range arc.Tables {
		if tb.Name != "sales" {
			continue
		}
		col := slices.Index(tb.Columns, "client_ref")
		for _, row := range tb.Rows {
			if row[col] != nil {
				refs[*row[col]] = true
			}
		}
	}
	n := 0
	ok.Range(func(k, _ any) bool {
		n++
		if !refs[k.(string)] {
			t.Errorf("BAK-6: sale %s went through but is in neither the safety copy nor the restored shop", k)
		}
		return true
	})
	if n == 0 {
		t.Fatal("no sale went through")
	}
	s.audit(t)
}

// installShop is a battle shop whose backups folder is a temp dir.
func installShop(t *testing.T) (*shop, string) {
	if testing.Short() {
		t.Skip("battle: skipped with -short")
	}
	dir := t.TempDir()
	db, dsn := ltest.NewDBWithDSN(t)
	return newShopCfg(t, db, dsn, func(c *config.Config) { c.BackupDir = dir }), dir
}

// restoreFile brings back a backup in the shop's folder from the Data page and
// signs the admin in again (a restore signs them out, BAK-9).
func (s *shop) restoreFile(t *testing.T, path string) {
	s.must(t, s.admin.Do("POST", "/api/v1/admin/data/backups/"+filepath.Base(path)+"/restore", nil, nil))
	if err := s.admin.Do("GET", "/api/v1/admin/data", nil, nil); err == nil {
		t.Error("BAK-9: the admin who restored is still signed in")
	}
	var err error
	s.admin, err = newClient(s.h, "0770000001", "1234", "battle-admin-again")
	s.must(t, err)
}

// BAK-2: after a restore the shop carries on — the next sale, payment slip and
// receipt number don't collide with the restored ones.
func TestBAK2_CarriesOnAfterRestore(t *testing.T) {
	t.Parallel()
	s, dir := installShop(t)
	s.open(t, s.till1)
	for range 3 {
		s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	}
	b, err := backup.WriteSnapshot(context.Background(), s.db, dir, time.Now(), "")
	s.must(t, err)
	for range 2 {
		s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	}
	s.restoreFile(t, b)
	for range 3 {
		s.sell(t, s.till1, "0", []line{{"productId": s.prod["odd"], "quantity": "1"}}, cash("50"))
	}
	var n, distinct int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM sales`))
	s.must(t, s.db.Get(&distinct, `SELECT count(DISTINCT receipt_no) FROM sales`))
	if n != 6 || distinct != 6 {
		t.Fatalf("BAK-2: %d sales, %d receipt numbers after a restore (want 6, 6)", n, distinct)
	}
	s.audit(t)
}

// BAK-9: after a restore a session whose user id now names someone else is
// signed out — it never becomes that person.
func TestBAK9_SessionOfAnotherPersonEnds(t *testing.T) {
	t.Parallel()
	s, dir := installShop(t)
	b, err := backup.WriteSnapshot(context.Background(), s.db, dir, time.Now(), "")
	s.must(t, err)
	var u struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/users", map[string]any{"name": "Till Three", "phone": "0770000004",
		"role": "cashier", "pin": "4321"}, &u))
	s.must(t, sqlxExec(context.Background(), s.db, `UPDATE users SET must_change_pin = false`))
	three, err := newClient(s.h, "0770000004", "4321", "battle-three")
	s.must(t, err)
	s.restoreFile(t, b)
	var boss struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/users", map[string]any{"name": "Boss", "phone": "0770000005",
		"role": "admin", "pin": "5678"}, &boss))
	if boss.ID != u.ID {
		t.Fatalf("setup: Boss got id %d, Till Three had %d", boss.ID, u.ID)
	}
	if err := three.Do("GET", "/api/v1/admin/data", nil, nil); err == nil {
		t.Fatal("BAK-9: Till Three's session reached an admin page as Boss after the restore")
	}
	if err := three.Do("GET", "/api/v1/me", nil, nil); err == nil {
		t.Fatal("BAK-9: Till Three is still signed in as someone else")
	}
}

// BAK-10: a setup-only export restores into an empty shop and carries no
// balances or history.
func TestBAK10_SetupOnlyRestoresIntoEmptyShop(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.sell(t, s.till1, "0", []line{{"productId": s.prod["plain"], "quantity": "1"}}, cash("100"))
	s.must(t, s.till1.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
		"customerId": s.customer, "items": []line{{"productId": s.prod["plain"], "quantity": "1"}},
		"payments": []pay{{"method": "credit", "amount": "100"}}}, nil))
	var raw []byte
	s.must(t, s.admin.Do("GET", "/api/v1/admin/data/backup?setupOnly=1", nil, &raw))
	cp := ltest.NewDB(t)
	if _, err := backup.Restore(context.Background(), cp, bytes.NewReader(raw)); err != nil {
		t.Fatalf("BAK-10: a setup-only export doesn't restore into an empty shop: %v", err)
	}
	var sales, products int
	var owed string
	s.must(t, cp.Get(&sales, `SELECT count(*) FROM sales`))
	s.must(t, cp.Get(&products, `SELECT count(*) FROM products`))
	s.must(t, cp.Get(&owed, `SELECT COALESCE(sum(outstanding_balance), 0)::text FROM customers`))
	if sales != 0 || products == 0 || dec(t, owed).Sign() != 0 {
		t.Fatalf("BAK-10: %d sales, %d products, Rs %s owed after a setup-only restore", sales, products, owed)
	}
	ltest.Check(t, context.Background(), cp)
}

// BAK-11: only an admin backs up or restores, and only a bare backup name in
// the folder is ever read.
func TestBAK11_Refusals(t *testing.T) {
	t.Parallel()
	s, dir := installShop(t)
	b, err := backup.WriteSnapshot(context.Background(), s.db, dir, time.Now(), "")
	s.must(t, err)
	name := filepath.Base(b)
	os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.json.gz"), []byte("x"), 0o644)
	before := ltest.State(t, s.db)
	for _, c := range []struct {
		who          *client
		method, path string
	}{
		{s.till1, "GET", "/api/v1/admin/data/backup"},
		{s.till1, "GET", "/api/v1/admin/data/backups/" + name},
		{s.till1, "POST", "/api/v1/admin/data/backups/" + name + "/restore"},
		{s.admin, "GET", "/api/v1/admin/data/backups/..%2Fsecret.json.gz"},
		{s.admin, "POST", "/api/v1/admin/data/backups/..%2Fsecret.json.gz/restore"},
		{s.admin, "GET", "/api/v1/admin/data/backups/notes.txt"},
		{s.admin, "POST", "/api/v1/admin/data/backups/missing.json.gz/restore"},
	} {
		if err := c.who.Do(c.method, c.path, nil, nil); err == nil {
			t.Errorf("BAK-11: %s %s went through", c.method, c.path)
		}
	}
	for _, tb := range ltest.Changed(before, ltest.State(t, s.db)) {
		if tb != "audit_log" && tb != "users" { // a refused call may stamp last seen
			t.Errorf("BAK-11: %s changed by a refused call", tb)
		}
	}
}
