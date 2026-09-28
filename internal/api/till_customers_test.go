package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestTillCustomers(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009201", "cashier")
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)

	rec := te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "Till Cust", Phone: phone, CreditLimit: "1000"}, ck)
	created := decode[CustomerCreated](t, rec)
	if rec.Code != 201 || created.Existed || created.Customer.CreditLimit != "1000.00" {
		t.Fatalf("create %d %+v", rec.Code, created)
	}
	rec = te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "Dup", Phone: phone}, ck)
	if again := decode[CustomerCreated](t, rec); rec.Code != 200 || !again.Existed || again.Customer.ID != created.Customer.ID {
		t.Fatalf("dedupe by phone %d %+v", rec.Code, again)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "No phone"}, ck); rec.Code != 400 && rec.Code != 422 {
		t.Fatalf("phone required, got %d", rec.Code)
	}

	path := fmt.Sprintf("/api/v1/till/customers/%d/credit-limit", created.Customer.ID)
	if rec := te.do(http.MethodPatch, path, CreditLimitRequest{CreditLimit: "5000"}, ck); rec.Code != 403 {
		t.Fatalf("cashier without flag must not raise limits: %d", rec.Code)
	}
	admin := te.login(t, "0770009202", "admin")
	if rec := te.do(http.MethodPatch, path, CreditLimitRequest{CreditLimit: "5000"}, admin); rec.Code != 204 {
		t.Fatalf("admin limit %d %s", rec.Code, rec.Body)
	}
	var got TillCustomer
	for _, c := range decode[[]TillCustomer](t, te.do(http.MethodGet, "/api/v1/till/customers", nil, ck)) {
		if c.ID == created.Customer.ID {
			got = c
		}
	}
	if got.CreditLimit != "5000.00" || got.Outstanding != "0.00" {
		t.Fatalf("list %+v", got)
	}
}

func TestTillQuickItemAndWriteOff(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	// A fresh phone each run: QuickCreate/Consume anchor this user via RESTRICT
	// FKs (products.created_by, stock_movements.user_id), so seedUser's cleanup
	// can never delete it and a fixed phone would collide on the next run.
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	ck := te.login(t, phone, "cashier")

	bc := decode[GeneratedBarcode](t, te.do(http.MethodGet, "/api/v1/till/barcode/generate", nil, ck))
	rec := te.do(http.MethodPost, "/api/v1/till/quick-item", QuickItemRequest{Name: "Quick thing", Price: "45", Qty: "2", Barcode: bc.Barcode, UnitID: 1}, ck)
	if p := decode[TillProduct](t, rec); rec.Code != 200 || p.ID == 0 || p.SellingPrice != "45.00" || p.Barcode != bc.Barcode {
		t.Fatalf("quick item %d %+v", rec.Code, p)
	}

	lots := decode[[]Lot](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/lots", f.expiringID), nil, ck))
	expired := lots[0] // FEFO: the expired one sorts first
	rec = te.do(http.MethodPost, "/api/v1/till/write-off", WriteOffRequest{ProductID: f.expiringID, BatchID: expired.BatchID, Quantity: expired.QtyRemaining, Note: "expired " + expired.ExpiryDate}, ck)
	if rec.Code != 204 {
		t.Fatalf("write-off %d %s", rec.Code, rec.Body)
	}
	opts := decode[map[string][]Lot](t, te.do(http.MethodGet, "/api/v1/till/price-options", nil, ck))
	if _, ok := opts[strconv.FormatInt(f.expiringID, 10)]; ok {
		t.Fatal("after writing off the expired lot the product must stop prompting")
	}
}
