package battle

import (
	"bytes"
	"encoding/csv"
	"testing"

	"karots-suite/internal/ledgercheck/ltest"
)

// IMP-1, 4 with a plugin's columns: a products sheet preview lists a custom
// field's change old → new and keeps nothing — the field's value is as before.
func TestIMPPreviewWithCustomFields(t *testing.T) {
	s := ltest.NewShop(t)
	g := field(t, s, map[string]any{"label": "Generic Name", "type": "text"})
	p := s.Product(t, "Imp Pp Panadol", "50", "0", nil)
	fill(t, s, cell(p, g.ID, "paracetamol 500"))
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	w.WriteAll([][]string{{"name", "Generic Name"}, {"Imp Pp Panadol", "Paracetamol"}}) //nolint:errcheck
	var r struct {
		Applied bool
		Changes []struct {
			Product string
			Fields  []struct{ Field, From, To string }
		}
	}
	before := ltest.State(t, s.DB)
	s.Must(t, s.Admin.Upload("/api/v1/admin/products/import?dry=1&extras=productplus", "p.csv", b.Bytes(), &r))
	if changed := ltest.Changed(before, ltest.State(t, s.DB)); len(changed) != 0 {
		t.Fatalf("IMP-1: the preview changed %v", changed)
	}
	if r.Applied || len(r.Changes) != 1 || len(r.Changes[0].Fields) != 1 {
		t.Fatalf("IMP-4: %+v", r)
	}
	if f := r.Changes[0].Fields[0]; f.Field != "Generic Name" || f.From != "paracetamol 500" || f.To != "Paracetamol" {
		t.Fatalf("IMP-4: custom field change %+v", f)
	}
	if v, _ := value(t, s, g.ID, p); v != "paracetamol 500" {
		t.Fatalf("IMP-1: the preview kept %q", v)
	}
}
