package repairs

import (
	"fmt"
	"testing"
)

// ?data=1 answers the job slip's print data for its React template (Image mode).
func TestSlipData(t *testing.T) {
	f := newFixture(t)
	j := decode[JobDTO](t, f.do("POST", "/jobs", fmt.Sprintf(`{"customerId":%d,"deviceModel":"Galaxy A12","fault":"Cracked"}`, f.cust)))
	path := fmt.Sprintf("/jobs/%d", j.ID)
	decode[JobDTO](t, f.do("POST", path+"/parts", fmt.Sprintf(`{"productId":%d,"qty":"1"}`, f.part)))
	decode[JobDTO](t, f.do("POST", path+"/charges", `{"label":"Fitting","amount":"1500"}`))

	d := decode[slipData](t, f.do("POST", path+"/print?data=1", ""))
	if d.Kind != "repairs.job" || d.Dots != 576 || d.Columns != 48 || d.Shop.Name == "" {
		t.Fatalf("slip data: %+v", d)
	}
	if s := d.Job; s.TicketNo != "R-0001" || !s.Open || s.Customer != "Kamal Silva" || s.Device != "Galaxy A12" || s.Fault != "Cracked" ||
		len(s.Parts) != 1 || s.Parts[0].Name != "A12 screen" || s.Parts[0].Qty != "1" || len(s.Charges) != 1 || s.Charges[0] != "Fitting" || s.Estimate != "5500.00" {
		t.Fatalf("job: %+v", s)
	}
	if d58 := decode[slipData](t, f.do("POST", path+"/print?data=1&size=58", "")); d58.Dots != 384 || d58.Width != "58" {
		t.Fatalf("size=58: %+v", d58)
	}
}
