// Package battle is the plugin's battle tests in a binary of their own: one
// shop per process (plugin hooks are process-wide; the package's other tests
// register their own).
package battle

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"karots-suite/internal/api"
	"karots-suite/internal/ledgercheck"
	"karots-suite/internal/ledgercheck/ltest"
	"karots-suite/plugins/documents"
)

// Print & Copy's battle tests (docs/rules/plugins/documents.md): one throwaway
// shop on the real server with this plugin compiled in (ltest.Main).

func TestMain(m *testing.M) { os.Exit(ltest.Main(m)) }

var seq atomic.Int64

// copier is a metered photocopy service: A4 B&W 1-side Rs 10 (Rs 5 from 100), 2-side
// Rs 15, its own A4 paper (500 sheets) and an Urgent extra (Rs 100) and a Lanyard
// (Rs 80 a piece, takes one lanyard).
type copier struct {
	id, product, paper, lanyard int64
	name                        string
}

func newCopier(t *testing.T, s *ltest.Shop) copier {
	t.Helper()
	n := seq.Add(1)
	c := copier{name: fmt.Sprintf("Copy %d", n)}
	c.paper = s.Product(t, fmt.Sprintf("A4 paper %d", n), "2", "500", nil)
	c.lanyard = s.Product(t, fmt.Sprintf("Lanyard %d", n), "80", "20", nil)
	var r struct{ ID, ProductID int64 }
	s.Must(t, s.Admin.Do("POST", "/api/v1/x/documents/services", map[string]any{"name": c.name, "categoryId": s.Cat,
		"sellingPrice": "10", "kind": "metered", "category": "copy"}, &r))
	c.id, c.product = r.ID, r.ProductID
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/grid", c.id), map[string]any{"rows": []map[string]any{{
		"size": "A4", "material": "",
		"tiers": []map[string]any{{"minQty": 1, "cells": []string{"10", "15", "", ""}}, {"minQty": 100, "cells": []string{"5", "", "", ""}}},
		"paper": []map[string]any{{"productId": c.paper, "perSheet": "1"}},
	}}}, nil))
	s.Must(t, s.Admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/extras", c.id), []map[string]any{
		{"name": "Urgent", "price": "100"}, {"name": "Lanyard", "price": "80", "perPiece": true, "productId": c.lanyard, "qty": "1"}}, nil))
	return c
}

func (c copier) quote(t *testing.T, s *ltest.Shop, copies int, double bool) documents.QuoteDTO {
	t.Helper()
	d := "0"
	if double {
		d = "1"
	}
	var q documents.QuoteDTO
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/documents/quote?service=%d&size=A4&double=%s&copies=%d", c.id, d, copies), nil, &q))
	return q
}

// job is the till's job line for a quote (till.ts meteredLine): price, paper and tag.
func (c copier) job(q documents.QuoteDTO, copies int, double bool, group map[string]any) map[string]any {
	comps := []map[string]any{}
	for _, x := range q.Components {
		comps = append(comps, map[string]any{"productId": x.ProductID, "quantity": x.Quantity})
	}
	l := map[string]any{"productId": c.product, "quantity": fmt.Sprint(copies), "priceOverride": q.UnitPrice, "description": q.Description,
		"components": comps, "plugin": map[string]any{"documents": map[string]any{"serviceId": c.id, "size": "A4", "material": "",
			"color": false, "double": double, "sheets": q.Sheets}}}
	if group != nil {
		l["group"] = group
	}
	return l
}

// extra is an extra's line beside its job (till.ts extraLine).
func (c copier) extra(name, price string, n int, product int64, group map[string]any) map[string]any {
	comps := []map[string]any{}
	if product > 0 {
		comps = append(comps, map[string]any{"productId": product, "quantity": fmt.Sprint(n)})
	}
	return map[string]any{"productId": c.product, "quantity": fmt.Sprint(n), "priceOverride": price, "description": name,
		"components": comps, "noRecipe": true, "group": group, "plugin": map[string]any{"documents": map[string]any{"extraOf": c.id}}}
}

type sold struct {
	ID    int64
	Total string
	Lines []struct{ ID int64 }
}

