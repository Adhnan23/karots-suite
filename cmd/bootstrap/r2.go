package main

import (
	"bytes"
	"context"
	"errors"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// store is where releases go: the R2 bucket, or a fake in tests.
type store interface {
	put(key, path, contentType, cacheControl string) error
	delete(key string) error
}

// r2 uploads to and deletes from an R2 bucket over its S3-compatible API,
// signed with AWS Signature V4 — standard library only (forked from karots-pos
// cmd/release). Progress and retries go to log.
type r2 struct {
	account, keyID, secret, bucket string
	log                            io.Writer
}

var errR2NotSetUp = errors.New("R2 not set up — add R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY and R2_BUCKET to .env")

func r2FromEnv(log io.Writer) (*r2, error) {
	r := &r2{os.Getenv("R2_ACCOUNT_ID"), os.Getenv("R2_ACCESS_KEY_ID"), os.Getenv("R2_SECRET_ACCESS_KEY"), os.Getenv("R2_BUCKET"), log}
	if r.account == "" || r.keyID == "" || r.secret == "" || r.bucket == "" {
		return nil, errR2NotSetUp
	}
	return r, nil
}

// put uploads the file at path, retrying a dropped upload (slow lines).
func (r *r2) put(key, path, contentType, cacheControl string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	for attempt := 1; ; attempt++ {
		h := http.Header{"Content-Type": {contentType}}
		if cacheControl != "" {
			h.Set("Cache-Control", cacheControl)
		}
		err = r.do(http.MethodPut, key, body, h)
		if err == nil || attempt == 3 {
			return err
		}
		fmt.Fprintf(r.log, "\n! upload failed (%v) — retrying (%d/3)\n", err, attempt+1)
	}
}

func (r *r2) delete(key string) error { return r.do(http.MethodDelete, key, nil, http.Header{}) }

// r2Client prefers IPv4: on the release machine's ISP, large uploads over IPv6
// stall for good (packets lost, nothing acknowledged) while IPv4 is fine.
var r2Client = &http.Client{Transport: ipv4First()}

func ipv4First() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	d := &net.Dialer{Timeout: 30 * time.Second}
	t.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		if c, err := d.DialContext(ctx, "tcp4", addr); err == nil {
			return c, nil
		}
		return d.DialContext(ctx, "tcp", addr) // no IPv4 on this machine
	}
	return t
}

// stallLimit: an upload that sends nothing for this long is dropped and retried.
const stallLimit = 90 * time.Second

func (r *r2) do(method, key string, body []byte, h http.Header) error {
	host := r.account + ".r2.cloudflarestorage.com"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prog := &uploadProgress{r: bytes.NewReader(body), total: len(body), show: len(body) > 1<<20, last: time.Now(), log: r.log}
	go func() { // watchdog: cancel a stalled transfer instead of waiting forever
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				if prog.idle() > stallLimit {
					cancel()
					return
				}
			}
		}
	}()
	req, err := http.NewRequestWithContext(ctx, method, "https://"+host+"/"+r.bucket+"/"+key, prog)
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	req.Header = h
	signV4(req, host, "auto", r.keyID, r.secret, body, time.Now().UTC())
	// No overall timeout: on a slow uplink a binary takes minutes; the watchdog
	// catches a transfer that has stopped moving.
	res, err := r2Client.Do(req)
	if err != nil {
		if prog.idle() > stallLimit {
			return fmt.Errorf("upload stalled (nothing sent for %s)", stallLimit)
		}
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("R2 %s %s: %s %s", method, key, res.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

// uploadProgress feeds the request body, printing "  45% of 9.3 MB" for large
// uploads and remembering when bytes last moved (for the stall watchdog).
type uploadProgress struct {
	r        io.Reader
	total, n int
	lastPct  int
	show     bool
	log      io.Writer
	mu       sync.Mutex
	last     time.Time
}

func (p *uploadProgress) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.mu.Lock()
	p.last = time.Now()
	p.mu.Unlock()
	p.n += n
	if !p.show || p.total == 0 {
		return n, err
	}
	if pct := p.n * 100 / p.total; pct != p.lastPct || err == io.EOF {
		if pct/10 != p.lastPct/10 || err == io.EOF { // a line per 10%: the log is a page, not a terminal
			fmt.Fprintf(p.log, "  %3d%% of %.1f MB\n", pct, float64(p.total)/(1<<20))
		}
		p.lastPct = pct
	}
	return n, err
}

func (p *uploadProgress) idle() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Since(p.last)
}

// signV4 adds AWS Signature V4 headers (service s3) to req. It signs host,
// range and every x-amz-* header.
func signV4(req *http.Request, host, region, keyID, secret string, body []byte, now time.Time) {
	sum := sha256.Sum256(body)
	payload := hex.EncodeToString(sum[:])
	amzDate := now.Format("20060102T150405Z")
	day := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payload)

	headers := map[string]string{"host": host}
	for k, v := range req.Header {
		lk := strings.ToLower(k)
		if strings.HasPrefix(lk, "x-amz-") || lk == "range" {
			headers[lk] = strings.TrimSpace(strings.Join(v, ","))
		}
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + headers[k] + "\n")
	}
	signed := strings.Join(names, ";")
	canonical := strings.Join([]string{req.Method, req.URL.EscapedPath(), req.URL.RawQuery,
		canonHeaders.String(), signed, payload}, "\n")

	scope := day + "/" + region + "/s3/aws4_request"
	ch := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(ch[:])
	mac := func(key []byte, s string) []byte {
		m := hmac.New(sha256.New, key)
		m.Write([]byte(s))
		return m.Sum(nil)
	}
	k := mac(mac(mac(mac([]byte("AWS4"+secret), day), region), "s3"), "aws4_request")
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+keyID+"/"+scope+
		", SignedHeaders="+signed+", Signature="+hex.EncodeToString(mac(k, toSign)))
}
