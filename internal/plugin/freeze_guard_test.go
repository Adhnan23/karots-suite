package plugin

import (
	"context"
	"testing"
)

// Every plugin's refusals are merged; a product nobody refuses is free to freeze.
func TestFreezeRefusals(t *testing.T) {
	saved := freezeGuards
	t.Cleanup(func() { freezeGuards = saved })
	freezeGuards = hook[FreezeGuard]{}
	r := &Registry{}
	r.AddFreezeGuard(FreezeGuard{Refuse: func(_ context.Context, ids []int64) (map[int64]string, error) {
		return map[int64]string{1: "one"}, nil
	}})
	r.AddFreezeGuard(FreezeGuard{Refuse: func(_ context.Context, ids []int64) (map[int64]string, error) {
		return map[int64]string{2: "two"}, nil
	}})
	got, err := FreezeRefusals(context.Background(), []int64{1, 2, 3})
	if err != nil || len(got) != 2 || got[1] != "one" || got[2] != "two" {
		t.Fatalf("%v %v", got, err)
	}
}
