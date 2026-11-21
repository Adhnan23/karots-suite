// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide; the package's other tests
// register their own).
package battle

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/alternatives"
)

// Alternatives' battle tests (docs/rules/plugins/alternatives.md).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

// group makes a group, names its first tier and returns (group, tier).
func group(t *testing.T, s *ltest.Shop, name string) (int64, int64) {
	t.Helper()
	var g alternatives.GroupDTO
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/alternatives", map[string]string{"name": name}, &g))
	tid := g.Tiers[0].ID // a group starts with one unnamed tier; named, another can follow
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d", tid), map[string]any{"name": "Standard"}, nil))
	return g.ID, tid
}

func tier(t *testing.T, s *ltest.Shop, gid int64, name string) int64 {
	t.Helper()
	var g alternatives.GroupDTO
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/alternatives/%d/tiers", gid), map[string]any{"name": name}, &g))
	return g.Tiers[len(g.Tiers)-1].ID
}

func join(s *ltest.Shop, tid, pid int64, move bool) error {
	return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d/members", tid), map[string]any{"productId": pid, "move": move}, nil)
}

func shares(t *testing.T, s *ltest.Shop, tid int64) (map[int64]int, bool) {
	t.Helper()
	var rows []struct {
		P    int64 `db:"product_id"`
		N    *int  `db:"split_share"`
		Auto bool  `db:"split_auto"`
	}
	s.Must(t, s.DB.Select(&rows, `SELECT m.product_id, m.split_share, t.split_auto FROM alt_tiers t
		LEFT JOIN alt_members m ON m.tier_id = t.id WHERE t.id = $1`, tid))
	out, auto := map[int64]int{}, false
	for _, r := range rows {
		auto = r.Auto
		if r.N != nil {
			out[r.P] = *r.N
		}
	}
	return out, auto
}

func TestLedgerChecksFire(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	_, tid := group(t, s, "Fire group")
	a := s.Product(t, "Fire alt A", "10", "0", nil)
	b := s.Product(t, "Fire alt B", "10", "0", nil)
	svc := s.Service(t, "Fire alt service", "10")
	s.Must(t, join(s, tid, a, false))
	for _, c := range []struct{ rule, bad string }{
		{"ALT-5", fmt.Sprintf(`UPDATE alt_tiers SET split_auto = false WHERE id = %d; UPDATE alt_members SET split_share = 60 WHERE product_id = %d`, tid, a)},
		{"ALT-2", fmt.Sprintf(`INSERT INTO alt_members (product_id, tier_id) VALUES (999999, %d)`, tid)},
		{"ALT-2", fmt.Sprintf(`INSERT INTO alt_members (product_id, tier_id) VALUES (%d, %d)`, svc, tid)},
		{"ALT-2", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d`, b, a)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(c.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == c.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", c.rule, c.bad)
		}
	}
}

// ALT-1, ALT-2: one tier per product; a product in another group moves only when asked.
func TestALT2_Members(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	_, t1 := group(t, s, "Members one")
	_, t2 := group(t, s, "Members two")
	p := s.Product(t, "Members pen", "10", "5", nil)
	s.Must(t, join(s, t1, p, false))
	if err := join(s, t2, p, false); err == nil {
		t.Errorf("ALT-2: joined a second group without moving")
	}
	s.Must(t, join(s, t2, p, true))
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM alt_members WHERE product_id = $1`, p))
	if n != 1 {
		t.Errorf("ALT-1: %d memberships, want 1", n)
	}
	s.Audit(t)
}

