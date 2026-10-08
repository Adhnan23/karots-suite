package customers

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// Re-importing a customer list never duplicates a customer: the phone matches
// whatever its spacing (as Create does), and a row with no phone matches by
// name. Import commits its own transaction, so this cleans up after itself.
func TestImportOneMatchesNormalisedPhoneAndPhonelessByName(t *testing.T) {
	conn := testDB(t)
	defer conn.Close()
	ctx := context.Background()
	clean := func() { conn.ExecContext(ctx, `DELETE FROM customers WHERE name ILIKE 'TEST Imp%'`) } //nolint:errcheck
	clean()
	defer clean()
	svc := NewService(conn)

	rows := []ImportRow{
		{Name: "TEST Imp Phone", Phone: "077-555 1212", CreditLimit: decimal.NewFromInt(100), OpeningBalance: decimal.NewFromInt(50)},
		{Name: "TEST Imp Phone", Phone: "0775551212", CreditLimit: decimal.NewFromInt(200)},
		{Name: "TEST Imp NoPhone", CreditLimit: decimal.NewFromInt(10)},
		{Name: "test imp nophone", CreditLimit: decimal.NewFromInt(20)},
	}
	want := []string{"created", "updated", "created", "updated"}
	for i, r := range rows {
		res, err := svc.ImportOne(ctx, r)
		must(t, err)
		if res.Action != want[i] {
			t.Errorf("row %d: %s, want %s", i+1, res.Action, want[i])
		}
	}
	var n int
	must(t, conn.GetContext(ctx, &n, `SELECT count(*) FROM customers WHERE name ILIKE 'TEST Imp%'`))
	if n != 2 {
		t.Fatalf("%d customers, want 2", n)
	}
	var phone, limit, owed string
	must(t, conn.QueryRowContext(ctx, `SELECT phone, credit_limit::text, outstanding_balance::text FROM customers WHERE name = 'TEST Imp Phone'`).Scan(&phone, &limit, &owed))
	if phone != "0775551212" || limit != "200.00" || owed != "50.00" {
		t.Errorf("phone %q limit %s owed %s, want 0775551212 / 200.00 / 50.00", phone, limit, owed)
	}
}
