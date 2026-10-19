package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func signed(t *testing.T, priv ed25519.PrivateKey, version, sha string) Manifest {
	t.Helper()
	return Manifest{Version: version, SHA256: sha, Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, SignedMessage(version, sha)))}
}

func TestVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	m := signed(t, priv, "2026.10.19.1530", "abc123")
	if err := m.Verify(pub); err != nil {
		t.Fatalf("good signature: %v", err)
	}
	bad := m
	bad.SHA256 = "abc124"
	if bad.Verify(pub) == nil {
		t.Fatal("a changed hash must not verify")
	}
	// A karots-pos release can never pass for a suite one.
	pos := m
	pos.Sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte("karots-pos update\n2026.10.19.1530\nabc123")))
	if pos.Verify(pub) == nil {
		t.Fatal("a karots-pos signature must not verify")
	}
}

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"2026.10.19.1530", "2026.10.19.1529", true},
		{"2026.10.19.1530", "dev", true},
		{"2026.10.19.1530", "", true},
		{"dev", "2026.01.01.0000", false},
		{"2026.10.19.1530", "2026.10.19.1530", false},
		{"2026.10.19.1529", "2026.10.19.1530", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestNotesSince(t *testing.T) {
	m := Manifest{Notes: "latest only", Changelog: []Change{
		{Version: "2026.10.19.1530", Notes: "three"},
		{Version: "2026.10.18.1000", Notes: "two"},
		{Version: "2026.10.17.1000", Notes: "one"},
	}}
	got := m.NotesSince("2026.10.17.1000")
	if !strings.Contains(got, "2026.10.19.1530\nthree") || !strings.Contains(got, "2026.10.18.1000\ntwo") || strings.Contains(got, "one") {
		t.Fatalf("NotesSince = %q", got)
	}
	if strings.Index(got, "three") > strings.Index(got, "two") {
		t.Fatalf("newest first: %q", got)
	}
	if got := m.NotesSince("2026.10.19.1530"); got != "latest only" {
		t.Fatalf("nothing newer → m.Notes, got %q", got)
	}
}
