package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"

	"karots-suite/internal/demo/shops"
)

// childRunner runs each shop as `<this binary> -child` on its own fresh
// database in the launcher's PostgreSQL. A fresh process every time means every
// plugin registers exactly as in a shop build — the karots-pos demo's failure.
type childRunner struct {
	exe       string   // this binary (a test passes a built one)
	pg        *sqlx.DB // the launcher's connection, for CREATE / DROP DATABASE
	baseURL   string   // the launcher database's URL; the child gets a sibling
	jwt       string
	receipt   string
	label     string
	logOutput io.Writer

	mu      sync.Mutex
	n       int
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	dbName  string
	url     string
	control string
}

func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// siblingURL is baseURL pointing at database name.
func siblingURL(base, name string) string {
	i := strings.LastIndex(base, "/")
	q := ""
	if j := strings.Index(base[i:], "?"); j >= 0 {
		q = base[i+j:]
	}
	return base[:i+1] + name + q
}

func (r *childRunner) Start(p shops.Profile, progress func(int, string)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.n++
	name := "shop_" + strconv.Itoa(r.n)
	if _, err := r.pg.Exec(`CREATE DATABASE ` + name); err != nil {
		return fmt.Errorf("create the shop's database: %w", err)
	}
	r.dbName = name
	_, _ = r.pg.Exec(`ALTER DATABASE ` + name + ` SET synchronous_commit = off`) // throwaway data
	port, ctl := freePort(), freePort()
	args, _ := json.Marshal(childArgs{DB: siblingURL(r.baseURL, name), Port: port, Control: ctl, JWTSecret: r.jwt,
		ReceiptPrinter: r.receipt, LabelPrinter: r.label, Shop: p, Staff: p.Staff})
	cmd := exec.Command(r.exe, "-child", string(args))
	cmd.Stderr = r.logOutput
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if r.stdin, err = cmd.StdinPipe(); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start the shop: %w", err)
	}
	r.cmd = cmd
	ready := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			f := strings.SplitN(sc.Text(), " ", 3)
			switch {
			case f[0] == "ready":
				ready <- true
			case f[0] == "progress" && len(f) == 3:
				pct, _ := strconv.Atoi(f[1])
				progress(pct, f[2])
			}
		}
		ready <- false // the child ended
	}()
	select {
	case ok := <-ready:
		if !ok {
			r.stopLocked()
			return errors.New("the shop stopped while it was being built — see the demo's log")
		}
	case <-time.After(5 * time.Minute):
		r.stopLocked()
		return errors.New("the shop took too long to build")
	}
	r.url = fmt.Sprintf("http://127.0.0.1:%d", port)
	r.control = fmt.Sprintf("http://127.0.0.1:%d", ctl)
	return nil
}

func (r *childRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopLocked()
}

// stopLocked ends the child and drops its database.
func (r *childRunner) stopLocked() error {
	if r.cmd != nil {
		r.stdin.Close() // the child exits when its stdin closes…
		done := make(chan struct{})
		go func() { r.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second): // …or is made to
			_ = r.cmd.Process.Kill()
			<-done
		}
		r.cmd, r.url, r.control = nil, "", ""
	}
	if r.dbName == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := r.pg.ExecContext(ctx, `DROP DATABASE IF EXISTS `+r.dbName+` WITH (FORCE)`)
	r.dbName = ""
	return err
}

func (r *childRunner) URL() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.url
}

func (r *childRunner) Control() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.control
}
