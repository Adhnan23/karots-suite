package pharmacy

import (
	"context"
	"testing"
)

// The register is core's stock movements: purchase in, sale out with the
// dispense's patient + doctor, a return back in; the running balance ends at
// today's stock. A controlled sale with no record is listed until one is added.
func TestControlledRegister(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	drug := f.pana
	if err := f.st.SetFlags(ctx, drug, MedicineFlags{Controlled: true, Schedule: "Schedule II", RxOnly: true}); err != nil {
		t.Fatal(err)
	}
	flags, _ := f.st.Flags(ctx, []int64{drug, f.metfor})
	if fl := flags[drug]; !fl.Controlled || fl.Schedule != "Schedule II" || !fl.RxOnly || !fl.Medicine || len(flags) != 2 {
		t.Fatalf("flags: %+v", flags)
	}
	// A grouped product is a medicine (Days… at the till) without being marked.
	if fl := flags[f.metfor]; !fl.Medicine || fl.Controlled || fl.RxOnly {
		t.Fatalf("grouped: %+v", fl)
	}
	if other, _ := f.st.Flags(ctx, []int64{f.gluco + 999}); len(other) != 0 {
		t.Fatalf("an unknown product has no flags: %+v", other)
	}
	p, _ := f.st.Register(ctx, "Saman", "0714444444", Profile{})
	move := func(typ string, qty int, ref string, refID int64) {
		f.db.MustExec(`INSERT INTO stock_movements (product_id, type, quantity, reference_type, reference_id, user_id)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, 0), (SELECT id FROM users LIMIT 1))`, drug, typ, qty, ref, refID)
	}
	// stock was 100 before (an opening count, no movement); +50 bought, 10 + 4 sold, 2 back.
	s1, s2 := f.sale(t, [2]int64{drug, 10}), f.sale(t, [2]int64{drug, 4})
	move("purchase", 50, "purchase", 7)
	move("sale", -10, "sale", s1)
	move("sale", -4, "sale", s2)
	move("return", 2, "sale", s1)
	f.db.MustExec(`UPDATE stock SET quantity = 138 WHERE product_id = $1`, drug)
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: s1, PatientID: p.ID, Prescriber: "Dr Silva"}); err != nil {
		t.Fatal(err)
	}
	un, _ := f.st.Unrecorded(ctx)
	if len(un) != 1 || un[0].SaleID != s2 || !un[0].Qty.Equal(dec(4)) {
		t.Fatalf("unrecorded: %+v", un)
	}
	r, err := f.st.RegisterFor(ctx, drug, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 4 || !r.Opening.Equal(dec(100)) || !r.Rows[3].Balance.Equal(dec(138)) {
		t.Fatalf("register: opening %s rows %+v", r.Opening, r.Rows)
	}
	if out := r.Rows[1]; out.Patient != "Saman" || out.Prescriber != "Dr Silva" || out.SaleID == nil || out.Reference == "" {
		t.Fatalf("sale row: %+v", out)
	}
	if r.Rows[2].Patient != "" {
		t.Fatal("the unrecorded sale names nobody")
	}
	list, _ := f.st.Controlled(ctx)
	if len(list) != 1 || list[0].Unrecorded != 1 {
		t.Fatalf("list: %+v", list)
	}
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: s2, PatientID: p.ID, Prescriber: "Dr Silva"}); err != nil {
		t.Fatal(err)
	}
	if un, _ = f.st.Unrecorded(ctx); len(un) != 0 {
		t.Fatalf("still unrecorded: %+v", un)
	}
	// Unmarking keeps the Rx flag and the history.
	if err := f.st.SetFlags(ctx, drug, MedicineFlags{RxOnly: true}); err != nil {
		t.Fatal(err)
	}
	if flags, _ = f.st.Flags(ctx, []int64{drug}); flags[drug].Controlled || !flags[drug].RxOnly {
		t.Fatalf("after unmark: %+v", flags)
	}
}
