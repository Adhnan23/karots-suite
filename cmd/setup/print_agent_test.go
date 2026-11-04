package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRawQueuesOnlyNewUSBPrinters(t *testing.T) {
	devices := `network ipp
direct usb://Printer/POS-80?serial=197B04426832
direct usb://Xprinter/XP-365B?serial=0020416C83A2
direct hp
network socket
`
	have := "device for POS80: usb://Printer/POS-80?serial=197B04426832\n"
	got := rawQueues(devices, have)
	if len(got) != 1 || got[0].name != "XP-365B" || !strings.HasPrefix(got[0].uri, "usb://Xprinter/") {
		t.Fatalf("%+v", got)
	}
	// Two of one model need two names; a taken name isn't used again.
	two := rawQueues("direct usb://A/XP-80?serial=1\ndirect usb://A/XP-80?serial=2\n", "device for XP-80: tcp://x\n")
	if len(two) != 2 || two[0].name != "XP-80-2" || two[1].name != "XP-80-3" {
		t.Fatalf("%+v", two)
	}
}

func TestPrintAgentPrintsOnlyItsOwnQueues(t *testing.T) {
	var answers []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(printAgentHeader) != "tok" {
			t.Error("no token")
		}
		if strings.HasSuffix(r.URL.Path, "/poll") {
			io.WriteString(w, `{"approved":true,"jobs":[{"id":"1","queue":"XP80","data":"c2xpcA=="},{"id":"2","queue":"tcp://evil:9100","data":"c2xpcA=="},{"id":"3","queue":"XP365","data":"c2xpcA=="}]}`) //nolint:errcheck
			return
		}
		b, _ := io.ReadAll(r.Body)
		answers = append(answers, string(b))
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	var printed []string
	a := printAgent{base: srv.URL, token: "tok", name: "Laptop", client: srv.Client(),
		queues: func(context.Context) []string { return []string{"XP80", "XP365"} },
		spool: func(_ context.Context, q string, d []byte) error {
			if q == "XP365" {
				return errors.New("out of paper")
			}
			printed = append(printed, q+":"+string(d))
			return nil
		}}
	approved, err := a.once(context.Background())
	if err != nil || !approved {
		t.Fatal(approved, err)
	}
	if len(printed) != 1 || printed[0] != "XP80:slip" {
		t.Fatalf("printed %v", printed)
	}
	if len(answers) != 3 || !strings.Contains(answers[0], `"error":""`) || !strings.Contains(answers[1], "no printer called") || !strings.Contains(answers[2], "out of paper") {
		t.Fatalf("answers %v", answers)
	}
}
