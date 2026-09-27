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
		{"blank skin → studio", ValidSkin(""), "studio"},
		{"old karots-pos skin restored → studio", ValidSkin("teal"), "studio"},
		{"unknown density → comfortable", ValidDensity("huge"), "comfortable"},
		{"known density passes", ValidDensity("compact"), "compact"},
		{"blank receipt → classic", ValidReceiptStyle(""), "classic"},
		{"known receipt passes", ValidReceiptStyle("boxed"), "boxed"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %q want %q", c.name, c.got, c.want)
		}
	}
}
