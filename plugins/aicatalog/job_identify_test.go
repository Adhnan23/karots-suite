package aicatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

func rowOf(rows []JobRow, id int64, kind string) *JobRow {
	for i := range rows {
		if rows[i].ProductID == id && rows[i].Kind == kind {
			return &rows[i]
		}
	}
	return nil
}

func TestIdentifyRealUnknownIsNoAnswer(t *testing.T) {
	db := freshDB(t, "aicatalog_identify_unknown")
	p := &Plugin{store: NewStore(db)}
	c := cat(t, db, "Pain Relief", nil)
	x := prod(t, db, "xyzq 77", c)
	b := Batch{Products: []JobProduct{{ID: x, Name: "xyzq 77", Category: "Pain Relief"}}}
	for name, reply := range map[string]string{
		"chatgpt": fmt.Sprintf(`{"items":[{"id":%d,"name":"Unknown Product (XYZQ 77)","category":"Unclassified","specs":"cannot be established","explanation":"cannot be identified","confident":false}]}`, x),
		"gemini":  fmt.Sprintf(`{"items":[{"id":%d,"name":"","category":"","specs":"","explanation":"","confident":false}]}`, x),
	} {
		rows, err := identifyParse(context.Background(), p, b, reply)
		if err != nil || len(rows) != 0 {
			t.Errorf("%s: %+v %v", name, rows, err)
		}
	}
}

func TestIdentifyTicks(t *testing.T) {
	db := freshDB(t, "aicatalog_identify_ticks")
	p := &Plugin{store: NewStore(db)}
	ctx := context.Background()
	un := cat(t, db, "Uncategorized", nil)
	pain := cat(t, db, "Pain Relief", nil)
	a, b := prod(t, db, "panadol 500", un), prod(t, db, "amoxil 250 cap", pain)
	batch := Batch{Products: []JobProduct{{ID: a, Name: "panadol 500", CategoryID: un, Category: "Uncategorized"}, {ID: b, Name: "amoxil 250 cap", CategoryID: pain, Category: "Pain Relief"}}}
	rows, err := identifyParse(ctx, p, batch, fmt.Sprintf(`{"items":[
		{"id":%d,"confident":true,"name":"Panadol 500mg Tablets","category":"Pain Relief","specs":"Paracetamol 500mg","explanation":"For pain"},
		{"id":%d,"confident":true,"name":"Amoxil 250mg Capsules","category":"Antibiotics / Penicillins","specs":"Amoxicillin","explanation":"An antibiotic"}]}`, a, b))
	if err != nil {
		t.Fatal(err)
	}
	if r := rowOf(rows, a, "hint"); r == nil || !r.Ticked || r.Old != "" || r.Extra["specs"] != "Paracetamol 500mg" {
		t.Errorf("first hint is ticked: %+v", r)
	}
	if r := rowOf(rows, a, "category"); r == nil || !r.Ticked || r.IsNew || r.New != "Pain Relief" {
		t.Errorf("uncategorised: ticked, existing: %+v", r)
	}
	if r := rowOf(rows, b, "category"); r == nil || r.Ticked || !r.Change || !r.IsNew || r.New != "Antibiotics > Penicillins" {
		t.Errorf("a differing category starts unticked, marked new: %+v", r)
	}
	for _, id := range []int64{a, b} {
		if r := rowOf(rows, id, "name"); r == nil || r.Ticked || !r.Change {
			t.Errorf("a name starts unticked: %+v", r)
		}
	}
}

