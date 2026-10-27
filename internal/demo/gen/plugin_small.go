package gen

import (
	"fmt"
	"strings"
	"time"
)

// The quieter plugins: customer requests at the counter, alternative medicines
// grouped by tier, extra product fields filled in, clearance rules. The AI
// assistant needs a key, so it sits idle in the demo.

var requestNotes = map[string][]string{
	"pharmacy": {"Sugar-free cough syrup", "Vitamin D3 1000IU", "Baby nasal drops", "Zinc tablets", "Insulin pen needles"},
	"hardware": {"Wall paint, light green 4L", "Copper pipe 15mm", "Door hinge 4in brass", "Solar garden lights", "PVC pipe 2in"},
	"phone":    {"iPhone 14 back cover", "Samsung original charger", "OTG adapter", "Ring light", "Type-C to HDMI"},
	"grocery":  {"Oats 500g", "Brown bread", "Soy milk", "Keells sausages", "Kithul treacle"},
}

func requestsDaily(g *gen, day time.Time, until float64) error {
	if g.r.IntN(3) != 0 {
		return nil
	}
	notes := requestNotes[g.s.p.Catalogue]
	if notes == nil {
		notes = append(requestNotes["grocery"], requestNotes["pharmacy"]...)
	}
	h := 10 + g.r.Float64()*8
	if h >= until {
		return nil
	}
	g.tm.at(at(day, h))
	names := []string{"Nimal", "Shirani", "Faiz", "Kamal", "Dilrukshi", "Ravi", "Anoma"}
	return g.admin.Do("POST", "/api/v1/x/requests", map[string]string{"note": notes[g.r.IntN(len(notes))],
		"name": names[g.r.IntN(len(names))], "phone": fmt.Sprintf("07%d%07d", 1+g.r.IntN(8), g.r.IntN(10000000))}, nil)
}

// alternativesSetup groups medicines that can stand in for each other.
func alternativesSetup(g *gen) error {
	groups := []struct {
		name  string
		tiers [][2]string // tier name, product name prefix
	}{
		{"Pain relief", [][2]string{{"Paracetamol", "Paracetamol"}, {"Ibuprofen", "Ibuprofen"}}},
		{"Allergy", [][2]string{{"Cetirizine", "Cetirizine"}, {"Chlorpheniramine", "Chlorpheniramine"}}},
		{"Acidity", [][2]string{{"Omeprazole", "Omeprazole"}, {"Antacid", "Antacid"}}},
	}
	type group struct {
		Tiers []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"tiers"`
	}
	for _, gr := range groups {
		var out struct {
			ID int64 `json:"id"`
			group
		}
		if err := g.admin.Do("POST", "/api/v1/x/alternatives", map[string]string{"name": gr.name}, &out); err != nil {
			return err
		}
		grp := out.group
		for i, t := range gr.tiers {
			it, ok := g.itemNamed(t[1])
			if !ok {
				continue
			}
			// A new group comes with its first tier unnamed: name it, add the rest.
			if i == 0 && len(grp.Tiers) > 0 {
				if err := g.admin.Do("PUT", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d", grp.Tiers[0].ID), map[string]any{"name": t[0], "reorderLevel": 0}, &grp); err != nil {
					return err
				}
			} else if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/x/alternatives/%d/tiers", out.ID), map[string]any{"name": t[0], "reorderLevel": 0}, &grp); err != nil {
				return err
			}
			for _, tier := range grp.Tiers {
				if tier.Name == t[0] {
					if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/x/alternatives/tiers/%d/members", tier.ID), map[string]any{"productId": it.id, "move": false}, nil); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// productplusSetup adds the fields this kind of shop keeps and fills them in.
func productplusSetup(g *gen) error {
	type field struct {
		label, typ string
		options    []string
		value      func(it item) string
	}
	var fields []field
	switch {
	case g.on("pharmacy"):
		fields = []field{
			{"Generic name", "text", nil, func(it item) string { return strings.Fields(it.name)[0] }},
			{"Manufacturer", "select", []string{"SPC", "Hemas", "Cipla", "GSK"}, func(it item) string { return []string{"SPC", "Hemas", "Cipla", "GSK"}[len(it.name)%4] }},
		}
	default:
		fields = []field{
			{"Brand", "select", []string{"Tokyo", "S-Lon", "Orange", "Dulux", "Lanka Wall"}, func(it item) string {
				return []string{"Tokyo", "S-Lon", "Orange", "Dulux", "Lanka Wall"}[len(it.name)%5]
			}},
			{"Guarantee", "text", nil, func(it item) string { return []string{"", "6 months", "1 year"}[len(it.name)%3] }},
		}
	}
	var cells []map[string]any
	for _, f := range fields {
		var out struct {
			ID int64 `json:"id"`
		}
		if err := g.admin.Do("POST", "/api/v1/x/productplus/fields", map[string]any{"label": f.label, "type": f.typ, "defaultValue": "", "hint": "",
			"required": false, "searchable": true, "showAtTill": true, "printOnLabel": false, "showInIntake": true,
			"options": append([]string{}, f.options...), "scope": []int64{}}, &out); err != nil {
			return err
		}
		for _, it := range g.s.items {
			if v := f.value(it); v != "" {
				cells = append(cells, map[string]any{"productId": it.id, "fieldId": out.ID, "value": v})
			}
		}
	}
	return g.admin.Do("PUT", "/api/v1/x/productplus/values", cells, nil)
}

func clearanceSetup(g *gen) error {
	return g.admin.Do("PUT", "/api/v1/x/clearance/settings", map[string]any{"staleDays": 45, "defaultPercent": "15",
		"minMarginPercent": "5", "expiryDays": 60, "deadMonths": 4}, nil)
}
