package cashregister

import (
	"encoding/json"
	"strings"

	"karots-suite/internal/money"

	"github.com/shopspring/decimal"
)

// amountFrom resolves the drawer amount: prefer the denomination breakdown (the
// authoritative count) and fall back to a typed total. Returns the canonical
// breakdown JSON to persist (nil when none was supplied).
func amountFrom(typed string, breakdown json.RawMessage) (decimal.Decimal, []byte, bool) {
	if total, raw, ok := breakdownTotal(breakdown); ok {
		return total, raw, true
	}
	if strings.TrimSpace(typed) == "" {
		return decimal.Zero, nil, false
	}
	v, err := money.Parse(typed)
	if err != nil {
		return decimal.Zero, nil, false
	}
	return v, nil, true
}

// breakdownTotal sums value×qty over a {denominationValue: count} map.
func breakdownTotal(raw json.RawMessage) (decimal.Decimal, []byte, bool) {
	if len(raw) == 0 {
		return decimal.Zero, nil, false
	}
	m := map[string]int{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return decimal.Zero, nil, false
	}
	total := decimal.Zero
	any := false
	clean := map[string]int{}
	for k, qty := range m {
		if qty <= 0 {
			continue
		}
		v, err := decimal.NewFromString(k)
		if err != nil || !v.IsPositive() {
			continue
		}
		total = total.Add(v.Mul(decimal.NewFromInt(int64(qty))))
		clean[k] = qty
		any = true
	}
	if !any {
		// An all-zero count is still a valid (empty) drawer.
		return decimal.Zero, []byte("{}"), true
	}
	out, _ := json.Marshal(clean)
	return total, out, true
}

// nullJSON is also used by repository.go, so it moves here with the two
// helpers above rather than being lost when api.go is deleted.
func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
