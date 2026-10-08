package api

import "testing"

func TestAmountCell(t *testing.T) {
	for in, want := range map[string]string{
		"": "0", "1250": "1250", "1,250.00": "1250", " Rs. 1,250.50 ": "1250.5", "LKR 99": "99",
		"රු.40": "40", "$3.5": "3.5", "-12": "-12", "0.75": "0.75", "1 000": "1000",
	} {
		d, err := amountCell(in)
		if err != nil || d.String() != want {
			t.Errorf("amountCell(%q) = %s, %v; want %s", in, d, err, want)
		}
	}
	for _, bad := range []string{"abc", "12abc", "1.2.3", "--5"} {
		if _, err := amountCell(bad); err == nil {
			t.Errorf("amountCell(%q) accepted", bad)
		}
	}
	if n, err := wholeCell("5.0"); err != nil || n != 5 {
		t.Errorf("wholeCell(5.0) = %d %v", n, err)
	}
	if _, err := wholeCell("2.5"); err == nil {
		t.Error("wholeCell(2.5) accepted")
	}
}
