package main

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"testing"

	"karots-suite/internal/demo"
)

// The lock page is all a visitor sees before the code: it names whom to ask for one.
func TestInstallEndpointNamesWhomToAsk(t *testing.T) {
	l, _, _ := newTestLauncher(t, nil)
	w := httptest.NewRecorder()
	l.handler().ServeHTTP(w, httptest.NewRequest("GET", "/__demo/install", nil)) // no cookie: public
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var got struct {
		InstallID string `json:"installId"`
		Owner     Owner  `json:"owner"`
	}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.InstallID != "DEMO1234" || got.Owner.Phone != demo.OwnerPhone || got.Owner.Name != demo.OwnerName {
		t.Fatalf("got %+v", got)
	}
}

// On a server the Demo Room runs in a container behind a reverse proxy: it must listen beyond loopback when told.
func TestClaimPortListensOnTheGivenHost(t *testing.T) {
	ln, already := claimPort("0.0.0.0", freePort())
	if ln == nil {
		t.Fatalf("no listener (already %q)", already)
	}
	defer ln.Close()
	if ip := ln.Addr().(*net.TCPAddr).IP; !ip.IsUnspecified() {
		t.Fatalf("listening on %v, want every address", ip)
	}
	lo, _ := claimPort("127.0.0.1", freePort())
	if lo == nil {
		t.Fatal("no loopback listener")
	}
	defer lo.Close()
	if ip := lo.Addr().(*net.TCPAddr).IP; !ip.IsLoopback() {
		t.Fatalf("default listens on %v, want loopback only", ip)
	}
}
