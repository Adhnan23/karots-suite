package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"karots-suite/internal/filedb"
	"karots-suite/internal/support"
)

// The Demo Room build (make demo): every plugin compiled in, the SPA's plugin
// list gated per shop, each built-in shop's year built once and embedded, then
// one double-click binary per OS in dist/demo. The demo keeps one install ID
// (demoIDPath, committed — it isn't a secret) so the PIN app needs one entry.

var (
	demoIDPath  = filepath.Join("internal", "demo", "INSTALL_ID")
	demoTplDir  = filepath.Join("internal", "demo", "templates", "files")
	demoTargets = []string{"linux", "windows"}
)

// withDemoList writes the Demo Room's React list (every plugin, on per shop)
// for build and always puts the lists back core-only — even when build fails.
func withDemoList(goPath, tsPath string, all []manifest, build func() error) (err error) {
	defer func() {
		if rerr := writeSelection(goPath, tsPath, nil); rerr != nil && err == nil {
			err = fmt.Errorf("restore core-only plugin lists: %w", rerr)
		}
	}()
	if err := os.WriteFile(tsPath, renderDemoTS(all), 0o644); err != nil {
		return err
	}
	return build()
}

// demoInstallID is the demo's install ID, minted on the first build.
func demoInstallID() (string, error) {
	if b, err := os.ReadFile(demoIDPath); err == nil {
		return support.Normalise(string(b)), nil
	}
	id, err := support.NewInstallID()
	if err != nil {
		return "", err
	}
	return id, os.WriteFile(demoIDPath, []byte(id+"\n"), 0o644)
}

func buildDemo(all []manifest, log io.Writer) error {
	id, err := demoInstallID()
	if err != nil {
		return err
	}
	ldflags, _, pin, err := supportFlags(id, "", log)
	if err != nil {
		return err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	bundle := func(goos string) (func(), error) {
		src, err := filedb.FetchBundle(filepath.Join(cache, "karots-suite-filedb"), goos, "amd64")
		if err != nil {
			return nil, err
		}
		return placeBundle(src, bundlePath)
	}
	defer cleanTemplates()
	err = withDemoList(goListPath, tsListPath, all, func() error {
		if err := buildSPA(log); err != nil {
			return err
		}
		if err := shIn(log, "web", nil, "bunx", "vite", "build", "--config", "demo/vite.config.ts"); err != nil {
			return fmt.Errorf("demo pages: %w", err)
		}
		_ = os.WriteFile(filepath.Join("internal", "demo", "ui", "dist", ".gitkeep"), nil, 0o644)
		// 1. a builder for this machine makes each shop's year (minutes; all at once)
		host := filepath.Join(os.TempDir(), "karots-demo-builder")
		done, err := bundle(runtime.GOOS)
		if err != nil {
			return err
		}
		err = sh(log, []string{"CGO_ENABLED=0"}, "go", "build", "-tags", "filemode", "-o", host, "./cmd/demo")
		done()
		if err != nil {
			return err
		}
		defer os.Remove(host)
		fmt.Fprintln(log, "→ building each shop's year of trade (a few minutes)…")
		if err := sh(log, nil, host, "-templates", demoTplDir); err != nil {
			return fmt.Errorf("shop years: %w", err)
		}
		// 2. the shipped binaries carry those years, the demo's install ID and its seed
		for _, goos := range demoTargets {
			out := filepath.Join("dist", "demo", "karots-demo")
			if goos == "windows" {
				out += ".exe"
			}
			done, err := bundle(goos)
			if err != nil {
				return err
			}
			err = sh(log, []string{"GOOS=" + goos, "GOARCH=amd64", "GOAMD64=v1", "CGO_ENABLED=0"},
				"go", "build", "-tags", "filemode", "-ldflags="+ldflags, "-o", out, "./cmd/demo")
			done()
			if err != nil {
				return err
			}
			fmt.Fprintf(log, "✓ %s\n", out)
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(log, "\n✓ Demo Room built in dist/demo\n  install id      %s\n", id)
	if pin != "" {
		fmt.Fprintf(log, "  unlock code     %s   (rotates hourly; your PIN app gives it for this install id)\n", pin)
	}
	return nil
}

// cleanTemplates leaves only the placeholder, so no shop year lingers in the tree.
func cleanTemplates() {
	es, _ := os.ReadDir(demoTplDir)
	for _, e := range es {
		if strings.HasSuffix(e.Name(), ".ksb") {
			os.Remove(filepath.Join(demoTplDir, e.Name()))
		}
	}
}
