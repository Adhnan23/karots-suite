package lockers

import (
	"context"
	"os"
	"testing"

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
	b, err := ChequesInHandTx(ctx, tx)
	if err != nil || b != a {
		t.Fatalf("second call: %d (want %d), %v", b, a, err)
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
