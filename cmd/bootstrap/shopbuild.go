package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"karots-suite/internal/shopconf"
)

// shopBinary is every shop build's binary name: install.conf names it, so the
// installer finds it whatever the shop is called.
const shopBinary = "karots-suite"

type shopBuildOpts struct {
	Out       string // the folder holding dist/<key>
	Installer bool   // add karots-setup + install.conf beside the binary
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
	res, err := b(buildOpts{Plugins: withRequired(all, sel), OS: s.OS, Arch: s.Arch, Name: shopBinary, Out: dir, InstallID: s.InstallID, Mode: s.Mode, RequireSecret: true}, log)
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
	}
	s.LastBuilt = &Built{At: time.Now().UTC().Truncate(time.Second), Commit: gitCommit()}
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
	return sh(log, env, "go", "build", "-ldflags=-s -w", "-o", filepath.Join(out, name), "./cmd/setup")
}

func gitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// buildShops is -shop: one shop, or "all" in key order, stopping at the first
// failure. Each success is saved (LastBuilt) before the next starts.
func buildShops(path, which string, all []manifest, installer bool) error {
	shops, err := LoadShops(path)
	if err != nil {
		return err
	}
	built := 0
	for i := range shops.Shops {
		s := &shops.Shops[i]
		if which != "all" && s.Key != which {
			continue
		}
		res, err := buildShop(s, all, shopBuildOpts{Out: "dist", Installer: installer}, build, buildSetup, os.Stdout)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Key, err)
		}
		if err := SaveShops(path, shops); err != nil {
			return err
		}
		fmt.Printf("\n✓ %s → %s   (install id %s, support PIN now %s)\n", s.Name, filepath.Dir(res.Binary), res.InstallID, res.SupportPIN)
		built++
	}
	if built == 0 {
		return fmt.Errorf("no shop %q in %s (see -list)", which, path)
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
		last := "never"
		if s.LastBuilt != nil {
			last = s.LastBuilt.At.Local().Format("2006-01-02 15:04") + " " + s.LastBuilt.Commit
		}
		fmt.Printf("%-20s %-24s %s/%-5s %-6s :%-5d %-40s built %s\n", s.Key, s.Name, s.OS, s.Arch, s.Mode, s.Port, strings.Join(s.Plugins, ","), last)
	}
	return nil
}
