package aicatalog

import (
	"strings"
	"testing"
)

// Gemini's real reply to a spark plug, as pasted 2026-11-05: its inch marks aren't escaped.
const geminiInches = `{"confident":true,"name":"NGK CR7HSA Spark Plug","category":"Automotive & Motorcycle Parts / Ignition","specs":"Thread diameter: 10mm; Reach: 12.7mm (1/2"); Hex size: 16mm (5/8"); Type: Standard resistor spark plug; Vehicle fitment: Popular 4-stroke scooters.","explanation":"This is a standard replacement spark plug."}`

func TestRepairInchMarks(t *testing.T) {
	var got struct {
		Confident bool   `json:"confident"`
		Specs     string `json:"specs"`
		Exp       string `json:"explanation"`
	}
	if err := decodeReply(geminiInches, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Confident || !strings.Contains(got.Specs, `(1/2")`) || !strings.Contains(got.Specs, `(5/8")`) || !strings.HasPrefix(got.Exp, "This is") {
		t.Fatalf("%+v", got)
	}
}

func TestRepairMarkBeforeComma(t *testing.T) {
	var got struct {
		A string `json:"a"`
		B string `json:"b"`
		C []any  `json:"c"`
	}
	raw := `{"a":"Size 12", Hex 16","b":"ok","c":[1,true,"x"]}`
	if err := decodeReply(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.A != `Size 12", Hex 16` || got.B != "ok" || len(got.C) != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestRepairNewlineInString(t *testing.T) {
	var got struct {
		A string `json:"a"`
	}
	if err := decodeReply("{\"a\":\"one\ntwo\"}", &got); err != nil || got.A != "one\ntwo" {
		t.Fatalf("%q %v", got.A, err)
	}
}

func TestRepairLeavesValidJSON(t *testing.T) {
	for _, s := range []string{`{"a":"x \"y\" z","b":[1,2],"c":{"d":null}}`, `{}`} {
		if got := repairQuotes(s); got != s {
			t.Fatalf("%q became %q", s, got)
		}
	}
}

func TestRepairGivesUp(t *testing.T) {
	var got map[string]any
	if err := decodeReply(`{"name":`, &got); err != errNoAnswer {
		t.Fatalf("got %v", err)
	}
}

func TestParseIdentifyRealReply(t *testing.T) {
	// The reply's own flat shape: nothing under "best", so no product is read from it,
	// and the message asks again rather than saying the JSON was unreadable.
	if _, err := parseIdentify(geminiInches); err == nil || err == errNoAnswer {
		t.Fatalf("got %v", err)
	}
	wrapped := `{"confident":true,"best":{"name":"NGK","category":"Parts / Ignition","specs":"Reach (1/2"); Hex (5/8")"}}`
	r, err := parseIdentify(wrapped)
	if err != nil || r.Best.Category != "Parts > Ignition" || !strings.Contains(r.Best.Specs, `(1/2")`) {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestPromptsFitAnyShop(t *testing.T) {
	p := IdentifyPrompt("x", "", "", nil, Settings{CustomPrompt: "We are a bookshop."}, "")
	for _, bad := range []string{"spare-part", "Bearings", "Automotive", "cashier's hint"} {
		if strings.Contains(p, bad) {
			t.Errorf("prompt assumes a kind of shop: %q", bad)
		}
	}
	for _, want := range []string{"We are a bookshop.", "what it fits", "strength"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
}

// ChatGPT's real reply, copied by hand-selecting 2026-11-05: Markdown line breaks and HTML spaces.
const chatgptMarked = "{\\\n&#x20; \"items\": [\\\n&#x20;   {\\\n&#x20;     \"id\": 11,\\\n&#x20;     \"rx_only\": null,\\\n&#x20;     \"controlled\": false,\\\n&#x20;     \"reason\": \"Paracetamol and caffeine combination; not verified.\"\\\n&#x20;   }\\\n&#x20; ]\\\n}"

func TestRepairMarkdownCopy(t *testing.T) {
	var got struct {
		Items []struct {
			ID         int64  `json:"id"`
			RxOnly     *bool  `json:"rx_only"`
			Controlled *bool  `json:"controlled"`
			Reason     string `json:"reason"`
		} `json:"items"`
	}
	if err := decodeReply(chatgptMarked, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != 11 || got.Items[0].RxOnly != nil || got.Items[0].Controlled == nil || !strings.HasPrefix(got.Items[0].Reason, "Paracetamol") {
		t.Fatalf("%+v", got)
	}
}

func TestRepairMarkdownEscapes(t *testing.T) {
	var got struct {
		A []string `json:"rx_only"`
	}
	raw := "{\"rx\\_only\"\u00a0: \\[\"a\\_b &amp; c\", \"Reach (1/2\\\") ok\"\\]}"
	if err := decodeReply(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.A) != 2 || got.A[0] != "a_b & c" || got.A[1] != `Reach (1/2") ok` {
		t.Fatalf("%+v", got)
	}
}
