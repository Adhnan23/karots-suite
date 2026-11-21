package battle

import (
	"fmt"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/pharmacy"
)

// PHA-1: registering the same phone again is the same customer and patient; the
// profile never changes the customer's name, phone or balance.
func TestPHA1_OnePatientPerCustomer(t *testing.T) {
	s := ltest.Shared(t).Shop
	phone := fmt.Sprintf("077%07d", seq.Add(1))
	var a, b pharmacy.PatientDTO
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": "Saman", "phone": phone}, &a))
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": "Saman K", "phone": phone, "allergies": "Penicillin"}, &b))
	if a.ID != b.ID || a.CustomerID != b.CustomerID || b.Allergies != "Penicillin" {
		t.Errorf("PHA-1: %+v then %+v; want the same patient, allergies kept", a, b)
	}
	var again pharmacy.PatientDTO
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/x/pharmacy/patients/from-customer/%d", a.CustomerID), nil, &again))
	s.Must(t, s.Cashier.Do("PUT", fmt.Sprintf("/api/v1/x/pharmacy/patients/%d", a.ID), map[string]string{"name": "Changed", "phone": "0700000000", "notes": "x"}, nil))
	var name, ph string
	s.Must(t, s.DB.QueryRow(`SELECT name, phone FROM customers WHERE id = $1`, a.CustomerID).Scan(&name, &ph))
	if again.ID != a.ID || name != "Saman" || ph != phone {
		t.Errorf("PHA-1: from-customer gave %d (want %d); customer now %s / %s", again.ID, a.ID, name, ph)
	}
	s.Audit(t)
}

// PHA-2, PHA-11, PLG-5, PLG-3: bad requests are refused with a clear 4xx and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	openTill(t, s)
	drug := medicine(t, s, "Refuse drug", false)
	a, b := patient(t, s, "Refuse A"), patient(t, s, "Refuse B")
	rxB, _ := script(t, s, b.ID, drug, "10")
	sale := sell(t, s, 0, line(drug, "1", 0))
	plainSale := sell(t, s, 0, map[string]any{"productId": s.Product(t, fmt.Sprintf("Refuse soap %d", seq.Add(1)), "5", "5", nil), "quantity": "1"})
	scriptReq := func(pat int64, qty string) map[string]any {
		return map[string]any{"patientId": pat, "items": []map[string]any{{"productId": drug, "dosage": "1-0-1", "quantity": qty}}}
	}
	before := ltest.State(t, s.DB)
	for _, x := range []struct {
		name string
		do   func() error
	}{
		{"patient no name", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": " ", "phone": "0711111111"}, nil)
		}},
		{"patient bad birthday", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": "Bad dob", "phone": "0711111112", "dob": "31/12/1990"}, nil)
		}},
		{"from a customer that isn't", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients/from-customer/999999", nil, nil)
		}},
		{"edit no patient", func() error {
			return s.Cashier.Do("PUT", "/api/v1/x/pharmacy/patients/999999", map[string]string{"notes": "x"}, nil)
		}},
		{"script no patient", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions", scriptReq(999999, "10"), nil)
		}},
		{"script no medicine", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions", map[string]any{"patientId": a.ID, "items": []map[string]any{}}, nil)
		}},
		{"script negative qty", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions", scriptReq(a.ID, "-1"), nil)
		}},
		{"script no such product", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions", map[string]any{"patientId": a.ID, "items": []map[string]any{{"productId": 999999, "quantity": "1"}}}, nil)
		}},
		{"edit no script", func() error {
			return s.Cashier.Do("PUT", "/api/v1/x/pharmacy/prescriptions/999999", scriptReq(a.ID, "1"), nil)
		}},
		{"stop no script", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions/999999/active", map[string]bool{"active": false}, nil)
		}},
		{"dispense no sale", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": 999999, "patientId": a.ID}, nil)
		}},
		{"dispense no patient", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": sale.ID, "patientId": 999999}, nil)
		}},
		{"dispense another's script", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": sale.ID, "patientId": a.ID, "prescriptionId": rxB}, nil)
		}},
		{"dispense nothing medicinal", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": plainSale.ID, "patientId": a.ID}, nil)
		}},
		{"called no line", func() error { return s.Admin.Do("POST", "/api/v1/x/pharmacy/refills/999999/called", nil, nil) }},
		{"dose blank", func() error {
			return s.Admin.Do("POST", "/api/v1/x/pharmacy/doses", map[string]any{"dose": " ", "perDay": "2"}, nil)
		}},
		{"dose negative", func() error {
			return s.Admin.Do("POST", "/api/v1/x/pharmacy/doses", map[string]any{"dose": "9-9-9", "perDay": "-1"}, nil)
		}},
		{"dose twice", func() error {
			return s.Admin.Do("POST", "/api/v1/x/pharmacy/doses", map[string]any{"dose": " 1-0-1 ", "perDay": "2"}, nil)
		}},
		{"edit no dose", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/pharmacy/doses/999999", map[string]any{"dose": "7-7-7", "perDay": "3"}, nil)
		}},
		{"register of none", func() error { return s.Admin.Do("GET", "/api/v1/x/pharmacy/controlled/999999", nil, nil) }},
		{"cashier reads the register", func() error { return s.Cashier.Do("GET", "/api/v1/x/pharmacy/controlled", nil, nil) }},
		{"bad id", func() error { return s.Cashier.Do("GET", "/api/v1/x/pharmacy/patients/abc", nil, nil) }},
	} {
		if err := x.do(); err == nil {
			t.Errorf("PLG-5 %s: accepted", x.name)
		} else if se, ok := err.(*ltest.StatusError); !ok || se.Code >= 500 {
			t.Errorf("PLG-5 %s: %v, want a clear 4xx", x.name, err)
		}
	}
	if ch := ltest.Changed(before, ltest.State(t, s.DB)); len(ch) > 0 {
		t.Errorf("PLG-5: refused requests changed %v", ch)
	}
}

