package aicatalog

import (
	"context"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
)

func hintProduct(t *testing.T, db *sqlx.DB, name string) int64 {
	t.Helper()
	var cat, id int64
	mustExec(t, db, &cat, `SELECT id FROM categories ORDER BY id LIMIT 1`)
	mustExec(t, db, &id, `INSERT INTO products (name, category_id, unit_id) VALUES ($1, $2, (SELECT id FROM units ORDER BY id LIMIT 1)) RETURNING id`, name, cat)
	return id
}

func TestMatchHints(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_match")
	s, ctx := NewStore(db), context.Background()
	id := hintProduct(t, db, "BP-114")
	if err := s.SaveHint(ctx, id, "Fits CT100 and Boxer", "Brake pad for small bikes"); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"ct100", "CT100", "100 ct", "brake boxer", "brake"} {
		if got, err := s.MatchHints(ctx, q); err != nil || !slices.Contains(got, id) {
			t.Errorf("%q: %v %v", q, got, err)
		}
	}
	for _, q := range []string{"xyz", "brake xyz", "", "   "} {
		if got, _ := s.MatchHints(ctx, q); len(got) != 0 {
			t.Errorf("%q matched %v", q, got)
		}
	}
}

func TestMatchHintsLiteral(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_literal")
	s, ctx := NewStore(db), context.Background()
	plain, odd := hintProduct(t, db, "A"), hintProduct(t, db, "B")
	if err := s.SaveHint(ctx, plain, "plain text", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveHint(ctx, odd, `100% cotton_blend \ back`, ""); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"%", "_", `\`} {
		if got, err := s.MatchHints(ctx, q); err != nil || !slices.Equal(got, []int64{odd}) {
			t.Errorf("%q: %v %v", q, got, err)
		}
	}
}

func TestMatchOldNameAndQuery(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_old")
	s, ctx := NewStore(db), context.Background()
	id := hintProduct(t, db, "Ball Bearing")
	if err := s.AddOldName(ctx, db, id, "6201 ZZ"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddOldName(ctx, db, id, "6201 ZZ"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MatchHints(ctx, "6201"); !slices.Contains(got, id) {
		t.Fatal("old name isn't searchable")
	}
	it, _ := s.GetItem(ctx, id)
	if strings.Count(it.OldNames, "6201 ZZ") != 1 {
		t.Fatalf("old names: %q", it.OldNames)
	}
	other := hintProduct(t, db, "Spark plug")
	if err := s.LearnItem(ctx, LearnedItem{ProductID: other, ResolvedName: "Spark plug", Source: "user_picked", RawQuery: "CR7HSA"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MatchHints(ctx, "cr7hsa"); !slices.Contains(got, other) {
		t.Fatal("the code first typed isn't searchable")
	}
}

func TestSaveHintCaps(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_caps")
	s, ctx := NewStore(db), context.Background()
	id := hintProduct(t, db, "A")
	long := strings.Repeat("අ", 5000)
	if err := s.SaveHint(ctx, id, long, long); err != nil {
		t.Fatal(err)
	}
	it, _ := s.GetItem(ctx, id)
	if utf8.RuneCountInString(it.Specs) != maxHint || utf8.RuneCountInString(it.UserExplanation) != maxHint || !utf8.ValidString(it.Specs) {
		t.Fatalf("specs %d, explanation %d", utf8.RuneCountInString(it.Specs), utf8.RuneCountInString(it.UserExplanation))
	}
	// A chatbot's long answer through Identify is capped the same.
	if err := s.LearnItem(ctx, LearnedItem{ProductID: id, ResolvedName: "A", Source: "user_picked", Specs: long}); err != nil {
		t.Fatal(err)
	}
	it, _ = s.GetItem(ctx, id)
	if utf8.RuneCountInString(it.Specs) != maxHint {
		t.Fatalf("learned specs %d", utf8.RuneCountInString(it.Specs))
	}
}

func TestSaveHintBlankKeepsNames(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_blank")
	s, ctx := NewStore(db), context.Background()
	id := hintProduct(t, db, "A")
	if err := s.LearnItem(ctx, LearnedItem{ProductID: id, ResolvedName: "A", Source: "user_picked", Specs: "x", RawQuery: "code"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveHint(ctx, id, " corrected ", ""); err != nil {
		t.Fatal(err)
	}
	it, _ := s.GetItem(ctx, id)
	if it.Specs != "corrected" || it.RawQuery != "code" || it.Source != "user_picked" {
		t.Fatalf("%+v", it)
	}
}

func TestHintRows(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_rows")
	p := &Plugin{store: NewStore(db)}
	ctx := context.Background()
	id := hintProduct(t, db, "A")
	if rows, _ := p.detailRows(ctx, id); len(rows) != 0 {
		t.Fatalf("no item: %+v", rows)
	}
	if err := p.store.LearnItem(ctx, LearnedItem{ProductID: id, ResolvedName: "A", Source: "user_picked"}); err != nil {
		t.Fatal(err)
	}
	if rows, _ := p.detailRows(ctx, id); len(rows) != 0 {
		t.Fatalf("no details: %+v", rows)
	}
	if err := p.store.SaveHint(ctx, id, "Fits CT100", "A brake pad"); err != nil {
		t.Fatal(err)
	}
	rows, err := p.detailRows(ctx, id)
	if err != nil || len(rows) != 3 || rows[0].Label != "AI hint" || rows[0].Value != "Fits CT100" ||
		rows[1].Label != "What it is" || rows[1].Value != "A brake pad" || rows[2].Value != hintCaution {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestHintLines(t *testing.T) {
	db := freshDB(t, "aicatalog_hints_lines")
	s, ctx := NewStore(db), context.Background()
	a, b, c := hintProduct(t, db, "A"), hintProduct(t, db, "B"), hintProduct(t, db, "C")
	if err := s.SaveHint(ctx, a, strings.Repeat("x", 200), "y"); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveHint(ctx, b, "", "only explained"); err != nil {
		t.Fatal(err)
	}
	m, err := s.HintLines(ctx, []int64{a, b, c})
	if err != nil || utf8.RuneCountInString(m[a]) != 80+utf8.RuneCountInString(hintLinePrefix) || m[b] != hintLinePrefix+"only explained" {
		t.Fatalf("%v %v", m, err)
	}
	if _, ok := m[c]; ok {
		t.Fatal("no hint, no line")
	}
}
