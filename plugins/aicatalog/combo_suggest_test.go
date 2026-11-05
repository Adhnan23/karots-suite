package aicatalog

import (
	"context"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

func TestCombosRealReply(t *testing.T) {
	p := &Plugin{}
	prods := []JobProduct{{ID: 1, Name: "Panadol"}, {ID: 2, Name: "Vitamin C"}, {ID: 3, Name: "Baby Wipes"}, {ID: 4, Name: "Baby Powder"}, {ID: 5, Name: "Amoxil"}, {ID: 6, Name: "Sanitizer"}, {ID: 7, Name: "Masks"}}
	reply := `{"combos":[{"name":"Cold & Immunity Care Pack","product_ids":[1,2],"reason":"x"},{"name":"Baby Care","product_ids":[3,4],"reason":"y"},{"name":"Hygiene","product_ids":[6,7],"reason":"z"}]}`
	got, err := p.parseCombos(context.Background(), prods, reply)
	if err != nil || len(got) != 3 || got[0].Names[1] != "Vitamin C" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestCombosDrop(t *testing.T) {
	f := &fakeAttrs{vals: map[int64]map[string]string{5: {"rx_only": "yes"}}}
	p := &Plugin{attrSets: func() []plugin.ProductAttributeSet { return []plugin.ProductAttributeSet{f.hook("pharmacy")} }}
	prods := []JobProduct{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 5, Name: "Amoxil"}}
	got, err := p.parseCombos(context.Background(), prods, `{"combos":[
		{"name":"foreign","product_ids":[1,99]},
		{"name":"alone","product_ids":[1]},
		{"name":"twice the same","product_ids":[1,1]},
		{"name":"prescription","product_ids":[1,5]},
		{"name":"","product_ids":[1,2]},
		{"name":"fine","product_ids":[1,2,99],"price":500}]}`)
	if err != nil || len(got) != 1 || got[0].Name != "fine" || len(got[0].ProductIDs) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestCombosPromptNoPrices(t *testing.T) {
	if !strings.Contains(comboSystem, "Don't suggest prices") || !strings.Contains(comboSystem, "Never combine prescription") {
		t.Fatal(comboSystem)
	}
}
