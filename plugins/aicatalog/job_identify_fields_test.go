package aicatalog

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Identify also fills the ticked Product Plus fields, judged by the category it gives.
func identifyFieldEnv(t *testing.T, name string) (fieldEnvT, int64) {
	e := fieldEnv(t, name)
	e.p.jobs = []Job{identifyJob()}
	un := cat(t, e.p.store.db, "Uncategorized", nil)
	return e, prod(t, e.p.store.db, "panadol 500", un)
}

func TestIdentifyPromptAsksTickedFields(t *testing.T) {
	e, a := identifyFieldEnv(t, "aicatalog_identify_fields_prompt")
	b := Batch{Settings: Settings{IdentifyFields: "generic_name,manufacturer"}, Products: []JobProduct{{ID: a, Name: "panadol 500"}}}
	sys, user, err := identifyPrompt(context.Background(), e.p, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"generic_name (Generic name, text), only for categories: Medicine", "manufacturer (Manufacturer, text), existing: GSK"} {
		if !strings.Contains(user, want) {
			t.Errorf("no %q in %s", want, user)
		}
	}
	if strings.Contains(user, "rack") || !strings.Contains(sys, `"values"`) {
		t.Errorf("sys %s\nuser %s", sys, user)
	}
	sys, user, _ = identifyPrompt(context.Background(), e.p, Batch{Products: b.Products})
	if strings.Contains(user, "FIELDS") || strings.Contains(sys, `"values"`) {
		t.Errorf("nothing ticked, nothing asked: %s %s", sys, user)
	}
}

func TestIdentifyFieldRows(t *testing.T) {
	e, a := identifyFieldEnv(t, "aicatalog_identify_fields_rows")
	e.f.vals[e.spark] = map[string]string{"manufacturer": "Bosch"}
	b := Batch{Settings: Settings{IdentifyFields: "generic_name,manufacturer,fits_model"}, Products: []JobProduct{
		{ID: a, Name: "panadol 500", Category: "Uncategorized"},
		{ID: e.spark, Name: "NGK CR7HSA Spark Plug", CategoryID: e.part, Category: "Parts"}}}
	rows, err := identifyParse(context.Background(), e.p, b, fmt.Sprintf(`{"items":[
		{"id":%d,"confident":true,"name":"Panadol 500mg Tablets","category":"Medicine","values":[{"field":"generic_name","value":"Paracetamol"},{"field":"fits_model","value":"x"},{"field":"rack","value":"A1"}]},
		{"id":%d,"confident":true,"name":"NGK CR7HSA Spark Plug","category":"Parts","values":[{"field":"manufacturer","value":"ngk"},{"field":"generic_name","value":"Iridium"}]}]}`, a, e.spark))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		if r.Kind == "attr" {
			got = append(got, fmt.Sprintf("%d %s %q→%q ticked=%v change=%v", r.ProductID, r.Key, r.Old, r.New, r.Ticked, r.Change))
		}
	}
	want := []string{
		fmt.Sprintf(`%d generic_name ""→"Paracetamol" ticked=true change=false`, a),
		fmt.Sprintf(`%d manufacturer "Bosch"→"ngk" ticked=true change=true`, e.spark),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the category it gives decides the fields; a shop's value is replaced, ticked:\n%s", strings.Join(got, "\n"))
	}
	if rows[len(rows)-1].Kind != "attr" || rows[0].Kind == "attr" {
		t.Fatal("fields come after the category, so the category applies first")
	}
}

func TestIdentifyFieldsApplyAndUndo(t *testing.T) {
	e, a := identifyFieldEnv(t, "aicatalog_identify_fields_apply")
	res := applyIdentify(t, e.p,
		JobRow{ProductID: a, Kind: "category", Label: "Category", Old: "Uncategorized", New: "Medicine"},
		JobRow{ProductID: a, Kind: "attr", Key: "generic_name", Label: "Generic name", New: "Paracetamol"})
	if res.Applied != 2 || e.f.vals[a]["generic_name"] != "Paracetamol" {
		t.Fatalf("%+v %+v", res, e.f.vals)
	}
	if _, err := e.p.store.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if e.f.vals[a]["generic_name"] != "" {
		t.Fatalf("undo: %+v", e.f.vals)
	}
	// Category left unticked: a Medicine-only field doesn't fit, it's skipped.
	res = applyIdentify(t, e.p, JobRow{ProductID: a, Kind: "attr", Key: "generic_name", Label: "Generic name", New: "Paracetamol"})
	if res.Applied != 0 || len(res.Skipped) != 1 {
		t.Fatalf("%+v", res)
	}
}
