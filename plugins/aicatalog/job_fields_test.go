package aicatalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

// fakeAttrs behaves like a plugin keeping fields.
type fakeAttrs struct {
	attrs []plugin.ProductAttribute
	vals  map[int64]map[string]string
	known map[string][]string
}

func (f *fakeAttrs) hook(key string) plugin.ProductAttributeSet {
	return plugin.ProductAttributeSet{Key: key,
		Attributes: func(context.Context) ([]plugin.ProductAttribute, error) { return f.attrs, nil },
		Values: func(_ context.Context, ids []int64) (map[int64]map[string]string, error) {
			out := map[int64]map[string]string{}
			for _, id := range ids {
				if f.vals[id] != nil {
					out[id] = f.vals[id]
				}
			}
			return out, nil
		},
		Known: func(_ context.Context, a string, limit int) ([]plugin.KnownValue, error) {
			var out []plugin.KnownValue
			for i, v := range f.known[a] {
				if i < limit {
					out = append(out, plugin.KnownValue{Value: v})
				}
			}
			return out, nil
		},
		Set: func(_ context.Context, id int64, a, v string) error {
			if a == "weight" {
				if _, err := strconv.ParseFloat(v, 64); err != nil {
					return errors.New("Weight: not a number")
				}
			}
			if f.vals[id] == nil {
				f.vals[id] = map[string]string{}
			}
			f.vals[id][a] = v
			return nil
		},
	}
}

type fieldEnvT struct {
	p              *Plugin
	f              *fakeAttrs
	med, part      int64
	panadol, spark int64
}

func fieldEnv(t *testing.T, name string) fieldEnvT {
	t.Helper()
	db := freshDB(t, name)
	p, _ := testAPI(t, db)
	med, part := cat(t, db, "Medicine", nil), cat(t, db, "Parts", nil)
	f := &fakeAttrs{vals: map[int64]map[string]string{}, known: map[string][]string{"manufacturer": {"GSK", "State Pharmaceuticals"}}, attrs: []plugin.ProductAttribute{
		{Key: "generic_name", Label: "Generic name", Type: "text", CategoryIDs: []int64{med}},
		{Key: "manufacturer", Label: "Manufacturer", Type: "text"},
		{Key: "form", Label: "Form", Type: "select", Options: []string{"Tablet", "Capsule"}, CategoryIDs: []int64{med}},
		{Key: "fits_model", Label: "Fits", Type: "text", CategoryIDs: []int64{part}},
		{Key: "rack", Label: "Rack", Type: "text"},
		{Key: "weight", Label: "Weight", Type: "number"},
	}}
	p.jobs, p.attrSets = []Job{fieldsJob()}, func() []plugin.ProductAttributeSet { return []plugin.ProductAttributeSet{f.hook("productplus")} }
	return fieldEnvT{p, f, med, part, prod(t, db, "Panadol 500mg Tablets", med), prod(t, db, "NGK CR7HSA Spark Plug", part)}
}

func opts(fields ...string) json.RawMessage {
	b, _ := json.Marshal(fieldsOptions{Fields: fields})
	return b
}

func (e fieldEnvT) batch(fields ...string) Batch {
	return Batch{Options: opts(fields...), Products: []JobProduct{
		{ID: e.panadol, Name: "Panadol 500mg Tablets", CategoryID: e.med, Category: "Medicine"},
		{ID: e.spark, Name: "NGK CR7HSA Spark Plug", CategoryID: e.part, Category: "Parts"}}}
}

func TestFieldsNeedTicks(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_ticks")
	if _, _, err := fieldsSelect(context.Background(), e.p, JobRequest{}); err == nil {
		t.Fatal("nothing ticked, nothing asked")
	}
}

