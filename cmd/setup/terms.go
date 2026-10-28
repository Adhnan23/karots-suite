package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	karots "karots-suite"
)

// termsRecord is kept in the install folder: one block per install or Update,
// saying who accepted which terms and when.
const termsRecord = "terms-accepted.txt"

var errTerms = fmt.Errorf("tick “I agree” to the Karots Suite terms first (headless: add -accept-terms)")

// mountTerms serves what the "I agree" tick links to.
func mountTerms(mux *http.ServeMux) {
	mux.HandleFunc("GET /terms", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(karots.License) //nolint:errcheck
	})
	mux.HandleFunc("GET /terms/shop-rules.pdf", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(karots.ShopRulesPDF) //nolint:errcheck
	})
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// termsStep appends this acceptance to the record (the folder exists by now).
func termsStep(j *Job) error {
	host, _ := os.Hostname()
	block := strings.Join([]string{
		"Accepted: " + time.Now().Format(time.RFC1123),
		"Shop: " + j.Conf.Shop,
		"Install ID: " + j.Conf.InstallID,
		"By: " + j.User + " on " + host,
		"Licence and Service Agreement sha256: " + sum(karots.License),
		"Shop Rules & Service Guide sha256: " + sum(karots.ShopRulesPDF),
		"", "",
	}, "\n")
	path := joinOS(j, j.Conf.InstallDir, termsRecord)
	old, _ := os.ReadFile(at(path))
	data := append(old, block...)
	if j.windows() {
		return os.WriteFile(at(path), data, 0o644)
	}
	return writeAs(j, path, data, 0o644)
}
