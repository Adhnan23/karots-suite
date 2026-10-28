package gen

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// Pharmacy: medicines marked on the product form (Rx-only where they should be),
// usual doses, regular patients with prescriptions — chronic ones on repeat —
// and dispensing at the till, each recorded against the patient after the sale.

var (
	rxCategories = map[string]bool{"Antibiotics": true, "Diabetes": true, "Heart & BP": true}
	otcMedicines = map[string]bool{"Pain & Fever": true, "Stomach": true, "Cold & Allergy": true, "Vitamins": true}
	patients     = []struct{ name, phone, nic, dob, sex, allergies string }{
		{"Mr. Sunil Jayawardena", "0771112201", "561234567V", "1956-03-14", "M", ""},
		{"Mrs. Kumari Ratnayake", "0712223302", "625551234V", "1962-07-02", "F", "Penicillin"},
		{"Mr. Mohamed Rifkhan", "0753334403", "198012345678", "1980-11-20", "M", ""},
		{"Ms. Nadeesha Perera", "0764445504", "199245678901", "1992-01-08", "F", ""},
		{"Mr. A. Sivakumar", "0775556605", "701112223V", "1970-05-30", "M", "Sulfa drugs"},
		{"Mrs. Chandrika Silva", "0716667706", "581234321V", "1958-09-17", "F", ""},
		{"Master Kavindu Fernando", "0727778807", "", "2015-04-22", "M", ""},
		{"Mrs. Fathima Nazeer", "0768889908", "198865432109", "1988-12-03", "F", ""},
	}
	// controlledDrugs: on the register, given only against a prescription — by
	// patient index: what the doctor wrote for them.
	controlledDrugs = []struct {
		name, schedule, dosage, qty, perDay string
		patient                             int
	}{
		{"Tramadol 50mg", "Restricted (NMRA)", "1 tablet twice a day for pain", "10", "2", 0},
		{"Diazepam 5mg", "Psychotropic", "1 tablet at night", "10", "1", 5},
		{"Codeine Linctus", "Dangerous Drug (DDO)", "5 ml three times a day", "1", "3", 2},
	}
	chronic = map[string]string{"Metformin": "1 tablet twice a day", "Losartan": "1 tablet in the morning", "Amlodipine": "1 tablet at night", "Atorvastatin": "1 tablet at night"}
)

type rxPatient struct {
	id    int64
	items []rxItem
}

type rxItem struct{ id, productID int64 }

func (g *gen) medicines() (rx, otc []item) {
	cats := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(pharmacyLines), "\n") {
		f := strings.Split(l, ";")
		cats[f[1]] = f[0]
	}
	for _, it := range g.s.items {
		switch c := cats[it.name]; {
		case rxCategories[c]:
			rx = append(rx, it)
		case otcMedicines[c]:
			otc = append(otc, it)
		}
	}
	return rx, otc
}

