package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	sel := []manifest{{Key: "alpha", Name: "Alpha", Import: "karots-suite/plugins/alpha"}}
	if g := string(renderGo(sel)); !strings.Contains(g, `_ "karots-suite/plugins/alpha"`) {
		t.Fatalf("Go list:\n%s", g)
	}
	ts := string(renderTS(sel))
	if !strings.Contains(ts, `import alpha from './alpha'`) || !strings.Contains(ts, `alpha(slots)`) {
		t.Fatalf("TS list:\n%s", ts)
	}
	if g := string(renderGo(nil)); strings.Contains(g, "plugins/") || strings.Contains(g, "import") {
		t.Fatalf("core-only Go names a plugin:\n%s", g)
	}
	if ts := string(renderTS(nil)); strings.Contains(ts, "from './") {
		t.Fatalf("core-only TS imports a plugin:\n%s", ts)
	}
}

// A failed build still leaves both generated files core-only.
func TestWithSelectionRestoresOnFailure(t *testing.T) {
	dir := t.TempDir()
	goF, tsF := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.ts")
	sel := []manifest{{Key: "alpha", Import: "karots-suite/plugins/alpha"}}
	err := withSelection(goF, tsF, sel, func() error {
		if g, _ := os.ReadFile(goF); !bytes.Equal(g, renderGo(sel)) {
			t.Error("selection not written before the build")
		}
		return errors.New("build failed")
	})
	if err == nil || err.Error() != "build failed" {
		t.Fatalf("want the build error, got %v", err)
	}
	if g, _ := os.ReadFile(goF); !bytes.Equal(g, renderGo(nil)) {
		t.Fatal("Go list not restored to core-only")
	}
	if s, _ := os.ReadFile(tsF); !bytes.Equal(s, renderTS(nil)) {
		t.Fatal("TS list not restored to core-only")
	}
}

func TestSelect(t *testing.T) {
	all := []manifest{{Key: "alpha"}, {Key: "beta"}}
	for flag, want := range map[string]int{"all": 2, "none": 0, "beta": 1, "alpha, beta": 2} {
		got, err := selectPlugins(all, flag)
		if err != nil || len(got) != want {
			t.Errorf("%q: %d plugins, err %v; want %d", flag, len(got), err, want)
		}
	}
	if _, err := selectPlugins(all, "gamma"); err == nil {
		t.Error("an unknown key must be refused")
	}
	if _, err := selectPlugins(all, ""); err == nil {
		t.Error("no -plugins must be refused (say all or none)")
	}
}

// A plugin that needs others brings them along (pharmacy → productplus + alternatives).
func TestSelectRequires(t *testing.T) {
	all := []manifest{{Key: "alpha"}, {Key: "beta"}, {Key: "gamma", Requires: []string{"alpha"}}}
	got, err := selectPlugins(all, "gamma")
	if err != nil || names(got) != "alpha, gamma" {
		t.Fatalf("got %q, %v; want alpha, gamma", names(got), err)
	}
}
