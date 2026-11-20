package cashregister

import (
	"encoding/json"
	"testing"
)

// DRAW-17: a counted drawer adds up to the amount recorded — including the odd
// counts a till can send (blank, broken, zero or nonsense rows).
func TestDRAW17_AmountFrom(t *testing.T) {
	for _, c := range []struct {
		name, typed, breakdown string
		want, raw              string
		ok                     bool
	}{
		{"notes counted", "", `{"1000":2,"20":3,"0.50":4}`, "2062", `{"0.50":4,"1000":2,"20":3}`, true},
		{"the count wins over a typed total", "999", `{"100":1}`, "100", `{"100":1}`, true},
		{"zero, negative and nonsense rows are dropped", "", `{"1000":1,"500":0,"100":-3,"abc":5,"-50":2,"0":9}`, "1000", `{"1000":1}`, true},
		{"a coin under a cent isn't money the drawer can hold", "", `{"100":1,"0.005":3}`, "100", `{"100":1}`, true},
		{"an all-zero count is an empty drawer", "", `{"1000":0}`, "0", `{}`, true},
		{"an empty map is an empty drawer", "", `{}`, "0", `{}`, true},
		{"a broken count falls back to the typed total", "250.50", `{"1000":"two"}`, "250.5", "", true},
		{"no count: the typed total", "75", "", "75", "", true},
		{"nothing at all is refused", "  ", "", "0", "", false},
		{"a typed total that isn't money is refused", "lots", "", "0", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, raw, ok := amountFrom(c.typed, json.RawMessage(c.breakdown))
			if ok != c.ok || got.String() != c.want || string(raw) != c.raw {
				t.Errorf("amountFrom(%q, %s) = %s, %s, %v; want %s, %s, %v", c.typed, c.breakdown, got, raw, ok, c.want, c.raw, c.ok)
			}
		})
	}
}
