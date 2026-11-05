package aicatalog

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

func pharmEnv(t *testing.T, name string) (*Plugin, *fakeAttrs, int64) {
	t.Helper()
	db := freshDB(t, name)
	p, _ := testAPI(t, db)
	f := &fakeAttrs{vals: map[int64]map[string]string{}, known: map[string][]string{"dose": {"1-0-1", "1-1-1", "0-0-1"}}}
	p.attrSets = func() []plugin.ProductAttributeSet {
		s := f.hook("pharmacy")
		s.AddKnown = func(_ context.Context, a string, v plugin.KnownValue) error {
			f.known[a] = append(f.known[a], v.Value)
			return nil
		}
		s.RemoveKnown = func(_ context.Context, a, value string) error {
			for i, v := range f.known[a] {
				if v == value {
					f.known[a] = append(f.known[a][:i], f.known[a][i+1:]...)
				}
			}
			return nil
		}
		return []plugin.ProductAttributeSet{s}
	}
	p.jobs = []Job{marksJob(), doseJob()}
	return p, f, cat(t, db, "Medicine", nil)
}

func TestMarksRealReplyAndNull(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_marks_real")
	ctx := context.Background()
	db := p.store.db
	tram, unsure := prod(t, db, "Tramadol 50mg", c), prod(t, db, "Unsure", c)
	f.vals[tram] = map[string]string{"rx_only": "yes"}
	b := Batch{Products: []JobProduct{{ID: tram}, {ID: unsure}}}
	rows, err := marksParse(ctx, p, b, fmt.Sprintf(`{"items":[{"id":%d,"rx_only":true,"controlled":true,"reason":"Tramadol is a synthetic opioid."},{"id":%d,"rx_only":null,"controlled":null,"reason":""},{"id":555,"rx_only":true,"controlled":true}]}`, tram, unsure))
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, r := range rows {
		if r.Ticked || r.ProductID != tram || r.Reason == "" {
			t.Errorf("every mark starts unticked, with its reason: %+v", r)
		}
	}
	res, err := p.applyJob(ctx, p.job("marks"), JobApply{Rows: rows[:1], Asked: []int64{tram, unsure}, Suggested: []int64{tram}}, 0)
	if err != nil || res.Applied != 1 || f.vals[tram]["controlled"] != "yes" {
		t.Fatalf("%+v %v %+v", res, err, f.vals)
	}
	var state string
	mustExec2(t, db, &state, `SELECT state FROM aicatalog_job_items WHERE job = 'marks' AND product_id = $1`, unsure)
	if state != "no_answer" {
		t.Fatalf("no answer stays visible: %q", state)
	}
}

