package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"karots-suite/internal/support"
)

// buildOpts is one per-shop build, from the flags or the bootstrap app.
type buildOpts struct {
	Plugins   []manifest
	OS, Arch  string
	Name, Out string
	InstallID string // "" = mint a new one (only when POS_SUPPORT_SECRET is set)
	ExtraX    string
}

// buildResult is what the shop owner needs once the binary exists.
type buildResult struct {
	Binary     string `json:"binary"`
	InstallID  string `json:"installId"`
	SupportPIN string `json:"supportPin"`
}

var (
	validOS      = map[string]bool{"linux": true, "windows": true, "darwin": true, "freebsd": true}
	validArch    = map[string]bool{"amd64": true, "arm64": true}
	installIDFmt = regexp.MustCompile(`^[0-9A-Z]{4,32}$`)
	validName    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// check normalises and validates opts before anything is written or built.
func (o *buildOpts) check() error {
	o.OS, o.Arch = strings.ToLower(strings.TrimSpace(o.OS)), strings.ToLower(strings.TrimSpace(o.Arch))
	o.InstallID = support.Normalise(o.InstallID)
	switch {
	case !validOS[o.OS]:
		return fmt.Errorf("unsupported OS %q (linux | windows | darwin | freebsd)", o.OS)
	case !validArch[o.Arch]:
		return fmt.Errorf("unsupported architecture %q (amd64 | arm64)", o.Arch)
	case !validName.MatchString(o.Name):
		return fmt.Errorf("binary name %q: letters, digits, dot, dash or underscore", o.Name)
	case o.InstallID != "" && !installIDFmt.MatchString(o.InstallID):
		return fmt.Errorf("install id %q: 4–32 letters or digits (e.g. 3FA2C91B)", o.InstallID)
	}
	return nil
}

// build writes the plugin lists, builds the SPA and the static binary, restores
// the lists to core-only, and reports the binary and its support login. All
// command output goes to log.
func build(o buildOpts, log io.Writer) (buildResult, error) {
	if err := o.check(); err != nil {
		return buildResult{}, err
	}
	ldflags, id, pin, err := supportFlags(o.InstallID, o.ExtraX, log)
	if err != nil {
		return buildResult{}, err
	}
	bin := o.Name
	if o.OS == "windows" {
		bin += ".exe"
	}
	res := buildResult{Binary: filepath.Join(o.Out, bin), InstallID: id, SupportPIN: pin}
	fmt.Fprintf(log, "Building %s for %s/%s with %s\n", res.Binary, o.OS, o.Arch, names(o.Plugins))
	err = withSelection(goListPath, tsListPath, o.Plugins, func() error {
		if err := buildSPA(log); err != nil { // the SPA embeds the selected plugins' React halves
			return err
		}
		// CGO_ENABLED=0 → fully static; GOAMD64=v1 → runs on old CPUs too.
		env := []string{"GOOS=" + o.OS, "GOARCH=" + o.Arch, "CGO_ENABLED=0"}
		if o.Arch == "amd64" {
			env = append(env, "GOAMD64=v1")
		}
		return sh(log, env, "go", "build", "-ldflags="+ldflags, "-o", res.Binary, "./cmd/server")
	})
	return res, err
}

// supportFlags bakes this shop's install id and support seed — never the master
// secret, because -ldflags lands in the binary's build metadata.
func supportFlags(installID, extraX string, log io.Writer) (ldflags, id, pin string, err error) {
	ldflags = "-s -w"
	if secret := os.Getenv("POS_SUPPORT_SECRET"); secret != "" {
		id = installID
		if id == "" {
			if id, err = support.NewInstallID(); err != nil {
				return "", "", "", fmt.Errorf("generate install id: %w", err)
			}
		}
		pin = support.CodeForSecret(secret, id, time.Now())
		ldflags += " -X main.installIDBaked=" + id + " -X main.supportSeedHex=" + hex.EncodeToString(support.DeriveSeed(secret, id))
	} else {
		fmt.Fprintln(log, "! POS_SUPPORT_SECRET is not set — this build uses the fixed support PIN every bare build shares.")
	}
	for x := range strings.FieldsSeq(extraX) {
		ldflags += " -X " + x
	}
	return ldflags, id, pin, nil
}

// buildSPA is `make web` without make or a shell, so a build runs the same on
// Windows, macOS and Linux: bun builds web/ into internal/spa/dist (which Vite
// empties), then the .gitkeep that keeps the folder in git is put back.
// web/src/api/types.gen.ts is committed, so tygo isn't needed here.
func buildSPA(log io.Writer) error {
	if _, err := exec.LookPath("bun"); err != nil {
		return fmt.Errorf("bun is not installed or not on PATH — get it from https://bun.sh, then build again")
	}
	if err := shIn(log, "web", nil, "bun", "install", "--frozen-lockfile"); err != nil {
		return fmt.Errorf("bun install: %w", err)
	}
	if err := shIn(log, "web", nil, "bun", "run", "build"); err != nil {
		return fmt.Errorf("bun run build: %w", err)
	}
	return os.WriteFile(filepath.Join("internal", "spa", "dist", ".gitkeep"), nil, 0o644)
}

func sh(log io.Writer, env []string, name string, args ...string) error {
	return shIn(log, "", env, name, args...)
}

func shIn(log io.Writer, dir string, env []string, name string, args ...string) error {
	fmt.Fprintf(log, "→ %s %s\n", name, strings.Join(args, " "))
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), env...)
	c.Stdout, c.Stderr = log, log
	return c.Run()
}
