package gen

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/categories"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/lockers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/suppliers"
)

type item struct {
	id          int64
	name        string
	cost, price decimal.Decimal
	perWeek     float64
	supplier    int // index into shop.suppliers
	unit        string
}

type person struct {
	id    int64
	login shops.Login
}

// shop is what the generators know about the shop they're building.
type shop struct {
	p         shops.Profile
	items     []item
	suppliers []int64
	customers []int64
	safe      int64
	card      int64 // the card machine's account
	bank      int64 // the shop's current account
	owner     person
	cashiers  []person // the shown cashier first, then staff
}

func linesFor(catalogue string) (lines string, sups []string) {
	pick := map[string]string{"grocery": groceryLines, "pharmacy": pharmacyLines, "phone": phoneLines, "hardware": hardwareLines}
	if l, ok := pick[catalogue]; ok {
		return l, strings.Split(supplierLines[catalogue], "\n")
	}
	for _, k := range []string{"grocery", "pharmacy", "phone", "hardware"} { // mixed: everything
		lines += pick[k]
		sups = append(sups, strings.Split(supplierLines[k], "\n")...)
	}
	return lines, sups
}

// catalogue creates categories, products, suppliers, customers and the safe.
func catalogue(ctx context.Context, d Deps, p shops.Profile) (*shop, error) {
	s := &shop{p: p}
	lines, sups := linesFor(p.Catalogue)
	for _, l := range sups {
		f := strings.Split(l, ";")
		days, _ := strconv.Atoi(f[3])
		sup, err := suppliers.NewService(d.DB).Create(ctx, suppliers.CreateInput{Name: f[0], ContactPerson: &f[1], Phone: &f[2], CreditDays: days})
		if err != nil {
			return nil, fmt.Errorf("supplier %s: %w", f[0], err)
		}
		s.suppliers = append(s.suppliers, sup.ID)
	}
	units := map[string]int64{}
	var ur []struct {
		ID   int64  `db:"id"`
		Abbr string `db:"abbreviation"`
	}
	if err := d.DB.SelectContext(ctx, &ur, `SELECT id, abbreviation FROM units`); err != nil {
		return nil, err
	}
	for _, u := range ur {
		units[u.Abbr] = u.ID
	}
	cats := map[string]int64{}
	catSvc, prodSvc := categories.NewService(d.DB), products.NewService(d.DB)
	for _, l := range strings.Split(strings.TrimSpace(lines), "\n") {
		if strings.TrimSpace(l) == "" {
			continue // the mixed catalogue joins several lists
		}
		f := strings.Split(l, ";")
		if _, ok := cats[f[0]]; !ok {
			c, err := catSvc.Create(ctx, categories.CreateInput{Name: f[0]})
			if err != nil {
				return nil, fmt.Errorf("category %s: %w", f[0], err)
			}
			cats[f[0]] = c.ID
		}
		in := products.CreateInput{Name: f[1], CategoryID: cats[f[0]], UnitID: units[f[3]], CostPrice: f[4], SellingPrice: f[5]}
		if f[2] != "" {
			in.NameLocal = &f[2]
		}
		bc := ean13(fmt.Sprintf("479%09d", 100000+len(s.items)*7)) // 479 = Sri Lanka's GS1 prefix
		in.Barcode = &bc
		pr, err := prodSvc.Create(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("product %s: %w", f[1], err)
		}
		pw, _ := strconv.ParseFloat(f[6], 64)
		s.items = append(s.items, item{id: pr.ID, name: f[1], cost: decimal.RequireFromString(f[4]),
			price: decimal.RequireFromString(f[5]), perWeek: pw, supplier: (len(cats) - 1) % len(s.suppliers), unit: f[3]})
	}
	for _, l := range strings.Split(customerLines, "\n") {
		f := strings.Split(l, ";")
		in := customers.CreateInput{Name: f[0], Phone: &f[1], CreditLimit: f[3]}
		if f[2] != "" {
			in.Address = &f[2]
		}
		c, _, err := customers.NewService(d.DB).Create(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("customer %s: %w", f[0], err)
		}
		s.customers = append(s.customers, c.ID)
	}
	if err := s.loadPeople(ctx, d); err != nil {
		return nil, err
	}
	safe, err := lockers.NewService(d.DB).Create(ctx, lockers.CreateInput{Name: "Shop Safe", Kind: lockers.KindSafe}, s.owner.id)
	if err != nil {
		return nil, fmt.Errorf("safe: %w", err)
	}
	s.safe = safe.ID
	card, err := lockers.NewService(d.DB).Create(ctx, lockers.CreateInput{Name: "Card machine (BOC)", Kind: lockers.KindCard}, s.owner.id)
	if err != nil {
		return nil, fmt.Errorf("card account: %w", err)
	}
	s.card = card.ID
	bank, err := lockers.NewService(d.DB).Create(ctx, lockers.CreateInput{Name: "Commercial Bank — current", Kind: lockers.KindBank, OpeningBalance: "3500000"}, s.owner.id)
	if err != nil {
		return nil, fmt.Errorf("bank account: %w", err)
	}
	s.bank = bank.ID
	return s, nil
}

func (s *shop) loadPeople(ctx context.Context, d Deps) error {
	id := func(l shops.Login) (person, error) {
		var p person
		p.login = l
		err := d.DB.GetContext(ctx, &p.id, `SELECT id FROM users WHERE phone = $1`, l.Phone)
		return p, err
	}
	var err error
	if s.owner, err = id(s.p.Logins[0]); err != nil {
		return err
	}
	for _, l := range append([]shops.Login{s.p.Logins[2]}, s.p.Staff...) {
		c, err := id(l)
		if err != nil {
			return err
		}
		s.cashiers = append(s.cashiers, c)
	}
	return nil
}

// ean13 adds the check digit to 12 digits.
func ean13(d12 string) string {
	sum := 0
	for i, ch := range d12 {
		n := int(ch - '0')
		if i%2 == 1 {
			n *= 3
		}
		sum += n
	}
	return d12 + strconv.Itoa((10-sum%10)%10)
}
