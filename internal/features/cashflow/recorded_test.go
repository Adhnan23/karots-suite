package cashflow

import (
	"context"
	"os"
	"testing"

	"karots-suite/internal/db"
	"karots-suite/internal/features/sales"

	"github.com/shopspring/decimal"
)

// A Recorded move already happened outside the suite (a bill paid in the bank's
// app): it is written even past the balance; an ordinary move is still refused.
func TestRecordedMoveSkipsTheOverdrawGuard(t *testing.T) {
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
	var bank int64
	if err := tx.GetContext(ctx, &bank, `INSERT INTO lockers (name, kind, allow_negative) VALUES ('TEST recorded bank', 'bank', false) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	s := NewService(conn, sales.NewService(conn))
	mv := MoveInput{From: Locker(bank), To: External(), Amount: decimal.NewFromInt(500), Reason: "bill", Party: "CEB"}
	if _, err := s.MoveTx(ctx, tx, mv); err == nil {
		t.Fatal("an ordinary move overdrew an empty bank")
	}
	tx.Rollback() //nolint:errcheck
	if tx, err = conn.BeginTxx(ctx, nil); err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := tx.GetContext(ctx, &bank, `INSERT INTO lockers (name, kind, allow_negative) VALUES ('TEST recorded bank', 'bank', false) RETURNING id`); err != nil {
		t.Fatal(err)
	}
	mv.From, mv.Recorded = Locker(bank), true
	if _, err := s.MoveTx(ctx, tx, mv); err != nil {
		t.Fatalf("a recorded move was refused: %v", err)
	}
	var bal decimal.Decimal
	if err := tx.GetContext(ctx, &bal, `SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, bank); err != nil {
		t.Fatal(err)
	}
	if bal.String() != "-500" {
		t.Fatalf("bank balance %s, want -500", bal)
	}
}
