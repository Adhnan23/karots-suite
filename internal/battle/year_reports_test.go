package battle

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/app"
	"karots-suite/internal/config"
	"karots-suite/internal/datetime"
	"karots-suite/internal/demo/gen"
	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/ledgercheck/ltest"
)

// A grocery's year through the real API (the Demo Room's generator, a test-sized
// volume): every report agrees with the raw rows and each other, month by month
// and over the whole year (area 7). Long — make battle only.
func TestReportsAgreeOverAYear(t *testing.T) {
	t.Parallel()
	if os.Getenv("KS_BATTLE_STEPS") == "" {
		t.Skip("a year of reports runs in make battle")
	}
	ctx := context.Background()
	db := ltest.NewDB(t)
	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	v := app.SystemPINValidator(nil)
	svc := app.NewServices(db, cfg, v)
	h := app.BuildServer(db, cfg, v)
	p, _ := shops.ByKey("grocery")
	p.Volume = 25
	now := time.Now().In(datetime.Location)
	if err := gen.Past(ctx, gen.Deps{DB: db, Svc: svc, API: h, ReceiptPrinter: "127.0.0.1:9", LabelPrinter: "127.0.0.1:9"},
		p, now, 365, func(int, string) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Auth.CreateUser(ctx, auth.CreateUserInput{Name: "Year auditor", Phone: "0779999999", Role: "admin", PIN: "1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE users SET must_change_pin = false WHERE phone = '0779999999'`); err != nil {
		t.Fatal(err)
	}
	admin, err := newClient(h, "0779999999", "1234", "battle-year")
	if err != nil {
		t.Fatal(err)
	}
	s := &shop{db: db, admin: admin}
	ltest.Check(t, ctx, db)
	var sales, returns, expenses int
	s.must(t, db.Get(&sales, `SELECT count(*) FROM sales`))
	s.must(t, db.Get(&returns, `SELECT count(*) FROM sale_returns`))
	s.must(t, db.Get(&expenses, `SELECT count(*) FROM expenses`))
	if sales < 1000 || returns == 0 || expenses == 0 {
		t.Fatalf("the year is too thin to prove anything: %d sales, %d returns, %d expenses", sales, returns, expenses)
	}
	t.Logf("a year of %d sales, %d returns, %d expenses", sales, returns, expenses)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, datetime.Location).AddDate(-1, 0, 0)
	for m := first; !m.After(now); m = m.AddDate(0, 1, 0) {
		s.agree(t, m.Format(time.DateOnly), m.AddDate(0, 1, -1).Format(time.DateOnly))
	}
	s.agree(t, first.Format(time.DateOnly), now.Format(time.DateOnly))
}
