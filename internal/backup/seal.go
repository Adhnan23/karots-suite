package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

// SealedSuffix marks a backup sealed for Karots cloud.
const SealedSuffix = ".json.gz.enc"

// sealMagic starts every sealed file (format 1: magic, 12-byte nonce, AES-256-GCM).
var sealMagic = []byte("KBC1")

// ErrNotSealed is Open of a file that was never sealed.
var ErrNotSealed = errors.New("not a sealed Karots backup")

// ParseKey reads a shop's cloud key: 64 hex characters (32 bytes).
func ParseKey(s string) ([]byte, error) {
	k, err := hex.DecodeString(s)
	if err != nil || len(k) != 32 {
		return nil, fmt.Errorf("the cloud backup key must be 64 hex characters")
	}
	return k, nil
}

// Seal encrypts a backup for Karots cloud.
// ponytail: whole file in memory — stream if backups pass a few hundred MB.
func Seal(key, plain []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append(append([]byte(nil), sealMagic...), nonce...)
	return gcm.Seal(out, nonce, plain, sealMagic), nil
}

// Open decrypts a sealed backup; a wrong key or a changed byte is an error.
func Open(key, sealed []byte) ([]byte, error) {
	if !bytes.HasPrefix(sealed, sealMagic) {
		return nil, ErrNotSealed
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	rest := sealed[len(sealMagic):]
	if len(rest) < gcm.NonceSize() {
		return nil, ErrNotSealed
	}
	plain, err := gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], sealMagic)
	if err != nil {
		return nil, errors.New("this backup can't be opened with this shop's key (wrong key or damaged file)")
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
