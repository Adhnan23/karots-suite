package main

import (
	"context"
	"testing"

	"karots-suite/internal/config"
)

// A Docker / VPS container is updated by rebuilding its image: SELF_UPDATE=off
// keeps the updater (and its boot guard) off even though the binary has a channel.
func TestSelfUpdateOff(t *testing.T) {
	defer func(c, k string) { updateChannel, updatePubKey = c, k }(updateChannel, updatePubKey)
	updateChannel, updatePubKey = "c0ffee", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	t.Setenv("SELF_UPDATE", "off")
	if u := startUpdater(context.Background(), nil, &config.Config{Version: "2026.10.19.1530"}, make(chan struct{}, 1)); u != nil {
		t.Fatal("SELF_UPDATE=off must leave the updater off")
	}
	if bootGuard() {
		t.Fatal("no boot guard with SELF_UPDATE=off")
	}
}