func TestIdentifySameValuesNoRows(t *testing.T) {
	db := freshDB(t, "aicatalog_identify_same")
	p := &Plugin{store: NewStore(db)}
	c := cat(t, db, "Cleaning", nil)
	a := prod(t, db, "Harpic 500ml", c)
	b := Batch{Products: []JobProduct{{ID: a, Name: "Harpic 500ml", CategoryID: c, Category: "Cleaning"}}}
	rows, err := identifyParse(context.Background(), p, b, fmt.Sprintf(`{"items":[{"id":%d,"confident":true,"name":"harpic 500ML","category":"cleaning"}]}`, a))
	if err != nil || len(rows) != 0 {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestRedoShowsOldUnticked(t *testing.T) {
	db := freshDB(t, "aicatalog_identify_redo")
	p := &Plugin{store: NewStore(db)}
	c := cat(t, db, "Cleaning", nil)
	a := prod(t, db, "Harpic", c)
	b := Batch{Redo: true, Products: []JobProduct{{ID: a, Name: "Harpic", CategoryID: c, Category: "Cleaning", Specs: "old specs", Explain: "old words"}}}
	rows, err := identifyParse(context.Background(), p, b, fmt.Sprintf(`{"items":[{"id":%d,"confident":true,"name":"Harpic","category":"Cleaning","specs":"new specs","explanation":"new words"}]}`, a))
	if err != nil {
		t.Fatal(err)
	}
	if r := rowOf(rows, a, "hint"); r == nil || r.Ticked || r.Old != "old specs\nold words" {
		t.Fatalf("%+v", r)
	}
}

func applyIdentify(t *testing.T, p *Plugin, rows ...JobRow) JobApplied {
	t.Helper()
	var ids []int64
	for _, r := range rows {
		ids = append(ids, r.ProductID)
	}
	res, err := p.applyJob(context.Background(), &Job{Key: "identify", Label: "Identify", Apply: identifyApply, Undo: identifyUndo}, JobApply{Rows: rows, Asked: ids, Suggested: ids}, 0)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestIdentifyApplyAndUndo(t *testing.T) {
	db := freshDB(t, "aicatalog_identify_apply")
	p, _ := testAPI(t, db)
	p.jobs = []Job{identifyJob()}
	ctx := context.Background()
	c := cat(t, db, "Old", nil)
	a := prod(t, db, "6201 ZZ", c)
	mustExec(t, db, nil, `UPDATE products SET selling_price = 77, cost_price = 55 WHERE id = $1`, a)
	res := applyIdentify(t, p,
		JobRow{ProductID: a, Kind: "name", Label: "Name", Old: "6201 ZZ", New: "Ball Bearing 6201"},
		JobRow{ProductID: a, Kind: "category", Label: "Category", Old: "Old", New: "Bearings > Ball"},
		JobRow{ProductID: a, Kind: "hint", Label: "Hint", New: "12x32x10\nA bearing", Extra: map[string]string{"specs": "12x32x10", "explanation": "A bearing"}})
	if res.Applied != 3 || len(res.Skipped) != 0 {
		t.Fatalf("%+v", res)
	}
	var row struct {
		Name string  `db:"name"`
		Cat  int64   `db:"category_id"`
		Sell float64 `db:"selling_price"`
		Cost float64 `db:"cost_price"`
	}
	read := func() {
		t.Helper()
		if err := db.Get(&row, `SELECT name, category_id, selling_price, cost_price FROM products WHERE id = $1`, a); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if row.Name != "Ball Bearing 6201" || row.Cat == c || row.Sell != 77 || row.Cost != 55 {
		t.Fatalf("%+v", row)
	}
	if got, _ := p.store.MatchHints(ctx, "6201 zz"); len(got) != 1 {
		t.Fatal("the old name isn't searchable")
	}
	if got, _ := p.store.MatchHints(ctx, "12x32x10"); len(got) != 1 {
		t.Fatal("the hint isn't searchable")
	}
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	read()
	if row.Name != "6201 ZZ" || row.Cat != c {
		t.Fatalf("undo: %+v", row)
	}
	if n := count(t, db, `SELECT count(*) FROM categories WHERE name IN ('Bearings', 'Ball')`); n != 0 {
		t.Fatalf("new categories stay after undo: %d", n)
	}
	it, _ := p.store.GetItem(ctx, a)
	if it.Specs != "" {
		t.Fatalf("hint after undo: %+v", it)
	}
}

func TestIdentifyStaleRows(t *testing.T) {
	db := freshDB(t, "aicatalog_identify_stale")
	p, _ := testAPI(t, db)
	p.jobs = []Job{identifyJob()}
	c := cat(t, db, "Goods", nil)
	a := prod(t, db, "Edited since", c)
	if err := p.store.SaveHint(context.Background(), a, "written by hand", ""); err != nil {
		t.Fatal(err)
	}
	res := applyIdentify(t, p,
		JobRow{ProductID: a, Kind: "name", Old: "Before", New: "After", Label: "Name"},
		JobRow{ProductID: a, Kind: "hint", Old: "", New: "AI words", Extra: map[string]string{"specs": "AI words"}, Label: "Hint"})
	if res.Applied != 0 || len(res.Skipped) != 2 {
		t.Fatalf("%+v", res)
	}
	it, _ := p.store.GetItem(context.Background(), a)
	if it.Specs != "written by hand" {
		t.Fatal("overwrote a hand-written hint")
	}
}

func TestCleanupRenameKeepsOldName(t *testing.T) {
	db := freshDB(t, "aicatalog_cleanup_oldname")
	s := NewStore(db)
	ctx := context.Background()
	c := cat(t, db, "Goods", nil)
	a := prod(t, db, "PN-114", c)
	raw, _ := json.Marshal(map[string]any{"names": []map[string]any{{"product_id": a, "to": "Brake pad"}}})
	if _, _, err := s.ApplyPlan(ctx, planOf(t, string(raw)), 0, 0); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.MatchHints(ctx, "pn-114"); len(got) != 1 {
		t.Fatal("a clean-up rename lost the old name")
	}
}
