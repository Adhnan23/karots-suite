package pharmacy

import (
	"context"
	"testing"
)

// A controlled drug's register keeps a running balance: it can't be frozen.
func TestFreezeGuardRefusesControlled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.st.SetFlags(ctx, f.pana, MedicineFlags{Controlled: true, Schedule: "Schedule II", RxOnly: true}); err != nil {
		t.Fatal(err)
	}
	got, err := f.st.freezeRefusals(ctx, []int64{f.pana, f.metfor})
	if err != nil || len(got) != 1 || got[f.pana] != "a controlled drug: count it in full" {
		t.Fatalf("%v %v", got, err)
	}
}
