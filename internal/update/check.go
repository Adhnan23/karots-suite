package update

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// State is what the Data page's Updates card shows.
type State string

const (
	StateUnchecked    State = "unchecked"
	StateUpToDate     State = "up-to-date"
	StateAvailable    State = "available"
	StateNotPublished State = "not-published" // the channel has no manifest yet: calm, not an error
	StateOffline      State = "offline"       // normal at a shop; shown quietly
	StateBadSignature State = "bad-signature" // tampering or a wrong key: a warning
	StateInstalling   State = "installing"
	StateFailed       State = "failed" // an install failed; nothing changed
)

// Status is a copy of the updater's state for the Data page.
type Status struct {
	Current        string // this binary's version
	Kind           string // "test" | "release" | "" (dev)
	Latest         string // "" unless State is available
	Notes          string // what's new since Current
	State          State
	CheckedAt      time.Time
	Message        string // words for the card; never carries the channel
	Progress       string // while installing: "Downloading… 45%", "Backing up…"
	RolledBackFrom string // the version BootGuard gave up on, until dismissed
}

// Updater checks the shop's channel and applies updates to the running binary.
type Updater struct {
	Current string // this binary's version
	Kind    string // "test" | "release"
	BaseURL string // the bucket's public URL (no trailing slash needed)
	Channel string
	PubKey  ed25519.PublicKey
	Exe     string                                          // path of the running binary
	Backup  func(ctx context.Context, version string) error // snapshot taken before swapping
	Restart func()                                          // asks the server to exit with RestartCode
	Client  *http.Client

	mu     sync.Mutex
	status Status
	latest *Manifest
}

func (u *Updater) url(name string) string {
	return strings.TrimRight(u.BaseURL, "/") + "/" + u.Channel + "/" + name
}

func (u *Updater) client() *http.Client {
	if u.Client != nil {
		return u.Client
	}
	// No overall timeout: a shop on a slow line may need many minutes for a
	// binary. Callers bound each request with their context instead.
	return &http.Client{Transport: ipv4First()}
}

// ipv4First dials IPv4 when it can: on some Sri Lankan ISPs large transfers
// over IPv6 stall for good while IPv4 works.
func ipv4First() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	d := &net.Dialer{Timeout: 30 * time.Second}
	t.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		if c, err := d.DialContext(ctx, "tcp4", addr); err == nil {
			return c, nil
		}
		return d.DialContext(ctx, "tcp", addr)
	}
	return t
}

// Status returns a copy of the current state.
func (u *Updater) Status() Status {
	u.mu.Lock()
	defer u.mu.Unlock()
	s := u.status
	s.Current, s.Kind = u.Current, u.Kind
	if s.State == "" {
		s.State = StateUnchecked
	}
	return s
}

var errNotPublished = errors.New("not published")

// Check fetches and verifies the channel's manifest, remembering it when it is
// newer than the running binary. Being offline is just a state here. A check
// while an update is installing leaves the installing state alone.
func (u *Updater) Check(ctx context.Context) error {
	m, err := u.fetchManifest(ctx)
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	u.status.CheckedAt = now
	if u.status.State == StateInstalling {
		return err
	}
	u.latest, u.status.Latest, u.status.Notes, u.status.Message = nil, "", "", ""
	switch {
	case errors.Is(err, errNotPublished):
		u.status.State, u.status.Message = StateNotPublished, "No update published for this shop yet"
	case errors.Is(err, errBadSignature):
		u.status.State, u.status.Message = StateBadSignature, "The update server offered a file not signed by Karots — it was ignored. Tell support."
	case err != nil:
		u.status.State, u.status.Message = StateOffline, "Couldn't reach the update server — "+err.Error()
	case Newer(m.Version, u.Current):
		u.latest = m
		u.status.State, u.status.Latest, u.status.Notes = StateAvailable, m.Version, m.NotesSince(u.Current)
	default:
		u.status.State = StateUpToDate
	}
	return err
}

// fetchManifest gets and verifies the manifest. Its errors never name the URL
// (it carries the channel).
func (u *Updater) fetchManifest(ctx context.Context) (*Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.url("manifest.json"), nil)
	if err != nil {
		return nil, errors.New("bad update address")
	}
	res, err := u.client().Do(req)
	if err != nil {
		return nil, errors.New(reason(err))
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound:
		return nil, errNotPublished
	case res.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("it answered %s", res.Status)
	}
	var m Manifest
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&m); err != nil {
		return nil, errors.New("it sent a damaged update list")
	}
	if err := m.Verify(u.PubKey); err != nil {
		return nil, err
	}
	return &m, nil
}

// reason turns a transport error into words without the URL it carries.
func reason(err error) string {
	var dns *net.DNSError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "it took too long (slow or no internet?)"
	case errors.As(err, &dns):
		return "no internet (couldn't look up the server)"
	default:
		return "no internet?"
	}
}

// Run checks now and then every interval until ctx ends.
func (u *Updater) Run(ctx context.Context, every time.Duration) {
	for {
		_ = u.Check(ctx) // offline is normal; the state shows on the Data page
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}
