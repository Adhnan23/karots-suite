package main

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// AWS's published Signature V4 example ("GET Object", S3 docs, sig-v4-header-based-auth).
func TestSignV4MatchesAWSExample(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	req.Header.Set("Range", "bytes=0-9")
	signV4(req, "examplebucket.s3.amazonaws.com", "us-east-1",
		"AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", nil,
		time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC))
	const want = "Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41"
	if got := req.Header.Get("Authorization"); !strings.HasSuffix(got, want) {
		t.Fatalf("Authorization = %s\nwant suffix %s", got, want)
	}
}