// ALT-3, ALT-4: the till sees live members only; the swap chip offers in-stock
// live alternatives, never the product itself.
func TestALT3_TillSeesLiveOnly(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	gid, tid := group(t, s, "Zqlive pendrive")
	short := s.Product(t, "Zqlive Kingston", "10", "0", nil)
	stocked := s.Product(t, "Zqlive Sandisk", "10", "4", nil)
	empty := s.Product(t, "Zqlive HP", "10", "0", nil)
	off := s.Product(t, "Zqlive Off", "10", "9", nil)
	for _, p := range []int64{short, stocked, empty, off} {
		s.Must(t, join(s, tid, p, false))
	}
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", off), nil, nil))
	swap := func() []int64 {
		var alts []alternatives.AltDTO
		s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/alternatives/till/%d", short), nil, &alts))
		var ids []int64
		for _, a := range alts {
			ids = append(ids, a.ProductID)
		}
		return ids
	}
	if got := swap(); len(got) != 1 || got[0] != stocked {
		t.Errorf("ALT-4: swap offers %v, want only the in-stock live %d", got, stocked)
	}
	search := func() map[int64]bool {
		var l struct{ Items []struct{ ID int64 } }
		s.Must(t, s.Cashier.Do("GET", "/api/v1/till/products?search=zqlive+pendrive", nil, &l))
		out := map[int64]bool{}
		for _, x := range l.Items {
			out[x.ID] = true
		}
		return out
	}
	if got := search(); !got[short] || !got[stocked] || !got[empty] || got[off] {
		t.Errorf("ALT-3: the group's name finds %v, want its three live members", got)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/alternatives/%d/active", gid), map[string]bool{"active": false}, nil))
	if got := swap(); len(got) != 0 {
		t.Errorf("ALT-3: a group switched off still offers %v", got)
	}
	if got := search(); got[short] || got[stocked] {
		t.Errorf("ALT-3: a group switched off is still searched")
	}
	s.Audit(t)
}

// ALT-5: a hand split that loses a member (removed, moved, merged away) is
// rescaled to 100; with no share left it goes back to Auto.
func TestALT5_HandSplitRescaled(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	gid, tid := group(t, s, "Split rice")
	other := tier(t, s, gid, "Cheap")
	p := map[string]int64{}
	for _, n := range []string{"A", "B", "C", "D"} {
		p[n] = s.Product(t, "Split rice "+n, "10", "0", nil)
		s.Must(t, join(s, tid, p[n], false))
	}
	set := func(sh map[int64]int) {
		s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d/split", tid), map[string]any{"auto": false, "shares": sh}, nil))
	}
	set(map[int64]int{p["A"]: 10, p["B"]: 20, p["C"]: 30, p["D"]: 40})
	check := func(why string, want map[int64]int) {
		t.Helper()
		got, auto := shares(t, s, tid)
		sum := 0
		for _, n := range got {
			sum += n
		}
		if auto || sum != 100 || fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("ALT-5 %s: shares %v (auto %v), want %v adding up to 100", why, got, auto, want)
		}
	}
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/alternatives/members/%d", p["D"]), nil, nil))
	check("D removed", map[int64]int{p["A"]: 17, p["B"]: 33, p["C"]: 50})
	s.Must(t, join(s, other, p["C"], true))
	check("C moved", map[int64]int{p["A"]: 34, p["B"]: 66})
	into := s.Product(t, "Split rice kept", "10", "0", nil)
	s.Must(t, join(s, other, into, false))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", p["B"], into), map[string]string{"lotPrice": "follow"}, nil))
	check("B merged into a product in another tier", map[int64]int{p["A"]: 100})
	// A member with no share joins, the one with all of it leaves: back to Auto.
	e := s.Product(t, "Split rice E", "10", "0", nil)
	s.Must(t, join(s, tid, e, false))
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/alternatives/members/%d", p["A"]), nil, nil))
	if got, auto := shares(t, s, tid); !auto || len(got) != 0 {
		t.Errorf("ALT-5: nothing left to share, shares %v auto %v; want Auto", got, auto)
	}
	s.Audit(t)
}

