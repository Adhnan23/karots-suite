package settings

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"karots-suite/internal/apperr"
)

// SellLayout is how the Sell page looks on one screen size (Appearance, system
// user only): the arrangement of its three sections, the columns' shares of the
// width, the top part's share of a stacked column, the product card size and the
// text size in percent.
type SellLayout struct {
	Arr   string    `json:"arr"`
	Fr    []float64 `json:"fr"`
	Split float64   `json:"split"`
	Card  string    `json:"card"`
	Text  int       `json:"text"`
}

// SellLayouts is one layout per screen size, keyed "<w>x<h>" in CSS px; stored as JSONB.
type SellLayouts map[string]SellLayout

func (l *SellLayouts) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	case nil:
		*l = SellLayouts{}
		return nil
	default:
		return fmt.Errorf("sell_layouts: unexpected %T", src)
	}
	m := SellLayouts{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	*l = m
	return nil
}

func (l SellLayouts) Value() (driver.Value, error) {
	if l == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]SellLayout(l))
}

// SellArrangements: each id's columns, left to right; a column of two stacks
// them top to bottom. P products, C cart, Y payment. The SPA keeps the same list
// (web/src/features/till/layout/arrangements.ts).
var SellArrangements = map[string][]string{
	"PCY": {"P", "C", "Y"}, "CPY": {"C", "P", "Y"}, "PYC": {"P", "Y", "C"}, "CYP": {"C", "Y", "P"}, "YCP": {"Y", "C", "P"},
	"P_CY": {"P", "CY"}, "CY_P": {"CY", "P"}, "PY_C": {"PY", "C"}, "C_PY": {"C", "PY"}, "P_YC": {"P", "YC"},
}

// sellMin is each section's narrowest width in px; sellPad the page's padding on
// each side and sellGap the gap between columns.
var (
	sellMin  = map[rune]float64{'P': 300, 'C': 340, 'Y': 330}
	sellName = map[rune]string{'P': "Products", 'C': "Cart", 'Y': "Payment"}
)

const (
	sellPad      = 14
	sellGap      = 14
	maxSellSizes = 32
)

var sizeKeyRe = regexp.MustCompile(`^(\d{3,4})x(\d{3,4})$`)

// ValidSellLayouts checks every layout before it's saved: a real size, a known
// arrangement, one share per column (normalised to add up to one), no section
// below its narrowest width, a split of 0.2–0.8 on stacked ones, and known card
// and text sizes (blank = L, 0 = 100).
func ValidSellLayouts(in map[string]SellLayout) (SellLayouts, error) {
	if len(in) > maxSellSizes {
		return nil, apperr.BadRequest("too many screen sizes")
	}
	out := SellLayouts{}
	for key, l := range in {
		m := sizeKeyRe.FindStringSubmatch(key)
		if m == nil {
			return nil, apperr.BadRequest(fmt.Sprintf("%q isn't a screen size", key))
		}
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		if w < 300 || h < 300 {
			return nil, apperr.BadRequest(fmt.Sprintf("%q isn't a screen size", key))
		}
		cols, ok := SellArrangements[l.Arr]
		if !ok {
			return nil, apperr.BadRequest(fmt.Sprintf("unknown arrangement %q", l.Arr))
		}
		if len(l.Fr) != len(cols) {
			return nil, apperr.BadRequest(fmt.Sprintf("%d columns need %d shares", len(cols), len(cols)))
		}
		sum := 0.0
		for _, f := range l.Fr {
			if f <= 0 {
				return nil, apperr.BadRequest("every share must be above zero")
			}
			sum += f
		}
		fr := make([]float64, len(cols))
		inner := float64(w - 2*sellPad - sellGap*(len(cols)-1))
		stacked := false
		for i, col := range cols {
			fr[i] = l.Fr[i] / sum
			for _, a := range col {
				if px := fr[i] * inner; px < sellMin[a] {
					return nil, apperr.BadRequest(fmt.Sprintf("%s is too narrow at %d × %d (%.0f px, at least %.0f)", sellName[a], w, h, px, sellMin[a]))
				}
			}
			stacked = stacked || len(col) == 2
		}
		split := 0.0
		if stacked {
			if l.Split < 0.2 || l.Split > 0.8 {
				return nil, apperr.BadRequest("the split must be between 0.2 and 0.8")
			}
			split = l.Split
		}
		card := l.Card
		switch card {
		case "":
			card = "L"
		case "L", "M", "S", "T":
		default:
			return nil, apperr.BadRequest(fmt.Sprintf("unknown card size %q", l.Card))
		}
		text := l.Text
		switch text {
		case 0:
			text = 100
		case 90, 100, 110:
		default:
			return nil, apperr.BadRequest(fmt.Sprintf("unknown text size %d", l.Text))
		}
		out[key] = SellLayout{Arr: l.Arr, Fr: fr, Split: split, Card: card, Text: text}
	}
	return out, nil
}
