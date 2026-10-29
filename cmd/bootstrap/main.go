// Command bootstrap builds a per-shop binary with a chosen set of plugins
// compiled in. It discovers plugins from plugins/*/plugin.json, writes the Go
// (cmd/server/enabled_plugins.go) and React (web/src/plugins/enabled-plugins.ts)
// plugin lists, builds the SPA and the static binary, then restores both lists
// to core-only.
//
// Usage (from the repo root):
//
//	go run ./cmd/bootstrap                            # opens the bootstrap app in the browser
//	go run ./cmd/bootstrap -plugins all -os windows -name acme
//	go run ./cmd/bootstrap -plugins clearance,requests
//	go run ./cmd/bootstrap -plugins all -write-only   # dev: make plugins P=all
//	go run ./cmd/bootstrap -plugins none -write-only  # back to core-only: make core-only
//	go run ./cmd/bootstrap -shop al-raj               # build a shop from shops.json (three files in dist/al-raj)
//	go run ./cmd/bootstrap -shop all                  # every shop, one after another
//	go run ./cmd/bootstrap -list                      # the shops in shops.json
//
// Flags:
//
//	-plugins     comma-separated plugin keys, "all" or "none" (omit every flag for the app)
//	-os / -arch  target GOOS (linux | windows | darwin | freebsd) / GOARCH (amd64 | arm64)
//	-name        output binary base name (default karots-suite)
//	-out         output directory (default dist)
//	-mode        server (DATABASE_URL, default) | file (embeds its own PostgreSQL, one PC)
//	-install-id  reuse a shop's install id (keeps its support PIN)
//	-X           extra space-separated -X name=value pairs for -ldflags (M8 release tool)
//	-write-only  write the two plugin lists and stop (no build, no restore)
//	-addr        where the bootstrap app listens (default 127.0.0.1:3199)
//	-shops       the shop profiles file (default shops.json, or $SHOPS_FILE)
//	-shop        build this shop's profile ("all" = every shop)
//	-list        print the shop profiles
//	-no-installer  with -shop: only the binary, no karots-setup / install.conf
//	-import-pos  add karots-pos's shops (same install ids, new channels, fresh secrets)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"karots-suite/internal/config"
)

