package pocketcashier

import (
	"crypto/tls"
	"fmt"
	"os"
	"sync"
	"time"
)

// certSource picks the phones' certificate: a public one when .env names it
// (POCKET_CERT_FILE / POCKET_KEY_FILE — a VPS or Tailscale shop, nothing to
// install on a phone), else a leaf from the shop's own CA.
func certSource(s *Store) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cf, kf := os.Getenv("POCKET_CERT_FILE"), os.Getenv("POCKET_KEY_FILE")
	if cf != "" && kf != "" {
		return (&fileCert{cert: cf, key: kf}).GetCertificate
	}
	return (&leafCache{ca: s.CA}).GetCertificate
}

// publicCert reports whether the phones get a public certificate (no CA steps).
func publicCert() bool { return os.Getenv("POCKET_CERT_FILE") != "" && os.Getenv("POCKET_KEY_FILE") != "" }

// fileCert serves a certificate pair from files, read again when the
// certificate file changes (a renewal).
type fileCert struct {
	cert, key string

	mu   sync.Mutex
	pair *tls.Certificate
	mod  time.Time
}

func (f *fileCert) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, err := os.Stat(f.cert)
	if err != nil {
		return nil, fmt.Errorf("phone certificate: %w", err)
	}
	if f.pair == nil || !st.ModTime().Equal(f.mod) {
		pair, err := tls.LoadX509KeyPair(f.cert, f.key)
		if err != nil {
			return nil, fmt.Errorf("phone certificate: %w", err)
		}
		f.pair, f.mod = &pair, st.ModTime()
	}
	return f.pair, nil
}
