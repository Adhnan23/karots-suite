// Package battle is the Catalogue's battle tests in a binary of their own: one shop per process, the real server
// with the Catalogue and Product Plus compiled in, syncing from a stand-in published folder.
package battle

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	_ "karots-suite/plugins/catalogue"
	_ "karots-suite/plugins/productplus"
)

type item struct {
	ID     int64             `json:"id"`
	Name   string            `json:"name"`
	Unit   string            `json:"unit"`
	Kinds  string            `json:"kinds"`
	Fields map[string]string `json:"fields"`
}

// pub is the published folder the shop syncs from (as publish.py writes it; whole versions only: a shop with an
// empty copy, or a gap, loads the whole version, CTL-3).
var pub = struct {
	sync.Mutex
	files map[string][]byte
	v     int
}{files: map[string][]byte{}}

func publish(items []item) {
	pub.Lock()
	defer pub.Unlock()
	pub.v++
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_ = json.NewEncoder(w).Encode(map[string]any{"version": pub.v, "products": items})
	_ = w.Close()
	sum := sha256.Sum256(b.Bytes())
	full := fmt.Sprintf("v%d.json.gz", pub.v)
	pub.files[full] = b.Bytes()
	pub.files["index.json"], _ = json.Marshal(map[string]any{"version": pub.v, "count": len(items),
		"fields": []map[string]string{{"name": "Generic Name", "type": "text"}},
		"full":   map[string]string{"file": full, "sha256": hex.EncodeToString(sum[:])}, "changes": []any{}})
}

