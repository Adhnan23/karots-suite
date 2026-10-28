package main

import (
	"bytes"
	"errors"
	"io"
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

func TestFileModeCheck(t *testing.T) {
	ok := buildOpts{OS: "windows", Arch: "amd64", Name: "shop", Mode: "file"}
	if err := ok.check(); err != nil {
		t.Fatal(err)
	}
	for _, o := range []buildOpts{
		{OS: "freebsd", Arch: "amd64", Name: "shop", Mode: "file"},
		{OS: "windows", Arch: "arm64", Name: "shop", Mode: "file"},
		{OS: "linux", Arch: "amd64", Name: "shop", Mode: "cloud"},
	} {
		if err := o.check(); err == nil {
			t.Errorf("%+v: expected a refusal", o)
		}
	}
	if o := (buildOpts{OS: "linux", Arch: "amd64", Name: "shop"}); o.check() != nil || o.Mode != "server" {
		t.Fatalf("no mode = server: %+v", o)
	}
}

func TestPlaceBundleCleansUp(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "src.txz"), filepath.Join(dir, "bundle", "postgres.txz")
	if err := os.WriteFile(src, []byte("pg"), 0o644); err != nil {
		t.Fatal(err)
	}
	cleanup, err := placeBundle(src, dst)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "pg" {
		t.Fatal("bundle not placed")
	}
	cleanup()
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("bundle left in the tree")
	}
}

func TestRenderDemoTSGatesEveryPlugin(t *testing.T) {
	sel := []manifest{{Key: "alpha"}, {Key: "beta"}}
	ts := string(renderDemoTS(sel))
	for _, want := range []string{
		`import { pluginOn } from './active'`,
		`import alpha from './alpha'`,
		`if (pluginOn('alpha')) alpha(slots)`,
		`if (pluginOn('beta')) beta(slots)`,
	} {
		if !strings.Contains(ts, want) {
			t.Fatalf("demo TS list lacks %q:\n%s", want, ts)
		}
	}
}

func TestWithDemoListRestoresCoreOnlyEvenOnFailure(t *testing.T) {
	dir := t.TempDir()
	goF, tsF := filepath.Join(dir, "enabled_plugins.go"), filepath.Join(dir, "enabled-plugins.ts")
	all := []manifest{{Key: "alpha", Import: "karots-suite/plugins/alpha"}}
	var during string
	err := withDemoList(goF, tsF, all, func() error {
		b, _ := os.ReadFile(tsF)
		during = string(b)
		return errors.New("build broke")
	})
	if err == nil || !strings.Contains(during, "if (pluginOn('alpha')) alpha(slots)") {
		t.Fatalf("err %v, list during the build:\n%s", err, during)
	}
	if s, _ := os.ReadFile(tsF); !bytes.Equal(s, renderTS(nil)) {
		t.Fatalf("TS list not core-only after a failed build:\n%s", s)
	}
	if g, _ := os.ReadFile(goF); !bytes.Equal(g, renderGo(nil)) {
		t.Fatalf("Go list not core-only after a failed build:\n%s", g)
	}
}

func TestDemoBuildRefusesWithoutTheSecret(t *testing.T) {
	t.Setenv("POS_SUPPORT_SECRET", "")
	err := buildDemo(nil, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "POS_SUPPORT_SECRET") {
		t.Fatalf("err = %v, want a refusal naming POS_SUPPORT_SECRET", err)
	}
}
