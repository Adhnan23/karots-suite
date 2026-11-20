package battle

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
)

// cutBuying is the supplier the purchasing worker buys from, and its safe.
type cutBuying struct{ sup, safe, product int64 }

func (s *shop) buyingForCuts(t *testing.T, safe int64) cutBuying {
	return cutBuying{sup: s.newSup(t, "Cut Wholesale", "100000"), safe: safe, product: s.stocked(t, "Cut biscuits", s.unitPcs, 0, nil)}
}

// busy takes deliveries (some paid on delivery), pays and is refunded, sends
// goods back, and writes stock off and settles it with the supplier (sometimes
// undone) until the power goes: each whole or absent (PUR-2, SUP-1, SUP-3,
// PUR-9, REC-7).
func (k cutBuying) busy(c *hclient, stop chan struct{}) {
	src := fmt.Sprintf("locker:%d", k.safe)
	for {
		select {
		case <-stop:
			return
		default:
		}
		switch rand.IntN(5) {
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
		case 4:
			k.loseAndSettle(c)
		}
	}
}

// loseAndSettle writes one unit off and settles it with the supplier — a
// replacement with an extra, a credit, or nothing — and sometimes undoes it.
func (k cutBuying) loseAndSettle(c *hclient) {
	var w struct{ LossID int64 }
	if c.postInto("/api/v1/admin/stock/damage", map[string]any{"productId": k.product, "quantity": "1", "reason": "damage"}, &w) != nil || w.LossID == 0 {
		return
	}
	body := map[string]any{"sourceType": "damage", "sourceId": w.LossID, "outcome": "written_off"}
	switch rand.IntN(3) {
	case 0:
		body["outcome"], body["supplierId"], body["extraPerUnit"], body["expiry"] = "replacement", k.sup, "1.50", "2031-01-01"
	case 1:
		body["outcome"], body["supplierId"], body["amount"] = "paid", k.sup, "7.77"
	}
	if c.post("/api/v1/admin/recovery", body) != nil || rand.IntN(3) != 0 {
		return
	}
	day := time.Now().Format(time.DateOnly)
	var rows []struct{ ID, RecoveryID int64 }
	if c.get("/api/v1/admin/reports/losses/lines?from="+day+"&to="+day, &rows) == nil {
		for _, r := range rows {
			if r.ID == w.LossID && r.RecoveryID > 0 {
				c.del(fmt.Sprintf("/api/v1/admin/recovery/%d", r.RecoveryID)) //nolint:errcheck
			}
		}
	}
}
