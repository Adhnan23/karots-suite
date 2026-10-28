package gen

import (
	"fmt"
	"net/url"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/features/categories"
)

// Print & Copy: photocopies and printouts on a price grid (photo paper a type of
// its own), lamination by size and Gloss / Matte (each its own pouch), comb
// binding with its covers, CV typing at a typed price — and extras (Urgent, Hole
// punch, a lanyard for an ID card). A job with extras rings as one itemised
// group, as the till does; each sold job also goes into the job ledger.

// docService is a print service as the till's job panel sees it.
type docService struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Category  string `json:"category"`
	Offers    []struct {
		Size     string `json:"size"`
		Material string `json:"material"`
		Color    bool   `json:"color"`
		Double   bool   `json:"double"`
	} `json:"offers"`
	Extras []struct {
		Name      string `json:"name"`
		Price     string `json:"price"`
		PerPiece  bool   `json:"perPiece"`
		ProductID *int64 `json:"productId"`
		Qty       string `json:"qty"`
	} `json:"extras"`
}

// docSupplies are the stock the services use (catalogue lines, in pluginLines)
// and what a week of jobs takes of each, so the back room keeps them in.
var docSupplies = map[string]float64{
	"A4 Paper (sheet)": 450, "Copier Toner Cartridge": 0.2, "A4 Photo Paper (sheet)": 8,
	"Laminating Pouch A4 Gloss": 6, "Laminating Pouch A4 Matte": 2, "Laminating Pouch ID Gloss": 8, "Laminating Pouch ID Matte": 3,
	"Binding Comb 10mm": 2, "Binding Cover A4 Clear": 2, "Binding Cover A4 Card": 2, "ID Card Lanyard": 3, "Staple Pins (box of 1000)": 0.03,
}

type docRow struct {
	Size     string           `json:"size"`
	Material string           `json:"material"`
	Tiers    []map[string]any `json:"tiers"`
	Paper    []map[string]any `json:"paper"`
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
	id := func(name string) int64 { it, _ := g.itemNamed(name); return it.id }
	paper, toner := id("A4 Paper (sheet)"), id("Copier Toner Cartridge")
	if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/packs", paper),
		map[string]string{"name": "Ream (500)", "size": "500", "sellingPrice": "2300"}, nil); err != nil {
		return err
	}
	uses := func(names ...string) []map[string]any {
		out := []map[string]any{}
		for _, n := range names {
			out = append(out, map[string]any{"productId": id(n), "perSheet": "1"})
		}
		return out
	}
	tier := func(min int, cells ...string) map[string]any { return map[string]any{"minQty": min, "cells": cells} }
	row := func(size, material string, paper []map[string]any, tiers ...map[string]any) docRow {
		return docRow{size, material, tiers, paper}
	}
	extra := func(name, price string, perPiece bool, stock string) map[string]any {
		x := map[string]any{"name": name, "price": price, "perPiece": perPiece, "qty": "1"}
		if stock != "" {
			x["productId"] = id(stock)
		}
		return x
	}
	for _, s := range []struct {
		name, kind, category, price string
		rows                        []docRow
		toner                       bool
		extras                      []map[string]any
	}{
		{"Photocopy", "metered", "copy", "", []docRow{row("A4", "", uses("A4 Paper (sheet)"), tier(1, "10", "15", "40", "60"), tier(50, "8", "12", "35", "50"))}, true,
			[]map[string]any{extra("Urgent", "50", false, ""), extra("Stapling", "10", false, ""), extra("Hole punch", "5", true, "")}},
		{"Printout", "metered", "print", "", []docRow{row("A4", "", uses("A4 Paper (sheet)"), tier(1, "15", "25", "50", "80")),
			row("A4", "Photo paper", uses("A4 Photo Paper (sheet)"), tier(1, "", "", "120", ""))}, true,
			[]map[string]any{extra("Urgent", "100", false, "")}},
		{"Laminate", "metered", "laminate", "", []docRow{
			row("A4", "Gloss", uses("Laminating Pouch A4 Gloss"), tier(1, "100", "", "", ""), tier(20, "85", "", "", "")),
			row("A4", "Matte", uses("Laminating Pouch A4 Matte"), tier(1, "130", "", "", "")),
			row("ID card", "Gloss", uses("Laminating Pouch ID Gloss"), tier(1, "40", "", "", "")),
			row("ID card", "Matte", uses("Laminating Pouch ID Matte"), tier(1, "50", "", "", ""))}, false,
			[]map[string]any{extra("Urgent", "100", false, ""), extra("Corner rounding", "10", true, ""), extra("Lanyard", "80", true, "ID Card Lanyard")}},
		{"Binding", "metered", "bind", "", []docRow{row("Comb 10mm", "", uses("Binding Comb 10mm", "Binding Cover A4 Clear", "Binding Cover A4 Card"), tier(1, "200", "", "", ""))}, false,
			[]map[string]any{extra("Urgent", "150", false, "")}},
		{"CV typing", "custom", "other", "750", nil, false, []map[string]any{extra("Urgent", "300", false, "")}},
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
		if s.rows != nil {
			if err := g.admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/grid", out.ID), map[string]any{"rows": s.rows}, nil); err != nil {
				return err
			}
		}
		if s.toner { // a cartridge does about 2,500 pages
			if err := g.admin.Do("POST", fmt.Sprintf("/api/v1/admin/products/%d/recipe", out.ProductID), map[string]any{
				"components": []map[string]any{{"productId": toner, "amount": "2500", "mode": "yield"}},
				"costs":      []map[string]string{{"label": "Copier upkeep", "amount": "1.5"}}}, nil); err != nil {
				return err
			}
		}
		if err := g.admin.Do("PUT", fmt.Sprintf("/api/v1/x/documents/services/%d/extras", out.ID), s.extras, nil); err != nil {
			return err
		}
	}
	return nil
}

