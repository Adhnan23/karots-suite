package products

import (
	"context"
	"testing"
)

func TestDecorateTillLine(t *testing.T) {
	prev := TillLineProvider
	t.Cleanup(func() { TillLineProvider = prev })
	TillLineProvider = func(_ context.Context, ids []int64) map[int64]string {
		if len(ids) != 2 {
			t.Fatalf("ids %v", ids)
		}
		return map[int64]string{1: "Fits CT100"}
	}
	rows := []Product{{ID: 1}, {ID: 2}}
	Decorate(context.Background(), rows)
	if rows[0].TillLine != "Fits CT100" || rows[1].TillLine != "" {
		t.Fatalf("%+v", rows)
	}
}
