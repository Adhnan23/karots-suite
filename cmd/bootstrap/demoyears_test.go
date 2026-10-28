package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestYearsStampFollowsOnlyWhatShapesTheData(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "migrations", "00001.sql"), "a")
	writeFile(t, filepath.Join(root, "internal", "demo", "gen", "gen.go"), "a")
	stamp := func() string {
		s, err := yearsStamp(root)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	first := stamp()
	writeFile(t, filepath.Join(root, "web", "src", "x.ts"), "a")                   // the screens
	writeFile(t, filepath.Join(root, "cmd", "demo", "launcher.go"), "a")           // the launcher
	writeFile(t, filepath.Join(root, "internal", "demo", "gen", "x_test.go"), "a") // a test
	if stamp() != first {
		t.Fatal("stamp moved for a change that doesn't touch the data")
	}
	writeFile(t, filepath.Join(root, "migrations", "00001.sql"), "b")
	if stamp() == first {
		t.Fatal("stamp ignored a migration change")
	}
}

func TestYearsCacheRoundTrip(t *testing.T) {
	cache, tpl := t.TempDir(), t.TempDir()
	keys := []string{"a", "b"}
	if takeYears(cache, "s1", tpl, keys) {
		t.Fatal("empty cache used")
	}
	writeFile(t, filepath.Join(tpl, "a.ksb"), "A")
	writeFile(t, filepath.Join(tpl, "b.ksb"), "B")
	if err := keepYears(cache, "s1", tpl, keys); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(tpl, "a.ksb"))
	os.Remove(filepath.Join(tpl, "b.ksb"))
	if takeYears(cache, "s2", tpl, keys) {
		t.Fatal("another stamp's years used")
	}
	if !takeYears(cache, "s1", tpl, keys) {
		t.Fatal("cached years not used")
	}
	if b, _ := os.ReadFile(filepath.Join(tpl, "b.ksb")); string(b) != "B" {
		t.Fatalf("b.ksb = %q", b)
	}
	if err := keepYears(cache, "s2", tpl, keys); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cache, "s1")); !os.IsNotExist(err) {
		t.Fatal("old stamp's years kept")
	}
}
