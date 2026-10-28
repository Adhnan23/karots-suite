package documents

import (
	"sort"
	"strings"

	"karots-suite/internal/apperr"

	"github.com/shopspring/decimal"
)

// The four cells of a tier, in order: B&W 1-side, B&W 2-side, Colour 1-side,
// Colour 2-side. A blank cell is not offered.
var cells = [4]struct{ color, double bool }{{false, false}, {false, true}, {true, false}, {true, true}}

// GridDTO is a service's price grid and paper, one row per size (empty = any
// size) and type (empty = none).
type GridDTO struct {
	Rows []GridRow `json:"rows"`
}

type GridRow struct {
	Size     string      `json:"size"`
	Material string      `json:"material"`
	Tiers    []GridTier  `json:"tiers"` // the first is the base price (from 1 copy)
	Paper    []PaperLine `json:"paper"`
}

type GridTier struct {
	MinQty int       `json:"minQty"`
	Cells  [4]string `json:"cells"` // money or ''
}

type PaperLine struct {
	ProductID int64  `json:"productId"`
	Name      string `json:"name"`
	PerSheet  string `json:"perSheet"`
}

func sizeKey(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func toGrid(prices []Price, paper []Paper) GridDTO {
	type key struct{ size, material string }
	rows := map[key]*GridRow{}
	var order []key
	row := func(size, material string) *GridRow {
		k := key{size, material}
		if r, ok := rows[k]; ok {
			return r
		}
		r := &GridRow{Size: size, Material: material, Tiers: []GridTier{}, Paper: []PaperLine{}}
		rows[k] = r
		order = append(order, k)
		return r
	}
	for _, p := range prices {
		r := row(sizeKey(p.Size), p.Material)
		i := len(r.Tiers) - 1
		if i < 0 || r.Tiers[i].MinQty != p.MinQty {
			r.Tiers = append(r.Tiers, GridTier{MinQty: p.MinQty})
			i++
		}
		for c, cell := range cells {
			if cell.color == p.Color && cell.double == p.DoubleSide {
				r.Tiers[i].Cells[c] = p.UnitPrice.StringFixed(2)
			}
		}
	}
	for _, p := range paper {
		r := row(sizeKey(p.Size), p.Material)
		r.Paper = append(r.Paper, PaperLine{ProductID: p.ProductID, Name: p.Name, PerSheet: p.QtyPerUnit.String()})
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i].size == "" && order[j].size != "" })
	out := GridDTO{Rows: make([]GridRow, 0, len(order))}
	for _, s := range order {
		out.Rows = append(out.Rows, *rows[s])
	}
	return out
}

// fromGrid validates a grid as typed and turns it back into rows.
func fromGrid(g GridDTO) ([]Price, []Paper, error) {
	var prices []Price
	var paper []Paper
	seen := map[string]bool{}
	for _, r := range g.Rows {
		size := strings.TrimSpace(r.Size)
		material := strings.TrimSpace(r.Material)
		if len(size) > 20 {
			return nil, nil, apperr.Validation("a size name is at most 20 letters")
		}
		if len(material) > 30 {
			return nil, nil, apperr.Validation("a type name is at most 30 letters")
		}
		name := strings.TrimSpace(orAny(size) + " " + material)
		k := strings.ToLower(size + "\x00" + material)
		if seen[k] {
			return nil, nil, apperr.Validation("each size and type once — " + name + " is there twice")
		}
		seen[k] = true
		var sp *string
		if size != "" {
			sp = &size
		}
		tiers := map[int]bool{}
		for _, t := range r.Tiers {
			if t.MinQty < 1 {
				return nil, nil, apperr.Validation("a price band starts from 1 copy or more")
			}
			if tiers[t.MinQty] {
				return nil, nil, apperr.Validation(name + " has two bands from the same number of copies")
			}
			tiers[t.MinQty] = true
			for c, v := range t.Cells {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				d, err := decimal.NewFromString(v)
				if err != nil || d.IsNegative() {
					return nil, nil, apperr.Validation("prices must be numbers of 0 or more")
				}
				prices = append(prices, Price{Size: sp, Material: material, Color: cells[c].color, DoubleSide: cells[c].double, MinQty: t.MinQty, UnitPrice: d.Round(2)})
			}
		}
		for _, p := range r.Paper {
			d, err := decimal.NewFromString(strings.TrimSpace(p.PerSheet))
			if err != nil || !d.IsPositive() || p.ProductID <= 0 {
				return nil, nil, apperr.Validation("pick the paper and how much one sheet uses")
			}
			paper = append(paper, Paper{Size: sp, Material: material, ProductID: p.ProductID, QtyPerUnit: d})
		}
	}
	return prices, paper, nil
}

func orAny(size string) string {
	if size == "" {
		return "Any size"
	}
	return size
}
