package battle

import (
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