var v1 = []item{
	{ID: 1, Name: "Battle Panadol 500mg", Unit: "pcs", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Paracetamol", "Company": "GSK"}},
	{ID: 2, Name: "Battle Bisolol 5mg", Unit: "pcs", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Bisoprolol"}},
	{ID: 3, Name: "Battle Sugar 1kg", Unit: "pcs", Kinds: "grocery", Fields: map[string]string{}},
}

func TestMain(m *testing.M) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pub.Lock()
		defer pub.Unlock()
		b, ok := pub.files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	defer srv.Close()
	publish(v1)
	os.Setenv("KAROTS_CATALOGUE_URL", srv.URL) // read when the shop's plugins are set up
	os.Exit(ltest.Main(m))
}

var synced sync.Once

// shop is the shared shop with its copy synced and a Product Plus "Generic Name" field.
func shop(t *testing.T) *ltest.FaultyShop {
	t.Helper()
	s := ltest.Shared(t)
	synced.Do(func() {
		s.Must(t, s.Admin.Do("POST", "/api/v1/x/catalogue/check", nil, nil))
		s.DB.MustExec(`INSERT INTO pp_fields (key, label, type) VALUES ('generic', 'Generic Name', 'text') ON CONFLICT DO NOTHING`)
	})
	return s
}

func generic(t *testing.T, s *ltest.FaultyShop, pid int64) (v string) {
	t.Helper()
	_ = s.DB.Get(&v, `SELECT v.value FROM pp_values v JOIN pp_fields f ON f.id = v.field_id WHERE f.key = 'generic' AND v.product_id = $1`, pid)
	return
}

// pick makes a product from catalogue item id, as the product form does.
func pick(t *testing.T, s *ltest.FaultyShop, name string, id int64) int64 {
	t.Helper()
	return s.Product(t, name, "10", "0", map[string]any{"extra": map[string]any{"catalogue": map[string]int64{"itemId": id}}})
}

func onReview(t *testing.T, s *ltest.FaultyShop, pid int64) []string {
	t.Helper()
	var rows []struct {
		ID          int64
		ReviewNotes []struct{ Plugin, Text string }
	}
	s.Must(t, s.Admin.Do("GET", "/api/v1/admin/products/review", nil, &rows))
	for _, r := range rows {
		if r.ID == pid {
			var out []string
			for _, n := range r.ReviewNotes {
				out = append(out, n.Text)
			}
			return out
		}
	}
	return nil
}

func TestLedgerChecksFire(t *testing.T) {
	t.Parallel()
	s := shop(t)
	pid := pick(t, s, "Fire pick", 3)
	dup := s.Product(t, "Fire pick dup", "10", "0", nil)
	for _, c := range []struct{ rule, bad string }{
		{"CTL-2", `UPDATE catalogue_state SET count = count + 1`},
		{"CTL-7", `INSERT INTO catalogue_links (product_id, item_id) VALUES (999999, 1)`},
		{"CTL-7", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d`, dup, pid)},
		{"CTL-7", `INSERT INTO catalogue_reviews (product_id, item_id, changes) VALUES (999998, 1, '[]')`},
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

// CTL-5, CTL-6, CTL-7 through the real routes: search, a pick on the product form and the till's quick item,
// the shop's field filled, Items to review until marked reviewed (REV-1, REV-2).
func TestCTL7_PickToReview(t *testing.T) {
	t.Parallel()
	s := shop(t)
	var res []struct {
		ID       int64
		UnitID   int64
		Sections map[string]map[string]map[string]string
	}
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/catalogue/search?q=bisolol+battle", nil, &res)) // item 2: no later version changes it
	if len(res) != 1 || res[0].ID != 2 || res[0].Sections["productplus"]["values"]["generic"] != "Bisoprolol" {
		t.Fatalf("CTL-5/6 search: %+v", res)
	}
	pid := pick(t, s, "Battle Bisolol 5mg form", 2)
	if g := generic(t, s, pid); g != "Bisoprolol" {
		t.Errorf("CTL-6: generic %q", g)
	}
	if n := onReview(t, s, pid); n != nil {
		t.Errorf("CTL-7: a pick on the whole product form is on review: %v", n)
	}

	var q struct{ ID int64 }
	s.OpenTill(t, s.Cashier)
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/quick-item", map[string]any{"name": "Battle Bisolol 5mg", "price": "20", "qty": "1",
		"extra": map[string]any{"catalogue": map[string]int64{"itemId": 2}}}, &q))
	var linked int64
	s.Must(t, s.DB.Get(&linked, `SELECT item_id FROM catalogue_links WHERE product_id = $1`, q.ID))
	if linked != 2 || generic(t, s, q.ID) != "Bisoprolol" {
		t.Errorf("CTL-7 quick item: linked to %d, generic %q", linked, generic(t, s, q.ID))
	}
	if n := onReview(t, s, q.ID); !slices.Contains(n, "From the catalogue") {
		t.Errorf("CTL-7: the quick item's pick isn't on review: %v", n)
	}
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/review-done", q.ID), nil, nil))
	if n := onReview(t, s, q.ID); n != nil {
		t.Errorf("REV-2: still on review %v", n)
	}
	s.Audit(t)
}

// CTL-11: a product merge moves the link to the kept product when it has none; otherwise it goes.
func TestCTL11_ProductMerge(t *testing.T) {
	t.Parallel()
	s := shop(t)
	a := pick(t, s, "Merge pick A", 3)
	b := s.Product(t, "Merge pick B", "10", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", a, b), map[string]string{"lotPrice": "follow"}, nil))
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM catalogue_links WHERE product_id = $1`, b))
	if n != 1 {
		t.Errorf("CTL-11: kept product has %d links", n)
	}
	c, d := pick(t, s, "Merge pick C", 3), pick(t, s, "Merge pick D", 2)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", c, d), map[string]string{"lotPrice": "follow"}, nil))
	var item int64
	s.Must(t, s.DB.Get(&item, `SELECT item_id FROM catalogue_links WHERE product_id = $1`, d))
	if item != 2 {
		t.Errorf("CTL-11: the kept product's own link changed to %d", item)
	}
	s.Audit(t)
}

// PLG-5, PLG-3: bad requests are refused and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := shop(t)
	plain := s.Product(t, "Refuse plain", "10", "0", nil)
	before := ltest.State(t, s.DB)
	for _, c := range []struct {
		name string
		do   func() error
	}{
		{"pick a missing item", func() error {
			return s.Admin.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Refused pick", "categoryId": s.Cat, "unitId": 1,
				"costPrice": "1", "sellingPrice": "2", "extra": map[string]any{"catalogue": map[string]int64{"itemId": 999999}}}, nil)
		}},
		{"pick without an item", func() error {
			return s.Admin.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Refused pick 2", "categoryId": s.Cat, "unitId": 1,
				"costPrice": "1", "sellingPrice": "2", "extra": map[string]any{"catalogue": map[string]any{}}}, nil)
		}},
		{"a long search", func() error {
			return s.Cashier.Do("GET", "/api/v1/x/catalogue/search?q="+strings.Repeat("a", 300), nil, nil)
		}},
		{"use with nothing waiting", func() error {
			return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/catalogue/reviews/%d/use", plain), nil, nil)
		}},
		{"bad product", func() error { return s.Admin.Do("POST", "/api/v1/x/catalogue/reviews/abc/keep", nil, nil) }},
		{"a cashier checks", func() error { return s.Cashier.Do("POST", "/api/v1/x/catalogue/check", nil, nil) }},
		{"a cashier uses", func() error {
			return s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/catalogue/reviews/%d/use", plain), nil, nil)
		}},
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