func sell(t *testing.T, s *ltest.Shop, billOff string, items ...map[string]any) sold {
	t.Helper()
	var r sold
	s.Must(t, s.Cashier.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": billOff, "discountType": "fixed",
		"items": items, "payments": []map[string]string{{"method": "cash", "amount": "10000"}}}, &r))
	s.Must(t, s.Cashier.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", r.ID), nil, &r))
	return r
}

func stock(t *testing.T, s *ltest.Shop, id int64) string {
	t.Helper()
	var q string
	s.Must(t, s.DB.Get(&q, `SELECT COALESCE((SELECT quantity::int::text FROM stock WHERE product_id = $1), '0')`, id))
	return q
}

func report(t *testing.T, s *ltest.Shop) documents.ReportDTO {
	t.Helper()
	var r documents.ReportDTO
	day := time.Now().Format("2006-01-02")
	s.Must(t, s.Admin.Do("GET", "/api/v1/x/documents/report?from="+day+"&to="+day, nil, &r))
	return r
}

func reportRow(t *testing.T, s *ltest.Shop, name string) documents.ReportRowDTO {
	t.Helper()
	for _, r := range report(t, s).Rows {
		if r.Name == name && r.Size != nil {
			return r
		}
	}
	return documents.ReportRowDTO{}
}

var tillOnce atomic.Bool

func openTill(t *testing.T, s *ltest.Shop) {
	if tillOnce.CompareAndSwap(false, true) {
		s.OpenTill(t, s.Cashier)
	}
}

