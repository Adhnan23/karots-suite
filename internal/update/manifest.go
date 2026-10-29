// Package update lets an installed shop binary update itself from the release
// bucket (R2, read publicly).
//
// Each shop build has its own channel: a random folder name baked in at build
// time (the Shop Builder). The folder holds manifest.json plus the binaries.
// The channel is the only thing that keeps a shop's binary private — it carries
// the shop's support seed — so it never shows in a page, a log line or an error.
//
// A release is trusted only if its manifest is signed by the suite release key
// (ed25519; the public half is baked in, the private half stays in the
// developer's .env) and the downloaded file matches the signed SHA-256.
package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
)

// RestartCode is the exit code that asks the supervisor (systemd, or the
// Windows launcher karots-start) to start Karots again — after an update has
// swapped the binary, or after BootGuard put the old one back.
const RestartCode = 75

// Manifest is <channel>/manifest.json in the bucket.
type Manifest struct {
	Version string `json:"version"`         // "2026.10.19.1530": fixed width, so newer sorts later
	File    string `json:"file"`            // the gzipped binary's object name inside the channel
	SHA256  string `json:"sha256"`          // hex, of the unpacked binary
	Sig     string `json:"sig"`             // base64 ed25519 over SignedMessage
	Notes   string `json:"notes,omitempty"` // this release's "what's new"
	// Changelog is the notes of recent releases, newest first, so a shop that
	// skipped some still sees every change it is about to get.
	Changelog []Change `json:"changelog,omitempty"`
	// Commit is the git commit the release was built from; the next release
	// offers the commits since it as its notes.
	Commit string `json:"commit,omitempty"`
	// History lists the binaries still kept in the channel, newest first. Only
	// the Builder reads it (to prune old ones; a public bucket can't be listed).
	History []string `json:"history,omitempty"`
}

// Change is one release's notes in the changelog.
type Change struct {
	Version string `json:"version"`
	Notes   string `json:"notes"`
}

// NotesSince joins the changelog entries newer than current — everything a shop
// on that version gains by updating — newest first, each under its version.
func (m *Manifest) NotesSince(current string) string {
	var parts []string
	for _, c := range m.Changelog {
		if Newer(c.Version, current) && c.Notes != "" {
			parts = append(parts, c.Version+"\n"+c.Notes)
		}
	}
	if len(parts) == 0 {
		return m.Notes
	}
	return strings.Join(parts, "\n\n")
}

// SignedMessage is what the release key signs: the version and the file's
// hash, so neither can be swapped without breaking the signature. The prefix
// is the suite's own, so another product's release never installs here.
func SignedMessage(version, sha string) []byte {
	return []byte("karots-suite update\n" + version + "\n" + sha)
}

var errBadSignature = errors.New("update is not signed by the release key")

// Verify checks the manifest's signature against pub.
func (m *Manifest) Verify(pub ed25519.PublicKey) error {
	sig, err := base64.StdEncoding.DecodeString(m.Sig)
	if err != nil || len(pub) != ed25519.PublicKeySize || !ed25519.Verify(pub, SignedMessage(m.Version, m.SHA256), sig) {
		return errBadSignature
	}
	return nil
}

// Newer reports whether version a is newer than b. Versions are fixed-width
// dates (YYYY.MM.DD.HHMM), so plain string order is version order; a dev
// build ("" or "dev") is older than any release.
func Newer(a, b string) bool {
	if a == "" || a == "dev" {
		return false
	}
	if b == "" || b == "dev" {
		return true
	}
	return a > b
}
