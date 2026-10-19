package update

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// A download that receives nothing for stallLimit is abandoned; the watchdog
// looks every stallPoll. Variables so tests can shorten them.
var (
	stallLimit = 2 * time.Minute
	stallPoll  = 5 * time.Second
)

// Apply starts installing the known newer release in the background and
// returns at once: on a slow line the download can take minutes. Progress and
// any failure show in Status; when the new binary is in place Restart is called.
func (u *Updater) Apply(_ context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.status.State == StateInstalling {
		return errors.New("an update is already being installed")
	}
	m := u.latest
	if m == nil {
		return errors.New("no update available — check first")
	}
	u.status.State, u.status.Message, u.status.Progress = StateInstalling, "", "Starting…"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		if err := u.apply(ctx, m); err != nil {
			u.mu.Lock()
			u.status.State, u.status.Progress = StateFailed, ""
			u.status.Message = "Update failed, nothing changed: " + err.Error()
			u.mu.Unlock()
			return
		}
		u.setProgress("Restarting…")
		u.Restart()
	}()
	return nil
}

func (u *Updater) setProgress(p string) {
	u.mu.Lock()
	u.status.Progress = p
	u.mu.Unlock()
}

// apply: download to <exe>.new and verify it → backup → swap → mark pending.
func (u *Updater) apply(ctx context.Context, m *Manifest) error {
	newPath := u.Exe + ".new"
	if err := u.download(ctx, m, newPath); err != nil {
		os.Remove(newPath)
		return err
	}
	if u.Backup != nil {
		u.setProgress("Backing up…")
		if err := u.Backup(ctx, m.Version); err != nil {
			os.Remove(newPath)
			return fmt.Errorf("the backup before updating didn't work: %w", err)
		}
	}
	if err := Swap(u.Exe, newPath); err != nil {
		os.Remove(newPath)
		return err
	}
	return markPending(u.Exe, m.Version)
}

func (u *Updater) download(ctx context.Context, m *Manifest, dst string) error {
	if m.File == "" || strings.ContainsAny(m.File, `/\`) {
		return errors.New("bad file name in the update list")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.url(m.File), nil)
	if err != nil {
		return errors.New("bad update address")
	}
	u.setProgress("Downloading…")
	res, err := u.client().Do(req)
	if err != nil {
		return errors.New("download failed — " + reason(err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: the server answered %s", res.Status)
	}
	prog := &progressReader{r: res.Body, total: res.ContentLength, report: u.setProgress, last: time.Now()}
	go func() { // watchdog: give up on a download that has stopped moving
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(stallPoll):
				if prog.idle() > stallLimit {
					cancel()
					return
				}
			}
		}
	}()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	h := sha256.New()
	// Releases are gzipped (a third of the size); the signed hash is of the
	// binary itself, so it is checked after unpacking.
	err = func() error {
		gz, err := gzip.NewReader(prog)
		if err != nil {
			return err
		}
		defer gz.Close()
		_, err = io.Copy(io.MultiWriter(f, h), gz)
		return err
	}()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		if prog.idle() > stallLimit {
			return fmt.Errorf("the download stalled (nothing received for %s) — check the internet and try again", stallLimit)
		}
		return errors.New("download failed — the connection dropped or the file is damaged; try again")
	}
	if hex.EncodeToString(h.Sum(nil)) != m.SHA256 {
		return errors.New("the downloaded file doesn't match the signed release (damaged download?)")
	}
	return nil
}

// progressReader reports "Downloading… N%" as the body is read, and when
// bytes last arrived (for the stall watchdog).
type progressReader struct {
	r      io.Reader
	total  int64
	n, pct int64
	report func(string)
	mu     sync.Mutex
	last   time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.mu.Lock()
		p.last = time.Now()
		p.mu.Unlock()
	}
	p.n += int64(n)
	if p.total > 0 {
		if pct := p.n * 100 / p.total; pct != p.pct {
			p.pct = pct
			p.report(fmt.Sprintf("Downloading… %d%%", pct))
		}
	}
	return n, err
}

func (p *progressReader) idle() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Since(p.last)
}
