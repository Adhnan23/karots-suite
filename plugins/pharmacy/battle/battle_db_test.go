// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide; the package's other tests
// register their own).
package battle

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/pharmacy"
)

// Pharmacy's battle tests (docs/rules/plugins/pharmacy.md): one throwaway shop on
// the real server with this plugin (and Product Plus, Alternatives) compiled in.

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

var seq atomic.Int64

// medicine makes a stocked medicine (Rs 10 a tablet, 100 in stock); controlled
// when asked (always Rx-only then).
func medicine(t *testing.T, s *ltest.Shop, name string, controlled bool) int64 {
	t.Helper()
	return s.Product(t, fmt.Sprintf("%s %d", name, seq.Add(1)), "10", "100", map[string]any{
		"extra": map[string]any{"pharmacy": map[string]any{"controlled": controlled, "schedule": "S4", "rxOnly": true}}})
}

func patient(t *testing.T, s *ltest.Shop, name string) pharmacy.PatientDTO {
	t.Helper()
	var p pharmacy.PatientDTO
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/pharmacy/patients", map[string]string{"name": name, "phone": fmt.Sprintf("071%07d", seq.Add(1))}, &p))
	return p
}

// script writes a prescription of qty tablets of product, 1-0-1, and returns its id and its line's id.
func script(t *testing.T, s *ltest.Shop, pat, product int64, qty string) (rx, item int64) {
	t.Helper()
	var r struct{ ID int64 }
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/pharmacy/prescriptions", map[string]any{"patientId": pat, "prescriberName": "Dr Perera",
		"items": []map[string]any{{"productId": product, "dosage": "1-0-1", "quantity": qty}}}, &r))
	var scs []pharmacy.ScriptDTO
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/pharmacy/patients/%d/scripts", pat), nil, &scs))
	for _, sc := range scs {
		if sc.ID == r.ID {
			return r.ID, sc.Items[0].ID
		}
	}
	t.Fatalf("script %d not listed", r.ID)
	return 0, 0
}

type sold struct {
	ID    int64
	Lines []struct{ ID int64 }
}

// sell rings items with the patient on the sale (0 = none), as the till does: the
// patient in the sale's own data, a script's line tagged with its line.
func sell(t *testing.T, s *ltest.Shop, pat int64, items ...map[string]any) sold {
	t.Helper()
	req := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "items": items,
		"payments": []map[string]string{{"method": "cash", "amount": "10000"}}}
	if pat != 0 {
		req["plugin"] = map[string]any{"pharmacy": map[string]any{"patientId": pat, "name": "x"}}
	}
	var r sold
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", req, &r))
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", r.ID), nil, &r))
	return r
}

func line(product int64, qty string, rxItem int64) map[string]any {
	l := map[string]any{"productId": product, "quantity": qty}
	if rxItem > 0 {
		l["plugin"] = map[string]any{"pharmacy": map[string]any{"rxItemId": rxItem}}
	}
	return l
}

type given struct {
	Patient int64  `db:"patient_id"`
	Product int64  `db:"product_id"`
	Item    *int64 `db:"prescription_item_id"`
	Qty     string `db:"qty"`
}

func dispensed(t *testing.T, s *ltest.Shop, sale int64) []given {
	t.Helper()
	var out []given
	s.Must(t, s.DB.Select(&out, `SELECT d.patient_id, i.product_id, i.prescription_item_id, i.quantity::int::text AS qty
		FROM pharmacy_dispenses d JOIN pharmacy_dispense_items i ON i.dispense_id = d.id WHERE d.sale_id = $1 ORDER BY i.id`, sale))
	return out
}

func unrecorded(t *testing.T, s *ltest.Shop, sale int64) bool {
	t.Helper()
	var rows []pharmacy.UnrecordedDTO
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/pharmacy/controlled/missing", nil, &rows))
	for _, r := range rows {
		if r.SaleID == sale {
			return true
		}
	}
	return false
}

var tillOnce sync.Once

func openTill(t *testing.T, s *ltest.Shop) { tillOnce.Do(func() { s.OpenTill(t, s.Cashier) }) }

