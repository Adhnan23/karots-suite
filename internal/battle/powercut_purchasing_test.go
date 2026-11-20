package battle

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// cutBuying is the supplier the purchasing worker buys from, and its safe.
type cutBuying struct{ sup, safe, product int64 }

func (s *shop) buyingForCuts(t *testing.T, safe int64) cutBuying {
	return cutBuying{sup: s.newSup(t, "Cut Wholesale", "100000"), safe: safe, product: s.stocked(t, "Cut biscuits", s.unitPcs, 0, nil)}
}

// busy takes deliveries (some paid on delivery), pays and is refunded, and
// sends goods back until the power goes: each whole or absent (PUR-2, SUP-1,
// SUP-3, PUR-9).
func (k cutBuying) busy(c *hclient, stop chan struct{}) {
	src := fmt.Sprintf("locker:%d", k.safe)
	for {
		select {
		case <-stop:
			return
		default:
		}
		switch rand.IntN(4) {
		case 0:
			var r struct{ PurchaseID int64 }
			body := map[string]any{"supplierId": k.sup, "discount": "1.10", "discountType": "fixed",
				"lines": []sl{{"productId": k.product, "quantity": fmt.Sprint(5 + rand.IntN(20)), "freeQty": "1", "costPrice": "13.37"}}}
			if rand.IntN(2) == 0 {
				body["pay"] = map[string]string{"amount": "40", "method": "cash", "source": src}
			}
			if c.postInto("/api/v1/admin/purchases/receive", body, &r) == nil && rand.IntN(2) == 0 {
				var ret struct {
					Lines []struct{ BatchID int64 }
				}
				if c.get(fmt.Sprintf("/api/v1/admin/purchases/%d/returnable", r.PurchaseID), &ret) == nil && len(ret.Lines) > 0 {
					c.post("/api/v1/admin/purchase-returns", map[string]any{"supplierId": k.sup, "purchaseId": r.PurchaseID, //nolint:errcheck
						"lines": []map[string]any{{"productId": k.product, "quantity": "2", "batchId": ret.Lines[0].BatchID}}})
				}
			}
		case 1:
			c.post(supPath(k.sup, "payment"), map[string]string{"amount": "75.25", "method": "cash", "source": src}) //nolint:errcheck
		case 2:
			c.post(supPath(k.sup, "payment"), map[string]string{"oldDebt": "10", "method": "cash", "source": src}) //nolint:errcheck
		case 3:
			c.post(supPath(k.sup, "refund"), map[string]string{"amount": "5", "method": "cash", "source": src}) //nolint:errcheck
		}
	}
}
