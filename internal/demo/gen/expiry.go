package gen

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"karots-suite/internal/features/purchases"
)

// shelfMonths is how long a delivery of each category lasts; a category not here
// (tools, phones, vegetables) has no date.
var shelfMonths = map[string]int{
	"Antibiotics": 18, "Cold & Allergy": 20, "Stomach": 20, "Pain & Fever": 24, "Heart & BP": 20, "Diabetes": 18,
	"Controlled": 18, "Vitamins": 16, "Skin Care": 18, "First Aid": 24, "Mother & Baby": 12,
	"Dairy": 4, "Bakery": 1, "Groceries": 9, "Rice & Grains": 12, "Beverages": 9, "Tea & Coffee": 18, "Spices": 12, "Snacks": 6,
}

// A date is written from the real clock, not the day being built: the time machine
// moves it back with the delivery it came on.
func (g *gen) shelfLife(it item) string {
	m, ok := shelfMonths[it.cat]
	if !ok {
		return ""
	}
	return time.Now().AddDate(0, m, g.r.IntN(25)).Format(time.DateOnly)
}

// shortDatedAgo is the day, counted back from today, of the short-dated delivery.
const shortDatedAgo = 14

// shortDatedDays is where each of its lots stands today: the slowest sellers, two
// already past their date and the rest coming up to it.
var shortDatedDays = []int{-3, -8, 5, 11, 18, 26}

// shortDated receives slow sellers on short dates, which is how a shop ends up with
// something to write off or give back. Enough of each to outlast those days, since
// the earliest date sells first.
func (g *gen) shortDated(day time.Time) error {
	var slow []item
	for i, it := range g.s.items {
		if _, loose := g.s.bagFor(i); !loose && !it.service && shelfMonths[it.cat] > 0 {
			slow = append(slow, it)
		}
	}
	slices.SortFunc(slow, func(a, b item) int {
		if w := g.weekly(a) - g.weekly(b); w != 0 {
			return int(math.Copysign(1, w))
		}
		return strings.Compare(a.name, b.name)
	})
	bySup := map[int][]purchases.ItemInput{}
	for i, it := range slow[:min(len(slow), len(shortDatedDays))] {
		qty := math.Ceil(g.weekly(it)*6) + 10
		bySup[it.supplier] = append(bySup[it.supplier], purchases.ItemInput{
			ProductID: it.id, Quantity: fmt.Sprint(qty), CostPrice: it.cost.String(), SellingPrice: it.price.String(),
			ExpiryDate: time.Now().AddDate(0, 0, shortDatedAgo+shortDatedDays[i]).Format(time.DateOnly),
		})
		g.stock[it.id] += qty
	}
	if len(bySup) == 0 {
		return nil
	}
	return g.deliver(day, "SD", bySup)
}
