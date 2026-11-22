// Package battle drives a throwaway shop on the real server (core only, its own
// database) through random days, checking the rule book after every step
// (docs/rules, internal/ledgercheck). Test files only: nothing ships.
package battle

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/app"
	"karots-suite/internal/config"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/ledgercheck/ltest"
)

// shop is one throwaway shop: its database, an admin and two cashiers signed
// in, and a product of every kind the rule grid names (prod["plain"], …).
type shop struct {
	db                   *sqlx.DB
	dsn                  string
	h                    http.Handler
	admin, till1, till2  *client
	prod                 map[string]int64
	customer, cardAcct   int64
	unitPcs, unitKg, cat int64
	checks               int // agreeSometimes' count
}

type saleResult struct {
	ID          int64  `json:"id"`
	Total       string `json:"total"`
	PaidAmount  string `json:"paidAmount"`
	ChangeGiven string `json:"changeGiven"`
}

// steps is how long a random run is: short inside make test, long in make battle.
func steps() int {
	if n, err := strconv.Atoi(os.Getenv("KS_BATTLE_STEPS")); err == nil && n > 0 {
		return n
	}
	return 200
}

func newShop(t ltest.T) *shop {
	if testing.Short() { // the battle is skipped with -short
		t.Skip("battle: skipped with -short")
	}
	db, dsn := ltest.NewDBWithDSN(t)
	return newShopOn(t, db, dsn)
}

// newShopOn opens the shop on a database already made (a faulty one, say).
func newShopOn(t ltest.T, db *sqlx.DB, dsn string) *shop { return newShopCfg(t, db, dsn, nil) }

// newShopCfg is newShopOn with the server's config changed by mod first.
func newShopCfg(t ltest.T, db *sqlx.DB, dsn string, mod func(*config.Config)) *shop {
	ctx := context.Background()
	cfg := &config.Config{Env: "production", JWTSecret: strings.Repeat("x", 40), JWTAccessTTL: time.Hour, JWTRefreshTTL: time.Hour}
	if mod != nil {
		mod(cfg)
	}
	v := app.SystemPINValidator(nil)
	svc := app.NewServices(db, cfg, v)
	h := app.BuildServer(db, cfg, v)
	s := &shop{db: db, dsn: dsn, h: h, prod: map[string]int64{}, unitPcs: 1, unitKg: 2}
	for _, u := range []struct{ name, phone, role string }{
		{"Admin", "0770000001", "admin"}, {"Till One", "0770000002", "cashier"}, {"Till Two", "0770000003", "cashier"},
	} {
		if _, err := svc.Auth.CreateUser(ctx, auth.CreateUserInput{Name: u.name, Phone: u.phone, Role: u.role, PIN: "1234",
			CanManageCredit: "on", CanSeeCost: "on"}); err != nil {
			t.Fatal(err)
		}
	}
	s.must(t, sqlxExec(ctx, db, `UPDATE users SET must_change_pin = false`))
	var err error
	for i, c := range []**client{&s.admin, &s.till1, &s.till2} {
		if *c, err = newClient(h, fmt.Sprintf("077000000%d", i+1), "1234", fmt.Sprintf("battle-%p-%d", s, i)); err != nil {
			t.Fatal(err)
		}
	}
	s.fixtures(t)
	return s
}

// fixtures makes one product of every kind in the grid, each with plenty in
// stock, a credit customer and a card account.
func (s *shop) fixtures(t ltest.T) {
	var c struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/categories", map[string]string{"name": "Battle"}, &c))
	s.cat = c.ID
	mk := func(key, price string, unit int64, extra map[string]any) {
		body := map[string]any{"name": "Battle " + key, "categoryId": s.cat, "unitId": unit,
			"costPrice": "1", "sellingPrice": price, "wholesalePrice": price, "reorderAuto": false}
		for k, v := range extra {
			body[k] = v
		}
		var p struct{ ID int64 }
		s.must(t, s.admin.Do("POST", "/api/v1/admin/products", body, &p))
		s.prod[key] = p.ID
		if extra["trackSerial"] == true {
			return // serial units come in one by one in the tests that need them
		}
		s.must(t, s.admin.Do("POST", "/api/v1/admin/stock/adjust", map[string]any{"productId": p.ID, "newQuantity": "100000", "note": "battle"}, nil))
	}
	mk("plain", "100", s.unitPcs, nil)
	mk("odd", "33.33", s.unitPcs, nil)
	mk("cheap", "0.99", s.unitPcs, nil)
	mk("loose", "245.50", s.unitKg, nil)
	mk("pack", "12.50", s.unitPcs, map[string]any{"packs": []map[string]string{{"name": "Card", "size": "12", "sellingPrice": "140"}}})
	mk("serial", "15000", s.unitPcs, map[string]any{"trackSerial": true, "warrantyMonths": 12})

	var cu struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/customers", map[string]string{"name": "Battle Customer", "phone": "0711111111", "creditLimit": "1000000"}, &cu))
	s.customer = cu.ID
	var l struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": "Battle card", "kind": "card", "openingBalance": "0"}, &l))
	s.cardAcct = l.ID
}

// open starts a till with Rs 5,000 float.
func (s *shop) open(t ltest.T, c *client) {
	s.must(t, c.Do("POST", "/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}}, nil))
}

// close counts exactly what the till expects.
func (s *shop) close(t ltest.T, c *client) {
	var st struct {
		Summary struct{ Expected string } `json:"summary"`
	}
	s.must(t, c.Do("GET", "/api/v1/till/drawer", nil, &st))
	count := map[string]int{}
	if st.Summary.Expected != "0.00" {
		count[st.Summary.Expected] = 1
	}
	s.must(t, c.Do("POST", "/api/v1/till/drawer/close", map[string]any{"breakdown": count}, nil))
}

// audit fails the test with every rule the shop's data breaks.
func (s *shop) audit(t ltest.T) {
	ltest.Check(t, context.Background(), s.db)
}

func (s *shop) must(t ltest.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func sqlxExec(ctx context.Context, db *sqlx.DB, q string) error {
	_, err := db.ExecContext(ctx, q)
	return err
}
