package requests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"
	"karots-suite/plugins/requests/migrations"

	"github.com/jmoiron/sqlx"
)

// testStore migrates requests onto the dev DB. Every request a test adds is
// removed afterwards; notes carry a per-test tag so List / Similar see only them.
func testStore(t *testing.T) (*Store, *sqlx.DB, string) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := appdb.RunMigrationsTable(db.DB, migrations.FS, "goose_db_version_requests"); err != nil {
		t.Fatal(err)
	}
	tag := fmt.Sprintf("zq%d", time.Now().UnixNano())
	t.Cleanup(func() { db.Exec(`DELETE FROM requests WHERE note LIKE '%' || $1 || '%'`, tag) }) //nolint:errcheck
	return NewStore(db), db, tag
}

// product makes a product of the test's own with qty on hand.
func product(t *testing.T, db *sqlx.DB, qty string) int64 {
	t.Helper()
	var pid int64
	if err := db.Get(&pid, `INSERT INTO products (name, category_id, unit_id, cost_price, selling_price)
		VALUES ($1, (SELECT id FROM categories LIMIT 1), (SELECT id FROM units LIMIT 1), 0, 100) RETURNING id`,
		fmt.Sprintf("Req test %d", time.Now().UnixNano())); err != nil {
		t.Fatal(err)
	}
	setStock(t, db, pid, qty)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM stock WHERE product_id = $1`, pid) //nolint:errcheck
		db.Exec(`DELETE FROM products WHERE id = $1`, pid)      //nolint:errcheck
	})
	return pid
}

func setStock(t *testing.T, db *sqlx.DB, pid int64, qty string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO stock (product_id, quantity) VALUES ($1, $2)
		ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, pid, qty); err != nil {
		t.Fatal(err)
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func add(t *testing.T, s *Store, note, name, phone string) int64 {
	t.Helper()
	id, err := s.Add(context.Background(), note, name, phone, 0)
	noErr(t, err)
	return id
}

func similar(t *testing.T, s *Store, q string) []Request {
	t.Helper()
	rows, err := s.Similar(context.Background(), q)
	noErr(t, err)
	return rows
}

func get(t *testing.T, s *Store, tab, tag string, id int64) (Request, bool) {
	t.Helper()
	rows, _, err := s.List(context.Background(), tab, tag, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ID == id {
			return r, true
		}
	}
	return Request{}, false
}

func TestAsksAddUp(t *testing.T) {
	s, _, tag := testStore(t)
	ctx := context.Background()
	one := add(t, s, "Red umbrella "+tag, "Nimal", "0771234567")
	two := add(t, s, "Blue pen "+tag, "", "")
	if err := s.AddAsk(ctx, two, "Kamala", "0712223334", 0); err != nil {
		t.Fatal(err)
	}
	rows, counts, err := s.List(ctx, "open", tag, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != two || len(rows[0].Asks) != 2 || len(rows[1].Asks) != 1 {
		t.Fatalf("want the pen first with 2 asks, got %+v", rows)
	}
	if a := rows[1].Asks[0]; a.ID != 0 || a.Name != "Nimal" || a.Phone != "0771234567" || rows[1].ID != one {
		t.Fatalf("first ask is the row itself: %+v", a)
	}
	if counts.Open < 2 {
		t.Fatalf("counts: %+v", counts)
	}
}

func TestSimilar(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	open := add(t, s, "Red umbrella large "+tag, "", "")
	declined := add(t, s, "Red umbrella small "+tag, "", "")
	if err := s.Decline(ctx, declined, "discontinued"); err != nil {
		t.Fatal(err)
	}
	stocked := add(t, s, "Red umbrella kids "+tag, "", "0770000000")
	if err := s.MarkConverted(ctx, stocked, product(t, db, "5")); err != nil {
		t.Fatal(err)
	}
	got := similar(t, s, "red umb "+tag)
	ids := map[int64]Request{}
	for _, r := range got {
		ids[r.ID] = r
	}
	if _, ok := ids[open]; !ok {
		t.Fatalf("open one not found: %+v", got)
	}
	if r, ok := ids[declined]; !ok || r.Reason != "discontinued" || r.Status != "declined" {
		t.Fatalf("declined one with its reason: %+v", got)
	}
	if _, ok := ids[stocked]; ok {
		t.Fatal("a request whose product is in stock is not offered")
	}
	if len(similar(t, s, "blue "+tag)) != 0 {
		t.Fatal("every word must match")
	}
}

func TestWaitingThenCallThenDone(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	pid := product(t, db, "0")
	id := add(t, s, "Torch "+tag, "Sunil", "0779998887")
	if err := s.MarkConverted(ctx, id, pid); err != nil {
		t.Fatal(err)
	}
	if r, ok := get(t, s, "open", tag, id); !ok || r.Stage != "waiting" || r.ProductName == "" {
		t.Fatalf("no stock yet → waiting on Open: %+v %v", r, ok)
	}
	setStock(t, db, pid, "5")
	if r, ok := get(t, s, "call", tag, id); !ok || r.Stage != "call" {
		t.Fatalf("in stock → call: %+v", r)
	}
	if err := s.MarkCalled(ctx, id, 0); err != nil {
		t.Fatal(err)
	}
	if r, ok := get(t, s, "done", tag, id); !ok || r.Stage != "done" || r.Asks[0].CalledAt == nil {
		t.Fatalf("called → done: %+v", r)
	}
	if err := s.AddAsk(ctx, id, "", "", 0); !errors.Is(err, ErrClosed) {
		t.Fatalf("asking again on a done request: %v", err)
	}
}

func TestLinkWithNoPhoneIsDone(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	id := add(t, s, "Glue "+tag, "", "")
	if err := s.MarkConverted(ctx, id, product(t, db, "0")); err != nil {
		t.Fatal(err)
	}
	if _, ok := get(t, s, "done", tag, id); !ok {
		t.Fatal("nobody to call → done straight away")
	}
}

func TestClearDoneSparesDeclinedAndCalls(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	done := add(t, s, "Done "+tag, "", "")
	declined := add(t, s, "Declined "+tag, "", "")
	call := add(t, s, "Call "+tag, "", "0771110000")
	noErr(t, s.MarkConverted(ctx, done, product(t, db, "0")))
	noErr(t, s.Decline(ctx, declined, "too dear"))
	noErr(t, s.MarkConverted(ctx, call, product(t, db, "3")))
	n, err := s.ClearDone(ctx)
	noErr(t, err)
	if n < 1 {
		t.Fatalf("cleared %d", n)
	}
	if _, ok := get(t, s, "done", tag, done); ok {
		t.Fatal("the converted done one is cleared")
	}
	if _, ok := get(t, s, "done", tag, declined); !ok {
		t.Fatal("won't-stock stays")
	}
	if _, ok := get(t, s, "call", tag, call); !ok {
		t.Fatal("someone still to call stays")
	}
}

func TestMerge(t *testing.T) {
	s, _, tag := testStore(t)
	ctx := context.Background()
	a := add(t, s, "Phone cover A "+tag, "Ruwan", "")
	b := add(t, s, "Phone cover B "+tag, "Mala", "0715556667")
	noErr(t, s.AddAsk(ctx, b, "Saman", "", 0))
	if err := s.Merge(ctx, b, a); err != nil {
		t.Fatal(err)
	}
	r, ok := get(t, s, "open", tag, a)
	if !ok || len(r.Asks) != 3 {
		t.Fatalf("a holds all three asks: %+v", r)
	}
	if r.Asks[1].Name != "Mala" || r.Asks[1].Phone != "0715556667" || r.Asks[1].ID == 0 {
		t.Fatalf("b's own ask moved as a row: %+v", r.Asks)
	}
	if _, ok := get(t, s, "open", tag, b); ok {
		t.Fatal("b is gone")
	}
	if err := s.Merge(ctx, a, a); err == nil {
		t.Fatal("merging into itself is refused")
	}
	c := add(t, s, "Phone cover C "+tag, "", "")
	noErr(t, s.Decline(ctx, c, "no"))
	if err := s.Merge(ctx, a, c); !errors.Is(err, ErrClosed) {
		t.Fatalf("merging into a declined one: %v", err)
	}
}

func TestReopenClears(t *testing.T) {
	s, db, tag := testStore(t)
	ctx := context.Background()
	id := add(t, s, "Reopen "+tag, "", "")
	noErr(t, s.MarkConverted(ctx, id, product(t, db, "0")))
	noErr(t, s.Reopen(ctx, id))
	r, ok := get(t, s, "open", tag, id)
	if !ok || r.ProductID != nil || r.Reason != "" || r.Status != "open" {
		t.Fatalf("reopened: %+v", r)
	}
}
