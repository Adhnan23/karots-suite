package plugin

import (
	"context"
	"testing"
)

func TestOfflineDataCollects(t *testing.T) {
	t.Cleanup(ResetOfflineDataForTest)
	r := &Registry{}
	r.AddOfflineData(OfflineData{Key: "a", Build: func(context.Context) (any, error) { return 1, nil }})
	r.AddOfflineData(OfflineData{Key: "b", Build: func(context.Context) (any, error) { return 2, nil }})
	all := OfflineDataAll()
	if len(all) != 2 || all[0].Key != "a" || all[1].Key != "b" {
		t.Fatalf("%+v", all)
	}
}
