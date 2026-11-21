package battle

import (
	"fmt"
	"testing"
)

// The last refusals the coverage push found unreached (2026-11-21): warranty
// searches, loose versions and folds, conversion edits, cheque fixes, edits of
// things that don't exist, and lots / tills by a wrong id.
func TestRefusalsEdits(t *testing.T) {
	t.Parallel()
	s := newSweepShop(t)
	a := s.admin
	s.open(t, s.till1)
	plain, odd := s.prod["plain"], s.prod["odd"]
	safe := s.locker(t, "Safe", "safe", "1000")
	bank := s.locker(t, "Bank", "bank", "1000")
	sup := s.newSup(t, "Edit Ltd", "0")
	cust := s.newCustomer(t, "0712000911", "100000", "0")

	// a serial phone sold, a plain sale with no warranty, a loss to settle
	s.must(t, a.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": s.prod["serial"], "newQuantity": "5"}, nil))
	unit := s.phoneSold(t, "IMEI-ED1", 0)
	r := s.sell(t, s.till1, "0", []line{{"productId": plain, "quantity": "1"}}, cash("100"))
	loss := s.damage(t, plain, 0, "1")

	// a pending cheque from the customer, a closed till
	s.must(t, a.Do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/payment", cust), map[string]any{"method": "cheque",
		"cheques": []map[string]string{{"bank": "BOC", "number": "77", "amount": "100", "date": "2026-12-01"}}}, nil))
	chq := s.last(t, "cheques")
	s.open(t, s.till2)
	s.close(t, s.till2)
	closed := s.last(t, "cash_register")

	// a conversion plain → odd, and loose versions cut 2.5 and 12 into kilos
	var cv struct{ ID int64 }
	s.must(t, a.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": plain, "toProductId": odd, "ratio": "10"}, &cv))
	looseOf := func(name, ratio string) (prod, conv int64) {
		var p struct{ ProductID, ConversionID int64 }
		s.must(t, a.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/loose", plain), map[string]any{"name": name, "unitId": s.unitKg, "ratio": ratio}, &p))
		return p.ProductID, p.ConversionID
	}
	l1, c1 := looseOf("Plain by 2.5", "2.5")
	l2, c2 := looseOf("Plain by 12", "12")
	var svc struct{ ID int64 }
	s.must(t, a.Do("POST", "/api/v1/admin/services", map[string]any{"name": "Battle service", "categoryId": s.cat, "unitId": s.unitPcs, "sellingPrice": "50"}, &svc))
	var off struct{ ID int64 }
	s.must(t, a.Do("POST", "/api/v1/admin/products", map[string]any{"name": "Battle off", "categoryId": s.cat, "unitId": s.unitPcs,
		"costPrice": "1", "sellingPrice": "5", "wholesalePrice": "5"}, &off))
	s.must(t, a.Do("DELETE", fmt.Sprintf("/api/v1/admin/products/%d", off.ID), nil, nil))
	loose := func(p int64, body map[string]any) bad {
		b := map[string]any{"name": "Loose", "unitId": s.unitKg, "ratio": "2"}
		for k, v := range body {
			b[k] = v
		}
		return bad{fmt.Sprintf("loose of %d %v", p, body), a, "POST", fmt.Sprintf("/api/v1/admin/products/%d/loose", p), b}
	}
	expense := map[string]any{"category": "Tea", "amount": "5", "source": fmt.Sprintf("locker:%d", safe)}

	s.refuseAll(t, []bad{
		// warranty
		{"a serial search of spaces", s.till1, "GET", "/api/v1/till/warranty/serial/%20", nil},
		{"a receipt search of nothing", s.till1, "GET", "/api/v1/till/warranty/receipt?no=", nil},
		{"a receipt that doesn't exist", s.till1, "GET", "/api/v1/till/warranty/receipt?no=NOPE-1", nil},
		{"swap by a receipt that doesn't exist", s.till1, "POST", "/api/v1/till/warranty/receipt-replace", map[string]any{"saleId": 999999, "productId": plain, "reason": "x"}},
		{"swap a line with no warranty", s.till1, "POST", "/api/v1/till/warranty/receipt-replace", map[string]any{"saleId": r.ID, "productId": plain, "reason": "x"}},
		{"swap to no serial", s.till1, "POST", "/api/v1/till/warranty/replace", map[string]any{"unitId": unit, "newSerial": " ", "reason": "dead"}},
		{"print a claim that doesn't exist", s.till1, "POST", "/api/v1/till/warranty-claims/999999/print", nil},
		// conversions
		{"edit a conversion that doesn't exist", a, "PUT", "/api/v1/admin/conversions/999999", map[string]any{"fromProductId": plain, "toProductId": odd, "ratio": "10"}},
		{"a second conversion plain → odd", a, "POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": plain, "toProductId": odd, "ratio": "3"}},
		{"edit a conversion to ratio 0", a, "PUT", fmt.Sprintf("/api/v1/admin/conversions/%d", cv.ID), map[string]any{"fromProductId": plain, "toProductId": odd, "ratio": "0"}},
		{"run a conversion that makes under a thousandth", a, "POST", fmt.Sprintf("/api/v1/admin/conversions/%d/run", cv.ID), map[string]string{"quantity": "0.00001"}},
		loose(999999, nil),
		loose(svc.ID, nil),
		loose(off.ID, nil),
		loose(plain, map[string]any{"convertNow": "-1"}),
		{"fold a pack of 2.5", a, "POST", fmt.Sprintf("/api/v1/admin/products/%d/fold", l1), map[string]any{"conversionId": c1, "name": "Bag"}},
		{"fold into kilos", a, "POST", fmt.Sprintf("/api/v1/admin/products/%d/fold", l2), map[string]any{"conversionId": c2, "name": "Bag"}},
		{"fold someone else's conversion", a, "POST", fmt.Sprintf("/api/v1/admin/products/%d/fold", l2), map[string]any{"conversionId": c1, "name": "Bag"}},
		// cheques
		{"fix a cheque to no number", a, "PUT", fmt.Sprintf("/api/v1/admin/cheques/%d", chq), map[string]any{"bank": "BOC", "number": " ", "date": "2026-12-01"}},
		{"fix a cheque to no date", a, "PUT", fmt.Sprintf("/api/v1/admin/cheques/%d", chq), map[string]any{"bank": "BOC", "number": "77", "date": "01/12/2026"}},
		{"fix a cheque to no bank", a, "PUT", fmt.Sprintf("/api/v1/admin/cheques/%d", chq), map[string]any{"bank": " ", "number": "77", "date": "2026-12-01"}},
		{"fix a cheque that doesn't exist", a, "PUT", "/api/v1/admin/cheques/999999", map[string]any{"bank": "BOC", "number": "77", "date": "2026-12-01"}},
		{"pay a supplier by cheque from the safe", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/payment", sup), map[string]any{"method": "cheque",
			"cheques": []map[string]any{{"lockerId": safe, "number": "9", "amount": "10", "date": "2026-12-01"}}}},
		// money places
		{"edit an expense that doesn't exist", a, "PUT", "/api/v1/admin/expenses/999999", expense},
		{"void info of an expense that doesn't exist", a, "GET", "/api/v1/admin/expenses/999999/void", nil},
		{"pay an expense that doesn't exist", a, "POST", "/api/v1/admin/expenses/999999/pay", map[string]any{"source": fmt.Sprintf("locker:%d", safe)}},
		{"rename a locker to nothing", a, "PUT", fmt.Sprintf("/api/v1/admin/lockers/%d", safe), map[string]any{"name": " ", "kind": "safe"}},
		{"edit a locker that doesn't exist", a, "PUT", "/api/v1/admin/lockers/999999", map[string]any{"name": "X", "kind": "safe"}},
		{"archive a locker that doesn't exist", a, "DELETE", "/api/v1/admin/lockers/999999", nil},
		{"bring back a locker that doesn't exist", a, "POST", "/api/v1/admin/lockers/999999/activate", nil},
		{"a locker that doesn't exist", a, "GET", "/api/v1/admin/lockers/999999", nil},
		{"move more than the safe holds", a, "POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": safe, "toId": bank, "amount": "999999"}},
		{"a till that doesn't exist", a, "GET", "/api/v1/admin/tills/999999", nil},
		{"close a closed till", a, "POST", fmt.Sprintf("/api/v1/admin/tills/%d/close", closed), map[string]any{"breakdown": map[string]int{"100": 1}}},
		// people
		{"rename a supplier to nothing", a, "PUT", fmt.Sprintf("/api/v1/admin/suppliers/%d", sup), map[string]any{"name": " "}},
		{"edit a supplier that doesn't exist", a, "PUT", "/api/v1/admin/suppliers/999999", map[string]any{"name": "X"}},
		{"rename a customer to nothing", a, "PUT", fmt.Sprintf("/api/v1/admin/customers/%d", cust), map[string]any{"name": " ", "phone": "0712000911", "creditLimit": "0"}},
		{"a customer's limit of -5", a, "PATCH", fmt.Sprintf("/api/v1/till/customers/%d/credit-limit", cust), map[string]any{"creditLimit": "-5"}},
		{"till payment from a customer that doesn't exist", s.till1, "POST", "/api/v1/till/customers/999999/payment", map[string]any{"amount": "5", "method": "cash"}},
		// buying, stock, losses
		{"a purchase that doesn't exist", a, "GET", "/api/v1/admin/purchases/999999", nil},
		{"edit a purchase that doesn't exist", a, "PUT", "/api/v1/admin/purchases/999999", map[string]any{"supplierId": sup, "lines": []map[string]any{{"productId": plain, "quantity": "1", "costPrice": "1"}}}},
		{"a send-back that doesn't exist", a, "GET", "/api/v1/admin/purchase-returns/999999", nil},
		{"date a lot that doesn't exist", a, "POST", "/api/v1/admin/stock/batches/999999/expiry", map[string]string{"expiryDate": "2027-01-01"}},
		{"add a lot to a product that doesn't exist", a, "POST", "/api/v1/admin/stock/lots/999999/add", map[string]string{"qty": "1", "cost": "1"}},
		{"unfreeze a product that doesn't exist", a, "POST", "/api/v1/admin/stock/999999/unfreeze", map[string]string{"quantity": "1"}},
		{"give back a lot that doesn't exist", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/999999/give-back", sup), map[string]string{"quantity": "1"}},
		{"tell about a lot that doesn't exist", a, "POST", fmt.Sprintf("/api/v1/admin/suppliers/%d/claims/lots/999999/told", sup), map[string]string{}},
		{"a loss settled by a supplier that doesn't exist", a, "POST", "/api/v1/admin/recovery", map[string]any{"sourceType": "damage", "sourceId": loss, "outcome": "paid", "supplierId": 999999, "amount": "5"}},
		{"a replacement extra of abc", a, "POST", "/api/v1/admin/recovery", map[string]any{"sourceType": "damage", "sourceId": loss, "outcome": "replaced", "supplierId": sup, "amount": "abc"}},
	})
}
