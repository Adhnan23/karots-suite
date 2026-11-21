package requests

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
)

// Customer requests' battle tests (docs/rules/plugins/requests.md): one throwaway
// shop on the real server with this plugin compiled in (ltest.Main).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

func note(t *testing.T, s *ltest.Shop, note, name, phone string) int64 {
	t.Helper()
	var r RequestRow
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/requests", map[string]string{"note": note, "name": name, "phone": phone}, &r))
	return r.ID
}

func askAgain(s *ltest.Shop, id int64, name, phone string) error {
	return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/asks", id), map[string]string{"name": name, "phone": phone}, nil)
}

// listed reads one request as the admin page shows it (its note searched on every tab).
func listed(t *testing.T, s *ltest.Shop, id int64) (RequestRow, bool) {
	t.Helper()
	var note string
	if s.DB.Get(&note, `SELECT note FROM requests WHERE id = $1`, id) != nil {
		return RequestRow{}, false
	}
	for _, tab := range []string{"open", "call", "done"} {
		var l struct{ Rows []RequestRow }
		s.Must(t, s.Admin.Do("GET", "/api/v1/x/requests?tab="+tab+"&q="+url.QueryEscape(note), nil, &l))
		for _, r := range l.Rows {
			if r.ID == id {
				return r, true
			}
		}
	}
	return RequestRow{}, false
}

func asks(t *testing.T, s *ltest.Shop, id int64) int {
	t.Helper()
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT 1 + (SELECT count(*) FROM request_asks WHERE request_id = $1) FROM requests WHERE id = $1`, id))
	return n
}

func TestLedgerChecksFire(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	id := note(t, s, "Fire request", "", "")
	p := s.Product(t, "Fire request product", "10", "0", nil)
	dup := s.Product(t, "Fire request dup", "10", "0", nil)
	for _, c := range []struct{ rule, bad string }{
		{"REQ-9", fmt.Sprintf(`UPDATE requests SET status = 'converted', product_id = 999999 WHERE id = %d`, id)},
		{"REQ-9", fmt.Sprintf(`UPDATE requests SET product_id = %d WHERE id = %d`, p, id)},
		{"REQ-9", fmt.Sprintf(`UPDATE requests SET status = 'declined' WHERE id = %d`, id)},
		{"REQ-9", fmt.Sprintf(`UPDATE requests SET status = 'lost' WHERE id = %d`, id)},
		{"REQ-7", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d; UPDATE requests SET status = 'converted', product_id = %[2]d WHERE id = %d`, p, dup, id)},
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

