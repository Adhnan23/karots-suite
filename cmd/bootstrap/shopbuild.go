package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"karots-suite/internal/shopconf"
)

// shopBinary is every shop build's binary name: install.conf names it, so the
// installer finds it whatever the shop is called.
const shopBinary = "karots-suite"

type shopBuildOpts struct {
	Out       string // the folder holding dist/<key>
	Installer bool   // add karots-setup + install.conf beside the binary
	// Version, Kind ("test" | "release") and PubKey (the release key's public
	// half) stamp the binary and turn its updater on; empty Version = neither.
	Version, Kind, PubKey string
	// PGInstaller is the cached PostgreSQL installer to put beside the three
	// files of a Windows main PC (needsPG); "" = none.
	PGInstaller string
}

// buildShop makes <out>/<key>: the shop's binary and, with Installer, the
// installer and its install.conf — the three files carried to the shop.
// It records LastBuilt on s; the caller saves shops.json.
func buildShop(s *Shop, all []manifest, o shopBuildOpts, b func(buildOpts, io.Writer) (buildResult, error), setup func(osName, arch, out string, log io.Writer) error, log io.Writer) (buildResult, error) {
	if _, err := s.Check(all); err != nil {
		return buildResult{}, err
	}
	byKey := map[string]manifest{}
	for _, m := range all {
		byKey[m.Key] = m
	}
	var sel []manifest
	for _, k := range s.Plugins {
		sel = append(sel, byKey[k])
	}
	dir := filepath.Join(o.Out, s.Key)
	if err := os.RemoveAll(dir); err != nil {
		return buildResult{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return buildResult{}, err
	}
	bo := buildOpts{Plugins: withRequired(all, sel), OS: s.OS, Arch: s.Arch, Name: shopBinary, Out: dir, InstallID: s.InstallID, Mode: s.Mode, RequireSecret: true}
	if o.Version != "" {
		bo.ExtraX = updateX(*s, o.Version, o.Kind, o.PubKey)
	}
	res, err := b(bo, log)
	if err != nil {
		return res, err
	}
	if o.Installer {
		fmt.Fprintf(log, "→ karots-setup for %s/%s\n", s.OS, s.Arch)
		if err := setup(s.OS, s.Arch, dir, log); err != nil {
			return res, fmt.Errorf("build karots-setup: %w", err)
		}
		if err := shopconf.Write(filepath.Join(dir, shopconf.FileName), s.Conf(filepath.Base(res.Binary))); err != nil {
			return res, err
		}
		if o.PGInstaller != "" && needsPG(*s) {
			fmt.Fprintf(log, "→ %s beside the installer (no download at the shop)\n", shopconf.PGInstallerName)
			if err := linkOrCopy(o.PGInstaller, filepath.Join(dir, shopconf.PGInstallerName)); err != nil {
				return res, err
			}
		}
	}
	res.Folder = dir
	s.LastBuilt = &Built{At: time.Now().UTC().Truncate(time.Second), Commit: gitCommit(), Version: o.Version, Kind: o.Kind}
	return res, nil
}

// buildSetup cross-builds the installer, static like the server.
func buildSetup(osName, arch, out string, log io.Writer) error {
	name := "karots-setup"
	if osName == "windows" {
		name += ".exe"
	}
	env := []string{"GOOS=" + osName, "GOARCH=" + arch, "CGO_ENABLED=0"}
	if arch == "amd64" {
		env = append(env, "GOAMD64=v1")
	}
	ldflags := "-s -w"
	if osName == "windows" {
		ldflags += " -H windowsgui" // double-clicked: no console window; errors show in a message box
	}
	return sh(log, env, "go", "build", "-ldflags="+ldflags, "-o", filepath.Join(out, name), "./cmd/setup")
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// buildShops is -shop: a Release of one shop, or "all" in key order (one
// version for all), or with test a Test build (nothing uploaded). A failing
// shop doesn't stop the rest; the failures are listed at the end.
func buildShops(path, which string, all []manifest, add extras, test bool) error {
	shops, err := LoadShops(path)
	if err != nil {
		return err
	}
	cfg := shopsCfg{Path: path, Out: "dist", Setup: buildSetup}
	kind := "release"
	if test {
		kind = "test"
	} else if dirty, err := cfg.dirty(); err != nil {
		return err
	} else if dirty {
		return fmt.Errorf(`uncommitted changes — commit first, so the shops' "what's new" matches what they get (or -test for a local build)`)
	}
	var mu sync.Mutex
	version := versionAt(time.Now())
	built, failed := 0, []string(nil)
	for i := range shops.Shops {
		s := shops.Shops[i]
		if which != "all" && s.Key != which {
			continue
		}
		built++
		before := s.LastBuilt
		res, err := shopRun(&s, all, build, cfg, kind, version, defaultNotes(s, all), add, os.Stdout)
		if s.LastBuilt != before {
			if serr := saveShopState(cfg, &mu, s); serr != nil {
				return serr
			}
		}
		if err != nil {
			fmt.Printf("\n✗ %s: %v\n", s.Key, err)
			failed = append(failed, s.Key)
			continue
		}
		fmt.Printf("\n✓ %s %s %s → %s   (install id %s, support PIN now %s)\n", s.Name, kind, version, filepath.Dir(res.Binary), res.InstallID, res.SupportPIN)
	}
	if built == 0 {
		return fmt.Errorf("no shop %q in %s (see -list)", which, path)
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d of %d shops failed: %s", len(failed), built, strings.Join(failed, ", "))
	}
	return nil
}

// listShops is -list.
func listShops(path string) error {
	shops, err := LoadShops(path)
	if err != nil {
		return err
	}
	if len(shops.Shops) == 0 {
		fmt.Printf("no shops in %s — add one in the Shop Builder (go run ./cmd/bootstrap)\n", path)
		return nil
	}
	for _, s := range shops.Shops {
		last, pub := "never", "not published"
		if s.LastBuilt != nil {
			last = s.LastBuilt.At.Local().Format("2006-01-02 15:04") + " " + s.LastBuilt.Commit + " " + s.LastBuilt.Kind
		}
		if s.Published != nil {
			pub = "published " + s.Published.Version
		}
		fmt.Printf("%-20s %-24s %s/%-5s %-6s :%-5d %-40s built %s · %s\n", s.Key, s.Name, s.OS, s.Arch, s.Mode, s.Port, strings.Join(s.Plugins, ","), last, pub)
	}
	return nil
}