// ALT-7, ALT-8: a product merge keeps the kept product's membership; deleting a
// group removes memberships, never products.
func TestALT7_MergeAndDelete(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	gid, tid := group(t, s, "Merge alt")
	_, t2 := group(t, s, "Merge alt two")
	from := s.Product(t, "Merge alt from", "10", "0", nil)
	into := s.Product(t, "Merge alt into", "10", "0", nil)
	lone := s.Product(t, "Merge alt lone", "10", "0", nil)
	s.Must(t, join(s, tid, from, false))
	s.Must(t, join(s, t2, into, false))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", from, into), map[string]string{"lotPrice": "follow"}, nil))
	var at int64
	s.Must(t, s.DB.Get(&at, `SELECT tier_id FROM alt_members WHERE product_id = $1`, into))
	if at != t2 {
		t.Errorf("ALT-7: kept product sits in tier %d, want its own %d", at, t2)
	}
	s.Must(t, join(s, tid, lone, false))
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/alternatives/%d", gid), nil, nil))
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM alt_members WHERE product_id = $1`, lone))
	var active bool
	s.Must(t, s.DB.Get(&active, `SELECT is_active FROM products WHERE id = $1`, lone))
	if n != 0 || !active {
		t.Errorf("ALT-8: after deleting the group: %d memberships, product active %v", n, active)
	}
	s.Audit(t)
}

// PLG-6: joining at once ends with one membership and a split adding up.
func TestPLG6_AtOnce(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	_, t1 := group(t, s, "Once one")
	_, t2 := group(t, s, "Once two")
	p := s.Product(t, "Once pen", "10", "0", nil)
	q := s.Product(t, "Once pencil", "10", "0", nil)
	s.Must(t, join(s, t1, p, false))
	s.Must(t, join(s, t1, q, false))
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d/split", t1), map[string]any{"auto": false, "shares": map[int64]int{p: 50, q: 50}}, nil))
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			join(s, []int64{t1, t2}[i%2], p, true) //nolint:errcheck
		}()
	}
	wg.Wait()
	s.Audit(t)
}

// PLG-5, ALT-2: bad requests are refused and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	gid, tid := group(t, s, "Refuse alt")
	p := s.Product(t, "Refuse alt A", "10", "0", nil)
	svc := s.Service(t, "Refuse alt service", "10")
	off := s.Product(t, "Refuse alt off", "10", "0", nil)
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", off), nil, nil))
	s.Must(t, join(s, tid, p, false))
	before := ltest.State(t, s.DB)
	for name, do := range map[string]func() error{
		"no product":     func() error { return join(s, tid, 999999, false) },
		"a service":      func() error { return join(s, tid, svc, false) },
		"a disabled one": func() error { return join(s, tid, off, false) },
		"no tier":        func() error { return join(s, 999999, p, true) },
		"no name":        func() error { return s.Admin.Do("POST", "/api/v1/x/alternatives", map[string]string{"name": " "}, nil) },
		"tier on no group": func() error {
			return s.Admin.Do("POST", "/api/v1/x/alternatives/999999/tiers", map[string]any{"name": "x"}, nil)
		},
		"remove no member": func() error { return s.Admin.Do("DELETE", "/api/v1/x/alternatives/members/999999", nil, nil) },
		"delete no group":  func() error { return s.Admin.Do("DELETE", "/api/v1/x/alternatives/999999", nil, nil) },
		"split not 100":    func() error { return split(s, tid, map[int64]int{p: 90}) },
		"split negative":   func() error { return split(s, tid, map[int64]int{p: 120, 999: -20}) },
		"split a stranger": func() error { return split(s, tid, map[int64]int{999999: 100}) },
		"cashier makes one": func() error {
			return s.Cashier.Do("POST", "/api/v1/x/alternatives", map[string]string{"name": "c"}, nil)
		},
		"rename no group": func() error {
			return s.Admin.Do("PUT", "/api/v1/x/alternatives/999999", map[string]string{"name": "x"}, nil)
		},
		"group bad id": func() error { return s.Admin.Do("GET", fmt.Sprintf("/api/v1/x/alternatives/%dx", gid), nil, nil) },
	} {
		if err := do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

func split(s *ltest.Shop, tid int64, sh map[int64]int) error {
	return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d/split", tid), map[string]any{"auto": false, "shares": sh}, nil)
}

// PLG-1: groups, tiers and members move no money or stock.
func TestPLG1_NothingMoves(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	p := s.Product(t, "Still alt", "10", "3", nil)
	before := ltest.State(t, s.DB)
	gid, tid := group(t, s, "Still group")
	s.Must(t, join(s, tid, p, false))
	s.Must(t, split(s, tid, map[int64]int{p: 100}))
	s.Must(t, s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/alternatives/%d", gid), nil, nil))
	ltest.Untouched(t, before, ltest.State(t, s.DB), ltest.MoneyAndStock)
}

// PLG-4: a failure at any statement leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	gid, tid := group(t, s.Shop, "Sweep alt")
	other := tier(t, s.Shop, gid, "Sweep other")
	a := s.Product(t, "Sweep alt A", "10", "0", nil)
	b := s.Product(t, "Sweep alt B", "10", "0", nil)
	ready := func() {
		join(s.Shop, tid, a, true) //nolint:errcheck
		join(s.Shop, tid, b, true) //nolint:errcheck
		s.Must(t, split(s.Shop, tid, map[int64]int{a: 30, b: 70}))
	}
	n := 0
	s.Sweep(t, "make a group", nil, func() error {
		n++
		return s.Admin.Do("POST", "/api/v1/x/alternatives", map[string]string{"name": fmt.Sprintf("Sweep g%d", n)}, nil)
	})
	s.Sweep(t, "hand split", ready, func() error { return split(s.Shop, tid, map[int64]int{a: 60, b: 40}) })
	s.Sweep(t, "remove from a hand split", ready, func() error {
		return s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/alternatives/members/%d", a), nil, nil)
	})
	s.Sweep(t, "move out of a hand split", ready, func() error { return join(s.Shop, other, b, true) })
	s.Sweep(t, "delete a tier", func() { other = tier(t, s.Shop, gid, strings.Repeat("z", n%5+1)) }, func() error {
		return s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d", other), nil, nil)
	})
}

func TestPLG7_DataGroup(t *testing.T) {
	t.Parallel()
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "alternatives")
}
