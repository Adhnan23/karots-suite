package shops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const root = "../../.."

func TestBuiltInsAreTheFive(t *testing.T) {
	want := map[string][]string{
		"grocery":  nil,
		"pharmacy": {"pharmacy", "alternatives", "productplus", "requests"},
		"phone":    {"repairs", "recharge", "combos", "documents"},
		"hardware": {"clearance", "productplus", "documents", "requests"},
	}
	got := BuiltIn()
	if len(got) != 5 {
		t.Fatalf("%d built-in shops, want 5", len(got))
	}
	for _, p := range got {
		if p.Key == "everything" {
			continue
		}
		w, ok := want[p.Key]
		if !ok || !slices.Equal(p.Plugins, w) {
			t.Errorf("%s plugins = %v, want %v", p.Key, p.Plugins, w)
		}
	}
}

func TestEverythingHasAll(t *testing.T) {
	p, _ := ByKey("everything")
	dirs, _ := filepath.Glob(filepath.Join(root, "plugins", "*", "plugin.json"))
	if len(p.Plugins) != len(dirs) {
		t.Fatalf("everything has %d plugins, %d are compiled", len(p.Plugins), len(dirs))
	}
}

func TestBuiltInsAreReal(t *testing.T) {
	for _, p := range append(BuiltIn(), Custom([]string{"repairs"}, "", "")) {
		if _, err := os.Stat(filepath.Join(root, "web/src/theme/palettes/dock", p.Palette+".css")); err != nil {
			t.Errorf("%s: palette %q doesn't exist", p.Key, p.Palette)
		}
		if _, err := os.Stat(filepath.Join(root, "web/src/features/print/styles", p.ReceiptStyle)); err != nil {
			t.Errorf("%s: receipt style %q doesn't exist", p.Key, p.ReceiptStyle)
		}
		for _, k := range p.Plugins {
			if _, err := os.Stat(filepath.Join(root, "plugins", k, "plugin.json")); err != nil {
				t.Errorf("%s: plugin %q isn't compiled", p.Key, k)
			}
		}
		if len(p.Logins) != 3 || len(p.Staff) < 2 || p.Volume <= 0 {
			t.Errorf("%s: logins %d, staff %d, volume %d", p.Key, len(p.Logins), len(p.Staff), p.Volume)
		}
	}
}

// The launcher hands a shop to its child as JSON: nothing it trades on may be lost.
func TestProfileSurvivesJSON(t *testing.T) {
	p, _ := ByKey("phone")
	b, _ := json.Marshal(p)
	var back Profile
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Volume != p.Volume || back.Catalogue != p.Catalogue || len(back.Plugins) != len(p.Plugins) || len(back.Logins) != 3 {
		t.Fatalf("after JSON: %+v", back)
	}
}
