package documents

import (
	"encoding/json"
	"os"
	"testing"

	"karots-suite/internal/features/recipes"

	"github.com/shopspring/decimal"
)

type parityFixture struct {
	Prices []struct {
		Size      *string `json:"size"`
		Material  string  `json:"material"`
		Color     bool    `json:"color"`
		Double    bool    `json:"double"`
		MinQty    int     `json:"minQty"`
		UnitPrice string  `json:"unitPrice"`
	} `json:"prices"`
	Paper []struct {
		Size       *string `json:"size"`
		Material   string  `json:"material"`
		ProductID  int64   `json:"productId"`
		QtyPerUnit string  `json:"qtyPerUnit"`
	} `json:"paper"`
	Recipe []struct {
		ProductID  int64   `json:"productId"`
		QtyPerUnit *string `json:"qtyPerUnit"`
		YieldUnits *string `json:"yieldUnits"`
		WholeUnits bool    `json:"wholeUnits"`
	} `json:"recipe"`
	Cases []struct {
		Name     string `json:"name"`
		Size     string `json:"size"`
		Material string `json:"material"`
		Color    bool   `json:"color"`
		Double   bool   `json:"double"`
		Copies   int    `json:"copies"`
		Want     *struct {
			UnitPrice  string `json:"unitPrice"`
			Total      string `json:"total"`
			MinQty     int    `json:"minQty"`
			Sheets     int    `json:"sheets"`
			Components []struct {
				ProductID int64  `json:"productId"`
				Quantity  string `json:"quantity"`
			} `json:"components"`
		} `json:"want"`
	} `json:"cases"`
}

func nullDec(s *string) decimal.NullDecimal {
	if s == nil {
		return decimal.NullDecimal{}
	}
	return decimal.NullDecimal{Decimal: decimal.RequireFromString(*s), Valid: true}
}

// The phone prices a job itself when the PC is down (web/.../offline/routes/
// documents.ts). This fixture is read by both, so the two can't drift apart.
func TestQuoteMatchesThePhone(t *testing.T) {
	raw, err := os.ReadFile("../../web/src/plugins/pocketcashier/offline/routes/documents.fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	var f parityFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	var prices []Price
	for _, p := range f.Prices {
		prices = append(prices, Price{Size: p.Size, Material: p.Material, Color: p.Color, DoubleSide: p.Double, MinQty: p.MinQty, UnitPrice: decimal.RequireFromString(p.UnitPrice)})
	}
	var paper []Paper
	for _, p := range f.Paper {
		paper = append(paper, Paper{Size: p.Size, Material: p.Material, ProductID: p.ProductID, QtyPerUnit: decimal.RequireFromString(p.QtyPerUnit)})
	}
	var recipe []recipes.Component
	for _, r := range f.Recipe {
		recipe = append(recipe, recipes.Component{ComponentProductID: r.ProductID, QtyPerUnit: nullDec(r.QtyPerUnit), YieldUnits: nullDec(r.YieldUnits), WholeUnits: r.WholeUnits})
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			o := JobOptions{Size: c.Size, Material: c.Material, Color: c.Color, Double: c.Double, Copies: c.Copies}
			pr, ok := PickPrice(prices, o)
			if c.Want == nil {
				if ok {
					t.Fatalf("priced at %s", pr.UnitPrice)
				}
				return
			}
			if !ok || pr.UnitPrice.StringFixed(2) != c.Want.UnitPrice || pr.MinQty != c.Want.MinQty || Sheets(o) != c.Want.Sheets {
				t.Fatalf("%v %s band %d sheets %d", ok, pr.UnitPrice, pr.MinQty, Sheets(o))
			}
			if total := pr.UnitPrice.Mul(decimal.NewFromInt(int64(c.Copies))).StringFixed(2); total != c.Want.Total {
				t.Fatalf("total %s", total)
			}
			got, _ := Components(PaperFor(paper, o.Size, o.Material), recipe, o)
			kept := c.Want.Components
			if len(got) != len(kept) {
				t.Fatalf("components %+v, want %+v", got, kept)
			}
			for i := range got {
				if got[i].ProductID != kept[i].ProductID || !got[i].Qty.Equal(decimal.RequireFromString(kept[i].Quantity)) {
					t.Fatalf("component %d: %+v, want %+v", i, got[i], kept[i])
				}
			}
		})
	}
}
