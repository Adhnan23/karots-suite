package gen

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// heldLabels are what a cashier types when a customer steps away mid-sale.
var heldLabels = []string{"Lady in red saree — gone to ATM", "Three-wheeler driver, back at 5", "Saman — forgot his wallet",
	"School van uncle (pays Friday)", "Hotel order — picking up later", "Grandma — son coming to pay"}

// heldCarts leaves a sale or two on hold at the shown cashier's till this
// morning, the way the till's Hold button keeps them (its own cart document).
func (g *gen) heldCarts(day time.Time, until float64) error {
	if until < 9.5 {
		return nil
	}
	cl, err := g.tillOf(g.s.cashiers[0])
	if err != nil {
		return err
	}
	for n := 0; n < 2; n++ {
		var lines []map[string]any
		total := decimal.Zero
		for len(lines) < 2+g.r.IntN(4) {
			it := g.pick()
			if it.service || it.warranty > 0 || g.stock[it.id] < 3 {
				continue
			}
			q := float64(1 + g.r.IntN(2))
			lines = append(lines, map[string]any{"key": len(lines) + 1, "productId": it.id, "name": it.name,
				"unitPrice": it.price.InexactFloat64(), "taxRate": 0, "qty": q, "stock": g.stock[it.id], "isService": false,
				"allowDecimal": it.unit == "kg" || it.unit == "m", "batchId": 0, "batchRemaining": 0, "batchLabel": "",
				"allowOversell": false, "discount": "", "discountType": "fixed", "trackSerial": false, "warrantyMonths": 0,
				"serials": []string{}, "priceOverride": "", "description": "", "components": []any{}})
			total = total.Add(it.price.Mul(decimal.NewFromFloat(q)))
		}
		blob, _ := json.Marshal(map[string]any{"v": 2, "lines": lines, "billDiscountType": "fixed", "meta": map[string]any{}})
		g.tm.at(at(day, 8.6+float64(n)*0.7+g.r.Float64()*0.3))
		if err := cl.Do("POST", "/api/v1/till/held", map[string]any{"label": heldLabels[g.r.IntN(len(heldLabels))],
			"saleType": "retail", "discount": "0", "cart": string(blob), "itemCount": len(lines), "total": total.StringFixed(2)}, nil); err != nil {
			return fmt.Errorf("held sale: %w", err)
		}
	}
	return nil
}
