package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestImportPOS(t *testing.T) {
	p := filepath.Join(t.TempDir(), "shops.json")
	os.WriteFile(p, []byte(`{"shops":[
	  {"key":"mum-digital","name":"Mum Digital","os":"linux","arch":"amd64","plugins":["alpha","oldthing"],"install_id":"BB876E88","channel":"59aa243a54382b4da4f647bbc594431b"},
	  {"key":"kept","name":"Kept","os":"windows","arch":"amd64","plugins":[],"install_id":"A3B07B1A","channel":"dcea3713cece4486652dfa322934ac39"}
	]}`), 0o644)
	existing := mustShop(t, "kept", "linux", "file")
	into := Shops{Shops: []Shop{existing}}
	added, dropped, err := importPOS(p, []manifest{{Key: "alpha"}}, &into)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(added, []string{"mum-digital"}) || !slices.Equal(dropped["mum-digital"], []string{"oldthing"}) {
		t.Fatalf("added %v dropped %v", added, dropped)
	}
	if len(into.Shops) != 2 || into.Shops[0].InstallID != existing.InstallID {
		t.Fatalf("existing shop overwritten: %+v", into.Shops)
	}
	s := into.Shops[1]
	switch {
	case s.Key != "mum-digital" || s.Name != "Mum Digital" || s.OS != "linux" || s.Arch != "amd64" || s.InstallID != "BB876E88":
		t.Fatalf("identity not kept: %+v", s)
	case s.Mode != "server" || s.Port != 3000 || s.DB == nil || len(s.JWTSecret) != 48:
		t.Fatalf("defaults: %+v", s)
	case s.Channel == "59aa243a54382b4da4f647bbc594431b" || len(s.Channel) != 32:
		t.Fatalf("channel must be new: %q", s.Channel)
	case !slices.Equal(s.Plugins, []string{"alpha"}):
		t.Fatalf("plugins %v", s.Plugins)
	}
}
