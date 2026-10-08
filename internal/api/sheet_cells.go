package api

import (
	"errors"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

// currencyLabel is a leading "Rs.", "LKR", "රු." or "$" on a typed amount.
var currencyLabel = regexp.MustCompile(`^(?:[\p{L}\p{M}]+\.?|[$€£₹])\s*`)

var errCell = errors.New("not a number")

// amountCell reads a money / quantity cell as people type it in a sheet:
// "1,250.00", "Rs. 1250", " 99 ". Blank is zero. Anything else is an error —
// never a silent 0 (karots-pos stored every "1,250.00" price as 0).
func amountCell(s string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, nil
	}
	s = currencyLabel.ReplaceAllString(s, "")
	s = strings.NewReplacer(",", "", " ", "", " ", "").Replace(s)
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, errCell
	}
	return d, nil
}

// wholeCell reads a whole-number cell ("5", "5.0", "1,000"); blank is zero.
func wholeCell(s string) (int, error) {
	d, err := amountCell(s)
	if err != nil || !d.Equal(d.Truncate(0)) {
		return 0, errCell
	}
	return int(d.IntPart()), nil
}

// yesCell reads a yes / no cell.
func yesCell(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y":
		return true
	}
	return false
}
