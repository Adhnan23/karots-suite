package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// SET-11: a USB printer's queue is named after its model, letters, digits, - and _ only.
func TestSET11_USBQueueNames(t *testing.T) {
	got := rawQueues("direct usb://Maker/aAzZ09-_.%20(x)@[`{:?serial=1\ndirect usb://Maker/%20?serial=2\n", "")
	if len(got) != 2 || got[0].name != "aAzZ09-_20x" || got[1].name != "20" {
		t.Fatalf("%+v", got)
	}
	if got := rawQueues("direct usb://Maker/.?serial=1\n", ""); len(got) != 1 || got[0].name != "USB-printer" {
		t.Fatalf("nothing usable left: %+v", got)
	}
}

// SET-11: a main PC that refuses (any answer but 2xx) is an error: nothing prints.
func TestSET11_MainPCRefuses(t *testing.T) {
	for _, code := range []int{http.StatusMultipleChoices, http.StatusForbidden, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			io.WriteString(w, `{"approved":true,"jobs":[{"id":"1","queue":"XP80","data":"c2xpcA=="}]}`) //nolint:errcheck
		}))
		printed := 0
		a := printAgent{base: srv.URL, token: "tok", client: srv.Client(),
			queues: func(context.Context) []string { return []string{"XP80"} },
			spool:  func(context.Context, string, []byte) error { printed++; return nil }}
		approved, err := a.once(context.Background())
		srv.Close()
		if err == nil || approved || printed != 0 || !strings.Contains(err.Error(), "answered") {
			t.Fatalf("%d: approved %v printed %d err %v", code, approved, printed, err)
		}
	}
}

// SET-11: the token is made once, readable by the shop user only, and reused.
func TestSET11_TokenMadeOnce(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	path := filepath.Join(cfg, "karots", "print-helper-token")
	a, err := agentToken()
	if err != nil || len(a) != 64 {
		t.Fatalf("%q %v", a, err)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("token file: %v %v", st, err)
	}
	if b, _ := agentToken(); b != a {
		t.Fatal("a second start made a new token: the main PC would see a new helper")
	}
	os.WriteFile(path, []byte(strings.Repeat("k", 32)+"\n"), 0o600)
	if b, _ := agentToken(); b != strings.Repeat("k", 32) {
		t.Fatalf("a 32-character token was replaced: %q", b)
	}
	os.WriteFile(path, []byte(strings.Repeat("k", 31)), 0o600)
	if b, _ := agentToken(); len(b) != 64 {
		t.Fatalf("a short token was kept: %q", b)
	}
}

// SET-11: on Linux it runs as a service, as the shop user, pointed at the main PC;
// the running copy is stopped before it is replaced.
func TestSET11_LinuxService(t *testing.T) {
	j, rc, r := linuxJob(t, "server", true)
	j.Role, j.Address = RoleCashier, "192.168.1.10:3000"
	if err := linuxPrintHelper(j); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r, "opt/karots-suite", printAgentName)); err != nil {
		t.Fatal("the helper wasn't copied in")
	}
	unit, _ := os.ReadFile(filepath.Join(r, "etc/systemd/system", printAgentName+".service"))
	if !strings.Contains(string(unit), "User=shop\n") || !strings.Contains(string(unit), "ExecStart=/opt/karots-suite/"+printAgentName+" -print-agent 192.168.1.10:3000\n") {
		t.Fatalf("unit:\n%s", unit)
	}
	all := strings.Join(rc.cmds, "\n")
	if want := "systemctl stop " + printAgentName + "\nsystemctl daemon-reload\nsystemctl enable " + printAgentName + "\nsystemctl restart " + printAgentName; all != want {
		t.Fatalf("commands:\n%s", all)
	}
	j, rc, _ = linuxJob(t, "server", true)
	j.Role, j.Address = RoleCashier, "192.168.1.10:3000"
	rc.fail = map[string]error{"systemctl enable": errors.New("no systemd")}
	if err := linuxPrintHelper(j); err == nil {
		t.Fatal("a service that couldn't be enabled was called done")
	}
}

// SET-11: on Windows it runs at the shop user's sign-in, through karots-start.exe.
func TestSET11_WindowsSignInTask(t *testing.T) {
	j, rc := windowsJob(t, "file")
	j.Role, j.Address = RoleCashier, "192.168.1.10:3000"
	if err := windowsPrintHelper(j); err != nil {
		t.Fatal(err)
	}
	all := rc.all()
	if !strings.Contains(all, "# karots:print-helper") || !strings.Contains(all, "-print-agent 192.168.1.10:3000") ||
		!strings.Contains(all, `-AtLogOn -User 'SHOP\owner'`) || !strings.Contains(all, `C:\karots-suite\karots-start.exe`) {
		t.Fatalf("script:\n%s", all)
	}
	j, rc = windowsJob(t, "file")
	j.Role, j.Address = RoleCashier, "192.168.1.10:3000"
	rc.fail = map[string]error{"ps> # karots:stop-launcher": errors.New("denied")}
	if err := windowsPrintHelper(j); err == nil || strings.Contains(rc.all(), "# karots:print-helper") {
		t.Fatalf("no launcher, yet a task: %v", err)
	}
}
