package main

import (
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"karots-suite/internal/update"
)

// Published is what a shop's folder in the bucket holds now (shops.json).
type Published struct {
	Version string    `json:"version"`
	Commit  string    `json:"commit"`
	At      time.Time `json:"at"`
}

const (
	keepVersions  = 3  // binaries kept per channel
	keepChangelog = 10 // releases listed in the manifest's changelog
)

// builderWrites are the tracked files a build itself changes: a release from a
// tree where only these differ is still a clean release.
var builderWrites = []string{"shops.json", "cmd/server/enabled_plugins.go", "web/src/plugins/enabled-plugins.ts"}

// versionAt is a release version: fixed width, so string order is time order.
func versionAt(t time.Time) string { return t.Format("2006.01.02.1504") }

// releaseKey is the suite release key from SUITE_RELEASE_KEY. The first shop
// build makes one and writes it into envPath (filling an empty
// SUITE_RELEASE_KEY= line, or appending one).
func releaseKey(envPath string, log io.Writer) (ed25519.PrivateKey, string, error) {
	pubOf := func(k ed25519.PrivateKey) string {
		return base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey))
	}
	if v := os.Getenv("SUITE_RELEASE_KEY"); v != "" {
		seed, err := base64.StdEncoding.DecodeString(v)
		if err != nil || len(seed) != ed25519.SeedSize {
			return nil, "", errors.New("SUITE_RELEASE_KEY in .env is damaged — put the saved copy back (a new key cuts every shop off from updates)")
		}
		k := ed25519.NewKeyFromSeed(seed)
		return k, pubOf(k), nil
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, "", err
	}
	v := base64.StdEncoding.EncodeToString(seed)
	b, err := os.ReadFile(envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	lines, found := strings.Split(strings.TrimRight(string(b), "\n"), "\n"), false
	for i, l := range lines {
		if strings.TrimSpace(l) == "SUITE_RELEASE_KEY=" {
			lines[i], found = "SUITE_RELEASE_KEY="+v, true
		}
	}
	if !found {
		lines = append(lines, "SUITE_RELEASE_KEY="+v)
	}
	if err := os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return nil, "", err
	}
	os.Setenv("SUITE_RELEASE_KEY", v)
	fmt.Fprintln(log, "→ new release key written to .env (SUITE_RELEASE_KEY) — keep a copy: shops only take updates signed by it")
	k := ed25519.NewKeyFromSeed(seed)
	return k, pubOf(k), nil
}

// treeDirty reports uncommitted changes in dir's git tree, apart from the
// files a build writes itself. A release from a dirty tree would ship code
// its "what's new" doesn't list.
func treeDirty(dir string) (bool, error) {
	c := exec.Command("git", "status", "--porcelain", "--untracked-files=no")
	c.Dir = dir
	out, err := c.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	for l := range strings.Lines(string(out)) {
		if path := strings.TrimSpace(l[min(3, len(l)):]); path != "" && !slices.Contains(builderWrites, path) {
			return true, nil
		}
	}
	return false, nil
}

// updateX is the -X flags that give a shop build its version and updater.
func updateX(s Shop, version, kind, pub string) string {
	return "main.version=" + version + " main.buildKind=" + kind +
		" main.updateBaseURL=" + strings.TrimRight(os.Getenv("UPDATE_BASE_URL"), "/") +
		" main.updateChannel=" + s.Channel + " main.updatePubKey=" + pub
}

// defaultNotes offers the commits since the shop's last release.
func defaultNotes(s Shop, all []manifest) string {
	if s.Published == nil {
		return ""
	}
	var keys []string
	for _, m := range all {
		keys = append(keys, m.Key)
	}
	return changesSince(s.Published.Commit, s.Plugins, keys)
}

// publish puts the binary built in dir into the shop's folder in the bucket:
// gzipped binary first, then the signed manifest (no-store), then old binaries
// pruned. baseURL is the bucket's public URL (to read the current manifest).
func publish(s *Shop, dir, version, notes, head, baseURL string, st store, priv ed25519.PrivateKey, log io.Writer) error {
	ext := ""
	if s.OS == "windows" {
		ext = ".exe"
	}
	bin := filepath.Join(dir, shopBinary+ext)
	prev, err := fetchManifest(strings.TrimRight(baseURL, "/") + "/" + s.Channel + "/manifest.json")
	if err != nil {
		return err
	}
	if prev != nil && !update.Newer(version, prev.Version) {
		return fmt.Errorf("version %s is not newer than the published %s — wait a minute and try again", version, prev.Version)
	}
	sha, err := fileSHA256(bin) // of the binary itself: the shop checks it after unpacking
	if err != nil {
		return err
	}
	file := shopBinary + "-" + version + ext + ".gz"
	gzPath := filepath.Join(dir, file)
	if err := gzipFile(bin, gzPath); err != nil {
		return err
	}
	defer os.Remove(gzPath) // the three files carried to the shop stay three
	m := update.Manifest{
		Version: version, File: file, SHA256: sha, Notes: notes, Commit: head,
		Sig:       base64.StdEncoding.EncodeToString(ed25519.Sign(priv, update.SignedMessage(version, sha))),
		History:   []string{file},
		Changelog: []update.Change{{Version: version, Notes: notes}},
	}
	if prev != nil {
		m.History = append(m.History, prev.History...)
		m.Changelog = append(m.Changelog, prev.Changelog...)
	}
	m.Changelog = m.Changelog[:min(len(m.Changelog), keepChangelog)]
	var prune []string
	if len(m.History) > keepVersions {
		m.History, prune = m.History[:keepVersions], m.History[keepVersions:]
	}
	mb, _ := json.MarshalIndent(m, "", "  ")
	manPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manPath, mb, 0o644); err != nil {
		return err
	}
	defer os.Remove(manPath)

	fmt.Fprintf(log, "→ uploading %s\n", file)
	if err := st.put(s.Channel+"/"+file, gzPath, "application/gzip", ""); err != nil {
		return err
	}
	if err := st.put(s.Channel+"/manifest.json", manPath, "application/json", "no-store"); err != nil {
		return err
	}
	for _, old := range prune {
		if err := st.delete(s.Channel + "/" + old); err != nil {
			fmt.Fprintf(log, "! couldn't delete old %s: %v\n", old, err)
		}
	}
	s.Published = &Published{Version: version, Commit: head, At: time.Now().UTC().Truncate(time.Second)}
	fmt.Fprintf(log, "✓ published %s (%d kept, %d old removed)\n", version, len(m.History), len(prune))
	return nil
}

// fetchManifest reads the shop's current manifest; nil when none is published yet.
func fetchManifest(url string) (*update.Manifest, error) {
	res, err := (&http.Client{Timeout: time.Minute}).Get(url)
	if err != nil {
		return nil, errors.New("can't reach the bucket to read the published version (offline?)")
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reading the published version: the bucket answered %s", res.Status)
	}
	var m update.Manifest
	return &m, json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&m)
}

func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	zw, _ := gzip.NewWriterLevel(out, gzip.BestCompression)
	if _, err := io.Copy(zw, in); err != nil {
		out.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