// docWeight: how often each kind of job walks in.
var docWeight = map[string]int{"copy": 6, "print": 3, "laminate": 3, "bind": 1, "other": 2}

func documentsDaily(g *gen, day time.Time, until float64) error {
	if len(g.docs) == 0 { // built or restored: the services as the till sees them
		if err := g.admin.Do("GET", "/api/v1/x/documents/till", nil, &g.docs); err != nil {
			return err
		}
	}
	var bag []int
	for i, s := range g.docs {
		for range docWeight[s.Category] {
			bag = append(bag, i)
		}
	}
	if len(bag) == 0 {
		return nil
	}
	for i := 0; i < 2+g.r.IntN(6); i++ {
		h := 8.5 + g.r.Float64()*10
		if h >= until {
			continue
		}
		g.tm.at(at(day, h))
		if err := g.docSale(g.docs[bag[g.r.IntN(len(bag))]]); err != nil {
			return err
		}
	}
	return nil
}

type docUse struct {
	ProductID int64  `json:"productId"`
	Quantity  string `json:"quantity"`
}

// takes reports whether stock covers what a line uses, and takes it if so.
func (g *gen) takes(used []docUse) bool {
	for _, u := range used {
		if g.stock[u.ProductID] < dec(u.Quantity, "0").InexactFloat64()+1 {
			return false // out until the next delivery
		}
	}
	for _, u := range used {
		g.stock[u.ProductID] -= dec(u.Quantity, "0").InexactFloat64()
	}
	return true
}

func (g *gen) docSale(s docService) error {
	plain := s.Category == "laminate" || s.Category == "bind"
	job := map[string]any{"serviceId": s.ID}
	line := map[string]any{"productId": s.ProductID, "discount": "0", "discountType": "fixed"}
	var total decimal.Decimal
	pieces := 1
	if s.Kind == "metered" {
		if len(s.Offers) == 0 {
			return nil
		}
		o := s.Offers[0] // most jobs are the plain one
		if g.r.IntN(5) < 2 {
			o = s.Offers[g.r.IntN(len(s.Offers))]
		}
		if counts, ok := map[string][]int{"laminate": {1, 1, 2, 3, 5, 25}, "bind": {1, 1, 1, 2}}[s.Category]; ok {
			pieces = counts[g.r.IntN(len(counts))]
		} else {
			pieces = []int{1, 2, 5, 10, 20, 60}[g.r.IntN(6)]
		}
		var q struct {
			UnitPrice   string   `json:"unitPrice"`
			Total       string   `json:"total"`
			Description string   `json:"description"`
			Sheets      int      `json:"sheets"`
			Components  []docUse `json:"components"`
		}
		if err := g.admin.Do("GET", fmt.Sprintf("/api/v1/x/documents/quote?service=%d&size=%s&material=%s&color=%d&double=%d&copies=%d",
			s.ID, url.QueryEscape(o.Size), url.QueryEscape(o.Material), b2i(o.Color), b2i(o.Double), pieces), nil, &q); err != nil {
			return err
		}
		if !g.takes(q.Components) {
			return nil
		}
		total = dec(q.Total, "0")
		job["size"], job["material"], job["sheets"], job["description"] = o.Size, o.Material, q.Sheets, q.Description
		if !plain {
			job["color"], job["double"] = o.Color, o.Double
		}
		line["quantity"], line["priceOverride"], line["description"] = fmt.Sprint(pieces), q.UnitPrice, q.Description
		if len(q.Components) > 0 {
			line["components"] = q.Components
		}
	} else {
		total = g.priceOf(s.ProductID)
		job["description"] = s.Name
		line["quantity"], line["priceOverride"], line["description"] = "1", total.String(), s.Name
	}
	line["plugin"] = map[string]any{"documents": job}
	items := []map[string]any{line}
	for _, x := range s.Extras {
		if g.r.IntN(100) >= map[bool]int{true: 8, false: 15}[x.Name == "Urgent"] {
			continue
		}
		n := 1
		if x.PerPiece {
			n = pieces
		}
		ex := map[string]any{"productId": s.ProductID, "quantity": fmt.Sprint(n), "priceOverride": x.Price, "description": x.Name,
			"discount": "0", "discountType": "fixed", "plugin": map[string]any{"documents": map[string]any{"extraOf": s.ID}}}
		if x.ProductID != nil {
			used := []docUse{{*x.ProductID, dec(x.Qty, "1").Mul(decimal.NewFromInt(int64(n))).String()}}
			if !g.takes(used) {
				continue
			}
			ex["components"] = used
		}
		items = append(items, ex)
		total = total.Add(dec(x.Price, "0").Mul(decimal.NewFromInt(int64(n))))
	}
	if len(items) > 1 { // a job and its extras: one block, each line priced (the till's addServiceGroup)
		for _, it := range items {
			it["group"] = map[string]any{"no": 1, "label": s.Name, "sets": "1", "itemised": true}
		}
	}
	saleID, err := g.tillSale(g.cashier(), 0, items, total)
	if err != nil {
		return err
	}
	for _, k := range []string{"size", "color", "double", "sheets"} {
		if _, ok := job[k]; !ok {
			job[k] = nil
		}
	}
	return g.lastTill.Do("POST", "/api/v1/x/documents/jobs", map[string]any{"saleId": saleID, "jobs": []any{job}}, nil)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