func TestMarksYesToNoIsAChange(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_marks_change")
	a := prod(t, p.store.db, "A", c)
	f.vals[a] = map[string]string{"rx_only": "yes"}
	rows, err := marksParse(context.Background(), p, Batch{Products: []JobProduct{{ID: a}}}, fmt.Sprintf(`{"items":[{"id":%d,"rx_only":false,"controlled":null}]}`, a))
	r := rowOf2(rows, "rx_only")
	if err != nil || r == nil || !r.Change || r.Ticked {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestMarksHasAWarning(t *testing.T) {
	j := marksJob()
	if !strings.Contains(j.Warning, "does not mean cleared") {
		t.Fatal(j.Warning)
	}
}

func doseBatch(ids ...int64) Batch {
	b := Batch{}
	for _, id := range ids {
		b.Products = append(b.Products, JobProduct{ID: id})
	}
	return b
}

// The two chatbots' real answers: "new" as the dose, and a blank one with new true.
func TestDoseRealNewAnswers(t *testing.T) {
	p, _, c := pharmEnv(t, "aicatalog_dose_new")
	a := prod(t, p.store.db, "Piriton", c)
	for name, reply := range map[string]string{
		"chatgpt": `{"items":[{"id":%d,"dose":"new","new":true,"meaning":"As needed, up to every 4–6 hours","per_day":4,"reason":"x"}]}`,
		"gemini":  `{"items":[{"id":%d,"dose":"","new":true,"meaning":"As needed","per_day":3,"reason":"x"}]}`,
	} {
		rows, err := doseParse(context.Background(), p, doseBatch(a), fmt.Sprintf(reply, a))
		if err != nil || len(rows) != 1 {
			t.Fatalf("%s: %+v %v", name, rows, err)
		}
		r := rows[0]
		if r.New != "" || r.Needs == "" || r.Ticked || !r.IsNew || r.Extra["meaning"] == "" {
			t.Errorf("%s: never stored as a dose; the pharmacist types it: %+v", name, r)
		}
	}
	res, err := p.applyJob(context.Background(), p.job("dose"), JobApply{Rows: []JobRow{{ProductID: a, Kind: "dose", Key: "dose", Label: "Usual dose", New: "new", IsNew: true}}, Asked: []int64{a}}, 0)
	if err != nil || res.Applied != 0 {
		t.Fatalf("\"new\" must never apply: %+v %v", res, err)
	}
}

func TestDoseOnListAndClaims(t *testing.T) {
	p, _, c := pharmEnv(t, "aicatalog_dose_list")
	db := p.store.db
	a, b, w := prod(t, db, "Amoxil", c), prod(t, db, "Omeprazole", c), prod(t, db, "Warfarin", c)
	rows, err := doseParse(context.Background(), p, doseBatch(a, b, w), fmt.Sprintf(`{"items":[
		{"id":%d,"dose":" 1 - 1 - 1 ","new":true},
		{"id":%d,"dose":"1-0-0","new":false,"meaning":"Morning","per_day":1},
		{"id":%d,"dose":"","new":false,"reason":"Depends on INR"}]}`, a, b, w))
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	if r := rows[0]; r.New != "1-1-1" || r.IsNew || r.Needs != "" {
		t.Errorf("the list's own wording: %+v", r)
	}
	if r := rows[1]; r.New != "1-0-0" || !r.IsNew || r.Extra["perDay"] != "1" {
		t.Errorf("claimed on the list but isn't: %+v", r)
	}
	for _, r := range rows {
		if r.Ticked {
			t.Errorf("approved one by one: %+v", r)
		}
	}
}

func TestDoseNeverReplaces(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_dose_keep")
	ctx := context.Background()
	a, b := prod(t, p.store.db, "Has", c), prod(t, p.store.db, "Empty", c)
	f.vals[a] = map[string]string{"dose": "1-0-1"}
	got, total, err := doseSelect(ctx, p, JobRequest{Size: 5})
	if err != nil || total != 1 || got.Products[0].ID != b {
		t.Fatalf("%+v %d %v", got, total, err)
	}
	reply := fmt.Sprintf(`{"items":[{"id":%d,"dose":"1-1-1"}]}`, a)
	if rows, _ := doseParse(ctx, p, doseBatch(a), reply); len(rows) != 0 {
		t.Fatalf("%+v", rows)
	}
	redo := doseBatch(a)
	redo.Redo = true
	rows, err := doseParse(ctx, p, redo, reply)
	if err != nil || len(rows) != 1 || rows[0].Ticked || rows[0].Old != "1-0-1" {
		t.Fatalf("%+v %v", rows, err)
	}
}

func TestDoseNewAddsToListAndUndo(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_dose_add")
	ctx := context.Background()
	a := prod(t, p.store.db, "Omeprazole", c)
	row := JobRow{ProductID: a, Kind: "dose", Key: "dose", Label: "Usual dose", New: "1-0-0", IsNew: true, Extra: map[string]string{"meaning": "Morning", "perDay": "1"}}
	res, err := p.applyJob(ctx, p.job("dose"), JobApply{Rows: []JobRow{row}, Asked: []int64{a}, Suggested: []int64{a}}, 0)
	if err != nil || res.Applied != 1 || f.vals[a]["dose"] != "1-0-0" || !strings.Contains(strings.Join(f.known["dose"], ","), "1-0-0") {
		t.Fatalf("%+v %v %+v", res, err, f)
	}
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if f.vals[a]["dose"] != "" || strings.Contains(strings.Join(f.known["dose"], ","), "1-0-0") {
		t.Fatalf("undo: %+v", f)
	}
}

func TestDosePromptRules(t *testing.T) {
	p, _, _ := pharmEnv(t, "aicatalog_dose_prompt")
	sys, user, err := dosePromptFor(context.Background(), p, doseBatch(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"only as needed is 0", `never the word "new"`, "exact wording", "leave \"dose\" empty"} {
		if !strings.Contains(sys, want) {
			t.Errorf("no %q", want)
		}
	}
	if !strings.Contains(user, "1-0-1") {
		t.Errorf("no dose list: %s", user)
	}
	j := doseJob()
	if !j.OneByOne || j.Warning == "" {
		t.Fatalf("%+v", j.Key)
	}
}

// As Pharmacy does it: controlled implies prescription-only, in what's stored.
func controlledSet(t *testing.T, p *Plugin, f *fakeAttrs) {
	t.Helper()
	prev := p.attrSets
	p.attrSets = func() []plugin.ProductAttributeSet {
		s := prev()[0]
		s.Set = func(_ context.Context, id int64, a, v string) error {
			m := f.vals[id]
			if m == nil {
				m = map[string]string{}
				f.vals[id] = m
			}
			if v == "no" {
				v = ""
			}
			m[a] = v
			if m["controlled"] == "yes" {
				m["rx_only"] = "yes"
			}
			return nil
		}
		return []plugin.ProductAttributeSet{s}
	}
}

func TestUndoControlledClearsRxItBrought(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_marks_undo")
	controlledSet(t, p, f)
	ctx := context.Background()
	a := prod(t, p.store.db, "Diazepam", c)
	res, err := p.applyJob(ctx, p.job("marks"), JobApply{Asked: []int64{a}, Suggested: []int64{a}, Rows: []JobRow{{ProductID: a, Kind: "attr", Key: "controlled", Label: "Controlled", New: "yes"}}}, 0)
	if err != nil || res.Applied != 1 || f.vals[a]["rx_only"] != "yes" {
		t.Fatalf("%+v %v %+v", res, err, f.vals)
	}
	if _, err := p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if f.vals[a]["controlled"] != "" || f.vals[a]["rx_only"] != "" {
		t.Fatalf("back to no marks: %+v", f.vals[a])
	}
}

// Clearing both marks at once: the controlled row must go first, or Rx-only stays.
func TestMarksClearControlledFirst(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_marks_order")
	controlledSet(t, p, f)
	ctx := context.Background()
	a := prod(t, p.store.db, "Was controlled", c)
	f.vals[a] = map[string]string{"controlled": "yes", "rx_only": "yes"}
	rows, err := marksParse(ctx, p, Batch{Products: []JobProduct{{ID: a}}}, fmt.Sprintf(`{"items":[{"id":%d,"rx_only":false,"controlled":false}]}`, a))
	if err != nil || len(rows) != 2 || rows[0].Key != "controlled" {
		t.Fatalf("%+v %v", rows, err)
	}
	if _, err := p.applyJob(ctx, p.job("marks"), JobApply{Rows: rows, Asked: []int64{a}, Suggested: []int64{a}}, 0); err != nil {
		t.Fatal(err)
	}
	if f.vals[a]["controlled"] != "" || f.vals[a]["rx_only"] != "" {
		t.Fatalf("%+v", f.vals[a])
	}
}

// What the chatbot wouldn't answer stays in front of the pharmacist.
func TestMarksListNoAnswer(t *testing.T) {
	db := freshDB(t, "aicatalog_marks_list")
	p, do := testAPI(t, db)
	f := &fakeAttrs{vals: map[int64]map[string]string{}}
	p.attrSets = func() []plugin.ProductAttributeSet { return []plugin.ProductAttributeSet{f.hook("pharmacy")} }
	p.jobs = []Job{marksJob()}
	c := cat(t, db, "Medicine", nil)
	a, b := prod(t, db, "Unsure one", c), prod(t, db, "Answered", c)
	if _, err := p.applyJob(context.Background(), p.job("marks"), JobApply{Asked: []int64{a, b}, Suggested: []int64{b}}, 0); err != nil {
		t.Fatal(err)
	}
	list := decode[[]JobInfo](t, do("GET", "/jobs", ""))
	if len(list) != 1 || len(list[0].NoAnswer) != 1 || list[0].NoAnswer[0] != "Unsure one" {
		t.Fatalf("%+v", list)
	}
}

func TestDoseFailedSetLeavesNoOrphan(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_dose_orphan")
	a := prod(t, p.store.db, "A", c)
	prev := p.attrSets
	p.attrSets = func() []plugin.ProductAttributeSet {
		s := prev()[0]
		s.Set = func(context.Context, int64, string, string) error { return fmt.Errorf("refused") }
		return []plugin.ProductAttributeSet{s}
	}
	row := JobRow{ProductID: a, Kind: "dose", Key: "dose", Label: "Usual dose", New: "9-9-9", IsNew: true, Extra: map[string]string{"perDay": "27"}}
	res, err := p.applyJob(context.Background(), p.job("dose"), JobApply{Rows: []JobRow{row}, Asked: []int64{a}}, 0)
	if err != nil || res.Applied != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	if strings.Contains(strings.Join(f.known["dose"], ","), "9-9-9") {
		t.Fatalf("a dose nobody uses was left on the list: %v", f.known["dose"])
	}
}

// The real reply: sure it isn't controlled, not sure about the prescription. The "not
// sure" stays in front of the pharmacist instead of vanishing.
func TestMarksPartlyUnsure(t *testing.T) {
	p, f, c := pharmEnv(t, "aicatalog_marks_partly")
	ctx := context.Background()
	a := prod(t, p.store.db, "Panadol Extra", c)
	rows, err := marksParse(ctx, p, Batch{Products: []JobProduct{{ID: a}}}, fmt.Sprintf(`{"items":[{"id":%d,"rx_only":null,"controlled":false,"reason":"Not verified."}]}`, a))
	if err != nil || len(rows) != 2 {
		t.Fatalf("%+v %v", rows, err)
	}
	unsure := rowOf2(rows, "rx_only")
	if unsure == nil || unsure.Needs == "" || len(unsure.Choices) != 2 || unsure.New != "" {
		t.Fatalf("%+v", unsure)
	}
	// Left unpicked: the other mark applies, and the product stays on the list.
	res, err := p.applyJob(ctx, p.job("marks"), JobApply{Rows: []JobRow{*rowOf2(rows, "controlled")}, Asked: []int64{a}, Suggested: []int64{a}, Open: []int64{a}}, 0)
	if err != nil || res.Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	var state string
	mustExec2(t, p.store.db, &state, `SELECT state FROM aicatalog_job_items WHERE job = 'marks' AND product_id = $1`, a)
	if state != "no_answer" {
		t.Fatalf("a mark still open: %q", state)
	}
	// Picked by the pharmacist: that's what saves.
	picked := *unsure
	picked.Extra = map[string]string{"rx_only": "yes"}
	picked.Needs = ""
	if res, err := p.applyJob(ctx, p.job("marks"), JobApply{Rows: []JobRow{picked}, Asked: []int64{a}, Suggested: []int64{a}}, 0); err != nil || res.Applied != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if f.vals[a]["rx_only"] != "yes" {
		t.Fatalf("%+v", f.vals[a])
	}
}

func rowOf2(rows []JobRow, key string) *JobRow {
	for i := range rows {
		if rows[i].Key == key {
			return &rows[i]
		}
	}
	return nil
}
