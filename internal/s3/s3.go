// Package s3 is a small S3-compatible client (Cloudflare R2, MinIO, AWS) signed
// with AWS Signature V4 — standard library only. The Shop Builder publishes
// releases with SignV4; the shop's Karots cloud backup uses Client.
package s3

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ErrNotFound is a Get of a key the bucket doesn't have.
var ErrNotFound = errors.New("not found")

// Client talks to one bucket, path-style: URL is https://host/bucket.
type Client struct {
	URL, KeyID, Secret string
	HTTP               *http.Client // nil = 60 s timeout
}

// Object is one listed key.
type Object struct {
	Key      string
	Size     int64
	Modified time.Time
}

// Put stores body under key.
func (c Client) Put(ctx context.Context, key string, body []byte) error {
	_, err := c.do(ctx, http.MethodPut, "/"+key, nil, body)
	return err
}

// Get reads key; ErrNotFound when it isn't there.
func (c Client) Get(ctx context.Context, key string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/"+key, nil, nil)
}

// List returns every key under prefix (ListObjectsV2, all pages).
func (c Client) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	token := ""
	for {
		q := url.Values{"list-type": {"2"}, "prefix": {prefix}}
		if token != "" {
			q.Set("continuation-token", token)
		}
		raw, err := c.do(ctx, http.MethodGet, "", q, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Truncated bool   `xml:"IsTruncated"`
			Next      string `xml:"NextContinuationToken"`
			Contents  []struct {
				Key          string
				Size         int64
				LastModified time.Time
			}
		}
		if err := xml.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, err)
		}
		for _, o := range page.Contents {
			out = append(out, Object{o.Key, o.Size, o.LastModified})
		}
		if !page.Truncated || page.Next == "" {
			return out, nil
		}
		token = page.Next
	}
}

func (c Client) do(ctx context.Context, method, path string, q url.Values, body []byte) ([]byte, error) {
	u, err := url.Parse(strings.TrimRight(c.URL, "/") + path)
	if err != nil {
		return nil, err
	}
	// SigV4 wants the query sorted and %20-encoded; Values.Encode sorts and uses +.
	u.RawQuery = strings.ReplaceAll(q.Encode(), "+", "%20")
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.ContentLength = int64(len(body))
	SignV4(req, u.Host, "auto", c.KeyID, c.Secret, body, time.Now().UTC())
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound && method == http.MethodGet && path != "" {
		return nil, ErrNotFound
	}
	if res.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, fmt.Errorf("%s %s: %s %s", method, path, res.Status, strings.TrimSpace(string(msg)))
	}
	return io.ReadAll(res.Body)
}

// SignV4 adds AWS Signature V4 headers (service s3) to req. It signs host,
// range and every x-amz-* header.
func SignV4(req *http.Request, host, region, keyID, secret string, body []byte, now time.Time) {
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
