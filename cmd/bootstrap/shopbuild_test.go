package main

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"karots-suite/internal/shopconf"
)

// fakeBuild writes a dummy binary where build would, and remembers its opts.
func fakeBuild(got *buildOpts) func(buildOpts, io.Writer) (buildResult, error) {
	return func(o buildOpts, _ io.Writer) (buildResult, error) {
		*got = o
		bin := o.Name
		if o.OS == "windows" {
			bin += ".exe"
		}
		p := filepath.Join(o.Out, bin)
		return buildResult{Binary: p, InstallID: o.InstallID}, os.WriteFile(p, []byte("bin"), 0o755)
	}
}

func fakeSetup(osName, _, out string, _ io.Writer) error {
	name := "karots-setup"
	if osName == "windows" {
		name += ".exe"
	}
	return os.WriteFile(filepath.Join(out, name), []byte("setup"), 0o755)
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

func TestBuildShopThreeFiles(t *testing.T) {
	out := t.TempDir()
	s := mustShop(t, "al-raj", "windows", "server")
	os.MkdirAll(filepath.Join(out, "al-raj"), 0o755)
	os.WriteFile(filepath.Join(out, "al-raj", "stale.txt"), nil, 0o644)
	var got buildOpts
	if _, err := buildShop(&s, nil, shopBuildOpts{Out: out, Installer: true}, fakeBuild(&got), fakeSetup, io.Discard); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(out, "al-raj")
	if names := dirNames(t, dir); !slices.Equal(names, []string{"install.conf", "karots-setup.exe", "karots-suite.exe"}) {
		t.Fatalf("files %v", names)
	}
	c, err := shopconf.Read(filepath.Join(dir, "install.conf"))
	if err != nil || c.Binary != "karots-suite.exe" || c.InstallID != s.InstallID {
		t.Fatalf("install.conf %+v (%v)", c, err)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "install.conf")); fi.Mode().Perm() != 0o600 {
		t.Fatalf("install.conf mode %v", fi.Mode().Perm())
	}
	if s.LastBuilt == nil || s.LastBuilt.At.IsZero() {
		t.Fatal("lastBuilt not recorded")
	}
}

func TestBuildShopNoInstaller(t *testing.T) {
	out := t.TempDir()
	s := mustShop(t, "al-raj", "linux", "file")
	var got buildOpts
	if _, err := buildShop(&s, nil, shopBuildOpts{Out: out}, fakeBuild(&got), fakeSetup, io.Discard); err != nil {
		t.Fatal(err)
	}
	if names := dirNames(t, filepath.Join(out, "al-raj")); !slices.Equal(names, []string{"karots-suite"}) {
		t.Fatalf("files %v", names)
	}
}

func TestBuildShopPassesProfile(t *testing.T) {
	all := []manifest{{Key: "alpha", Requires: []string{"beta"}}, {Key: "beta"}}
	s := mustShop(t, "al-raj", "linux", "file")
	s.Plugins = []string{"alpha"}
	var got buildOpts
	if _, err := buildShop(&s, all, shopBuildOpts{Out: t.TempDir()}, fakeBuild(&got), fakeSetup, io.Discard); err != nil {
		t.Fatal(err)
	}
	if got.Mode != "file" || got.InstallID != s.InstallID || !got.RequireSecret || got.Name != "karots-suite" || names(got.Plugins) != "alpha, beta" {
		t.Fatalf("opts %+v", got)
	}
}

func TestBuildShopRefusesBadProfile(t *testing.T) {
	s := mustShop(t, "al-raj", "linux", "file")
	s.Plugins = []string{"gamma"}
	var got buildOpts
	if _, err := buildShop(&s, nil, shopBuildOpts{Out: t.TempDir()}, fakeBuild(&got), fakeSetup, io.Discard); err == nil {
		t.Fatal("built a shop with an unknown plugin")
	}
}

func TestBuildRequiresSecret(t *testing.T) {
	t.Setenv("POS_SUPPORT_SECRET", "")
	_, err := build(buildOpts{OS: "linux", Arch: "amd64", Name: "x", Out: t.TempDir(), RequireSecret: true}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "POS_SUPPORT_SECRET is not set") {
		t.Fatalf("err %v", err)
	}
}
