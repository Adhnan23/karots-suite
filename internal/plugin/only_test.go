package plugin

import (
	"io/fs"
	"testing"
)

type fake string

func (f fake) Name() string                { return string(f) }
func (f fake) Key() string                 { return string(f) }
func (f fake) Migrations() (fs.FS, string) { return nil, "" }
func (f fake) Setup(*Registry)             {}

func withFakes(t *testing.T, keys ...string) {
	t.Helper()
	saved := registered
	t.Cleanup(func() { registered = saved })
	registered = nil
	for _, k := range keys {
		Register(fake(k))
	}
}

func keysOf(ps []Plugin) []string {
	out := []string{}
	for _, p := range ps {
		out = append(out, p.Key())
	}
	return out
}

func TestOnlyKeepsListedInOrder(t *testing.T) {
	withFakes(t, "a", "b", "c")
	Only([]string{"c", "a"})
	if got := keysOf(All()); len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("All() = %v, want [a c]", got)
	}
}

func TestOnlyNilKeepsAll(t *testing.T) {
	withFakes(t, "a", "b")
	Only(nil)
	if got := keysOf(All()); len(got) != 2 {
		t.Fatalf("All() = %v, want [a b]", got)
	}
}

func TestOnlyEmptyKeepsNone(t *testing.T) {
	withFakes(t, "a", "b")
	Only([]string{})
	if got := All(); len(got) != 0 {
		t.Fatalf("All() = %v, want none", keysOf(got))
	}
}

func TestOnlyUnknownPanics(t *testing.T) {
	withFakes(t, "a")
	defer func() {
		if recover() == nil {
			t.Fatal("Only with an unknown key did not panic")
		}
	}()
	Only([]string{"zzz"})
}
