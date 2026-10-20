package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// AWS's published Signature V4 example ("GET Object", S3 docs, sig-v4-header-based-auth).
func TestSignV4MatchesAWSExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	SignV4(req, "examplebucket.s3.amazonaws.com", "us-east-1",
		"AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", nil,
		time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	const want = "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); !strings.HasSuffix(got, want) {
		t.Fatalf("Authorization = %s\nwant suffix %s", got, want)
	}
}

func TestClientPutGetList(t *testing.T) {
	var mu sync.Mutex
	objs := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=kid/") {
			http.Error(w, "unsigned", http.StatusForbidden)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPut:
			key, ok := strings.CutPrefix(r.URL.Path, "/bucket/")
			if !ok {
				http.Error(w, "path "+r.URL.Path, http.StatusBadRequest)
				return
			}
			objs[key], _ = io.ReadAll(r.Body)
		case r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2":
			if r.URL.Query().Get("prefix") != "shop/" {
				http.Error(w, "prefix", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("continuation-token") == "" {
				fmt.Fprint(w, `<ListBucketResult><IsTruncated>true</IsTruncated><NextContinuationToken>t2</NextContinuationToken>
<Contents><Key>shop/a.enc</Key><Size>3</Size><LastModified>2026-10-20T01:02:03.000Z</LastModified></Contents></ListBucketResult>`)
				return
			}
			fmt.Fprint(w, `<ListBucketResult><IsTruncated>false</IsTruncated>
<Contents><Key>shop/b.enc</Key><Size>5</Size><LastModified>2026-10-21T01:02:03.000Z</LastModified></Contents></ListBucketResult>`)
		case r.Method == http.MethodGet:
			b, ok := objs[strings.TrimPrefix(r.URL.Path, "/bucket/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(b)
		}
	}))
	defer srv.Close()
	c := Client{URL: srv.URL + "/bucket", KeyID: "kid", Secret: "sec"}
	ctx := context.Background()
	if err := c.Put(ctx, "shop/x.enc", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, err := c.Get(ctx, "shop/x.enc")
	if err != nil || string(got) != "hello" {
		t.Fatalf("Get = %q, %v", got, err)
	}
	if _, err := c.Get(ctx, "shop/none"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing Get err = %v, want ErrNotFound", err)
	}
	list, err := c.List(ctx, "shop/")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Key != "shop/a.enc" || list[0].Size != 3 || list[1].Key != "shop/b.enc" ||
		!list[1].Modified.Equal(time.Date(2026, 10, 21, 1, 2, 3, 0, time.UTC)) {
		t.Fatalf("List = %+v", list)
	}
	bad := Client{URL: srv.URL + "/bucket", KeyID: "other", Secret: "sec"}
	if err := bad.Put(ctx, "k", nil); err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("bad key Put err = %v", err)
	}
}
