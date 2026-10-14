package pharmacy

import (
	"context"
	"testing"
)

// Doses the shop has used are remembered with their doses a day (the most
// common one), most used first; the common Sri Lankan ones are always offered.
// A product's usual dose is one of them. Controlled is always prescription-only.
func TestDosesAndUsualDose(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _ := f.st.Register(ctx, "Dose P", "0718888888", Profile{})
	g := f.group
	for _, d := range []struct{ dose, perDay string }{{"1 every 8 hours", "3"}, {"1 every 8 hours", "3"}, {"1 every 8 hours", "2"}, {"1-0-1", ""}} {
		pd := dec(0)
		if d.perDay != "" {
			pd, _ = parseQty(d.perDay, "")
		}
		if _, err := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{Items: []ItemInput{{GroupID: &g, Dosage: d.dose, Quantity: dec(10), PerDay: pd}}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.st.SetFlags(ctx, f.pana, MedicineFlags{Dose: "0-0-1"}); err != nil {
		t.Fatal(err)
	}
	ds, err := f.st.Doses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ds[0].Dose != "1 every 8 hours" || !ds[0].PerDay.Equal(dec(3)) || ds[0].Uses != 3 {
		t.Fatalf("most used first, with its usual per day: %+v", ds[0])
	}
	seen := map[string]bool{}
	for _, d := range ds {
		if seen[d.Dose] {
			t.Fatalf("%q listed twice: %+v", d.Dose, ds)
		}
		seen[d.Dose] = true
	}
	for _, want := range []string{"1-0-1", "0-0-1", "tds", "1-1-1-1"} {
		if !seen[want] {
			t.Errorf("%q missing: %+v", want, ds)
		}
	}
	flags, _ := f.st.Flags(ctx, []int64{f.pana})
	if flags[f.pana].Dose != "0-0-1" || !flags[f.pana].Medicine {
		t.Fatalf("usual dose: %+v", flags[f.pana])
	}
	// Controlled ⇒ prescription-only, whatever was sent.
	if err := f.st.SetFlags(ctx, f.pana, MedicineFlags{Controlled: true}); err != nil {
		t.Fatal(err)
	}
	if flags, _ = f.st.Flags(ctx, []int64{f.pana}); !flags[f.pana].RxOnly {
		t.Fatalf("controlled must be Rx: %+v", flags[f.pana])
	}
	// The medicine search carries the usual dose — a group: its members'.
	f.st.SetFlags(ctx, f.metfor, MedicineFlags{Dose: "1-0-1"}) //nolint:errcheck
	ms, _ := f.st.Medicines(ctx, "metf")
	if len(ms) != 2 || ms[0].Kind != "group" || ms[0].Dose != "1-0-1" || ms[1].Dose != "1-0-1" {
		t.Fatalf("medicines: %+v", ms)
	}
}
