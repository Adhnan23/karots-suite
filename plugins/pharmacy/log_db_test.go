package pharmacy

import (
	"context"
	"testing"
	"time"
)

// The call list: a repeat drug about to run out shows (with the patient's
// phone); Called hides it for 3 days; a stopped script never shows. The log
// lists a dispense with its items, flags a controlled one and a returned sale.
func TestRefillsAndLog(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	p, _ := f.st.Register(ctx, "Chandra", "0716666666", Profile{})
	g, pana := f.group, f.pana
	rx, _ := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{IsRepeat: true, PrescriberName: "Dr X", Items: []ItemInput{{GroupID: &g, Dosage: "1-0-1", Quantity: dec(20)}}})
	stop, _ := f.st.CreateScript(ctx, p.ID, 0, ScriptInput{IsRepeat: true, Items: []ItemInput{{ProductID: &pana, Dosage: "od", Quantity: dec(3)}}})
	f.st.SetFlags(ctx, f.metfor, MedicineFlags{Controlled: true}) //nolint:errcheck
	sale := f.sale(t, [2]int64{f.metfor, 20}, [2]int64{f.pana, 3})
	if _, err := f.st.RecordDispense(ctx, DispenseInput{SaleID: sale, PatientID: p.ID}); err != nil {
		t.Fatal(err)
	}
	f.st.SetScriptActive(ctx, stop, false) //nolint:errcheck
	// 20 tablets at 2 a day = 10 days; 7 days on, it runs out in 3 (inside the 5-day lead).
	now := time.Now().AddDate(0, 0, 7)
	due, err := f.st.RefillsDue(ctx, now)
	if err != nil || len(due) != 1 || due[0].RxID != rx || due[0].PatientPhone != "0716666666" || due[0].Drug != "Metformin 500" {
		t.Fatalf("due: %+v %v", due, err)
	}
	if n, _ := f.st.RefillsDue(ctx, time.Now()); len(n) != 0 {
		t.Fatalf("today nothing is due: %+v", n)
	}
	if err := f.st.MarkCalled(ctx, due[0].ID); err != nil {
		t.Fatal(err)
	}
	f.db.MustExec(`UPDATE pharmacy_prescription_items SET called_at = called_at + interval '6 days'`) // as if called on day 6
	if again, _ := f.st.RefillsDue(ctx, now); len(again) != 0 {
		t.Fatal("called yesterday: off the list")
	}
	if back, _ := f.st.RefillsDue(ctx, now.AddDate(0, 0, 3)); len(back) != 1 {
		t.Fatal("three days after the call it is back (a repeat stays due)")
	}

	f.db.MustExec(`UPDATE sale_items SET returned_qty = quantity WHERE sale_id = $1 AND product_id = $2`, sale, f.pana)
	log, err := f.st.Dispenses(ctx, LogFilter{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour)})
	if err != nil || len(log) != 1 || len(log[0].Items) != 2 || !log[0].Controlled || !log[0].Returned || log[0].Patient != "Chandra" {
		t.Fatalf("log: %+v %v", log, err)
	}
	if only, _ := f.st.Dispenses(ctx, LogFilter{From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), PatientID: p.ID + 1}); len(only) != 0 {
		t.Fatal("another patient's history is empty")
	}
}