func TestFieldsPromptPerCategory(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_prompt")
	sys, user, err := fieldsPromptFor(context.Background(), e.p, e.batch("generic_name", "manufacturer", "form", "fits_model"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(user, "rack") || strings.Contains(user, "Rack") {
		t.Errorf("an unticked field was asked for: %s", user)
	}
	for _, want := range []string{"pick one only: Tablet, Capsule", "existing: GSK, State Pharmaceuticals",
		fmt.Sprintf("%d|Panadol 500mg Tablets|Medicine||generic_name, manufacturer, form", e.panadol),
		fmt.Sprintf("%d|NGK CR7HSA Spark Plug|Parts||manufacturer, fits_model", e.spark)} {
		if !strings.Contains(user, want) {
			t.Errorf("no %q in %s", want, user)
		}
	}
	if !strings.Contains(sys, "Reuse an existing value") || !strings.Contains(sys, "never guess") {
		t.Errorf("rules: %s", sys)
	}
}

func TestFieldsKnownValuesCapped(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_cap")
	for i := 0; i < 50; i++ {
		e.f.known["manufacturer"] = append(e.f.known["manufacturer"], fmt.Sprintf("Maker%02d", i))
	}
	_, user, err := fieldsPromptFor(context.Background(), e.p, e.batch("manufacturer"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(user, "GSK") || strings.Contains(user, "Maker40") {
		t.Fatalf("cap at %d, most used first: %s", knownValuesMax, user)
	}
}

// ChatGPT's real answer to the prompt test: fits_model empty is no suggestion.
func TestFieldsRealReply(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_real")
	reply := fmt.Sprintf(`{"items":[{"id":%d,"values":[{"field":"generic_name","value":"Paracetamol","new":false},{"field":"manufacturer","value":"gsk","new":false},{"field":"form","value":"Tablet","new":false}]},
		{"id":%d,"values":[{"field":"fits_model","value":"","new":false}]}]}`, e.panadol, e.spark)
	rows, err := fieldsParse(context.Background(), e.p, e.batch("generic_name", "manufacturer", "form", "fits_model"), reply)
	if err != nil || len(rows) != 3 {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, r := range rows {
		switch r.Key {
		case "manufacturer":
			if r.New != "GSK" || r.IsNew || !r.Ticked {
				t.Errorf("known values keep the shop's spelling: %+v", r)
			}
		case "generic_name":
			if !r.IsNew {
				t.Errorf("a value not on the list is marked new: %+v", r)
			}
		}
		if r.ProductID != e.panadol {
			t.Errorf("%+v", r)
		}
	}
}

func TestFieldsDropsWhatWasntAsked(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_drop")
	reply := fmt.Sprintf(`{"items":[
		{"id":%d,"values":[{"field":"rack","value":"A1"},{"field":"form","value":"Liquid"},{"field":"fits_model","value":"CT100"},{"field":"nope","value":"x"},{"field":"form","value":"Tablet"},{"field":"form","value":"Capsule"}]},
		{"id":%d,"values":[{"field":"generic_name","value":"Iridium"}]},
		{"id":999,"values":[{"field":"manufacturer","value":"GSK"}]}]}`, e.panadol, e.spark)
	rows, err := fieldsParse(context.Background(), e.p, e.batch("generic_name", "form", "fits_model"), reply)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Key != "form" || rows[0].New != "Tablet" {
		t.Fatalf("unticked, out of scope, off the options or unknown product: %+v", rows)
	}
}

func TestFieldsNeverOverwrite(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_keep")
	e.f.vals[e.panadol] = map[string]string{"manufacturer": "Haleon"}
	reply := fmt.Sprintf(`{"items":[{"id":%d,"values":[{"field":"manufacturer","value":"GSK"}]}]}`, e.panadol)
	b := e.batch("manufacturer")
	rows, err := fieldsParse(context.Background(), e.p, b, reply)
	if err != nil || len(rows) != 0 {
		t.Fatalf("%+v %v", rows, err)
	}
	b.Redo = true
	rows, err = fieldsParse(context.Background(), e.p, b, reply)
	if err != nil || len(rows) != 1 || rows[0].Ticked || rows[0].Old != "Haleon" || !rows[0].Change {
		t.Fatalf("a redo shows both, unticked: %+v %v", rows, err)
	}
}

func TestFieldsApplyBadTypeAndUndo(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_apply")
	ctx := context.Background()
	j := e.p.job("fields")
	res, err := e.p.applyJob(ctx, j, JobApply{State: "fields:manufacturer,weight", Asked: []int64{e.panadol}, Suggested: []int64{e.panadol}, Rows: []JobRow{
		{ProductID: e.panadol, Kind: "attr", Key: "weight", Label: "Weight", New: "abc"},
		{ProductID: e.panadol, Kind: "attr", Key: "manufacturer", Label: "Manufacturer", New: "GSK"},
	}}, 0)
	if err != nil || res.Applied != 1 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "not a number") {
		t.Fatalf("%+v %v", res, err)
	}
	if e.f.vals[e.panadol]["manufacturer"] != "GSK" {
		t.Fatalf("%+v", e.f.vals)
	}
	var job string
	mustExec2(t, e.p.store.db, &job, `SELECT job FROM aicatalog_job_items WHERE product_id = $1`, e.panadol)
	if job != "fields:manufacturer,weight" {
		t.Fatalf("recorded under %q", job)
	}
	if _, err := e.p.store.Revert(ctx); err != nil {
		t.Fatal(err)
	}
	if e.f.vals[e.panadol]["manufacturer"] != "" {
		t.Fatalf("undo: %+v", e.f.vals)
	}
	if n := count(t, e.p.store.db, `SELECT count(*) FROM aicatalog_job_items`); n != 0 {
		t.Fatalf("undo keeps the record: %d", n)
	}
}

func TestFieldsSelectBringsBackForAnotherField(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_select")
	ctx := context.Background()
	got, total, err := fieldsSelect(ctx, e.p, JobRequest{Options: opts("generic_name"), Size: 5})
	if err != nil || total != 1 || got.Products[0].ID != e.panadol || got.State != "fields:generic_name" {
		t.Fatalf("a medicine field never asks about a part: %+v %d %v", got, total, err)
	}
	e.f.vals[e.panadol] = map[string]string{"generic_name": "Paracetamol"}
	if _, total, _ := fieldsSelect(ctx, e.p, JobRequest{Options: opts("generic_name"), Size: 5}); total != 0 {
		t.Fatalf("filled: %d", total)
	}
	got, total, err = fieldsSelect(ctx, e.p, JobRequest{Options: opts("manufacturer"), Size: 1})
	if err != nil || total != 2 || len(got.Products) != 1 {
		t.Fatalf("batch of one, two to go: %+v %d %v", got, total, err)
	}
}

// The product moved to another category after the preview: the field no longer applies.
func TestFieldsScopeRecheckedAtApply(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_scope")
	mustExec(t, e.p.store.db, nil, `UPDATE products SET category_id = $2 WHERE id = $1`, e.panadol, e.part)
	res, err := e.p.applyJob(context.Background(), e.p.job("fields"), JobApply{Asked: []int64{e.panadol}, Rows: []JobRow{{ProductID: e.panadol, Kind: "attr", Key: "generic_name", Label: "Generic name", New: "Paracetamol"}}}, 0)
	if err != nil || res.Applied != 0 || len(res.Skipped) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if e.f.vals[e.panadol]["generic_name"] != "" {
		t.Fatal("saved onto a product it doesn't apply to")
	}
}

// Filled as a medicine, then moved to Parts: the new category's field is empty, so
// the same job offers it again. A product that didn't move stays answered.
func TestFieldsMovedProductComesBack(t *testing.T) {
	e := fieldEnv(t, "aicatalog_fields_moved")
	ctx := context.Background()
	ticks := opts("generic_name", "fits_model")
	got, _, err := fieldsSelect(ctx, e.p, JobRequest{Options: ticks, Size: 5})
	if err != nil || len(got.Products) != 2 {
		t.Fatalf("%+v %v", got.Products, err)
	}
	if _, err := e.p.applyJob(ctx, e.p.job("fields"), JobApply{State: got.State, Asked: []int64{e.panadol, e.spark}, Suggested: []int64{e.panadol, e.spark}}, 0); err != nil {
		t.Fatal(err)
	}
	if got, total, _ := fieldsSelect(ctx, e.p, JobRequest{Options: ticks, Size: 5}); total != 0 {
		t.Fatalf("both answered: %+v", got.Products)
	}
	mustExec(t, e.p.store.db, nil, `UPDATE products SET category_id = $2 WHERE id = $1`, e.panadol, e.part)
	got, total, err := fieldsSelect(ctx, e.p, JobRequest{Options: ticks, Size: 5})
	if err != nil || total != 1 || got.Products[0].ID != e.panadol {
		t.Fatalf("the moved product comes back: %+v %d %v", got.Products, total, err)
	}
	got.Options = ticks // as the page sends them
	_, user, err := fieldsPromptFor(ctx, e.p, got)
	if err != nil || !strings.Contains(user, "|fits_model") || strings.Contains(user, "|generic_name") {
		t.Fatalf("asks for the new category's field only: %q %v", user, err)
	}
}
