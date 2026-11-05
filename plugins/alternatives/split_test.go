package alternatives

import (
	"slices"
	"testing"
)

func TestSplitOrder(t *testing.T) {
	for _, c := range []struct {
		total   int
		weights []int64
		want    []int
	}{
		{100, []int64{70, 20, 10}, []int{70, 20, 10}},
		{10, []int64{1, 1, 1}, []int{4, 3, 3}},
		{5, []int64{0, 0}, []int{3, 2}},
		{0, []int64{3, 1}, []int{0, 0}},
		{-4, []int64{3, 1}, []int{0, 0}},
		{7, []int64{5}, []int{7}},
		{88, []int64{7, 2, 1}, []int{62, 17, 9}},
		{1, []int64{1, 9}, []int{0, 1}},
		{3, []int64{0, 5}, []int{0, 3}},
	} {
		got := splitOrder(c.total, c.weights)
		if !slices.Equal(got, c.want) {
			t.Errorf("%d %v: got %v, want %v", c.total, c.weights, got, c.want)
		}
	}
	if got := splitOrder(5, nil); len(got) != 0 {
		t.Errorf("no members: %v", got)
	}
	// Whatever the numbers, the parts add up to the total.
	for total := 0; total < 200; total += 7 {
		sum := 0
		for _, n := range splitOrder(total, []int64{13, 0, 7, 1, 29}) {
			sum += n
		}
		if sum != total {
			t.Errorf("%d: sums to %d", total, sum)
		}
	}
}

func TestFasterBrand(t *testing.T) {
	manual := map[int64]int{1: 50, 2: 50}
	if id, ok := fasterBrand(manual, map[int64]int64{1: 80, 2: 20}); !ok || id != 1 {
		t.Errorf("80%% against 50%%: %d %v", id, ok)
	}
	if _, ok := fasterBrand(manual, map[int64]int64{1: 60, 2: 40}); ok {
		t.Error("10 points isn't enough")
	}
	if _, ok := fasterBrand(manual, map[int64]int64{1: 8, 2: 1}); ok {
		t.Error("nine sales say nothing")
	}
	if id, ok := fasterBrand(map[int64]int{1: 100}, map[int64]int64{1: 50, 2: 50}); !ok || id != 2 {
		t.Errorf("a brand with no share that sells: %d %v", id, ok)
	}
	if _, ok := fasterBrand(manual, nil); ok {
		t.Error("no sales")
	}
}
