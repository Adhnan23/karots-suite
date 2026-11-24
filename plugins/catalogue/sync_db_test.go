package catalogue

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
	"reflect"
	"sort"
	"sync"
	"testing"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/catalogue/migrations"

	"github.com/jmoiron/sqlx"
)

// testDB is a throwaway shop database with the plugin's tables.
func testDB(t *testing.T) *sqlx.DB {
	t.Helper()
	db := ltest.NewDB(t)
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_catalogue"); err != nil {
		t.Fatal(err)
	}
	return db
}

// pub is a stand-in for the published folder: versions are built the way publish.py builds them.
type pub struct {
	mu    sync.Mutex
	files map[string][]byte
	idx   index
	prev  map[int64]Item
	srv   *httptest.Server
}

func newPub(t *testing.T) *pub {
	p := &pub{files: map[string][]byte{}, prev: map[int64]Item{}}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		b, ok := p.files[r.URL.Path[1:]]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func gzJSON(v any) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_ = json.NewEncoder(w).Encode(v)
	_ = w.Close()
	return b.Bytes()
}

func shaOf(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// publish puts out the next version (into: gone id → kept id).
func (p *pub) publish(items []Item, into map[int64]int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v := p.idx.Version + 1
	now := map[int64]Item{}
	for _, it := range items {
		now[it.ID] = it
	}
	c := changes{From: v - 1, To: v}
	for _, it := range items {
		if o, ok := p.prev[it.ID]; !ok {
			c.Added = append(c.Added, it)
		} else if !sameItem(o, it) {
			c.Changed = append(c.Changed, it)
		}
	}
	for id := range p.prev {
		if _, ok := now[id]; !ok {
			c.Gone = append(c.Gone, gone{ID: id, Into: into[id]})
		}
	}
	cb, vb := gzJSON(c), gzJSON(version{Version: v, Products: items})
	delete(p.files, p.idx.Full.File)
	p.files[fmt.Sprintf("c%d.json.gz", v)], p.files[fmt.Sprintf("v%d.json.gz", v)] = cb, vb
	p.idx = index{Version: v, Count: len(items), Fields: []Field{{Name: "Generic Name", Type: "text"}},
		Full:    fileRef{File: fmt.Sprintf("v%d.json.gz", v), Sha256: shaOf(vb)},
		Changes: append(p.idx.Changes, changeRef{To: v, File: fmt.Sprintf("c%d.json.gz", v), Sha256: shaOf(cb)})}
	p.files["index.json"], _ = json.Marshal(p.idx)
	p.prev = now
}

func it(id int64, name, generic string) Item {
	f := map[string]string{}
	if generic != "" {
		f["Generic Name"] = generic
	}
	return Item{ID: id, Name: name, Unit: "tab", Kinds: "pharmacy", Fields: f}
}

// copyOf is the shop's copy, sorted by id.
func copyOf(t *testing.T, db *sqlx.DB) []Item {
	t.Helper()
	m, err := loadItems(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]Item, 0, len(m))
	for _, v := range m {
		if len(v.Fields) == 0 {
			v.Fields = map[string]string{}
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func versionOf(t *testing.T, db *sqlx.DB) (v int, errText string) {
	t.Helper()
	if err := db.QueryRow(`SELECT version, error FROM catalogue_state WHERE id = 1`).Scan(&v, &errText); err != nil {
		t.Fatal(err)
	}
	return
}

// CTL-2: after a sync the copy is exactly the published version; the change files in order end the same as the
// whole version.
func TestSyncCopyIsThePublishedVersion(t *testing.T) {
	p := newPub(t)
	ctx := context.Background()
	stepwise, whole := testDB(t), testDB(t)
	a, b := NewSyncer(stepwise, p.srv.URL), NewSyncer(whole, p.srv.URL)

	v1 := []Item{it(1, "Panadol 500mg", "Paracetamol"), it(2, "Bisolol 2.5mg", "Bisoprolol"), it(3, "Tel OD 20", "")}
	p.publish(v1, nil)
	if v, err := a.Sync(ctx); err != nil || v != 1 {
		t.Fatalf("first sync: %d %v", v, err)
	}
	if got := copyOf(t, stepwise); !reflect.DeepEqual(got, v1) {
		t.Fatalf("copy %+v\nwant %+v", got, v1)
	}
	v2 := []Item{it(1, "Panadol 500mg", "Paracetamol"), it(2, "Bisolol 2.5mg", "Bisoprolol Fumarate"), it(4, "Concor 5mg", "Bisoprolol")}
	p.publish(v2, map[int64]int64{3: 1})
	v3 := []Item{it(1, "Panadol Tablets 500mg", "Paracetamol"), it(2, "Bisolol 2.5mg", "Bisoprolol Fumarate"), it(4, "Concor 5mg", "Bisoprolol"), it(5, "New One", "")}
	p.publish(v3, nil)
	if v, err := a.Sync(ctx); err != nil || v != 3 {
		t.Fatalf("change files: %d %v", v, err)
	}
	if v, err := b.Sync(ctx); err != nil || v != 3 { // an empty copy loads the whole version
		t.Fatalf("whole: %d %v", v, err)
	}
	if got, want := copyOf(t, stepwise), copyOf(t, whole); !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got, v3) {
		t.Fatalf("change files gave %+v\nwhole gave %+v\nwant %+v", got, want, v3)
	}
	if v, err := a.Sync(ctx); err != nil || v != 3 {
		t.Fatalf("nothing new: %d %v", v, err)
	}
	var fields string
	stepwise.Get(&fields, `SELECT fields::text FROM catalogue_state`) //nolint:errcheck
	if fields != `[{"name": "Generic Name", "type": "text"}]` {
		t.Fatalf("field list %s", fields)
	}
}

// CTL-3: a bad checksum, a cut file, a missing change file or an older index never leave a half copy and never
// go back; a gap loads the whole version.
func TestSyncAllOrNothingForwardOnly(t *testing.T) {
	p := newPub(t)
	ctx := context.Background()
	db := testDB(t)
	s := NewSyncer(db, p.srv.URL)
	v1 := []Item{it(1, "A", ""), it(2, "B", "")}
	p.publish(v1, nil)
	if _, err := s.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	p.publish([]Item{it(1, "A2", ""), it(2, "B", ""), it(3, "C", "")}, nil)

	good := p.files["c2.json.gz"]
	for name, bad := range map[string][]byte{
		"a wrong checksum": append(append([]byte{}, good...), 0),
		"a cut file":       good[:len(good)/2],
	} {
		p.files["c2.json.gz"] = bad
		if _, err := s.Sync(ctx); err == nil {
			t.Fatalf("%s: synced", name)
		}
		if v, e := versionOf(t, db); v != 1 || e == "" || !reflect.DeepEqual(copyOf(t, db), v1) {
			t.Fatalf("%s: version %d, error %q, copy %+v", name, v, e, copyOf(t, db))
		}
	}
	// A missing change file: the whole version instead.
	delete(p.files, "c2.json.gz")
	if v, err := s.Sync(ctx); err != nil || v != 2 || len(copyOf(t, db)) != 3 {
		t.Fatalf("gap: %d %v %+v", v, err, copyOf(t, db))
	}
	if _, e := versionOf(t, db); e != "" {
		t.Fatalf("error kept after a good sync: %q", e)
	}
	// An older index: refused, the copy stays.
	p.idx.Version = 1
	p.files["index.json"], _ = json.Marshal(p.idx)
	if v, err := s.Sync(ctx); err == nil || v != 2 || len(copyOf(t, db)) != 3 {
		t.Fatalf("went back: %d %v", v, err)
	}
	// No address: says so.
	if _, err := NewSyncer(db, "").Sync(ctx); err == nil {
		t.Fatal("synced without an address")
	}
}
