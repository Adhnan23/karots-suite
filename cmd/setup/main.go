// Command karots-setup installs a shop from the three files the Shop Builder
// makes: karots-setup itself, install.conf and the shop's binary. Double-click
// it: a page opens, press Install. It only applies what the Builder decided —
// it never makes a secret or changes a setting.
//
//	karots-setup                     the page (asks for admin rights first)
//	sudo ./karots-setup -yes         headless: install as the shop's main PC, in the terminal
//	sudo ./karots-setup -yes -role cashier -address 192.168.1.10:3000
//	karots-setup -dry                pretend: the page, every step, nothing installed (for a look)
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/shopconf"
)

func main() {
	if err := start(); err != nil {
		showError(err.Error())
		os.Exit(1)
	}
}

func start() error {
	if len(os.Args) > 1 && os.Args[1] == "-hidden" { // karots-start.exe: no install.conf needed
		runHidden(os.Args[2:])
		return nil
	}
	var (
		yes     = flag.Bool("yes", false, "install without the page (a server with no screen)")
		role    = flag.String("role", "server", "with -yes: server (the shop's main PC) | cashier")
		address = flag.String("address", "", "with -role cashier: the main PC's host[:port]")
		open    = flag.String("open", "", "with -yes: kiosk | background (default: the Shop Builder's)")
		backups = flag.String("backups", "", "with -yes: the backup folder (default: the Shop Builder's)")
		every   = flag.String("backup-every", "", "with -yes: back up every 1h, 6h, 24h… (default: the Shop Builder's)")
		keep    = flag.Int("backup-keep", 0, "with -yes: newest backups kept (default: the Shop Builder's)")
		addr    = flag.String("addr", "", "the page's loopback address (set by the launcher)")
		token   = flag.String("token", "", "the page's token (set by the launcher)")
		userArg = flag.String("user", "", "the shop user (set by the launcher)")
		dry     = flag.Bool("dry", false, "show the page and pretend: nothing is installed")
	)
	flag.Parse()
	if *userArg != "" {
		os.Setenv("KAROTS_SETUP_USER", *userArg)
	}
	if _, err := newJob(Choice{}); err != nil { // install.conf beside us, before anything else
		return err
	}
	if *yes {
		j, err := newJob(Choice{Role: Role(*role), Address: *address, Open: *open, BackupDir: *backups, BackupEvery: *every, BackupKeep: *keep})
		if err != nil {
			return err
		}
		return headless(j)
	}
	if *token == "" && !*dry && !isAdmin() {
		return launch()
	}
	return worker(*addr, *token, *dry)
}

// launch is the double-clicked, not-yet-admin Setup: it asks for admin rights
// (UAC on Windows, the desktop's password box on Linux), starts the admin
// worker on a random loopback port with a random token, opens the page in this
// user's browser once the worker answers, and waits for it.
func launch() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	addr, err := freeAddr()
	if err != nil {
		return err
	}
	tok := randToken()
	wait, err := elevate(exe, []string{"-addr", addr, "-token", tok, "-user", shopUser()})
	if err != nil {
		return err
	}
	url := "http://" + addr + "/?t=" + tok
	c := http.Client{Timeout: time.Second}
	for range 300 { // the password / UAC prompt may take a while
		if res, err := c.Get("http://" + addr + "/api/ping?t=" + tok); err == nil {
			res.Body.Close()
			openBrowser(url)
			return wait()
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("Setup didn't start with admin rights — start it again and answer Yes")
}

// worker serves the page (as admin). Launched: the launcher opens the browser.
// Started as admin directly (sudo, Run as administrator, -dry): it opens it itself.
func worker(addr, tok string, dry bool) error {
	launched := tok != ""
	if !launched {
		tok = randToken()
	}
	if addr == "" {
		a, err := freeAddr()
		if err != nil {
			return err
		}
		addr = a
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	mk, steps := newJob, Steps
	if dry {
		mk, steps = dryJob, drySteps
	}
	idle := time.AfterFunc(30*time.Minute, func() { os.Exit(0) })
	h := pageHandler(tok, mk, steps, func() { time.Sleep(200 * time.Millisecond); os.Exit(0) })
	url := "http://" + addr + "/?t=" + tok
	fmt.Println("Karots Setup:", url)
	if !launched {
		openBrowser(url)
	}
	return http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idle.Reset(30 * time.Minute)
		h.ServeHTTP(w, r)
	}))
}

func freeAddr() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	return ln.Addr().String(), nil
}

func randToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Choice is what the person at the PC picked on the page (or with -yes flags).
// Empty Open / BackupDir keep the Shop Builder's defaults.
type Choice struct {
	Role        Role   `json:"role"`
	Address     string `json:"address"`     // cashier PC: the main PC
	Open        string `json:"open"`        // kiosk | background
	BackupDir   string `json:"backupDir"`   // where automatic backups go
	BackupEvery string `json:"backupEvery"` // 1h, 6h, 24h…
	BackupKeep  int    `json:"backupKeep"`  // newest backups kept
}

