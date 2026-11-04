package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"karots-suite/internal/printing"
)

// The print helper on a cashier PC (Setup's own copy, run with -print-agent): it
// calls the main PC, reports this PC's printers and prints the jobs it is given.
// Nothing listens here. The main PC's admin approves it in Settings → Printers.
const (
	printAgentFlag   = "-print-agent"
	printAgentHeader = "X-Ks-Print-Agent" // internal/api printAgentHeader
	printAgentName   = "karots-print-helper"
)

type printAgent struct {
	base, token, name string
	client            *http.Client
	queues            func(context.Context) []string
	spool             func(context.Context, string, []byte) error
}

type agentJob struct {
	ID    string `json:"id"`
	Queue string `json:"queue"`
	Data  []byte `json:"data"`
}

func runPrintAgent(address string) error {
	token, err := agentToken()
	if err != nil {
		return err
	}
	name, _ := os.Hostname()
	a := printAgent{base: "http://" + address, token: token, name: name, client: &http.Client{Timeout: 40 * time.Second},
		queues: printing.Queues, spool: printing.Raw}
	for {
		approved, err := a.once(context.Background())
		if err != nil {
			log.Printf("print helper: %v", err)
		}
		if err != nil || !approved {
			time.Sleep(10 * time.Second)
		}
	}
}

// once is one call to the main PC, and any jobs it hands over.
func (a printAgent) once(ctx context.Context) (approved bool, err error) {
	have := a.queues(ctx)
	var reply struct {
		Approved bool       `json:"approved"`
		Jobs     []agentJob `json:"jobs"`
	}
	if err := a.post(ctx, "poll", map[string]any{"name": a.name, "printers": have}, &reply); err != nil {
		return false, err
	}
	for _, j := range reply.Jobs {
		failure := ""
		if !slices.Contains(have, j.Queue) { // only this PC's own printers, never a target it is told
			failure = "no printer called " + j.Queue + " on this PC"
		} else {
			c, cancel := context.WithTimeout(ctx, 20*time.Second)
			if err := a.spool(c, j.Queue, j.Data); err != nil {
				failure = err.Error()
			}
			cancel()
		}
		if err := a.post(ctx, "done", map[string]string{"id": j.ID, "error": failure}, nil); err != nil {
			return reply.Approved, err
		}
	}
	return reply.Approved, nil
}

func (a printAgent) post(ctx context.Context, path string, body, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+"/api/v1/print-agent/"+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(printAgentHeader, a.token)
	res, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 300))
		return fmt.Errorf("main PC answered %d: %s", res.StatusCode, msg)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// agentToken is this PC's identity: made once, kept in the user's config folder.
func agentToken() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "karots", "print-helper-token")
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) >= 32 {
		return strings.TrimSpace(string(b)), nil
	}
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b[:])
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return token, os.WriteFile(path, []byte(token), 0o600)
}

type rawQueue struct{ name, uri string }

// rawQueues reads `lpinfo -v` and `lpstat -v`: the USB printers with no queue yet.
func rawQueues(devices, have string) []rawQueue {
	var out []rawQueue
	taken := func(name string) bool {
		return strings.Contains(have, "device for "+name+":") || slices.ContainsFunc(out, func(q rawQueue) bool { return q.name == name })
	}
	for _, line := range strings.Split(devices, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || f[0] != "direct" || !strings.HasPrefix(f[1], "usb://") || strings.Contains(have, f[1]) {
			continue
		}
		path, _, _ := strings.Cut(strings.TrimPrefix(f[1], "usb://"), "?")
		name := strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
				return r
			}
			return -1
		}, path[strings.LastIndex(path, "/")+1:])
		if name == "" {
			name = "USB-printer"
		}
		for base, n := name, 2; taken(name); n++ {
			name = fmt.Sprintf("%s-%d", base, n)
		}
		out = append(out, rawQueue{name, f[1]})
	}
	return out
}

// linuxPrintHelper copies Setup in and runs it as a service, as the shop user.
func linuxPrintHelper(j *Job) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(at(j.Conf.InstallDir), 0o755); err != nil {
		return err
	}
	bin := joinOS(j, j.Conf.InstallDir, printAgentName)
	run(j, "systemctl", "stop", printAgentName) //nolint:errcheck // an Update: the running copy can't be replaced
	if err := copyFile(self, at(bin), 0o755); err != nil {
		return fmt.Errorf("copy the print helper in: %w", err)
	}
	unit := fmt.Sprintf(`[Unit]
Description=Karots Suite print helper
After=network-online.target cups.service
Wants=network-online.target

[Service]
Type=simple
User=%s
ExecStart=%s %s %s
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
`, j.User, bin, printAgentFlag, j.Address)
	path := at("/etc/systemd/system/" + printAgentName + ".service")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(unit), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", printAgentName}, {"restart", printAgentName}} {
		if _, err := run(j, "systemctl", args...); err != nil {
			return err
		}
	}
	return nil
}

func linuxRemovePrintHelper(j *Job) error {
	run(j, "systemctl", "disable", "--now", printAgentName) //nolint:errcheck // none = nothing to stop
	os.Remove(at("/etc/systemd/system/" + printAgentName + ".service"))
	os.Remove(at(joinOS(j, j.Conf.InstallDir, printAgentName)))
	_, err := run(j, "systemctl", "daemon-reload")
	return err
}

const windowsPrintTask = "KarotsPrintHelper"

// windowsPrintHelper runs the helper at sign-in, in the user's session like the
// till itself: that's where it sees their USB printers.
func windowsPrintHelper(j *Job) error {
	launcher, err := ensureLauncher(j)
	if err != nil {
		return err
	}
	args := fmt.Sprintf(`-hidden -log "%s" "%s" %s %s`, joinOS(j, j.Conf.InstallDir, "logs", "print-helper.log"), launcher, printAgentFlag, j.Address)
	_, err = ps(j, fmt.Sprintf(`# karots:print-helper
$action = New-ScheduledTaskAction -Execute %s -Argument %s -WorkingDirectory %s
$trigger = New-ScheduledTaskTrigger -AtLogOn -User %s
$principal = New-ScheduledTaskPrincipal -UserId %s -LogonType Interactive
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName %s -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Start-ScheduledTask -TaskName %s`, psq(launcher), psq(args), psq(j.Conf.InstallDir), psq(j.User), psq(j.User), windowsPrintTask, windowsPrintTask))
	return err
}

func windowsRemovePrintHelper(j *Job) error {
	_, err := ps(j, fmt.Sprintf(`# karots:remove-print-helper
Stop-ScheduledTask -TaskName %s -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName %s -Confirm:$false -ErrorAction SilentlyContinue
exit 0`, windowsPrintTask, windowsPrintTask))
	return err
}