func pharmacySetup(g *gen) error {
	rx, otc := g.medicines()
	mark := func(it item, rxOnly bool, schedule string) error {
		var p struct {
			Name      string  `db:"name"`
			NameLocal *string `db:"name_local"`
			Barcode   *string `db:"barcode"`
			Cat       int64   `db:"category_id"`
			Unit      int64   `db:"unit_id"`
		}
		if err := g.d.DB.GetContext(g.ctx, &p, `SELECT name, name_local, barcode, category_id, unit_id FROM products WHERE id = $1`, it.id); err != nil {
			return err
		}
		flags, _ := json.Marshal(map[string]any{"medicine": true, "rxOnly": rxOnly, "controlled": schedule != "", "schedule": schedule,
			"dose": "1 tablet three times a day", "note": "After meals"})
		return g.admin.Do("PUT", fmt.Sprintf("/api/v1/admin/products/%d", it.id), map[string]any{
			"name": p.Name, "nameLocal": deref(p.NameLocal), "barcode": deref(p.Barcode), "categoryId": p.Cat, "unitId": p.Unit,
			"costPrice": it.cost.String(), "sellingPrice": it.price.String(), "wholesalePrice": "", "taxRate": "",
			"extra": map[string]json.RawMessage{"pharmacy": flags},
		}, nil)
	}
	for _, cd := range controlledDrugs {
		if it, ok := g.itemNamed(cd.name); ok {
			if err := mark(it, true, cd.schedule); err != nil {
				return fmt.Errorf("mark %s: %w", it.name, err)
			}
		}
	}
	for _, it := range rx {
		if err := mark(it, true, ""); err != nil {
			return fmt.Errorf("mark %s: %w", it.name, err)
		}
	}
	for _, it := range otc {
		if err := mark(it, false, ""); err != nil {
			return fmt.Errorf("mark %s: %w", it.name, err)
		}
	}
	for _, d := range [][3]string{{"1 tablet", "three times a day", "3"}, {"1 tablet", "twice a day", "2"}, {"1 tablet", "at night", "1"}, {"5 ml", "three times a day", "3"}} {
		err := g.admin.Do("POST", "/api/v1/x/pharmacy/doses", map[string]string{"dose": d[0], "words": d[1], "perDay": d[2]}, nil)
		if err != nil && !strings.Contains(err.Error(), "already on the list") { // the plugin ships common ones
			return err
		}
	}
	for i, pt := range patients {
		var out struct {
			ID int64 `json:"id"`
		}
		if err := g.admin.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": pt.name, "phone": pt.phone,
			"allergies": pt.allergies, "nic": pt.nic, "dob": pt.dob, "sex": pt.sex}, &out); err != nil {
			return err
		}
		var items []map[string]any
		for _, name := range slices.Sorted(maps.Keys(chronic)) {
			dosage := chronic[name]
			if (i+len(name))%3 != 0 {
				continue
			}
			if it, ok := g.itemNamed(name); ok {
				items = append(items, map[string]any{"productId": it.id, "drugText": it.name, "dosage": dosage, "note": "", "quantity": "30", "perDay": "1"})
			}
		}
		repeat := len(items) > 0 // chronic medicines come back every month
		for _, cd := range controlledDrugs {
			if it, ok := g.itemNamed(cd.name); ok && cd.patient == i {
				items = append(items, map[string]any{"productId": it.id, "drugText": it.name, "dosage": cd.dosage, "note": "Controlled — record in the register",
					"quantity": cd.qty, "perDay": cd.perDay})
			}
		}
		if len(items) == 0 && len(rx) > 0 {
			it := rx[i%len(rx)]
			items = append(items, map[string]any{"productId": it.id, "drugText": it.name, "dosage": "1 tablet twice a day", "note": "Finish the course", "quantity": "10", "perDay": "2"})
		}
		if err := g.admin.Do("POST", "/api/v1/x/pharmacy/prescriptions", map[string]any{"patientId": out.ID,
			"prescriberName": []string{"Dr. R. Wickramasinghe", "Dr. S. Hameed", "Dr. N. de Silva"}[i%3], "prescriberReg": fmt.Sprintf("SLMC %d", 12000+i*37),
			"prescribedOn": "", "notes": "", "isRepeat": repeat, "repeatIntervalDays": 30, "items": items}, nil); err != nil {
			return err
		}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// loadPatients reads the patients and their prescription items once per run.
func (g *gen) loadPatients() error {
	var ids []int64
	if err := g.d.DB.SelectContext(g.ctx, &ids, `SELECT id FROM pharmacy_patients ORDER BY id`); err != nil {
		return err
	}
	for _, id := range ids {
		var scripts []struct {
			Items []struct {
				ID        int64  `json:"id"`
				ProductID *int64 `json:"productId"`
			} `json:"items"`
		}
		if err := g.admin.Do("GET", fmt.Sprintf("/api/v1/x/pharmacy/patients/%d/scripts", id), nil, &scripts); err != nil {
			return err
		}
		p := rxPatient{id: id}
		for _, s := range scripts {
			for _, it := range s.Items {
				if it.ProductID != nil {
					p.items = append(p.items, rxItem{it.ID, *it.ProductID})
				}
			}
		}
		g.patients = append(g.patients, p)
	}
	return nil
}

func pharmacyDaily(g *gen, day time.Time, until float64) error {
	if g.patients == nil {
		if err := g.loadPatients(); err != nil {
			return err
		}
	}
	_, otc := g.medicines()
	for i := 0; i < 2+g.r.IntN(5); i++ {
		h := 8.5 + g.r.Float64()*11
		if h >= until || len(g.patients) == 0 {
			continue
		}
		p := g.patients[g.r.IntN(len(g.patients))]
		var items []map[string]any
		var tags []map[string]int64
		total := dec("0", "0")
		for _, rx := range p.items {
			if g.stock[rx.productID] < 1 {
				continue
			}
			g.stock[rx.productID]--
			items = append(items, map[string]any{"productId": rx.productID, "quantity": "1", "discount": "0", "discountType": "fixed",
				"plugin": map[string]any{"pharmacy": map[string]int64{"rxItemId": rx.id}}})
			tags = append(tags, map[string]int64{"productId": rx.productID, "rxItemId": rx.id})
			total = total.Add(g.priceOf(rx.productID))
		}
		if len(otc) > 0 && g.r.IntN(2) == 0 {
			if it := otc[g.r.IntN(len(otc))]; g.stock[it.id] >= 1 {
				g.stock[it.id]--
				items = append(items, map[string]any{"productId": it.id, "quantity": "1", "discount": "0", "discountType": "fixed"})
				total = total.Add(g.priceOf(it.id))
			}
		}
		if len(items) == 0 {
			continue
		}
		g.tm.at(at(day, h))
		saleID, err := g.tillSale(g.cashier(), 0, items, total)
		if err != nil {
			return err
		}
		if err := g.lastTill.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": saleID, "patientId": p.id, "lines": tags}, nil); err != nil {
			return fmt.Errorf("dispense: %w", err)
		}
	}
	return nil
}