// newJob reads install.conf from beside the program and applies this PC's choices.
func newJob(ch Choice) (*Job, error) {
	dir, err := exeDir()
	if err != nil {
		return nil, err
	}
	c, err := loadConf(dir)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(filepath.Join(c.InstallDir, c.Binary))
	updating := err == nil
	var kept []string
	if updating && ch.Role != RoleCashier { // keep this PC's own choices (see installed.go)
		c, kept = applyInstalled(c, readEnvFile(filepath.Join(c.InstallDir, ".env")), installedOpen(c.InstallDir))
	}
	j, err := applyChoice(c, dir, ch)
	if err != nil {
		return nil, err
	}
	j.Updating, j.Kept = updating, kept
	return j, nil
}

// applyChoice fills in the job from install.conf and what was picked here.
func applyChoice(c shopconf.Conf, dir string, ch Choice) (*Job, error) {
	if ch.Role == "" {
		ch.Role = RoleServer
	}
	if ch.Role != RoleServer && ch.Role != RoleCashier {
		return nil, fmt.Errorf("role %q (server | cashier)", ch.Role)
	}
	switch ch.Open {
	case "":
	case "kiosk", "background":
		c.Open = ch.Open
	default:
		return nil, fmt.Errorf("open %q (kiosk | background)", ch.Open)
	}
	env := make(map[string]string, len(c.Env)+3) // never change the shared install.conf's map
	for k, v := range c.Env {
		env[k] = v
	}
	c.Env = env
	if b := strings.TrimSpace(ch.BackupDir); b != "" {
		if !absFor(c.OS, b) {
			return nil, fmt.Errorf("the backup folder must be a full path, like %s", map[bool]string{true: `D:\Karots backups`, false: "/home/backups/karots"}[c.OS == "windows"])
		}
		env["BACKUP_DIR"] = b
	}
	if e := ch.BackupEvery; e != "" {
		if d, err := time.ParseDuration(e); err != nil || d < 30*time.Minute {
			return nil, fmt.Errorf("back up every %q — use 1h, 6h, 24h… (at least 30 minutes)", e)
		}
		env["BACKUP_INTERVAL"] = e
	}
	if k := ch.BackupKeep; k != 0 {
		if k < 1 || k > 999 {
			return nil, fmt.Errorf("keep %d backups — 1 to 999", k)
		}
		env["BACKUP_KEEP"] = strconv.Itoa(k)
	}
	j := &Job{Conf: c, Role: ch.Role, Dir: dir, Run: osRunner{}, Log: os.Stdout, User: shopUser()}
	if ch.Role == RoleCashier {
		if j.Address = withPort(ch.Address, c.Port); j.Address == "" {
			if j.Address = withPort(c.LanAddress, c.Port); j.Address == "" {
				return nil, fmt.Errorf("a cashier PC needs the main PC's address (-address 192.168.1.10)")
			}
		}
	}
	return j, nil
}

// absFor: a full path on the shop's OS (tests build Windows paths on Linux).
func absFor(osName, p string) bool {
	if osName == "windows" {
		return len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') || strings.HasPrefix(p, `\\`)
	}
	return strings.HasPrefix(p, "/")
}

// withPort adds the till port when the address has none.
func withPort(addr string, port int) string {
	addr = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://"))
	addr = strings.TrimRight(addr, "/")
	if addr == "" || strings.Contains(addr, ":") {
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, port)
}

func headless(j *Job) error {
	if !isAdmin() {
		return fmt.Errorf("run it with sudo:  sudo ./karots-setup -yes")
	}
	fmt.Printf("Karots Suite — installing %s (%s)\n", j.Conf.Shop, j.Role)
	err := RunAll(j, func(id string, err error) {
		if err != nil {
			fmt.Printf("✗ %s: %v\n", id, err)
		} else {
			fmt.Printf("✓ %s\n", id)
		}
	})
	if err == nil {
		fmt.Printf("\nDone. The till: %s   (install id %s — support reads the PIN from it)\n", j.tillURL(), j.Conf.InstallID)
	}
	return err
}

func exeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return filepath.Dir(exe), nil
}

// shopUser is who the service, kiosk and shortcuts belong to: the person who
// started Setup, not root / the elevated admin.
func shopUser() string {
	for _, k := range []string{"KAROTS_SETUP_USER", "SUDO_USER"} {
		if u := os.Getenv(k); u != "" && u != "root" {
			return u
		}
	}
	if os.Getenv("USERDOMAIN") != "" {
		return os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	}
	if u := os.Getenv("USER"); u != "root" {
		return u
	}
	return ""
}

// osRunner runs the real tools, logging each command and its output.
type osRunner struct{}

func (osRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return string(out), err
}

func (r osRunner) Script(ps string) (string, error) {
	return r.Run("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
}
