package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"karots-suite/internal/plugin"

	"github.com/shopspring/decimal"
)

var (
	exactOnce  sync.Once
	exactMu    sync.Mutex
	exactNotes = map[int64]plugin.ReorderNote{}
)

func useFakeExact() {
	exactOnce.Do(func() {
		plugin.NewRegistry(plugin.Core{}, plugin.NewMux(), nil).AddProductReorderAnnotator(plugin.ProductReorderAnnotator{
			Batch: func(_ context.Context, ids []int64) (map[int64]plugin.ReorderNote, error) {
				exactMu.Lock()
				defer exactMu.Unlock()
				out := map[int64]plugin.ReorderNote{}
				for _, id := range ids {
					if n, ok := exactNotes[id]; ok {
						out[id] = n
					}
				}
				return out, nil
			},
		})
	})
}

func setExact(t *testing.T, m map[int64]plugin.ReorderNote) {
	t.Helper()
	exactMu.Lock()
	exactNotes = m
	exactMu.Unlock()
	t.Cleanup(func() { exactMu.Lock(); exactNotes = map[int64]plugin.ReorderNote{}; exactMu.Unlock() })
}

// A plugin's Exact share replaces core's own figure; without it, the larger wins as before.
func TestExactReplacesOwnNeed(t *testing.T) {
	useFakeExact()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	sup := decode[AdminSupplierRow](t, te.do(http.MethodPost, "/api/v1/admin/suppliers", SupplierRequest{Name: fmt.Sprintf("T Exact %d", time.Now().UnixNano())}, ck))
	te.db.Exec(`UPDATE products SET reorder_level=50, preferred_supplier_id=$1 WHERE id=$2`, sup.ID, f.scarceID) //nolint:errcheck
	row := func() ReorderRow {
		t.Helper()
		l := decode[ReorderList](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/admin/reorder?supplier=%d", sup.ID), nil, ck))
		for _, r := range l.Items {
			if r.ProductID == f.scarceID {
				return r
			}
		}
		t.Fatalf("not listed: %+v", l)
		return ReorderRow{}
	}
	setExact(t, map[int64]plugin.ReorderNote{f.scarceID: {Suggest: decimal.NewFromInt(3), Exact: true, Block: "Twins", BlockLine: "LOW"}})
	if r := row(); r.Suggested != "3" || r.Block != "Twins" || r.BlockLine != "LOW" {
		t.Fatalf("exact: %+v", r)
	}
	setExact(t, map[int64]plugin.ReorderNote{f.scarceID: {Suggest: decimal.NewFromInt(3)}})
	if r := row(); r.Suggested != "99" || r.Block != "" {
		t.Fatalf("without Exact, core's 2×50−1 stands: %+v", r)
	}
	setExact(t, map[int64]plugin.ReorderNote{f.scarceID: {Suggest: decimal.NewFromInt(500)}})
	if r := row(); r.Suggested != "500" {
		t.Fatalf("without Exact, the larger: %+v", r)
	}
}

// The till's tab takes the same filter as Reorder.
func TestTillListHidesCovered(t *testing.T) {
	useFakeCover()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	te.db.Exec(`UPDATE products SET reorder_level=500 WHERE id=$1`, f.lotID) //nolint:errcheck
	fakeCoverMu.Lock()
	coverIDs[f.lotID], pullIDs = true, []int64{f.plainID}
	fakeCoverMu.Unlock()
	t.Cleanup(func() { fakeCoverMu.Lock(); coverIDs, pullIDs = map[int64]bool{}, nil; fakeCoverMu.Unlock() })
	has := func(l ReorderList, id int64) bool {
		for _, r := range l.Items {
			if r.ProductID == id {
				return true
			}
		}
		return false
	}
	shown := decode[ReorderList](t, te.do(http.MethodGet, "/api/v1/till/low-stock?all=1", nil, ck))
	if !has(shown, f.lotID) {
		t.Fatalf("show covered: %+v", shown.Total)
	}
	hid := decode[ReorderList](t, te.do(http.MethodGet, "/api/v1/till/low-stock?all=1&hideCovered=1", nil, ck))
	if has(hid, f.lotID) || !has(hid, f.plainID) || hid.Hidden < 1 {
		t.Fatalf("hide covered: %d hidden, %d rows", hid.Hidden, hid.Total)
	}
}

// The dashboard's count agrees with the list, rather than counting what a twin covers.
func TestDashboardCountsCoverage(t *testing.T) {
	useFakeCover()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	te.db.Exec(`UPDATE products SET reorder_level=500 WHERE id=$1`, f.lotID) //nolint:errcheck
	fakeCoverMu.Lock()
	coverIDs[f.lotID], pullIDs = true, []int64{f.plainID}
	fakeCoverMu.Unlock()
	t.Cleanup(func() { fakeCoverMu.Lock(); coverIDs, pullIDs = map[int64]bool{}, nil; fakeCoverMu.Unlock() })
	d := decode[Dashboard](t, te.do(http.MethodGet, "/api/v1/admin/dashboard", nil, ck))
	l := decode[ReorderList](t, te.do(http.MethodGet, "/api/v1/admin/reorder?all=1&hideCovered=1", nil, ck))
	if d.Attention.LowStock != l.Total {
		t.Fatalf("dashboard %d, list %d", d.Attention.LowStock, l.Total)
	}
}

// A low group's other brands are always listed, covered hidden or not: each row's order
// is only a share, so a missing brand would be a short order.
func TestLowMembersAlwaysPulled(t *testing.T) {
	useFakeCover()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	te.db.Exec(`UPDATE products SET reorder_level=0 WHERE id=$1`, f.plainID) //nolint:errcheck
	fakeCoverMu.Lock()
	pullIDs = []int64{f.plainID}
	fakeCoverMu.Unlock()
	t.Cleanup(func() { fakeCoverMu.Lock(); pullIDs = nil; fakeCoverMu.Unlock() })
	l := decode[ReorderList](t, te.do(http.MethodGet, "/api/v1/admin/reorder?all=1", nil, ck))
	for _, r := range l.Items {
		if r.ProductID == f.plainID {
			return
		}
	}
	t.Fatalf("not pulled in: %d rows", l.Total)
}

func TestBlockKeyReachesTheRow(t *testing.T) {
	useFakeExact()
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "admin")
	te.db.Exec(`UPDATE products SET reorder_level=50 WHERE id=$1`, f.scarceID) //nolint:errcheck
	setExact(t, map[int64]plugin.ReorderNote{f.scarceID: {Block: "Twins", BlockKey: "t7"}})
	l := decode[ReorderList](t, te.do(http.MethodGet, "/api/v1/admin/reorder?all=1", nil, ck))
	for _, r := range l.Items {
		if r.ProductID == f.scarceID && r.BlockKey == "t7" {
			return
		}
	}
	t.Fatal("no block key")
}