// CTL-8, CTL-9, CTL-1/PLG-1, PLG-4, PLG-6: a new version changes a picked product's generic; it waits on review;
// Use the catalogue's writes it; nothing moves money or stock; a failure anywhere leaves nothing behind; sent
// twice at once it's done once.
func TestCTL9_LaterChange(t *testing.T) { // syncs the one copy and compares every table: runs alone
	s := shop(t)
	a := pick(t, s, "Battle Panadol 500mg A", 1)
	b := pick(t, s, "Battle Panadol 500mg B", 1)
	c := pick(t, s, "Battle Panadol 500mg C", 1)
	before := ltest.State(t, s.DB)
	v2 := append([]item{}, v1...)
	v2[0] = item{ID: 1, Name: "Battle Panadol 500mg", Unit: "pcs", Kinds: "pharmacy", Fields: map[string]string{"Generic Name": "Acetaminophen", "Company": "GSK"}}
	publish(v2)
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/catalogue/check", nil, nil))
	var waiting []struct{ What, Then, Now string }
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/x/catalogue/reviews/%d", a), nil, &waiting))
	if len(waiting) != 1 || waiting[0].What != "Generic Name" || waiting[0].Now != "Acetaminophen" {
		t.Fatalf("CTL-8: waiting %+v", waiting)
	}

	s.Sweep(t, "use the catalogue's", nil, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/catalogue/reviews/%d/use", a), nil, nil)
	})
	if g := generic(t, s, a); g != "Acetaminophen" {
		t.Errorf("CTL-9: generic %q", g)
	}
	s.Sweep(t, "keep mine", nil, func() error {
		return s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/catalogue/reviews/%d/keep", b), nil, nil)
	})
	if g := generic(t, s, b); g != "Paracetamol" {
		t.Errorf("CTL-9 keep mine: generic %q", g)
	}
	var wg sync.WaitGroup // PLG-6: twice at once
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/catalogue/reviews/%d/use", c), nil, nil)
		}()
	}
	wg.Wait()
	if (errs[0] == nil) == (errs[1] == nil) {
		t.Errorf("PLG-6: twice at once gave %v and %v, want one done and one refused", errs[0], errs[1])
	}
	ltest.Untouched(t, before, ltest.State(t, s.DB), ltest.MoneyAndStock)
	s.Audit(t)
}

// PLG-7: the data group holds every table.
func TestPLG7_DataGroup(t *testing.T) {
	t.Parallel()
	shop(t)
	ltest.DataGroupHoldsTables(t, "catalogue")
}
