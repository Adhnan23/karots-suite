package lockers

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"karots-suite/internal/db"
)

func TestChequeKindRules(t *testing.T) {
	for _, m := range []string{"", "cash", "card", "online"} {
		if Accepts(KindCheque, m) {
			t.Fatalf("cheque place accepts %q", m)
		}
	}
	if HoldsCash(KindCheque) {
		t.Fatal("cheque place holds cash")
	}
	if validKind(KindCheque) {
		t.Fatal("cheque must not be a creatable kind")
	}
}

func TestChequesInHandRecreated(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`UPDATE settings SET default_locker_id = NULL WHERE default_locker_id IN (SELECT id FROM lockers WHERE kind='cheque')`,
		`DELETE FROM locker_ledger WHERE locker_id IN (SELECT id FROM lockers WHERE kind='cheque') OR counter_locker_id IN (SELECT id FROM lockers WHERE kind='cheque')`,
		`DELETE FROM lockers WHERE kind='cheque'`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	a, err := ChequesInHandTx(ctx, tx)
	if err != nil || a == 0 {
		t.Fatalf("first call: %d, %v", a, err)
	}
	var seq1, seq2 int64
	tx.GetContext(ctx, &seq1, `SELECT last_value FROM lockers_id_seq`) //nolint:errcheck
	b, err := ChequesInHandTx(ctx, tx)
	if err != nil || b != a {
		t.Fatalf("second call: %d (want %d), %v", b, a, err)
	}
	tx.GetContext(ctx, &seq2, `SELECT last_value FROM lockers_id_seq`) //nolint:errcheck
	if seq2 != seq1 {
		t.Fatalf("a call with the place there used up a locker id (%d → %d)", seq1, seq2)
	}

	svc := NewService(conn)
	if _, err := svc.Create(ctx, CreateInput{Name: "X", Kind: "cheque"}, 0); err == nil {
		t.Fatal("Create accepted cheque kind")
	}
	// The real row (outside the rolled-back tx) is the one the service must protect.
	var real int64
	if err := conn.Get(&real, `SELECT id FROM lockers WHERE kind='cheque'`); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(ctx, real, UpdateInput{Name: "Z", Kind: "safe", IsActive: "on"}); err == nil {
		t.Fatal("Update touched Cheques in hand")
	}
	if err := svc.SetActive(ctx, real, false); err == nil {
		t.Fatal("SetActive touched Cheques in hand")
	}
	ls, err := svc.ListForCashier(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range ls {
		if l.Kind == KindCheque {
			t.Fatal("ListForCashier returned the cheque place")
		}
	}
}

// A bank place with the shop's cheques still waiting on it can't be archived
// or turned into another kind — those cheques could never be cleared.
func TestBankWithWaitingChequesStays(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	var bank, sup, chq int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	name := fmt.Sprintf("TEST waiting bank %d", time.Now().UnixNano())
	must(conn.Get(&bank, `INSERT INTO lockers (name, kind) VALUES ($1, 'bank') RETURNING id`, name))
	must(conn.Get(&sup, `INSERT INTO suppliers (name) VALUES ($1) RETURNING id`, name))
	must(conn.Get(&chq, `INSERT INTO cheques (direction, locker_id, number, amount, cheque_date, supplier_id) VALUES ('out', $1, 'W1', 100, CURRENT_DATE, $2) RETURNING id`, bank, sup))
	t.Cleanup(func() {
		conn.Exec(`DELETE FROM cheques WHERE id = $1`, chq)   //nolint:errcheck
		conn.Exec(`DELETE FROM lockers WHERE id = $1`, bank)  //nolint:errcheck
		conn.Exec(`DELETE FROM suppliers WHERE id = $1`, sup) //nolint:errcheck
	})
	svc := NewService(conn)
	if err := svc.SetActive(ctx, bank, false); err == nil {
		t.Fatal("archived a bank with a cheque waiting on it")
	}
	if err := svc.Update(ctx, bank, UpdateInput{Name: name, Kind: "safe", IsActive: "on"}); err == nil {
		t.Fatal("turned a bank with a cheque waiting on it into a safe")
	}
	must(svc.Update(ctx, bank, UpdateInput{Name: name + " 2", Kind: "bank", IsActive: "on"})) // a rename is fine
	conn.MustExec(`UPDATE cheques SET status = 'cleared', status_date = CURRENT_DATE WHERE id = $1`, chq)
	must(svc.SetActive(ctx, bank, false))
}
