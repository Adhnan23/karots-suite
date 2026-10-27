package settings

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// LabelStyles is the Image-mode label style per label kind ("price",
// "<plugin>.<kind>", …), stored as JSONB. Kinds and styles are open strings:
// plugins add kinds, and the SPA reads an unknown style as classic.
type LabelStyles map[string]string

func (l *LabelStyles) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	case nil:
		*l = LabelStyles{}
		return nil
	default:
		return fmt.Errorf("label_styles: unexpected %T", src)
	}
	m := LabelStyles{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	*l = m
	return nil
}

func (l LabelStyles) Value() (driver.Value, error) {
	if l == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]string(l))
}

// maxLabelKinds bounds the map so a junk request can't grow the settings row.
const maxLabelKinds = 32

// ValidLabelStyles trims keys and values and drops blank ones (a blank style
// means "back to classic"), keeping at most 32 kinds.
func ValidLabelStyles(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "" || v == "" || len(k) > 64 || len(v) > 32 || len(out) >= maxLabelKinds {
			continue
		}
		out[k] = v
	}
	return out
}

// MergeLabelStyles applies a change to the saved styles: a kind with a blank
// style is removed, the others are set. nil change = keep everything.
func MergeLabelStyles(cur LabelStyles, change map[string]string) LabelStyles {
	out := LabelStyles{}
	for k, v := range cur {
		out[k] = v
	}
	for k, v := range change {
		k = strings.TrimSpace(k)
		if strings.TrimSpace(v) == "" {
			delete(out, k)
		}
	}
	for k, v := range ValidLabelStyles(change) {
		if len(out) < maxLabelKinds || out[k] != "" {
			out[k] = v
		}
	}
	return out
}