// REQ-1, REQ-8: the stage follows the status, the product's stock and who's left to call.
func TestREQ1_Stages(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	id := note(t, s, "Stage blue ink", "Nimal", "0711000001")
	stage := func(want, why string) {
		t.Helper()
		if r, _ := listed(t, s, id); r.Stage != want {
			t.Errorf("REQ-1 %s: stage %q, want %q", why, r.Stage, want)
		}
	}
	stage("open", "new")
	s.Must(t, askAgain(s, id, "Kamal", "0711000002"))
	var made map[string]int64
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", id), map[string]any{"name": "Stage blue ink pen", "categoryId": s.Cat, "unitId": 1, "sellingPrice": "50"}, &made))
	stage("waiting", "made, no stock")
	if err := askAgain(s, id, "Sunil", "0711000003"); err != nil {
		t.Errorf("REQ-2: asking again while waiting refused: %v", err)
	}
	s.Must(t, s.Admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": made["productId"], "newQuantity": "5", "note": "in"}, nil))
	stage("call", "in stock")
	if err := askAgain(s, id, "Late", "0711000009"); err == nil {
		t.Errorf("REQ-2: asking again once it's in stock was taken")
	}
	r, _ := listed(t, s, id)
	for _, a := range r.Asks {
		s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/asks/%d/called", id, a.ID), nil, nil))
	}
	stage("done", "everyone called")
	s.Audit(t)
}

// REQ-3, PLG-6: make-product sent twice at once makes one product.
func TestREQ3_MakeProductAtOnce(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	id := note(t, s, "Twice green tea", "", "")
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", id), map[string]any{"name": fmt.Sprintf("Twice green tea %d", id), "categoryId": s.Cat, "unitId": 1, "sellingPrice": "300"}, nil)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if se, isSE := err.(*ltest.StatusError); !isSE || se.Code >= 500 {
			t.Errorf("REQ-3: %v, want a clear refusal", err)
		}
	}
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM products WHERE name = $1`, fmt.Sprintf("Twice green tea %d", id)))
	if ok != 1 || n != 1 {
		t.Errorf("REQ-3: %d answered done, %d products made; want 1 and 1", ok, n)
	}
	s.Audit(t)
}

// REQ-3: link needs a product that exists and is still one (not merged away).
func TestREQ3_LinkRefusals(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	id := note(t, s, "Link soap", "", "")
	a := s.Product(t, "Link soap A", "10", "0", nil)
	b := s.Product(t, "Link soap B", "10", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", a, b), map[string]string{"lotPrice": "follow"}, nil))
	for name, pid := range map[string]int64{"none": 999999, "merged away": a} {
		if err := s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/link", id), map[string]int64{"productId": pid}, nil); err == nil {
			t.Errorf("REQ-3 / PLG-3: linked to a product that's %s", name)
		}
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/link", id), map[string]int64{"productId": b}, nil))
	if err := s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/link", id), map[string]int64{"productId": b}, nil); err == nil {
		t.Errorf("REQ-3: linked again once converted")
	}
	s.Audit(t)
}

// REQ-4: a merge keeps every ask; never into itself or a closed one.
func TestREQ4_Merge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	from := note(t, s, "Merge red pen", "A", "0711100001")
	s.Must(t, askAgain(s, from, "B", "0711100002"))
	into := note(t, s, "Merge pen red", "C", "0711100003")
	want := asks(t, s, from) + asks(t, s, into)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", from), map[string]int64{"into": into}, nil))
	if got := asks(t, s, into); got != want {
		t.Errorf("REQ-4: %d asks after the merge, want %d", got, want)
	}
	if _, ok := listed(t, s, from); ok {
		t.Errorf("REQ-4: the merged request is still listed")
	}
	if err := s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", into), map[string]int64{"into": into}, nil); err == nil {
		t.Errorf("REQ-4: merged into itself")
	}
	closed := note(t, s, "Merge closed", "", "")
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/decline", closed), map[string]string{"reason": "no"}, nil))
	if err := s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", into), map[string]int64{"into": closed}, nil); err == nil {
		t.Errorf("REQ-4: merged into a declined request")
	}
	s.Audit(t)
}

// REQ-2, REQ-4: asks sent while merges run are never lost; two crossed merges
// leave every ask on a request that's still there.
func TestREQ2_AskDuringMerge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	for round := range 10 {
		a := note(t, s, fmt.Sprintf("Race lamp %d", round), "A", "0712000001")
		b := note(t, s, fmt.Sprintf("Race lamp %d b", round), "B", "0712000002")
		var wg sync.WaitGroup
		took := make([]bool, 6)
		for i := range 6 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				switch i {
				case 0:
					s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", a), map[string]int64{"into": b}, nil) //nolint:errcheck
				case 1:
					s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", b), map[string]int64{"into": a}, nil) //nolint:errcheck
				default:
					took[i] = askAgain(s, []int64{a, b}[i%2], fmt.Sprintf("Asker %d", i), fmt.Sprintf("071300000%d", i)) == nil
				}
			}()
		}
		wg.Wait()
		n := 0
		for _, ok := range took {
			if ok {
				n++
			}
		}
		var left int
		s.Must(t, s.DB.Get(&left, `SELECT count(*) + COALESCE(sum((SELECT count(*) FROM request_asks q WHERE q.request_id = r.id)), 0)
			FROM requests r WHERE r.id IN ($1, $2)`, a, b))
		if left != 2+n {
			t.Errorf("REQ-2 round %d: %d asks left, want the 2 first asks + %d taken", round, left, n)
		}
	}
	s.Audit(t)
}

// REQ-5, REQ-6: decline needs a reason, reopen clears it; clear done keeps declined ones.
func TestREQ5_DeclineReopenClear(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	id := note(t, s, "Decline caviar", "", "")
	if err := s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/decline", id), map[string]string{"reason": "  "}, nil); err == nil {
		t.Errorf("REQ-5: declined with no reason")
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/decline", id), map[string]string{"reason": "too dear"}, nil))
	done := note(t, s, "Clear this one", "", "")
	p := s.Product(t, "Clear this product", "10", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/link", done), map[string]int64{"productId": p}, nil))
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/requests/clear-done", nil, nil))
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM requests WHERE id = $1`, id))
	if n != 1 {
		t.Errorf("REQ-6: clear done removed a declined request")
	}
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM requests WHERE id = $1`, done))
	if n != 0 {
		t.Errorf("REQ-6: clear done kept a converted one with nobody to call")
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/reopen", id), nil, nil))
	if r, _ := listed(t, s, id); r.Status != "open" || r.Reason != "" || r.ProductID != nil {
		t.Errorf("REQ-5: reopened %+v", r)
	}
	s.Audit(t)
}

// REQ-7: a product merge points the requests at the kept product.
func TestREQ7_ProductMerge(t *testing.T) {
	t.Parallel()
	s := ltest.Shared(t).Shop
	id := note(t, s, "Merge follow", "Wait", "0716000001") // someone to call: clear done keeps it
	a := s.Product(t, "Merge follow A", "10", "0", nil)
	b := s.Product(t, "Merge follow B", "10", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/link", id), map[string]int64{"productId": a}, nil))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", a, b), map[string]string{"lotPrice": "follow"}, nil))
	var got int64
	s.Must(t, s.DB.Get(&got, `SELECT product_id FROM requests WHERE id = $1`, id))
	if got != b {
		t.Errorf("REQ-7: request names %d, want the kept %d", got, b)
	}
	s.Audit(t)
}

// PLG-5, PLG-3: bad requests are refused and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	id := note(t, s, "Refuse request", "", "")
	before := ltest.State(t, s.DB)
	for _, c := range []struct {
		name string
		do   func() error
	}{
		{"empty note", func() error { return s.Cashier.Do("POST", "/api/v1/x/requests", map[string]string{"note": " "}, nil) }},
		{"ask on none", func() error { return askAgain(s, 999999, "x", "") }},
		{"cashier declines", func() error {
			return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/decline", id), map[string]string{"reason": "x"}, nil)
		}},
		{"note on none", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/requests/999999", map[string]string{"note": "x"}, nil)
		}},
		{"empty note edit", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/requests/%d", id), map[string]string{"note": ""}, nil)
		}},
		{"delete none", func() error { return s.Admin.Do("DELETE", "/api/v1/x/requests/999999", nil, nil) }},
		{"delete first ask", func() error { return s.Admin.Do("DELETE", fmt.Sprintf("/api/v1/x/requests/%d/asks/0", id), nil, nil) }},
		{"called none", func() error {
			return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/asks/999999/called", id), nil, nil)
		}},
		{"product no name", func() error {
			return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", id), map[string]any{"name": " ", "categoryId": s.Cat, "unitId": 1}, nil)
		}},
		{"product no category", func() error {
			return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", id), map[string]any{"name": "x"}, nil)
		}},
		{"product bad category", func() error {
			return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", id), map[string]any{"name": "Bad cat thing", "categoryId": 999999, "unitId": 1}, nil)
		}},
		{"product on none", func() error {
			return s.Admin.Do("POST", "/api/v1/x/requests/999999/product", map[string]any{"name": "No req thing", "categoryId": s.Cat, "unitId": 1}, nil)
		}},
		{"merge into none", func() error {
			return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", id), map[string]int64{"into": 999999}, nil)
		}},
		{"bad id", func() error { return s.Admin.Do("POST", "/api/v1/x/requests/abc/reopen", nil, nil) }},
		{"reopen none", func() error { return s.Admin.Do("POST", "/api/v1/x/requests/999999/reopen", nil, nil) }},
	} {
		if err := c.do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", c.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", c.name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

// PLG-1: nothing a request does moves money or stock.
func TestPLG1_NothingMoves(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	before := ltest.State(t, s.DB)
	a := note(t, s, "Still request", "A", "0714000001")
	b := note(t, s, "Still request b", "B", "0714000002")
	s.Must(t, askAgain(s, a, "C", "0714000003"))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", b), map[string]int64{"into": a}, nil))
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", a), map[string]any{"name": "Still request item", "categoryId": s.Cat, "unitId": 1, "sellingPrice": "10"}, nil))
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/requests/clear-done", nil, nil))
	ltest.Untouched(t, before, ltest.State(t, s.DB), ltest.MoneyAndStock)
}

// PLG-4: a failure at any statement leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	var id int64
	fresh := func() {
		id = note(t, s.Shop, "Sweep request", "A", "0715000001")
		s.Must(t, askAgain(s.Shop, id, "B", "0715000002"))
	}
	s.Sweep(t, "add", nil, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/requests", map[string]string{"note": "Sweep add"}, nil)
	})
	s.Sweep(t, "ask again", fresh, func() error { return askAgain(s.Shop, id, "C", "") })
	n := 0
	s.Sweep(t, "make product", fresh, func() error {
		n++
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/product", id), map[string]any{"name": fmt.Sprintf("Sweep made %d", n), "categoryId": s.Cat, "unitId": 1, "sellingPrice": "10"}, nil)
	})
	var into int64
	s.Sweep(t, "merge", func() { fresh(); into = note(t, s.Shop, "Sweep into", "", "") }, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/merge", id), map[string]int64{"into": into}, nil)
	})
	s.Sweep(t, "decline", fresh, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/decline", id), map[string]string{"reason": "no"}, nil)
	})
	s.Sweep(t, "clear done", nil, func() error { return s.Admin.Do("POST", "/api/v1/x/requests/clear-done", nil, nil) })
}

func TestPLG7_DataGroup(t *testing.T) {
	t.Parallel()
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "requests")
}
