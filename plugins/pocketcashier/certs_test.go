package pocketcashier

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCAStoredOnce(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	a, _, err := s.CA(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := s.CA(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.Get(&n, `SELECT count(*) FROM pocket_ca`) //nolint:errcheck
	if a.SerialNumber.Cmp(b.SerialNumber) != 0 || n != 1 || !a.IsCA {
		t.Fatalf("serials %v %v rows %d", a.SerialNumber, b.SerialNumber, n)
	}
}

func testCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	c, k, err := makeCA("Test shop")
	if err != nil {
		t.Fatal(err)
	}
	return c, k
}

func verifyFor(t *testing.T, ca *x509.Certificate, leaf *tls.Certificate, ip string) {
	t.Helper()
	c, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	if _, err := c.Verify(x509.VerifyOptions{Roots: pool, DNSName: ip}); err != nil {
		t.Fatalf("leaf for %s: %v", ip, err)
	}
}

func TestLeafVerifies(t *testing.T) {
	ca, key := testCA(t)
	leaf, err := makeLeaf(ca, key, []net.IP{net.ParseIP("192.168.1.20")})
	if err != nil {
		t.Fatal(err)
	}
	verifyFor(t, ca, &leaf, "192.168.1.20")
}

func TestLeafFollowsNewIP(t *testing.T) {
	ca, key := testCA(t)
	ip := "192.168.1.20"
	saved, savedEvery := lanIPs, checkEvery
	t.Cleanup(func() { lanIPs, checkEvery = saved, savedEvery })
	lanIPs = func() []net.IP { return []net.IP{net.ParseIP(ip)} }
	checkEvery = 0
	lc := &leafCache{ca: func(context.Context) (*x509.Certificate, *ecdsa.PrivateKey, error) { return ca, key, nil }}
	first, err := lc.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	verifyFor(t, ca, first, "192.168.1.20")
	ip = "10.0.0.7"
	second, err := lc.GetCertificate(nil)
	if err != nil {
		t.Fatal(err)
	}
	verifyFor(t, ca, second, "10.0.0.7")
	if again, _ := lc.GetCertificate(nil); again != second {
		t.Fatal("re-issued with nothing changed")
	}
}

func TestMobileconfig(t *testing.T) {
	ca, _ := testCA(t)
	m := mobileconfig(ca.Raw, "Test shop")
	for _, want := range []string{"com.apple.security.root", base64.StdEncoding.EncodeToString(ca.Raw), "Test shop"} {
		if !strings.Contains(m, want) {
			t.Fatalf("mobileconfig lacks %.40q", want)
		}
	}
}

func TestPublicCertWins(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "shop.example.lk"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"shop.example.lk"}}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	kb, _ := x509.MarshalECPrivateKey(key)
	dir := t.TempDir()
	cf, kf := filepath.Join(dir, "c.pem"), filepath.Join(dir, "k.pem")
	os.WriteFile(cf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)  //nolint:errcheck
	os.WriteFile(kf, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600) //nolint:errcheck
	t.Setenv("POCKET_CERT_FILE", cf)
	t.Setenv("POCKET_KEY_FILE", kf)
	get := certSource(nil)
	c, err := get(nil)
	if err != nil {
		t.Fatal(err)
	}
	if leaf, _ := x509.ParseCertificate(c.Certificate[0]); leaf.Subject.CommonName != "shop.example.lk" {
		t.Fatalf("served %q", leaf.Subject.CommonName)
	}
}
