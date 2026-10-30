package pocketcashier

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"html"
	"math/big"
	"net"
	"slices"
	"strings"
	"sync"
	"time"
)

// The shop's own certificate authority lives in the database (so a backup and
// restore carry it, and phones that trust it keep working). Phones trust the
// CA once; the server signs its own HTTPS certificate from it for whatever LAN
// IPs it has today, and signs a new one when they change.

// CA loads the shop's CA, making and saving it the first time.
func (s *Store) CA(ctx context.Context) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	var row struct {
		Cert string `db:"cert_pem"`
		Key  string `db:"key_pem"`
	}
	err := s.db.GetContext(ctx, &row, `SELECT cert_pem, key_pem FROM pocket_ca WHERE id = 1`)
	if err == nil {
		return parseCA(row.Cert, row.Key)
	}
	var shop string
	_ = s.db.GetContext(ctx, &shop, `SELECT shop_name FROM settings WHERE id = 1`)
	cert, key, err := makeCA(shop)
	if err != nil {
		return nil, nil, err
	}
	kb, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	// Two first calls at once: one row wins, both read it back.
	if _, err := s.db.ExecContext(ctx, `INSERT INTO pocket_ca (id, cert_pem, key_pem) VALUES (1, $1, $2) ON CONFLICT (id) DO NOTHING`,
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}))); err != nil {
		return nil, nil, err
	}
	return s.CA(ctx)
}

func parseCA(certPEM, keyPEM string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cb, _ := pem.Decode([]byte(certPEM))
	kb, _ := pem.Decode([]byte(keyPEM))
	if cb == nil || kb == nil {
		return nil, nil, fmt.Errorf("the shop certificate is damaged")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, nil, err
	}
	key, err := x509.ParseECPrivateKey(kb.Bytes)
	return cert, key, err
}

func caName(shop string) string {
	if shop = strings.TrimSpace(shop); shop == "" {
		return "Karots Shop CA"
	}
	return "Karots Shop CA — " + shop
}

func makeCA(shop string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: caName(shop), Organization: []string{"Karots"}},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(20, 0, 0),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	return cert, key, err
}

// makeLeaf is the server's HTTPS certificate for these IPs (and localhost).
func makeLeaf(ca *x509.Certificate, caKey *ecdsa.PrivateKey, ips []net.IP) (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial(), Subject: pkix.Name{CommonName: "karots-shop"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(0, 0, 397), // iOS / Chrome limits
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: ips, DNSNames: []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, nil
}

func serial() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	return n
}

// lanIPs is this PC's IPv4 addresses a phone may use (plus 127.0.0.1), without
// container and VM bridges. A var so tests can move the PC.
var lanIPs = func() []net.IP {
	var out []net.IP
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 ||
			slices.ContainsFunc([]string{"docker", "br-", "veth", "virbr", "vmnet", "vboxnet"}, func(p string) bool { return strings.HasPrefix(i.Name, p) }) {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				out = append(out, n.IP.To4())
			}
		}
	}
	return append(out, net.IPv4(127, 0, 0, 1).To4())
}

// checkEvery is how often a handshake looks at the PC's IPs again.
var checkEvery = 30 * time.Second

// leafCache serves the current leaf, signing a new one when the IPs change.
type leafCache struct {
	ca func(context.Context) (*x509.Certificate, *ecdsa.PrivateKey, error)

	mu      sync.Mutex
	cert    *tls.Certificate
	ips     []string
	checked time.Time
}

func (l *leafCache) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cert != nil && time.Since(l.checked) < checkEvery {
		return l.cert, nil
	}
	l.checked = time.Now()
	ips := lanIPs()
	keys := make([]string, len(ips))
	for i, ip := range ips {
		keys[i] = ip.String()
	}
	slices.Sort(keys)
	if l.cert != nil && slices.Equal(keys, l.ips) {
		return l.cert, nil
	}
	ca, key, err := l.ca(context.Background())
	if err != nil {
		return nil, err
	}
	leaf, err := makeLeaf(ca, key, ips)
	if err != nil {
		return nil, err
	}
	l.cert, l.ips = &leaf, keys
	return l.cert, nil
}

// mobileconfig wraps the CA in an Apple profile (unsigned, so iOS says "Not
// Verified"). UUIDs come from the cert hash, so installing again replaces it.
func mobileconfig(der []byte, shop string) string {
	h := sha256.Sum256(der)
	u := func(b []byte) string { return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]) }
	name := html.EscapeString(caName(shop))
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
 <key>PayloadContent</key><array><dict>
  <key>PayloadType</key><string>com.apple.security.root</string>
  <key>PayloadVersion</key><integer>1</integer>
  <key>PayloadIdentifier</key><string>lk.karots.shop-ca.cert</string>
  <key>PayloadUUID</key><string>` + u(h[0:16]) + `</string>
  <key>PayloadDisplayName</key><string>` + name + `</string>
  <key>PayloadCertificateFileName</key><string>karots-shop-ca.cer</string>
  <key>PayloadContent</key><data>` + base64.StdEncoding.EncodeToString(der) + `</data>
 </dict></array>
 <key>PayloadType</key><string>Configuration</string>
 <key>PayloadVersion</key><integer>1</integer>
 <key>PayloadIdentifier</key><string>lk.karots.shop-ca</string>
 <key>PayloadUUID</key><string>` + u(h[16:32]) + `</string>
 <key>PayloadDisplayName</key><string>` + name + `</string>
 <key>PayloadOrganization</key><string>Karots</string>
 <key>PayloadDescription</key><string>Lets this phone open the shop's Karots Suite securely on the shop wifi.</string>
</dict></plist>`
}

func pemOf(der []byte) []byte { return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}) }
