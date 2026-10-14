package pharmacy

import (
	"context"
	"testing"
)

// The Doses list starts with the usual doses, named for the label; a dose used
// on a prescription or as a usual dose joins it by itself (with its doses a
// day, unnamed until the shop names it); most used first. Instructions are
// remembered the same way, on top of the common ones.
func TestDosesAndUsualDose(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ds, err := f.st.Doses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byDose := func(ds []Dose) map[string]Dose {
		m := map[string]Dose{}
		for _, d := range ds {
			m[d.Dose] = d
		}
		return m
	}
	m := byDose(ds)
	if d := m["1-0-1"]; d.Words != "Morning · Night" || !d.PerDay.Equal(dec(2)) {
		t.Fatalf("seeded 1-0-1: %+v", d)
	}
	if d := m["sos"]; d.Words == "" || !d.PerDay.IsZero() {
		t.Fatalf("seeded sos (when needed, no days): %+v", d)
	}

	p, _ := f.st.Register(ctx, "Dose P", "0718888888", Profile{})
	g := f.group
	for range 2 {
		if _, err := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{Items: []ItemInput{{GroupID: &g, Dosage: "1 every 8 hours", Quantity: dec(9), PerDay: dec(3), Note: "Empty stomach, 30 min before food"}}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{Items: []ItemInput{{GroupID: &g, Dosage: "1-0-1 ", Quantity: dec(9)}}}); err != nil {
		t.Fatal(err)
	}
	ds, _ = f.st.Doses(ctx)
	m = byDose(ds)
	if d := m["1 every 8 hours"]; d.Words != "" || !d.PerDay.Equal(dec(3)) || d.Uses != 2 || d.ID == 0 {
		t.Fatalf("learnt dose: %+v", d)
	}
	if ds[0].Dose != "1 every 8 hours" {
		t.Fatalf("most used first: %+v", ds[:2])
	}
	if len(ds) != len(m) {
		t.Fatal("a dose listed twice (1-0-1 with a trailing space)")
	}
	// Name it; edit; delete.
	id := m["1 every 8 hours"].ID
	if err := f.st.SaveDose(ctx, id, DoseInput{Dose: "1 every 8 hours", Words: "Every 8 hours", PerDay: dec(3)}); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SaveDose(ctx, 0, DoseInput{Dose: "1-0-1", Words: "x", PerDay: dec(2)}); err == nil {
		t.Fatal("a second 1-0-1 must be refused")
	}
	nid, err := f.st.CreateDose(ctx, DoseInput{Dose: "2 at night", Words: "2 at night", PerDay: dec(2)})
	if err != nil || nid == 0 {
		t.Fatal(err)
	}
	if err := f.st.DeleteDose(ctx, nid); err != nil {
		t.Fatal(err)
	}
	if m = byDose(must(f.st.Doses(ctx))); m["1 every 8 hours"].Words != "Every 8 hours" || m["2 at night"].ID != 0 {
		t.Fatalf("after edits: %+v", m)
	}

	// Usual dose + instruction on a product; both learnt; the medicine search carries them.
	if err := f.st.SetFlags(ctx, f.metfor, MedicineFlags{Dose: "1-1-1", Note: "After meals"}); err != nil {
		t.Fatal(err)
	}
	ms, _ := f.st.Medicines(ctx, "metf")
	if len(ms) != 2 || ms[0].Kind != "group" || ms[0].Dose != "1-1-1" || ms[1].Note != "After meals" {
		t.Fatalf("medicines: %+v", ms)
	}
	notes, _ := f.st.Notes(ctx)
	if notes[0] != "Empty stomach, 30 min before food" {
		t.Fatalf("most used instruction first: %+v", notes)
	}
	seen := map[string]bool{}
	for _, n := range notes {
		if seen[n] {
			t.Fatalf("%q twice", n)
		}
		seen[n] = true
	}
	if !seen["After meals"] || !seen["Finish the full course"] {
		t.Fatalf("common instructions: %+v", notes)
	}
	// Controlled ⇒ prescription-only.
	if err := f.st.SetFlags(ctx, f.pana, MedicineFlags{Controlled: true}); err != nil {
		t.Fatal(err)
	}
	if flags, _ := f.st.Flags(ctx, []int64{f.pana}); !flags[f.pana].RxOnly {
		t.Fatalf("controlled must be Rx: %+v", flags[f.pana])
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
