package pharmacy

import (
	"context"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

func TestAttributesKeepTheOtherMarks(t *testing.T) {
	s, pid := attrEnv(t)
	ctx, set := context.Background(), s.attributes()
	noError(t, set.Set(ctx, pid, "dose", "1-0-1"))
	noError(t, set.Set(ctx, pid, "rx_only", "yes"))
	vals, err := set.Values(ctx, []int64{pid})
	noError(t, err)
	if vals[pid]["dose"] != "1-0-1" || vals[pid]["rx_only"] != "yes" || vals[pid]["controlled"] != "" {
		t.Fatalf("%+v", vals)
	}
	noError(t, set.Set(ctx, pid, "rx_only", "no"))
	noError(t, set.Set(ctx, pid, "controlled", "yes"))
	vals, _ = set.Values(ctx, []int64{pid})
	if vals[pid]["controlled"] != "yes" || vals[pid]["rx_only"] != "yes" || vals[pid]["dose"] != "1-0-1" {
		t.Fatalf("a controlled drug is prescription-only: %+v", vals)
	}
	noError(t, set.Set(ctx, pid, "controlled", ""))
	vals, _ = set.Values(ctx, []int64{pid})
	if vals[pid]["controlled"] != "" {
		t.Fatalf("%+v", vals)
	}
	if err := set.Set(ctx, pid, "nope", "x"); err == nil {
		t.Fatal("an unknown mark")
	}
}

func TestAttributesDoseList(t *testing.T) {
	s, pid := attrEnv(t)
	ctx, set := context.Background(), s.attributes()
	known, err := set.Known(ctx, "dose", 500)
	noError(t, err)
	found := false
	for _, k := range known {
		if k.Value == "1-0-1" && k.Meaning != "" && k.Extra["perDay"] == "2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%+v", known)
	}
	if k, err := set.Known(ctx, "rx_only", 5); err != nil || len(k) != 0 {
		t.Fatalf("%+v %v", k, err)
	}
	dose := "7-7-" + strings.ReplaceAll(t.Name(), "Test", "")[:3]
	noError(t, set.AddKnown(ctx, "dose", plugin.KnownValue{Value: dose, Meaning: "Test", Extra: map[string]string{"perDay": "2"}}))
	t.Cleanup(func() { _ = set.RemoveKnown(context.Background(), "dose", dose) })
	has := func() bool {
		ks, _ := set.Known(ctx, "dose", 1000)
		for _, k := range ks {
			if k.Value == dose {
				return true
			}
		}
		return false
	}
	if !has() {
		t.Fatal("not added")
	}
	noError(t, set.Set(ctx, pid, "dose", dose))
	noError(t, set.RemoveKnown(ctx, "dose", dose))
	if !has() {
		t.Fatal("removed a dose still in use")
	}
	noError(t, set.Set(ctx, pid, "dose", ""))
	noError(t, set.RemoveKnown(ctx, "dose", dose))
	if has() {
		t.Fatal("left an unused dose behind")
	}
}

func noError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func attrEnv(t *testing.T) (*Store, int64) {
	t.Helper()
	f := newFixture(t)
	return f.st, f.pana
}