// manifest mirrors plugins/<key>/plugin.json.
type manifest struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Import      string   `json:"import"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Requires    []string `json:"requires"` // plugins always built alongside this one
}

func main() {
	config.LoadDotEnv() // POS_SUPPORT_SECRET, as cmd/server reads it
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap: "+err.Error())
		os.Exit(1)
	}
}

func run() error {
	var (
		pluginsFlag = flag.String("plugins", "", `comma-separated plugin keys, "all" or "none" (omit to open the bootstrap app)`)
		osFlag      = flag.String("os", runtime.GOOS, "target GOOS: linux | windows | darwin | freebsd")
		archFlag    = flag.String("arch", "amd64", "target GOARCH: amd64 | arm64")
		nameFlag    = flag.String("name", "karots-suite", "output binary base name")
		outFlag     = flag.String("out", "dist", "output directory")
		modeFlag    = flag.String("mode", "server", "database: server (DATABASE_URL) | file (its own PostgreSQL, one PC)")
		installID   = flag.String("install-id", "", "reuse this shop's install id (keeps its support PIN)")
		extraX      = flag.String("X", "", "extra space-separated -X name=value pairs for -ldflags")
		writeOnly   = flag.Bool("write-only", false, "write the plugin lists and stop (dev)")
		demoList    = flag.Bool("demo-list", false, "with -write-only: the Demo Room's React list — every plugin, on per shop (dev)")
		demoFlag    = flag.Bool("demo", false, "build the Demo Room (dist/demo, Linux + Windows)")
		freshFlag   = flag.Bool("fresh", false, "with -demo: build the shop years even if nothing that shapes them changed")
		addr        = flag.String("addr", "127.0.0.1:3199", "where the bootstrap app listens")
		shopsFlag   = flag.String("shops", envOr("SHOPS_FILE", "shops.json"), "the shop profiles file")
		shopFlag    = flag.String("shop", "", `release this shop from the profiles: build + publish ("all" = every shop)`)
		testFlag    = flag.Bool("test", false, "with -shop: a Test build — local only, nothing uploaded")
		pgFlag      = flag.Bool("postgres", false, "with -shop: put the PostgreSQL installer beside a Windows main PC's files (no download at the shop)")
		listFlag    = flag.Bool("list", false, "print the shop profiles")
		noInstaller = flag.Bool("no-installer", false, "with -shop: only the binary")
	)
	flag.Parse()
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run from the repository root (go.mod not found)")
	}
	all, err := discover()
	if err != nil {
		return err
	}
	if *demoFlag {
		_, err := buildDemo(all, *freshFlag, os.Stdout)
		return err
	}
	if *listFlag {
		return listShops(*shopsFlag)
	}
	if *shopFlag != "" {
		return buildShops(*shopsFlag, *shopFlag, all, extras{Installer: !*noInstaller, Postgres: *pgFlag}, *testFlag)
	}
	if !isSet("plugins") && !isSet("write-only") && !*demoList && !*demoFlag {
		return serveApp(*addr, all, build, shopsCfg{Path: *shopsFlag, Out: "dist", Setup: buildSetup})
	}
	if *writeOnly && *demoList {
		return os.WriteFile(tsListPath, renderDemoTS(all), 0o644)
	}
	sel, err := selectPlugins(all, *pluginsFlag)
	if err != nil {
		return err
	}
	if *writeOnly {
		if err := writeSelection(goListPath, tsListPath, sel); err != nil {
			return err
		}
		fmt.Println("plugin lists:", names(sel))
		return nil
	}
	res, err := build(buildOpts{Plugins: sel, OS: *osFlag, Arch: *archFlag, Name: *nameFlag, Out: *outFlag, InstallID: *installID, ExtraX: *extraX, Mode: *modeFlag}, os.Stdout)
	if err != nil {
		return err
	}
	fmt.Printf("\n✓ Built %s\n", res.Binary)
	if res.InstallID != "" {
		fmt.Printf("  support login   0000000001 / %s   (rotates hourly)\n  install id      %s\n", res.SupportPIN, res.InstallID)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isSet(name string) (set bool) {
	flag.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

// discover reads every plugins/*/plugin.json, sorted by key.
func discover() ([]manifest, error) {
	paths, err := filepath.Glob("plugins/*/plugin.json")
	if err != nil {
		return nil, err
	}
	var out []manifest
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var m manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		if m.Key == "" || m.Import == "" || m.Key != filepath.Base(filepath.Dir(p)) {
			return nil, fmt.Errorf("%s: key (= its folder name) and import are required", p)
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// selectPlugins resolves -plugins: "all", "none" or a comma list of keys.
func selectPlugins(all []manifest, flagVal string) ([]manifest, error) {
	v := strings.TrimSpace(flagVal)
	switch strings.ToLower(v) {
	case "":
		return nil, fmt.Errorf(`-plugins is required: "all", "none" or keys (available: %s)`, names(all))
	case "all":
		return all, nil
	case "none":
		return nil, nil
	}
	byKey := map[string]manifest{}
	for _, m := range all {
		byKey[m.Key] = m
	}
	var sel []manifest
	for k := range strings.SplitSeq(v, ",") {
		k = strings.TrimSpace(k)
		m, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("unknown plugin %q (available: %s)", k, names(all))
		}
		sel = append(sel, m)
	}
	return withRequired(all, sel), nil
}

// withRequired adds the plugins the selection requires, in discovery order.
func withRequired(all, sel []manifest) []manifest {
	want := map[string]bool{}
	var add func(k string)
	add = func(k string) {
		if want[k] {
			return
		}
		want[k] = true
		for _, m := range all {
			if m.Key == k {
				for _, r := range m.Requires {
					add(r)
				}
			}
		}
	}
	for _, m := range sel {
		add(m.Key)
	}
	var out []manifest
	for _, m := range all {
		if want[m.Key] {
			out = append(out, m)
		}
	}
	return out
}

func names(sel []manifest) string {
	if len(sel) == 0 {
		return "core only"
	}
	ks := make([]string, len(sel))
	for i, m := range sel {
		ks[i] = m.Key
	}
	return strings.Join(ks, ", ")
}
