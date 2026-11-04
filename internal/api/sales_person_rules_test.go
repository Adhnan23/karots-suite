package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"karots-suite/internal/features/heldsales"
)

// salesFixture closes every till (other tests' are out of reach) and gives a sales
// person and a cashier, the cashier's till open.
func salesFixture(t *testing.T) (te *testEnv, s, k *http.Cookie, sellerID, cashierID int64) {
	t.Helper()
	te = newTestEnv(t)
	seller, cashier := devPhone(), devPhone()
	sellerID = te.seedUser(t, seller, "cashier")
	cashierID = te.seedUser(t, cashier, "cashier")
	te.db.MustExec(`UPDATE users SET sends_to_cashier = true WHERE id = $1`, sellerID)
	te.db.MustExec(`UPDATE cash_register SET closed_at = now() WHERE closed_at IS NULL`)
	t.Cleanup(func() {
		te.db.MustExec(`DELETE FROM held_sales WHERE cashier_id = ANY($1::bigint[])`, fmt.Sprintf("{%d,%d}", sellerID, cashierID))
		te.db.MustExec(`DELETE FROM cash_register WHERE user_id = ANY($1::bigint[])`, fmt.Sprintf("{%d,%d}", sellerID, cashierID))
	})
	sc, kc := te.signIn(t, "dev-s", "Laptop", seller), te.signIn(t, "dev-k", "Counter", cashier)
	if rec := te.on("dev-k", http.MethodPost, "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"500": 1}}, kc); rec.Code >= 300 {
		t.Fatalf("open %d %s", rec.Code, rec.Body)
	}
	return te, sc, kc, sellerID, cashierID
}

var aTicket = HoldRequest{SaleType: "retail", Cart: `{"v":2,"lines":[]}`, ItemCount: 1, Total: "1", ToCashier: true}

func TestTillWithATicketWaitingCannotClose(t *testing.T) {
	te, s, k, _, _ := salesFixture(t)
	rec := te.on("dev-s", http.MethodPost, "/api/v1/till/held", aTicket, s)
	if rec.Code != http.StatusCreated {
		t.Fatalf("send %d %s", rec.Code, rec.Body)
	}
	rec = te.on("dev-k", http.MethodPost, "/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{"500": 1}}, k)
	if rec.Code != http.StatusConflict {
		t.Fatalf("closed with a ticket waiting: %d %s", rec.Code, rec.Body)
	}
}

func TestTicketGoesBackToSenderWhenNoTillIsLeft(t *testing.T) {
	te, s, _, sellerID, _ := salesFixture(t)
	ticket := decode[HeldSale](t, te.on("dev-s", http.MethodPost, "/api/v1/till/held", aTicket, s))
	// Close for them / a phone's close: not stopped, so the ticket mustn't be stranded.
	te.db.MustExec(`UPDATE cash_register SET closed_at = now() WHERE closed_at IS NULL`)
	if got := decode[[]HeldSale](t, te.on("dev-s", http.MethodGet, "/api/v1/till/tickets", nil, s)); len(got) != 0 {
		t.Fatalf("still a ticket: %+v", got)
	}
	held := decode[[]HeldSale](t, te.on("dev-s", http.MethodGet, "/api/v1/till/held", nil, s))
	if len(held) != 1 || held[0].ID != ticket.ID {
		t.Fatalf("sender %d's Held: %+v", sellerID, held)
	}
}

func TestOnlyWorkingCashiersGetTickets(t *testing.T) {
	te, _, _, sellerID, cashierID := salesFixture(t)
	goneID := te.seedUser(t, devPhone(), "cashier")
	t.Cleanup(func() { te.db.MustExec(`DELETE FROM cash_register WHERE user_id = $1`, goneID) })
	for _, id := range []int64{sellerID, goneID} {
		te.db.MustExec(`INSERT INTO cash_register (user_id, opening_cash) VALUES ($1, 0)`, id)
	}
	te.db.MustExec(`UPDATE users SET is_active = false WHERE id = $1`, goneID)
	rows, err := heldsales.NewService(te.db).Cashiers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != cashierID {
		t.Fatalf("cashiers %+v, want only %d", rows, cashierID)
	}
}

func TestSalesPersonFlagWaitsForTheirTillToClose(t *testing.T) {
	te := newTestEnv(t)
	boss, phone := devPhone(), devPhone()
	te.seedUser(t, boss, "admin")
	id := te.seedUser(t, phone, "cashier")
	te.db.MustExec(`INSERT INTO cash_register (user_id, opening_cash) VALUES ($1, 0)`, id)
	t.Cleanup(func() { te.db.MustExec(`DELETE FROM cash_register WHERE user_id = $1`, id) })
	b := te.signIn(t, "dev-b", "Office", boss)
	body := map[string]any{"name": "Seller", "phone": phone, "role": "cashier", "sendsToCashier": true}
	rec := te.on("dev-b", http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", id), body, b)
	if rec.Code != http.StatusConflict {
		t.Fatalf("made a sales person with a till open: %d %s", rec.Code, rec.Body)
	}
}

func TestTicketNumbersStayUniqueUnderLoad(t *testing.T) {
	te, _, _, sellerID, _ := salesFixture(t)
	svc := heldsales.NewService(te.db)
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Hold(context.Background(), sellerID, heldsales.Input{Cart: []byte(`{}`), ToCashier: true})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var dup int
	if err := te.db.Get(&dup, `SELECT count(*) FROM (SELECT ticket_no FROM held_sales WHERE to_cashier GROUP BY 1 HAVING count(*) > 1) d`); err != nil {
		t.Fatal(err)
	}
	if dup != 0 {
		t.Fatalf("%d numbers used twice", dup)
	}
	var indexed bool
	te.db.Get(&indexed, `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'held_sales' AND indexname = 'held_sales_ticket_no')`) //nolint:errcheck
	if !indexed {
		t.Fatal("no unique index behind the numbers")
	}
}