func TestLedgerChecksFire(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c := newCopier(t, s)
	sale := sell(t, s, "0", c.job(c.quote(t, s, 3, false), 3, false, nil))
	other := sell(t, s, "0", map[string]any{"productId": c.paper, "quantity": "1"})
	dup := s.Product(t, "Fire dup", "10", "0", nil)
	for _, x := range []struct{ rule, bad string }{
		{"DOC-1", fmt.Sprintf(`UPDATE products SET is_service = false WHERE id = %d`, c.product)},
		{"DOC-1", fmt.Sprintf(`INSERT INTO doc_service (name, kind, category, product_id) VALUES ('twin', 'metered', 'copy', %d)`, c.product)},
		{"DOC-1", fmt.Sprintf(`UPDATE doc_service SET kind = 'free' WHERE id = %d`, c.id)},
		{"DOC-3", fmt.Sprintf(`UPDATE doc_price SET unit_price = -1 WHERE service_id = %d`, c.id)},
		{"DOC-3", fmt.Sprintf(`UPDATE doc_price SET min_qty = 0 WHERE service_id = %d`, c.id)},
		{"DOC-3", fmt.Sprintf(`INSERT INTO doc_price (service_id, size, material, color, double_side, min_qty, unit_price) VALUES (%d, 'A4', '', false, false, 1, 9)`, c.id)},
		{"DOC-3", fmt.Sprintf(`UPDATE doc_consumable SET qty_per_unit = 0 WHERE service_id = %d`, c.id)},
		{"DOC-3", fmt.Sprintf(`UPDATE doc_consumable SET product_id = %d WHERE service_id = %d`, c.product, c.id)},
		{"DOC-3", fmt.Sprintf(`UPDATE doc_extra SET price = -1 WHERE service_id = %d`, c.id)},
		{"DOC-3", fmt.Sprintf(`UPDATE doc_extra SET product_id = %d WHERE service_id = %d AND product_id IS NOT NULL`, c.product, c.id)},
		{"DOC-6", fmt.Sprintf(`UPDATE doc_job SET qty = qty + 1 WHERE sale_id = %d`, sale.ID)},
		{"DOC-6", fmt.Sprintf(`UPDATE doc_job SET line_total = line_total + 1 WHERE sale_id = %d`, sale.ID)},
		{"DOC-6", fmt.Sprintf(`UPDATE doc_job SET unit_price = unit_price + 1 WHERE sale_id = %d`, sale.ID)},
		{"DOC-6", fmt.Sprintf(`UPDATE doc_job SET sale_id = %d WHERE sale_id = %d`, other.ID, sale.ID)},
		{"DOC-6", fmt.Sprintf(`UPDATE doc_job SET sale_item_id = %d, sale_id = %d WHERE sale_id = %d`, other.Lines[0].ID, other.ID, sale.ID)},
		{"DOC-9", fmt.Sprintf(`UPDATE products SET merged_into_id = %d WHERE id = %d`, dup, c.paper)},
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

// DOC-4: a quote is the band's price × copies, sheets halved for 2-side, paper per sheet.
func TestDOC4_Quote(t *testing.T) {
	s := ltest.Shared(t).Shop
	c := newCopier(t, s)
	for _, x := range []struct {
		copies      int
		double      bool
		unit, total string
		sheets      int
	}{{1, false, "10.00", "10.00", 1}, {99, false, "10.00", "990.00", 99}, {100, false, "5.00", "500.00", 100}, {21, true, "15.00", "315.00", 11}} {
		q := c.quote(t, s, x.copies, x.double)
		if q.UnitPrice != x.unit || q.Total != x.total || q.Sheets != x.sheets || len(q.Components) != 1 || q.Components[0].Quantity != fmt.Sprint(x.sheets) {
			t.Errorf("DOC-4 %d copies (2-side %v): %+v", x.copies, x.double, q)
		}
	}
	for _, bad := range []string{"size=A4&copies=0", "size=A4&copies=-3", "size=A4&copies=5&color=1", "size=A3&copies=5"} {
		if err := s.Cashier.Do("GET", fmt.Sprintf("/api/v1/x/documents/quote?service=%d&%s", c.id, bad), nil, nil); err == nil {
			t.Errorf("DOC-4: quote %s accepted", bad)
		}
	}
}

// DOC-5, DOC-6: a job and its extras are an ordinary sale; the job row is written
// with it from the line itself, the extras get none; paper and the lanyard leave stock.
func TestDOC6_JobWithTheSale(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c := newCopier(t, s)
	q := c.quote(t, s, 5, true)
	g := map[string]any{"no": 1, "label": c.name, "sets": "1", "itemised": true}
	sale := sell(t, s, "0", c.job(q, 5, true, g), c.extra("Urgent", "100", 1, 0, g), c.extra("Lanyard", "80", 5, c.lanyard, g))
	if sale.Total != "575.00" {
		t.Errorf("DOC-5: total %s, want 575.00 (5 × 15 + 100 + 5 × 80)", sale.Total)
	}
	if p, l := stock(t, s, c.paper), stock(t, s, c.lanyard); p != "497" || l != "15" {
		t.Errorf("DOC-5: paper %s (want 497: 3 sheets), lanyards %s (want 15)", p, l)
	}
	var jobs []struct {
		Item   int64  `db:"sale_item_id"`
		Qty    string `db:"qty"`
		Total  string `db:"line_total"`
		Sheets string `db:"sheets"`
	}
	s.Must(t, s.DB.Select(&jobs, `SELECT sale_item_id, qty::int::text AS qty, line_total::text, sheets::int::text AS sheets FROM doc_job WHERE sale_id = $1`, sale.ID))
	if len(jobs) != 1 || jobs[0].Item != sale.Lines[0].ID || jobs[0].Qty != "5" || jobs[0].Total != "75.00" || jobs[0].Sheets != "3" {
		t.Errorf("DOC-6: jobs %+v, want one on the job line: 5 copies, 75.00, 3 sheets", jobs)
	}
	// A tag naming another service's id (or none) is skipped, never refused.
	stray := sell(t, s, "0", map[string]any{"productId": c.product, "quantity": "1", "priceOverride": "10",
		"plugin": map[string]any{"documents": map[string]any{"serviceId": 999999}}})
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM doc_job WHERE sale_id = $1`, stray.ID))
	if n != 0 {
		t.Errorf("DOC-6: %d jobs for a tag naming no service", n)
	}
	s.Audit(t)
}

// DOC-7, DOC-10: a returned job takes its copies and revenue off (sheets stay), and
// the report's revenue is core's for those lines (bill discount shared, RPT-12).
func TestDOC7_ReturnsAndAgreement(t *testing.T) {
	s := ltest.Shared(t).Shop
	openTill(t, s)
	c := newCopier(t, s)
	q := c.quote(t, s, 10, false)
	plain := s.Product(t, fmt.Sprintf("Pen %d", seq.Add(1)), "100", "10", nil)
	sale := sell(t, s, "20", c.job(q, 10, false, nil), map[string]any{"productId": plain, "quantity": "1"})
	s.Must(t, s.Cashier.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", sale.ID), map[string]any{
		"lines": []map[string]any{{"saleItemId": sale.Lines[0].ID, "quantity": "4"}}}, nil))
	r := reportRow(t, s, c.name)
	if r.Copies != "6" || r.Sheets != "10" || r.Revenue != "54.00" {
		t.Errorf("DOC-7: %+v; want 6 copies kept, 10 sheets, 54.00 (60 less its 10 %% share of the bill discount)", r)
	}
	var core api.ServiceProfitReport
	day := time.Now().Format("2006-01-02")
	s.Must(t, s.Admin.Do("GET", "/api/v1/admin/reports/service-profit?"+url.Values{"from": {day}, "to": {day}}.Encode(), nil, &core))
	for _, x := range core.Rows {
		if x.ProductID == c.product && x.Revenue != r.Revenue {
			t.Errorf("DOC-10: core's service report %s, Print & Copy's %s", x.Revenue, r.Revenue)
		}
	}
	s.Audit(t)
}

// DOC-8: shop use takes the quote's paper as own_use with one job; too little
// counted is refused and changes nothing.
func TestDOC8_ShopUse(t *testing.T) {
	s := ltest.Shared(t).Shop
	c := newCopier(t, s)
	s.Must(t, s.Cashier.Do("POST", "/api/v1/x/documents/own-use", map[string]any{"serviceId": c.id, "size": "A4", "double": true, "copies": 9}, nil))
	if p := stock(t, s, c.paper); p != "495" {
		t.Errorf("DOC-8: paper %s, want 495", p)
	}
	var n int
	s.Must(t, s.DB.Get(&n, `SELECT count(*) FROM doc_job WHERE service_id = $1 AND kind = 'own_use' AND sheets = 5`, c.id))
	if n != 1 {
		t.Errorf("DOC-8: %d shop-use jobs, want 1", n)
	}
	err := s.Cashier.Do("POST", "/api/v1/x/documents/own-use", map[string]any{"serviceId": c.id, "size": "A4", "copies": 600}, nil)
	if se, ok := err.(*ltest.StatusError); !ok || se.Code != 409 {
		t.Errorf("DOC-8: 600 sheets from 495 answered %v, want 409", err)
	}
	if p := stock(t, s, c.paper); p != "495" {
		t.Errorf("DOC-8: a refused shop use took paper: %s left", p)
	}
	s.Audit(t)
}

// DOC-2: the till offers exactly the services that are on.
func TestDOC2_TillOffers(t *testing.T) {
	s := ltest.Shared(t).Shop
	on, off := newCopier(t, s), newCopier(t, s)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/x/documents/services/%d/active", off.id), map[string]bool{"active": false}, nil))
	var till []documents.TillService
	s.Must(t, s.Cashier.Do("GET", "/api/v1/x/documents/till", nil, &till))
	seen := map[int64]documents.TillService{}
	for _, x := range till {
		seen[x.ID] = x
	}
	if _, ok := seen[off.id]; ok {
		t.Error("DOC-2: a service turned off is on the till")
	}
	if x, ok := seen[on.id]; !ok || len(x.Offers) != 2 || len(x.Offers[0].Bands) != 2 || len(x.Extras) != 2 {
		t.Errorf("DOC-2: the till's offer %+v", x)
	}
}

// DOC-9: a merge moves paper to the kept product.
func TestDOC9_ProductMerge(t *testing.T) {
	s := ltest.Shared(t).Shop
	c := newCopier(t, s)
	kept := s.Product(t, fmt.Sprintf("Kept paper %d", seq.Add(1)), "2", "0", nil)
	s.Must(t, s.Admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/merge/%d", c.paper, kept), map[string]string{"lotPrice": "follow"}, nil))
	var p int64
	s.Must(t, s.DB.Get(&p, `SELECT product_id FROM doc_consumable WHERE service_id = $1`, c.id))
	if p != kept {
		t.Errorf("DOC-9: paper is %d, want the kept %d", p, kept)
	}
	s.Audit(t)
}
