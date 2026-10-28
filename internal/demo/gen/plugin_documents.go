package gen

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/categories"
)

// Print & Copy: photocopies and printouts on a price grid, lamination and CV
// typing as custom jobs; each sold line also goes into the job ledger, as the
// till does after a sale.

type docService struct {
	id, productID int64
	metered       bool
	name          string
}

func documentsSetup(g *gen) error {
	cats, err := g.categories()
	if err != nil {
		return err
	}
	catID, ok := cats["Print & Copy"]
	if !ok {
		c, err := categories.NewService(g.d.DB).Create(g.ctx, categories.CreateInput{Name: "Print & Copy"})
		if err != nil {
			return err
		}
		catID = c.ID
	}
	paper, _ := g.itemNamed("A4 Paper (sheet)")
	toner, _ := g.itemNamed("Copier Toner Cartridge")
	if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/packs", paper.id),
		map[string]string{"name": "Ream (500)", "size": "500", "sellingPrice": "2300"}, nil); err != nil {
		return err
	}
	for _, s := range []struct {
		name, kind, category, price string
		grid                        []map[string]any
	}{
		{"Photocopy", "metered", "copy", "", []map[string]any{{"minQty": 1, "cells": []string{"10", "15", "40", "60"}}, {"minQty": 50, "cells": []string{"8", "12", "35", "50"}}}},
		{"Printout", "metered", "print", "", []map[string]any{{"minQty": 1, "cells": []string{"15", "25", "50", "80"}}}},
		{"Lamination", "custom", "laminate", "100", nil},
		{"CV typing", "custom", "other", "750", nil},
	} {
		var out struct {
			ID        int64 `json:"id"`
			ProductID int64 `json:"productId"`
		}
		body := map[string]any{"name": s.name, "categoryId": catID, "kind": s.kind, "category": s.category}
		if s.price != "" {
			body["sellingPrice"] = s.price
		}
		if err := g.admin.Do("POST", "/api/v1/x/documents/services", body, &out); err != nil {
			return err
		}
		if s.grid != nil {
			grid := map[string]any{"rows": []map[string]any{{"size": "A4", "tiers": s.grid,
				"paper": []map[string]any{{"productId": paper.id, "perSheet": "1"}}}}}
			if err := g.admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/grid", out.ID), grid, nil); err != nil {
				return err
			}
			// a cartridge does about 2,500 pages
			if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/recipe", out.ProductID), map[string]any{
				"components": []map[string]any{{"productId": toner.id, "amount": "2500", "mode": "yield"}},
				"costs":      []map[string]string{{"label": "Copier upkeep", "amount": "1.5"}}}, nil); err != nil {
				return err
			}
		}
		g.docs = append(g.docs, docService{out.ID, out.ProductID, s.kind == "metered", s.name})
	}
	return nil
}

func documentsDaily(g *gen, day time.Time, until float64) error {
	if len(g.docs) == 0 { // a restored shop: find its services again
		var rows []struct {
			ID        int64  `db:"id"`
			ProductID int64  `db:"product_id"`
			Kind      string `db:"kind"`
			Name      string `db:"name"`
		}
		if err := g.d.DB.SelectContext(g.ctx, &rows, `SELECT s.id, s.product_id, s.kind, p.name FROM doc_service s JOIN products p ON p.id = s.product_id ORDER BY s.id`); err != nil {
			return err
		}
		for _, r := range rows {
			g.docs = append(g.docs, docService{r.ID, r.ProductID, r.Kind == "metered", r.Name})
		}
	}
	for i := 0; i < 2+g.r.IntN(6); i++ {
		h := 8.5 + g.r.Float64()*10
		if h >= until {
			continue
		}
		g.tm.at(at(day, h))
		if err := g.docSale(g.docs[g.r.IntN(len(g.docs))]); err != nil {
			return err
		}
	}
	return nil
}

func (g *gen) docSale(s docService) error {
	job := map[string]any{"serviceId": s.id}
	line := map[string]any{"productId": s.productID, "discount": "0", "discountType": "fixed"}
	var total decimal.Decimal
	if s.metered {
		copies := []int{1, 2, 5, 10, 20, 60}[g.r.IntN(6)]
		color, double := g.r.IntN(5) == 0, g.r.IntN(4) == 0
		var q struct {
			UnitPrice   string          `json:"unitPrice"`
			Total       string          `json:"total"`
			Description string          `json:"description"`
			Sheets      int             `json:"sheets"`
			Components  json.RawMessage `json:"components"`
		}
		if err := g.admin.Do("GET", fmt.Sprintf("/api/v1/x/documents/quote?service=%d&size=A4&color=%d&double=%d&copies=%d",
			s.id, b2i(color), b2i(double), copies), nil, &q); err != nil {
			return err
		}
		total = dec(q.Total, "0")
		job["size"], job["color"], job["double"], job["sheets"], job["description"] = "A4", color, double, q.Sheets, q.Description
		line["quantity"], line["priceOverride"], line["description"] = fmt.Sprint(copies), q.UnitPrice, q.Description
		if len(q.Components) > 0 && string(q.Components) != "null" {
			var used []struct {
				ProductID int64  `json:"productId"`
				Quantity  string `json:"quantity"`
			}
			_ = json.Unmarshal(q.Components, &used)
			for _, u := range used {
				if g.stock[u.ProductID] < dec(u.Quantity, "0").InexactFloat64()+1 {
					return nil // out of paper until the next delivery
				}
			}
			for _, u := range used {
				g.stock[u.ProductID] -= dec(u.Quantity, "0").InexactFloat64()
			}
			line["components"] = q.Components
		}
	} else {
		total = g.priceOf(s.productID)
		job["description"] = s.name
		line["quantity"], line["priceOverride"], line["description"] = "1", total.String(), s.name
	}
	line["plugin"] = map[string]any{"documents": job}
	saleID, err := g.tillSale(g.cashier(), 0, []map[string]any{line}, total)
	if err != nil {
		return err
	}
	if _, ok := job["size"]; !ok {
		job["size"], job["color"], job["double"], job["sheets"] = nil, nil, nil, nil
	}
	return g.lastTill.Do("POST", "/api/v1/x/documents/jobs", map[string]any{"saleId": saleID, "jobs": []any{job}}, nil)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
