package gen

import (
	"fmt"
	"slices"
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
	names := []string{"Nimal", "Shirani", "Faiz", "Kamal", "Dilrukshi", "Ravi", "Anoma", "Tharindu", "Mrs. Fernando", "Uncle Siva"}
	note := notes[g.r.IntN(len(notes))]
	who := map[string]string{"name": names[g.r.IntN(len(names))], "phone": fmt.Sprintf("07%d%07d", 1+g.r.IntN(8), g.r.IntN(10000000))}
	var open int64 // someone already asked: one more name on the same request
	if err := g.d.DB.GetContext(g.ctx, &open, `SELECT id FROM requests WHERE note = $1 AND status = 'open' ORDER BY id LIMIT 1`, note); err == nil {
		return g.admin.Do("POST", fmt.Sprintf("/api/v1/x/requests/%d/asks", open), who, nil)
	}
	who["note"] = note
	return g.admin.Do("POST", "/api/v1/x/requests", who, nil)
}

// clearanceDaily: Monday mornings the owner looks down the clearance list and
// marks down the oldest few at the safe suggested percent.
func clearanceDaily(g *gen, day time.Time, until float64) error {
	if day.Weekday() != time.Monday || until < 9.5 {
		return nil
	}
	var page struct {
		Rows []struct {
			ProductID int64  `json:"productId"`
			Status    string `json:"status"`
			Percent   string `json:"percent"`
		} `json:"rows"`
	}
	if err := g.admin.Do("GET", "/api/v1/x/clearance", nil, &page); err != nil {
		return err
	}
	n := 1 + g.r.IntN(3)
	for _, r := range page.Rows {
		if n == 0 || r.Status != "" || r.Percent == "" || r.Percent == "0" {
			continue
		}
		g.tm.at(at(day, 9+g.r.Float64()*0.5))
		if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/x/clearance/%d/approve", r.ProductID), map[string]string{"percent": r.Percent}, nil); err != nil {
			return fmt.Errorf("markdown: %w", err)
		}
		n--
	}
	return nil
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
// ppField is an extra product field the owner added, shown only on the
// categories it belongs to, with each product's real value.
type ppField struct {
	label, typ string
	options    []string
	cats       []string
	value      func(it item) string
}

// brandOf reads the brand off the product's own name, else the usual one for its kind.
func brandOf(it item, known []string, byCat map[string]string) string {
	for _, b := range known {
		if strings.Contains(it.name, b) {
			return b
		}
	}
	return byCat[it.cat]
}

var ppFields = map[string][]ppField{
	"pharmacy": {
		{"Generic name", "text", nil, []string{"Pain & Fever", "Antibiotics", "Diabetes", "Heart & BP", "Stomach", "Cold & Allergy", "Controlled"},
			func(it item) string { return strings.Fields(it.name)[0] }},
		{"Manufacturer", "select", []string{"SPC", "Hemas", "Cipla", "Sun Pharma", "GSK"}, []string{"Pain & Fever", "Antibiotics", "Diabetes", "Heart & BP", "Stomach", "Cold & Allergy", "Vitamins", "Controlled"},
			func(it item) string { return []string{"SPC", "Hemas", "Cipla", "Sun Pharma", "GSK"}[len(it.name)%5] }},
	},
	"hardware": {
		{"Brand", "select", []string{"Tokyo", "INSEE", "S-Lon", "Anton", "Orange", "Kelani", "Dulux", "Multilac", "Lanwa"}, []string{"Cement & Sand", "Plumbing", "Electrical", "Paint", "Steel"},
			func(it item) string {
				return brandOf(it, []string{"Tokyo", "INSEE"}, map[string]string{"Cement & Sand": "Tokyo", "Plumbing": "S-Lon", "Electrical": "Orange", "Paint": "Dulux", "Steel": "Lanwa"})
			}},
		{"Guarantee", "text", nil, []string{"Tools", "Plumbing"},
			func(it item) string {
				if it.warranty > 0 {
					return fmt.Sprintf("%d months (agent)", it.warranty)
				}
				return ""
			}},
	},
	"phone": {
		{"Brand", "select", []string{"Samsung", "Redmi", "Infinix", "Nokia", "Anker", "Baseus", "Remax", "Joyroom"}, []string{"Phones", "Chargers", "Cables", "Audio", "Power"},
			func(it item) string {
				return brandOf(it, []string{"Samsung", "Redmi", "Infinix", "Nokia"}, map[string]string{"Chargers": "Baseus", "Cables": "Remax", "Audio": "Joyroom", "Power": "Anker"})
			}},
		{"Fits", "text", nil, []string{"Cases & Glass"}, func(it item) string { return "Most 6.5–6.7 inch phones" }},
	},
	"grocery": {
		{"Brand", "select", []string{"Anchor", "Milo", "Nescafe", "Elephant House", "Coca-Cola", "Sunlight", "Lux", "Maliban", "Munchee", "Prima", "Keells"},
			[]string{"Dairy", "Tea & Coffee", "Beverages", "Bakery", "Household", "Personal Care", "Snacks"},
			func(it item) string {
				return brandOf(it, []string{"Anchor", "Milo", "Nescafe", "Elephant House", "Coca-Cola", "Sunlight", "Lux", "Marie"},
					map[string]string{"Dairy": "Keells", "Tea & Coffee": "Keells", "Bakery": "Munchee", "Beverages": "Elephant House", "Household": "Keells", "Personal Care": "Keells", "Snacks": "Maliban"})
			}},
	},
}

func productplusSetup(g *gen) error {
	fields := ppFields[g.s.p.Catalogue]
	if g.s.p.Catalogue == "mixed" {
		fields = slices.Concat(ppFields["grocery"], ppFields["pharmacy"], ppFields["phone"], ppFields["hardware"])
	}
	cats, err := g.categories()
	if err != nil {
		return err
	}
	var cells []map[string]any
	made := map[string]int64{} // the mixed store shares one Brand field
	for _, f := range fields {
		scope := []int64{}
		for _, c := range f.cats {
			if id, ok := cats[c]; ok {
				scope = append(scope, id)
			}
		}
		id, ok := made[f.label]
		if !ok {
			var out struct {
				ID int64 `json:"id"`
			}
			opts := append([]string{}, f.options...)
			if f.label == "Brand" && g.s.p.Catalogue == "mixed" {
				opts, scope = nil, []int64{} // one free-text brand across the whole store
			}
			typ := f.typ
			if opts == nil {
				typ = "text"
			}
			if err := g.admin.Do("POST", "/api/v1/x/productplus/fields", map[string]any{"label": f.label, "type": typ, "defaultValue": "", "hint": "",
				"required": false, "searchable": true, "showAtTill": true, "printOnLabel": false, "showInIntake": true,
				"options": opts, "scope": scope}, &out); err != nil {
				return err
			}
			id, made[f.label] = out.ID, out.ID
		}
		for _, it := range g.s.items {
			if !slices.Contains(f.cats, it.cat) {
				continue
			}
			if v := f.value(it); v != "" {
				cells = append(cells, map[string]any{"productId": it.id, "fieldId": id, "value": v})
			}
		}
	}
	return g.admin.Do("PUT", "/api/v1/x/productplus/values", cells, nil)
}

func clearanceSetup(g *gen) error {
	return g.admin.Do("PUT", "/api/v1/x/clearance/settings", map[string]any{"staleDays": 45, "defaultPercent": "15",
		"minMarginPercent": "5", "expiryDays": 60, "deadMonths": 4}, nil)
}
