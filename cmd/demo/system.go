package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"karots-suite/internal/support"
)

// systemSignIn (a cheat): signs this browser in as the shop's hidden system user and
// lets it open Appearance, to show a customer the palettes, receipt styles and Sell
// page layouts. The launcher works out this hour's system PIN itself, so the PIN (the
// same code that unlocks the Demo Room) never reaches the browser.
func (l *launcher) systemSignIn(w http.ResponseWriter, r *http.Request) {
	if l.state().Phase != "ready" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the shop isn't ready yet"})
		return
	}
	seed, _, _ := demoSeed()
	pin := support.Code(seed, time.Now())
	if p := os.Getenv("POS_SYSTEM_PIN"); p != "" { // the child honours the same override
		pin = p
	}
	body, _ := json.Marshal(map[string]string{"phone": "0000000001", "pin": pin})
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, l.runner.URL()+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ks-Device", r.Header.Get("X-Ks-Device"))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the system user could not sign in (" + res.Status + ")"})
		return
	}
	for _, c := range res.Header.Values("Set-Cookie") {
		w.Header().Add("Set-Cookie", c)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
