package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestAdminStockAdjustAndLots(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone='0770009551')`) //nolint:errcheck
	ck := te.login(t, "0770009551", "admin")
	// Runs before the user's own cleanup (LIFO): its movements would block deleting it.
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM stock_movements WHERE user_id IN (SELECT id FROM users WHERE phone='0770009551')`) //nolint:errcheck
	})
	pid := strconv.FormatInt(f.lotID, 10)

	lots := decode[ProductLots](t, te.do(http.MethodGet, "/api/v1/admin/stock/lots/"+pid, nil, ck))
	if len(lots.Lots) != 2 {
		t.Fatalf("lots: %+v", lots)
	}
	// The 120-priced lot is the one that goes: take 2 off exactly that lot.
	var priced LotRow
	for _, l := range lots.Lots {
		if l.SellingPrice == "120.00" {
			priced = l
		}
	}
	if rec := te.do(http.MethodPost, "/api/v1/admin/stock/adjust", AdjustRequest{ProductID: f.lotID, NewQuantity: "8", Note: "broken", BatchID: priced.ID}, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("adjust: %d %s", rec.Code, rec.Body)
	}
	var left float64
	te.db.Get(&left, `SELECT qty_remaining FROM stock_batches WHERE id=$1`, priced.ID) //nolint:errcheck
	if left != 3 {
		t.Fatalf("chosen lot left %v", left)
	}

	mv := decode[MovementList](t, te.do(http.MethodGet, "/api/v1/admin/stock/movements?type=adjust&product="+pid+"&preset=today", nil, ck))
	if mv.Total != 1 || mv.Items[0].Quantity != "-2" || mv.Items[0].Note != "broken" || mv.Items[0].Cost != "100.00" {
		t.Fatalf("movements: %+v", mv)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/stock/movements?format=csv&type=adjust&product="+pid, nil, ck); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "broken") {
		t.Fatalf("csv: %d %.120s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/stock/movements?type=nonsense", nil, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad type: %d", rec.Code)
	}

	// Lot price: set, then blank → follows the product again.
	path := "/api/v1/admin/stock/batches/" + strconv.FormatInt(priced.ID, 10) + "/price"
	te.do(http.MethodPost, path, LotPriceRequest{SellingPrice: ""}, ck)
	for _, l := range decode[ProductLots](t, te.do(http.MethodGet, "/api/v1/admin/stock/lots/"+pid, nil, ck)).Lots {
		if l.SellingPrice != "" {
			t.Fatalf("lot still priced: %+v", l)
		}
	}
	if rec := te.do(http.MethodPost, path, LotPriceRequest{SellingPrice: "-1"}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("negative price: %d", rec.Code)
	}
}
