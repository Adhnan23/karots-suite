package battle

import (
	"fmt"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/documents"
)

// DOC-1, DOC-3, PLG-5, PLG-3: bad requests are refused with a clear 4xx and change nothing.
func TestPLG5_Refusals(t *testing.T) { // compares every table: runs alone
	s := ltest.Shared(t).Shop
	c := newCopier(t, s)
	plain := s.Product(t, fmt.Sprintf("Refuse stocked %d", seq.Add(1)), "10", "0", nil)
	svc := s.Service(t, fmt.Sprintf("Refuse service %d", seq.Add(1)), "10")
	grid := func(rows ...map[string]any) map[string]any { return map[string]any{"rows": rows} }
	row := func(tiers []map[string]any, paper []map[string]any) map[string]any {
		return map[string]any{"size": "A4", "material": "", "tiers": tiers, "paper": paper}
	}
	tier := func(min int, cell string) []map[string]any {
		return []map[string]any{{"minQty": min, "cells": []string{cell, "", "", ""}}}
	}
	putGrid := func(g map[string]any) error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/grid", c.id), g, nil)
	}
	putExtras := func(x ...map[string]any) error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/extras", c.id), x, nil)
	}
	newService := func(m map[string]any) error { return s.Admin.Do("POST", "/api/v1/x/documents/services", m, nil) }
	before := ltest.State(t, s.DB)
	for _, x := range []struct {
		name string
		do   func() error
	}{
		{"no name", func() error {
			return newService(map[string]any{"name": " ", "categoryId": s.Cat, "kind": "metered", "category": "copy"})
		}},
		{"bad kind", func() error {
			return newService(map[string]any{"name": "x", "categoryId": s.Cat, "kind": "free", "category": "copy"})
		}},
		{"bad category", func() error {
			return newService(map[string]any{"name": "x", "categoryId": s.Cat, "kind": "metered", "category": "fax"})
		}},
		{"no such category", func() error {
			return newService(map[string]any{"name": "No cat", "categoryId": 999999, "kind": "metered", "category": "copy"})
		}},
		{"link a stocked item", func() error {
			return newService(map[string]any{"productId": plain, "kind": "metered", "category": "copy"})
		}},
		{"link twice", func() error {
			return newService(map[string]any{"productId": c.product, "kind": "metered", "category": "copy"})
		}},
		{"link none", func() error {
			return newService(map[string]any{"productId": 999999, "kind": "metered", "category": "copy"})
		}},
		{"edit none", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/documents/services/999999", map[string]string{"kind": "metered", "category": "copy"}, nil)
		}},
		{"edit to bad kind", func() error {
			return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d", c.id), map[string]string{"kind": "x", "category": "copy"}, nil)
		}},
		{"switch none", func() error {
			return s.Admin.Do("POST", "/api/v1/x/documents/services/999999/active", map[string]bool{"active": false}, nil)
		}},
		{"bad id", func() error { return s.Admin.Do("GET", "/api/v1/x/documents/services/abc/grid", nil, nil) }},
		{"grid of none", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/documents/services/999999/grid", grid(row(tier(1, "5"), nil)), nil)
		}},
		{"negative price", func() error { return putGrid(grid(row(tier(1, "-1"), nil))) }},
		{"not a price", func() error { return putGrid(grid(row(tier(1, "five"), nil))) }},
		{"band from 0", func() error { return putGrid(grid(row(tier(0, "5"), nil))) }},
		{"two bands from 1", func() error {
			return putGrid(grid(row(append(tier(1, "5"), tier(1, "4")...), nil)))
		}},
		{"a size twice", func() error { return putGrid(grid(row(tier(1, "5"), nil), row(tier(1, "6"), nil))) }},
		{"paper a service", func() error {
			return putGrid(grid(row(tier(1, "5"), []map[string]any{{"productId": svc, "perSheet": "1"}})))
		}},
		{"paper none", func() error {
			return putGrid(grid(row(tier(1, "5"), []map[string]any{{"productId": 999999, "perSheet": "1"}})))
		}},
		{"paper 0 a sheet", func() error {
			return putGrid(grid(row(tier(1, "5"), []map[string]any{{"productId": plain, "perSheet": "0"}})))
		}},
		{"extra no name", func() error { return putExtras(map[string]any{"name": " ", "price": "1"}) }},
		{"extra twice", func() error {
			return putExtras(map[string]any{"name": "Urgent", "price": "1"}, map[string]any{"name": "urgent", "price": "2"})
		}},
		{"extra negative", func() error { return putExtras(map[string]any{"name": "Urgent", "price": "-1"}) }},
		{"extra a service", func() error {
			return putExtras(map[string]any{"name": "Tag", "price": "1", "productId": svc, "qty": "1"})
		}},
		{"extra no qty", func() error {
			return putExtras(map[string]any{"name": "Tag", "price": "1", "productId": plain, "qty": "0"})
		}},
		{"extras of none", func() error {
			return s.Admin.Do("PUT", "/api/v1/x/documents/services/999999/extras", []map[string]any{{"name": "x", "price": "1"}}, nil)
		}},
		{"shop use none", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/documents/own-use", map[string]any{"serviceId": 999999, "copies": 1}, nil)
		}},
		{"shop use 0", func() error {
			return s.Cashier.Do("POST", "/api/v1/x/documents/own-use", map[string]any{"serviceId": c.id, "size": "A4", "copies": 0}, nil)
		}},
		{"cashier saves a grid", func() error {
			return s.Cashier.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/grid", c.id), grid(row(tier(1, "5"), nil)), nil)
		}},
		{"bad report date", func() error { return s.Admin.Do("GET", "/api/v1/x/documents/report?from=x&to=y", nil, nil) }},
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
	var c copier
	fresh := func() { c = newCopier(t, s.Shop) }
	n := 0
	s.Sweep(t, "new service", nil, func() error {
		n++
		return s.Admin.Do("POST", "/api/v1/x/documents/services", map[string]any{"name": fmt.Sprintf("Sweep service %d", n), "categoryId": s.Cat,
			"sellingPrice": "5", "kind": "custom", "category": "other"}, nil)
	})
	s.Sweep(t, "edit", fresh, func() error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d", c.id), map[string]string{"kind": "metered", "category": "print"}, nil)
	})
	s.Sweep(t, "grid", fresh, func() error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/grid", c.id), map[string]any{"rows": []map[string]any{{
			"size": "A3", "material": "", "tiers": []map[string]any{{"minQty": 1, "cells": []string{"20", "", "", ""}}},
			"paper": []map[string]any{{"productId": c.paper, "perSheet": "2"}}}}}, nil)
	})
	s.Sweep(t, "extras", fresh, func() error {
		return s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/extras", c.id), []map[string]any{{"name": "Punch", "price": "5"}}, nil)
	})
	s.Sweep(t, "shop use", fresh, func() error {
		return s.Cashier.Do("POST", "/api/v1/x/documents/own-use", map[string]any{"serviceId": c.id, "size": "A4", "copies": 4}, nil)
	})
	var q documents.QuoteDTO
	s.Sweep(t, "job sale", func() { fresh(); q = c.quote(t, s.Shop, 4, false) }, func() error {
		return s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": []map[string]any{c.job(q, 4, false, nil)}, "payments": []map[string]string{{"method": "cash", "amount": "40"}}}, nil)
	})
}

// PLG-6: a Pay sent twice is one sale with one job; a print service saved twice
// at once is one link.
func TestPLG6_SentTwice(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c := newCopier(t, s)
	req := map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "clientRef": fmt.Sprintf("doc-twice-%d", c.id),
		"items": []map[string]any{c.job(c.quote(t, s, 2, false), 2, false, nil)}, "payments": []map[string]string{{"method": "cash", "amount": "20"}}}
	for range 2 {
		s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", req, nil))
	}
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM doc_job WHERE service_id = $1 AND kind = 'sale'`, c.id))
	if n != 1 {
		t.Errorf("PLG-6: %d jobs, want 1", n)
	}
	s.Audit(t)
}

func TestPLG7_DataGroup(t *testing.T) {
	ltest.Shared(t)
	ltest.DataGroupHoldsTables(t, "documents")
}
