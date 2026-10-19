package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"karots-suite/internal/support"
)

// posShop is one entry of karots-pos's shops.json (cmd/release).
type posShop struct {
	Key       string   `json:"key"`
	Name      string   `json:"name"`
	OS        string   `json:"os"`
	Arch      string   `json:"arch"`
	Plugins   []string `json:"plugins"`
	InstallID string   `json:"install_id"`
}

// importPOS adds karots-pos shops to into: same key, name, OS / CPU and install
// id (the support PIN the shop knows stays), server mode, fresh secrets and a
// NEW channel — the suite has its own update feed, never karots-pos's. Plugins
// the suite doesn't have are dropped and reported; a key already present is left alone.
func importPOS(path string, all []manifest, into *Shops) (added []string, dropped map[string][]string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var pos struct {
		Shops []posShop `json:"shops"`
	}
	if err := json.Unmarshal(raw, &pos); err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	known := map[string]bool{}
	for _, m := range all {
		known[m.Key] = true
	}
	dropped = map[string][]string{}
	for _, p := range pos.Shops {
		if slices.ContainsFunc(into.Shops, func(s Shop) bool { return s.Key == p.Key }) {
			continue
		}
		s, err := NewShop(p.Key, p.Name, p.OS, p.Arch, "server")
		if err != nil {
			return added, dropped, err
		}
		s.InstallID = support.Normalise(p.InstallID)
		for _, k := range p.Plugins {
			if known[k] {
				s.Plugins = append(s.Plugins, k)
			} else {
				dropped[p.Key] = append(dropped[p.Key], k)
			}
		}
		into.Shops = append(into.Shops, s)
		added = append(added, p.Key)
	}
	return added, dropped, nil
}

// importPOSFile is -import-pos: merge into the profiles file and say what happened.
func importPOSFile(shopsPath, posPath string, all []manifest) error {
	shops, err := LoadShops(shopsPath)
	if err != nil {
		return err
	}
	added, dropped, err := importPOS(posPath, all, &shops)
	if err != nil {
		return err
	}
	if err := SaveShops(shopsPath, shops); err != nil {
		return err
	}
	fmt.Printf("added %d shop(s) to %s: %v\n", len(added), shopsPath, added)
	for k, ps := range dropped {
		fmt.Printf("  %s: dropped plugins the suite doesn't have: %v\n", k, ps)
	}
	return nil
}
