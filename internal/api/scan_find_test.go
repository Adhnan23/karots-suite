package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// A scanned slip number finds its slip first, from any date: S-00231 opens
// S-00231 even when S-00231… (containing it) is newer and the slip is older
// than the list's range.
func TestScanFindsSlip(t *testing.T) {
	te := newTestEnv(t)
	te.db.Exec(`UPDATE settings SET allow_untracked_cash = true`) //nolint:errcheck
	f := te.seedTill(t)
	ck := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "cashier")
	admin := te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000+1), "admin")
	te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2}}, ck)
	defer te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{}, ck)
	sfx := time.Now().UnixNano()

	// first decodes a list and returns its first row's receipt number.
	first := func(path string, who *http.Cookie) string {
		t.Helper()
		rec := te.do(http.MethodGet, path, nil, who)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body)
		}
		rows := decode[[]struct {
			ReceiptNo string `json:"receiptNo"`
		}](t, rec)
		if len(rows) == 0 {
			return ""
		}
		return rows[0].ReceiptNo
	}

	// Sales: the old one, and a newer one whose number contains it.
	old := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "500"}), ck))
	newer := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "500"}), ck))
	te.db.Exec(`UPDATE sales SET created_at = now() - interval '1 year' WHERE id = $1`, old.ID)                       //nolint:errcheck
	te.db.Exec(`UPDATE sales SET receipt_no = $1 WHERE id = $2`, fmt.Sprintf("%s-T%d", old.ReceiptNo, sfx), newer.ID) //nolint:errcheck
	for _, q := range []string{old.ReceiptNo, strings.ToLower(old.ReceiptNo)} {
		if got := first("/api/v1/till/sales?preset=today&q="+url.QueryEscape(q), ck); got != old.ReceiptNo {
			t.Fatalf("till sales q=%s: first %q, want %q", q, got, old.ReceiptNo)
		}
		if got := first("/api/v1/admin/sales?preset=today&q="+url.QueryEscape(q), admin); got != old.ReceiptNo {
			t.Fatalf("admin sales q=%s: first %q, want %q", q, got, old.ReceiptNo)
		}
	}

	// CR- and DP-: two credit payments, the first a year old.
	var cid int64
	te.db.Get(&cid, `INSERT INTO customers (name, credit_limit, outstanding_balance) VALUES ($1, 5000, 500) RETURNING id`, fmt.Sprintf("T scan %d", sfx)) //nolint:errcheck
	p1 := decode[CreditPaymentResult](t, te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid), CreditPaymentRequest{Amount: "100", Method: "cash"}, ck))
	p2 := decode[CreditPaymentResult](t, te.do(http.MethodPost, fmt.Sprintf("/api/v1/till/customers/%d/payment", cid), CreditPaymentRequest{Amount: "100", Method: "cash"}, ck))
	te.db.Exec(`UPDATE customer_payments SET created_at = now() - interval '1 year' WHERE id = $1`, p1.PaymentID)                    //nolint:errcheck
	te.db.Exec(`UPDATE customer_payments SET receipt_no = $1 WHERE id = $2`, fmt.Sprintf("%s-T%d", p1.ReceiptNo, sfx), p2.PaymentID) //nolint:errcheck
	if got := first("/api/v1/till/customer-payments?preset=today&q="+url.QueryEscape(p1.ReceiptNo), ck); got != p1.ReceiptNo {
		t.Fatalf("debt receipts: first %q, want %q", got, p1.ReceiptNo)
	}

	var crs []struct {
		ID        int64
		ReceiptNo string `db:"receipt_no"`
	}
	te.db.Select(&crs, `SELECT id, receipt_no FROM money_receipts ORDER BY id DESC LIMIT 2`) //nolint:errcheck
	if len(crs) != 2 {
		t.Fatalf("money receipts: %+v", crs)
	}
	oldCR := crs[1]
	te.db.Exec(`UPDATE money_receipts SET created_at = now() - interval '1 year' WHERE id = $1`, oldCR.ID)                        //nolint:errcheck
	te.db.Exec(`UPDATE money_receipts SET receipt_no = $1 WHERE id = $2`, fmt.Sprintf("%s-T%d", oldCR.ReceiptNo, sfx), crs[0].ID) //nolint:errcheck
	if got := first("/api/v1/till/money-receipts?preset=today&q="+url.QueryEscape(oldCR.ReceiptNo), ck); got != oldCR.ReceiptNo {
		t.Fatalf("money receipts: first %q, want %q", got, oldCR.ReceiptNo)
	}
}
