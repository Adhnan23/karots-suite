package backup

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const testKeyHex = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestSealOpen(t *testing.T) {
	key, err := ParseKey(testKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("\x1f\x8b a gzipped backup")
	a, err := Seal(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Seal(key, plain)
	if bytes.Equal(a, b) {
		t.Fatal("two seals of the same backup are identical (nonce reused)")
	}
	got, err := Open(key, a)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("Open = %q, %v", got, err)
	}
	other, _ := ParseKey(strings.Repeat("ab", 32))
	if _, err := Open(other, a); err == nil {
		t.Fatal("opened with the wrong key")
	}
	bad := append([]byte(nil), a...)
	bad[len(bad)-1] ^= 1
	if _, err := Open(key, bad); err == nil {
		t.Fatal("opened a tampered file")
	}
	if _, err := Open(key, plain); !errors.Is(err, ErrNotSealed) {
		t.Fatalf("Open(plain) err = %v, want ErrNotSealed", err)
	}
}

func TestParseKey(t *testing.T) {
	for _, k := range []string{testKeyHex[:63], testKeyHex + "0", strings.Repeat("zz", 32), ""} {
		if _, err := ParseKey(k); err == nil {
			t.Errorf("ParseKey(%q) accepted", k)
		}
	}
}