func TestLedgerChecksFire(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	drug := medicine(t, s, "Fire drug", true)
	a, b := patient(t, s, "Fire A"), patient(t, s, "Fire B")
	_, itemB := script(t, s, b.ID, drug, "10")
	sale := sell(t, s, a.ID, line(drug, "4", 0))
	other := sell(t, s, 0, line(drug, "1", 0))
	dup := s.Product(t, "Fire dup", "10", "0", nil)
	for _, x := range []struct{ rule, bad string }{
		{"PHA-12", fmt.Sprintf(`UPDATE pharmacy_dispense_items SET quantity = 0 WHERE dispense_id IN (SELECT id FROM pharmacy_dispenses WHERE sale_id = %d)`, sale.ID)},
		{"PHA-12", fmt.Sprintf(`UPDATE pharmacy_dispense_items SET prescription_item_id = %d WHERE dispense_id IN (SELECT id FROM pharmacy_dispenses WHERE sale_id = %d)`, itemB, sale.ID)},
		{"PHA-12", fmt.Sprintf(`INSERT INTO pharmacy_dispenses (patient_id, sale_id) VALUES (%d, %d)`, b.ID, sale.ID)},
		{"PHA-5", fmt.Sprintf(`UPDATE pharmacy_dispense_items SET product_id = %d WHERE dispense_id IN (SELECT id FROM pharmacy_dispenses WHERE sale_id = %d)`, dup, sale.ID)},
		{"PHA-5", fmt.Sprintf(`UPDATE pharmacy_dispense_items SET quantity = 9 WHERE dispense_id IN (SELECT id FROM pharmacy_dispenses WHERE sale_id = %d)`, sale.ID)},
		{"PHA-5", fmt.Sprintf(`UPDATE pharmacy_dispenses SET sale_id = %d WHERE sale_id = %d`, other.ID, sale.ID)},
		{"PHA-8", fmt.Sprintf(`UPDATE pharmacy_medicines SET rx_only = false WHERE product_id = %d`, drug)},
		{"PHA-10", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d`, dup, drug)},
	} {
		tx := s.DB.MustBegin()
		tx.MustExec(x.bad)
		vs, err := ledgercheck.Run(context.Background(), tx)
		tx.Rollback() //nolint:errcheck
		if err != nil {
			t.Fatal(err)
		}
		fired := false
		for _, v := range vs {
			fired = fired || v.Rule == x.rule
		}
		if !fired {
			t.Errorf("%s did not fire on: %s", x.rule, x.bad)
		}
	}
}

// PHA-4, PHA-5, PHA-7: a patient picked at the till gets the dispense written with
// the sale — the tagged line fills its script line, the controlled register names
// the patient, and the sale isn't on the unrecorded list.
func TestPHA5_DispenseWithTheSale(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	drug, other := medicine(t, s, "Morphine", true), medicine(t, s, "Cough syrup", false)
	plain := s.Product(t, fmt.Sprintf("Soap %d", seq.Add(1)), "50", "10", nil)
	p := patient(t, s, "Nimal Silva")
	_, item := script(t, s, p.ID, drug, "20")
	sale := sell(t, s, p.ID, line(drug, "10", item), line(other, "2", 0), line(plain, "1", 0))
	got := dispensed(t, s, sale.ID)
	if len(got) != 2 || got[0].Product != drug || got[0].Item == nil || *got[0].Item != item || got[0].Qty != "10" ||
		got[1].Product != other || got[1].Item != nil || got[1].Qty != "2" {
		t.Fatalf("PHA-5: dispensed %+v; want 10 of the drug on its script line, 2 of the syrup off-script, no soap", got)
	}
	if unrecorded(t, s, sale.ID) {
		t.Error("PHA-7: a controlled sale recorded with the sale is still on the unrecorded list")
	}
	var reg pharmacy.RegisterDTO
	s.Must(t, s.Admin.Do("GET", fmt.Sprintf("/api/v1/x/pharmacy/controlled/%d", drug), nil, &reg))
	last := reg.Rows[len(reg.Rows)-1]
	if last.Patient != "Nimal Silva" || last.Prescriber != "Dr Perera" || last.Balance != reg.Stock || reg.Stock != "90" {
		t.Errorf("PHA-7: register's last row %+v, stock %s; want Nimal Silva / Dr Perera, balance = stock = 90", last, reg.Stock)
	}
	// Recording the same receipt by hand afterwards is refused, naming the patient.
	err := s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": sale.ID, "patientId": p.ID}, nil)
	if se, ok := err.(*ltest.StatusError); !ok || se.Code != 409 {
		t.Errorf("PHA-5: a second record for the receipt answered %v, want 409", err)
	}
	s.Audit(t)
}

// PHA-5: a sale is never stopped — a patient that doesn't exist, or nothing
// medicinal, writes no dispense and the sale stands.
func TestPHA5_NeverStopsTheSale(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	drug := medicine(t, s, "Ghost drug", true)
	plain := s.Product(t, fmt.Sprintf("Plaster %d", seq.Add(1)), "20", "10", nil)
	p := patient(t, s, "Kamal")
	for _, x := range []struct {
		name string
		pat  int64
		item map[string]any
	}{{"no such patient", 999999, line(drug, "1", 0)}, {"nothing medicinal", p.ID, line(plain, "1", 0)}, {"a script line that isn't one", p.ID, line(drug, "1", 999999)}} {
		sale := sell(t, s, x.pat, x.item)
		if x.name == "a script line that isn't one" {
			if got := dispensed(t, s, sale.ID); len(got) != 1 || got[0].Item != nil {
				t.Errorf("PHA-5 %s: %+v, want the drug off-script", x.name, got)
			}
			continue
		}
		if got := dispensed(t, s, sale.ID); len(got) != 0 {
			t.Errorf("PHA-5 %s: dispensed %+v", x.name, got)
		}
	}
	s.Audit(t)
}

// PHA-5: recording a past receipt by hand twice at once is one record.
func TestPHA5_PastReceiptAtOnce(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	drug := medicine(t, s, "Past drug", true)
	p := patient(t, s, "Sunil")
	sale := sell(t, s, 0, line(drug, "3", 0))
	if !unrecorded(t, s, sale.ID) {
		t.Fatal("PHA-7: a controlled sale with no patient is not on the unrecorded list")
	}
	var wg sync.WaitGroup
	var ok atomic.Int64
	for range 6 {
		wg.Go(func() {
			if s.Cashier.Do("POST", "/api/v1/x/pharmacy/dispenses", map[string]any{"saleId": sale.ID, "patientId": p.ID}, nil) == nil {
				ok.Add(1)
			}
		})
	}
	wg.Wait()
	if n := len(dispensed(t, s, sale.ID)); ok.Load() != 1 || n != 1 {
		t.Errorf("PHA-5: %d records accepted, %d rows; want one", ok.Load(), n)
	}
	if unrecorded(t, s, sale.ID) {
		t.Error("PHA-7: recorded, still on the unrecorded list")
	}
	s.Audit(t)
}

// PHA-6: a return shrinks what the script line was given; the record stays.
func TestPHA6_ReturnShrinksGiven(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	drug := medicine(t, s, "Returned drug", false)
	p := patient(t, s, "Ruwan")
	_, item := script(t, s, p.ID, drug, "20")
	sale := sell(t, s, p.ID, line(drug, "10", item))
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", sale.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": sale.Lines[0].ID, "quantity": "4"}}}, nil))
	var scs []pharmacy.ScriptDTO
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/pharmacy/patients/%d/scripts", p.ID), nil, &scs))
	if it := scs[0].Items[0]; it.Dispensed != "6" || it.Owed != "14" {
		t.Errorf("PHA-6: given %s, owed %s; want 6 and 14", it.Dispensed, it.Owed)
	}
	if got := dispensed(t, s, sale.ID); len(got) != 1 || got[0].Qty != "10" {
		t.Errorf("PHA-6: the record was rewritten: %+v", got)
	}
	s.Audit(t)
}

// PHA-10: a merge moves script lines and the marks to the kept product.
func TestPHA10_ProductMerge(t *testing.T) {
	s := ltest.Shared(t).Shop
	drug := medicine(t, s, "Merged drug", true)
	kept := s.Product(t, fmt.Sprintf("Kept drug %d", seq.Add(1)), "10", "0", nil)
	p := patient(t, s, "Merge")
	_, item := script(t, s, p.ID, drug, "5")
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", drug, kept), map[string]string{"lotPrice": "follow"}, nil))
	var pid int64
	var controlled bool
	s.Must(t, s.DB.Get(&pid, `SELECT product_id FROM pharmacy_prescription_items WHERE id = $1`, item))
	s.Must(t, s.DB.Get(&controlled, `SELECT EXISTS (SELECT 1 FROM pharmacy_controlled_products WHERE product_id = $1)`, kept))
	if pid != kept || !controlled {
		t.Errorf("PHA-10: script line on %d (want %d), kept product controlled %v", pid, kept, controlled)
	}
	s.Audit(t)
}
