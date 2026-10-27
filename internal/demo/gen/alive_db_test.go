package gen

import (
	"context"
	"testing"
	"time"

	"github.com/lib/pq"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/datacheck"
)

func newSim(t *testing.T) (*Sim, Deps) {
	t.Helper()
	d := newShopDB(t)
	p := small("grocery")
	if err := Run(context.Background(), d, p, time.Now(), 30, nop); err != nil {
		t.Fatal(err)
	}
	s, err := NewSim(context.Background(), d, p)
	if err != nil {
		t.Fatal(err)
	}
	s.tick = 20 * time.Millisecond
	return s, d
}

func saleCount(t *testing.T, d Deps) int {
	t.Helper()
	var n int
	if err := d.DB.Get(&n, `SELECT count(*) FROM sales`); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSimSellsAndPauseStops(t *testing.T) {
	s, d := newSim(t)
	before := saleCount(t, d)
	ctx, cancel := context.WithCancel(context.Background())
	go s.Run(ctx)
	time.Sleep(1500 * time.Millisecond)
	s.Pause(true)
	time.Sleep(200 * time.Millisecond) // let a step in flight finish
	during := saleCount(t, d)
	if during <= before {
		t.Fatalf("the shop sold nothing on its own (%d → %d)", before, during)
	}
	time.Sleep(600 * time.Millisecond)
	if after := saleCount(t, d); after != during {
		t.Fatalf("paused, yet %d more sales", after-during)
	}
	cancel()
	var staffSales int
	if err := d.DB.Get(&staffSales, `SELECT count(*) FROM sales s JOIN users u ON u.id = s.cashier_id
		WHERE s.id > (SELECT max(id) - $1 FROM sales) AND u.phone = $2`, during-before, s.g.s.p.Logins[2].Phone); err != nil {
		t.Fatal(err)
	}
	if staffSales != 0 {
		t.Errorf("the simulator sold %d on the shown cashier's till", staffSales)
	}
}

func TestCheats(t *testing.T) {
	s, d := newSim(t)
	ctx := context.Background()
	before := saleCount(t, d)
	if err := s.Cheat("busy"); err != nil {
		t.Fatal(err)
	}
	if n := saleCount(t, d) - before; n < 15 { // a sale skips what's sold out
		t.Errorf("busy added %d sales", n)
	}
	if err := s.Cheat("closing"); err != nil {
		t.Fatal(err)
	}
	staff := s.staff[0]
	sum, err := d.Svc.CashRegister.Summary(ctx, staff.id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Svc.CashRegister.Close(ctx, staff.id, cashregister.CloseInput{ClosingCash: sum.Expected.String(), DestLockerID: s.g.s.safe}); err != nil {
		t.Fatalf("closing a staff till after the closing move: %v", err)
	}
	if err := s.Cheat("open-till"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, d.DB, `SELECT count(*) FROM cash_register r JOIN users u ON u.id = r.user_id WHERE r.closed_at IS NULL AND u.phone = ANY($1)`,
		phones(s)); n != 3 {
		t.Errorf("%d shown logins have a till open, want 3", n)
	}
	if err := s.Cheat("low-stock"); err != nil {
		t.Fatal(err)
	}
	if err := s.Cheat("credit-due"); err != nil {
		t.Fatal(err)
	}
	var over bool
	if err := d.DB.Get(&over, `SELECT bool_or(outstanding_balance > credit_limit) FROM customers WHERE credit_limit > 0`); err != nil || !over {
		t.Errorf("no customer over their limit (%v)", err)
	}
	if err := s.Cheat("cycle-palette"); err != nil {
		t.Fatal(err)
	}
	var skin string
	d.DB.Get(&skin, `SELECT skin FROM settings WHERE id = 1`)
	if skin != "night" {
		t.Errorf("after studio the next palette is %q, want night", skin)
	}
	if err := s.Cheat("delivery"); err != nil {
		t.Fatal(err)
	}
	if err := s.Cheat("plugin:repairs"); err == nil {
		t.Error("a grocery took a repair")
	}
	f, err := datacheck.Run(ctx, d.DB)
	if err != nil {
		t.Fatal(err)
	}
	if len(f) != 0 {
		t.Fatalf("data check findings after the cheats: %+v", f)
	}
}

func phones(s *Sim) any {
	var out []string
	for _, l := range s.g.s.p.Logins {
		out = append(out, l.Phone)
	}
	return pq.Array(out)
}

func TestCheatNamesOnlyForTheShopsPlugins(t *testing.T) {
	for _, c := range CheatNames(small("grocery")) {
		if c.Group == "Plugins" {
			t.Errorf("grocery offers %s", c.Name)
		}
	}
}
