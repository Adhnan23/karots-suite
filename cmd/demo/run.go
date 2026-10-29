package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"karots-suite/internal/db"
	"karots-suite/internal/demo/printer"
	"karots-suite/internal/demo/ui"
	"karots-suite/internal/filedb"
)

// demoDir is where the demo keeps PostgreSQL's programs between runs (unpacking
// them each start costs ~40 s on Windows). Shop data lives in its pg/ folder and
// is wiped at every start and exit.
func demoDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "karots-suite-demo")
}

// prepareDir readies the demo's folder: a PostgreSQL a crashed run left running
// is stopped first (it would live on, and on Windows hold files), then its data goes.
func prepareDir(dir string) error {
	filedb.StopLeftover(dir)
	return wipeData(dir)
}

// wipeData removes everything a previous run left that isn't PostgreSQL's
// programs: the data folder, its password file and logs.
func wipeData(dir string) error {
	for _, n := range []string{"pg", "secret", "runtime", "postgres.log"} {
		if err := os.RemoveAll(filepath.Join(dir, n)); err != nil {
			return err
		}
	}
	return nil
}

// runLauncher is the double-clicked demo: lock → picker → shop.
func runLauncher(port int) int {
	addr, already := claimPort(port)
	if already != "" { // a demo is already open: show it instead of starting another
		openBrowser(already)
		return 0
	}
	if addr == nil {
		log.Printf("demo: no free port near %d", port)
		return 1
	}
	seed, installID, dev := demoSeed()
	if dev {
		log.Printf("demo (dev build): unlock code %s", currentCode(seed))
	}
	dir := demoDir()
	_ = os.MkdirAll(dir, 0o700)
	if err := prepareDir(dir); err != nil {
		log.Printf("demo: clearing the last run: %v", err)
		return 1
	}
	fdb, err := filedb.Open(filedb.Options{Dir: dir, Port: freePort()})
	if err != nil {
		log.Printf("demo: %v", err)
		return 1
	}
	defer func() { _ = fdb.Stop(); _ = wipeData(dir) }()
	pg, err := db.Connect(fdb.URL)
	if err != nil {
		log.Printf("demo: %v", err)
		return 1
	}
	defer pg.Close()
	receipt, err1 := printer.Listen(printer.Receipt)
	label, err2 := printer.Listen(printer.Label)
	if err := errors.Join(err1, err2); err != nil {
		log.Printf("demo: printers: %v", err)
		return 1
	}
	exe, _ := os.Executable()
	r := &childRunner{exe: exe, pg: pg, baseURL: fdb.URL,
		receipt: receipt.Addr, label: label.Addr, logOutput: os.Stderr}
	defer r.Kill() // on the way out: no DROP, the data is wiped anyway — PostgreSQL stops sooner
	l := newLauncher(seed, installID, r, []*printer.Printer{receipt, label}, uiFS())
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	l.exit = func() {
		time.Sleep(300 * time.Millisecond) // let the page get its answer
		select {
		case quit <- os.Interrupt:
		default:
		}
	}
	srv := &http.Server{Handler: l.handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(addr) }()
	url := "http://" + addr.Addr().String() + "/"
	log.Printf("Karots Demo Room: %s (install ID %s)", url, installID)
	openBrowser(url)

	<-quit
	log.Println("demo: closing — nothing is kept")
	return 0
}

// claimPort listens on 127.0.0.1:port or the next free one; if port already
// answers as a Demo Room, it returns that URL instead.
func claimPort(port int) (net.Listener, string) {
	for p := port; p < port+20; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, ""
		}
		c := http.Client{Timeout: time.Second}
		if resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/__demo/lock", p)); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil, fmt.Sprintf("http://127.0.0.1:%d/", p)
			}
		}
	}
	return nil, ""
}

// uiFS is the chrome's built files; nil until web/demo is built. KS_DEMO_UI_DIR
// serves them from disk instead (dev: rebuild the chrome without the binary).
func uiFS() fs.FS {
	if d := os.Getenv("KS_DEMO_UI_DIR"); d != "" {
		return os.DirFS(d)
	}
	return ui.FS()
}

func openBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}