// PLG-4: a failure at any statement leaves nothing behind.
func TestPLG4_Sweep(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t)
	openTill(t, s.Shop)
	var drug, rx, item int64
	var p pharmacy.PatientDTO
	var sale sold
	fresh := func() {
		drug = medicine(t, s.Shop, "Sweep drug", true)
		p = patient(t, s.Shop, "Sweep patient")
		rx, item = script(t, s.Shop, p.ID, drug, "10")
	}
	n := 0
	s.Sweep(t, "register", nil, func() error {
		n++
		return s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": "Sweep new", "phone": fmt.Sprintf("0759%06d", n), "allergies": "Dust"}, nil)
	})
	s.Sweep(t, "script", fresh, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions", map[string]any{"patientId": p.ID, "items": []map[string]any{{"productId": drug, "dosage": "1-1-1", "quantity": "9"}}}, nil)
	})
	s.Sweep(t, "edit script", fresh, func() error {
		return s.Cashier.Do("PUT", fmt.Sprintf("/api/v1/x/pharmacy/prescriptions/%d", rx), map[string]any{"patientId": p.ID,
			"items": []map[string]any{{"id": item, "productId": drug, "dosage": "1-0-0", "quantity": "5"}, {"drugText": "Vitamin C", "quantity": "30"}}}, nil)
	})
	s.Sweep(t, "sale with a patient", fresh, func() error {
		return s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"plugin": map[string]any{"pharmacy": map[string]any{"patientId": p.ID}}, "items": []map[string]any{line(drug, "2", item)},
			"payments": []map[string]string{{"method": "cash", "amount": "20"}}}, nil)
	})
	s.Sweep(t, "past receipt", func() { fresh(); sale = sell(t, s.Shop, 0, line(drug, "2", 0)) }, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": sale.ID, "patientId": p.ID}, nil)
	})
	s.Sweep(t, "dose", nil, func() error {
		n++
		return s.Admin.Do("POST", "/api/v1/x/pharmacy/doses", map[string]any{"dose": fmt.Sprintf("%d-0-%d", n, n), "words": "Sweep", "perDay": "2"}, nil)
	})
}

func TestPLG7_DataGroup(t *testing.T) {
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "pharmacy")
}
