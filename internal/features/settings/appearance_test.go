package settings

import "testing"

func TestValidAppearanceFallsBack(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"unknown skin → studio", ValidSkin("nope"), "studio"},
		{"known skin passes", ValidSkin("night"), "night"},
		{"custom palette passes", ValidSkin("custom-12"), "custom-12"},
		{"junk custom → studio", ValidSkin("custom-x"), "studio"},
		{"new palettes pass", ValidSkin("lagoon") + ValidSkin("bazaar") + ValidSkin("catppuccin"), "lagoonbazaarcatppuccin"},
		{"blank skin → studio", ValidSkin(""), "studio"},
		{"old karots-pos skin restored → studio", ValidSkin("teal"), "studio"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q want %q", c.name, c.got, c.want)
		}
	}
}

func TestValidReceiptStyle(t *testing.T) {
	want := []string{"classic", "bold", "minimal", "ticket", "poster", "grid", "stamp", "elegant", "cards", "sections"}
	if len(ReceiptStyles) != len(want) {
		t.Fatalf("ReceiptStyles = %v", ReceiptStyles)
	}
	for i, k := range want {
		if ReceiptStyles[i] != k || ValidReceiptStyle(k) != k {
			t.Errorf("%q: order or passthrough broken (%v)", k, ReceiptStyles)
		}
	}
	for _, old := range []string{"modern", "boxed", "compact", "", "x"} {
		if got := ValidReceiptStyle(old); got != "classic" {
			t.Errorf("%q → %q, want classic", old, got)
		}
	}
}

func TestValidLabelStyles(t *testing.T) {
	got := ValidLabelStyles(map[string]string{" price ": " qrtag ", "": "x", "some.plugin": "", "other.plugin": "job"})
	if len(got) != 2 || got["price"] != "qrtag" || got["other.plugin"] != "job" {
		t.Fatalf("got %v", got)
	}
	big := map[string]string{}
	for i := range 40 {
		big[string(rune('a'+i%26))+string(rune('a'+i/26))] = "classic"
	}
	if n := len(ValidLabelStyles(big)); n != 32 {
		t.Fatalf("kept %d kinds, want 32", n)
	}
}
