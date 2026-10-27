package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"karots-suite/internal/demo"
	"karots-suite/internal/demo/gen"
	"karots-suite/internal/demo/printer"
	"karots-suite/internal/demo/shops"
)

// shopRunner runs one shop at a time: the real server in a child process.
type shopRunner interface {
	// Start builds the shop and returns once it serves (or fails); progress is
	// called as the history is generated.
	Start(p shops.Profile, progress func(pct int, step string)) error
	Stop() error
	URL() string     // the shop's own server, for the proxy
	Control() string // the child's control server (pause, cheats)
}

type Owner struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// State is what the chrome reads from /__demo/state.
type State struct {
	InstallID string          `json:"installId"`
	Phase     string          `json:"phase"` // picker | building | ready | failed
	Progress  int             `json:"progress"`
	Step      string          `json:"step"`
	Error     string          `json:"error,omitempty"`
	Shop      *shops.Profile  `json:"shop"`
	Paused    bool            `json:"paused"`
	Owner     Owner           `json:"owner"`
	Shops     []shops.Profile `json:"shops"`
	Cheats    []gen.CheatInfo `json:"cheats"`
}

type launcher struct {
	seed      []byte
	installID string
	token     string
	runner    shopRunner
	printers  []*printer.Printer // receipt, label
	ui        fs.FS              // the chrome's built files; nil in tests
	now       func() time.Time

	mu     sync.Mutex
	st     State
	misses []time.Time
	busy   sync.WaitGroup // a start or switch in flight
}

func newLauncher(seed []byte, installID string, r shopRunner, printers []*printer.Printer, ui fs.FS) *launcher {
	return &launcher{
		seed: seed, installID: installID, token: newToken(), runner: r, printers: printers, ui: ui, now: time.Now,
		st: State{InstallID: installID, Phase: "picker", Owner: Owner{demo.OwnerName, demo.OwnerPhone}, Shops: shops.BuiltIn()},
	}
}

func (l *launcher) state() State {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.st
}

func (l *launcher) set(f func(*State)) {
	l.mu.Lock()
	f(&l.st)
	l.mu.Unlock()
}

func (l *launcher) waitIdle() { l.busy.Wait() }

func (l *launcher) handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /__demo/lock", l.page("lock.html"))
	m.HandleFunc("POST /__demo/unlock", l.unlock)
	m.HandleFunc("GET /__demo/install", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"installId": l.installID}) // public: the PIN app needs it
	})
	if l.ui != nil {
		m.Handle("GET /__demo/ui/", http.StripPrefix("/__demo/ui/", http.FileServerFS(l.ui)))
	}
	m.HandleFunc("GET /__demo/state", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, l.state()) })
	m.HandleFunc("GET /__demo/progress", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, l.state()) })
	m.HandleFunc("POST /__demo/start", l.start)
	m.HandleFunc("POST /__demo/switch", l.switchShop)
	m.HandleFunc("POST /__demo/pause", l.pause)
	m.HandleFunc("POST /__demo/cheat/{name}", l.cheat)
	m.HandleFunc("GET /__demo/prints", l.prints)
	m.HandleFunc("GET /__demo/print/{kind}/{id}", l.printPNG)
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if l.state().Phase == "ready" {
			l.proxy(w, r)
			return
		}
		if r.URL.Path != "/" {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no shop running"})
				return
			}
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		l.page("index.html")(w, r) // the picker / progress screen
	})
	return l.guard(m)
}

// page serves one of the chrome's HTML pages (a bare stand-in before Task 9's UI
// is built, so the launcher still works from a plain `go run`).
func (l *launcher) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if l.ui != nil {
			if b, err := fs.ReadFile(l.ui, name); err == nil {
				w.Write(b)
				return
			}
		}
		io.WriteString(w, "<!doctype html><title>Karots Demo</title><p>Karots Demo Room — "+name+"</p>")
	}
}

type startRequest struct {
	Shop   string `json:"shop"`
	Custom *struct {
		Plugins      []string `json:"plugins"`
		Palette      string   `json:"palette"`
		ReceiptStyle string   `json:"receiptStyle"`
	} `json:"custom"`
}

// start builds the chosen shop (built-in or make-your-own).
func (l *launcher) start(w http.ResponseWriter, r *http.Request) {
	var in startRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	var p shops.Profile
	if in.Custom != nil {
		p = shops.Custom(in.Custom.Plugins, in.Custom.Palette, in.Custom.ReceiptStyle)
	} else if b, ok := shops.ByKey(in.Shop); ok {
		p = b
	} else {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no such shop"})
		return
	}
	l.restart(p)
	writeJSON(w, http.StatusAccepted, l.state())
}

func (l *launcher) switchShop(w http.ResponseWriter, r *http.Request) {
	l.waitIdle()
	l.busy.Add(1)
	go func() {
		defer l.busy.Done()
		_ = l.runner.Stop()
		l.set(func(s *State) { *s = State{InstallID: s.InstallID, Owner: s.Owner, Shops: s.Shops, Phase: "picker"} })
	}()
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// control forwards a call to the child's control server.
func (l *launcher) control(path string) (int, []byte) {
	if l.state().Phase != "ready" || l.runner.Control() == "" {
		return http.StatusConflict, []byte(`{"error":"no shop running"}`)
	}
	resp, err := http.Post(l.runner.Control()+path, "application/json", nil)
	if err != nil {
		return http.StatusBadGateway, []byte(`{"error":"the shop didn't answer"}`)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

func (l *launcher) pause(w http.ResponseWriter, r *http.Request) {
	var in struct {
		On bool `json:"on"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	code, body := l.control("/pause?on=" + strconv.FormatBool(in.On))
	if code == http.StatusOK {
		l.set(func(s *State) { s.Paused = in.On })
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(body)
}

func (l *launcher) cheat(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "reset" { // a fresh copy of the same shop
		if sh := l.state().Shop; sh != nil {
			l.restart(*sh)
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
		return
	}
	code, body := l.control("/cheat/" + name + "?" + r.URL.RawQuery)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(body)
}

// restart throws away whatever shop is running and builds p. One at a time: a
// second pick waits for the first to settle, so only one child ever lives.
func (l *launcher) restart(p shops.Profile) {
	l.waitIdle()
	l.busy.Add(1)
	l.set(func(s *State) {
		*s = State{InstallID: s.InstallID, Owner: s.Owner, Shops: s.Shops, Phase: "building", Shop: &p}
	})
	go func() {
		defer l.busy.Done()
		_ = l.runner.Stop()
		err := l.runner.Start(p, func(pct int, step string) { l.set(func(s *State) { s.Progress, s.Step = pct, step }) })
		l.set(func(s *State) {
			if err != nil {
				s.Phase, s.Error = "failed", err.Error()
				return
			}
			s.Phase, s.Progress, s.Cheats = "ready", 100, gen.CheatNames(p)
		})
	}()
}

// prints lists new prints per printer: ?receipt=<last id>&label=<last id>.
func (l *launcher) prints(w http.ResponseWriter, r *http.Request) {
	out := map[string][]printer.Print{}
	for _, p := range l.printers {
		after, _ := strconv.ParseInt(r.URL.Query().Get(string(p.Kind())), 10, 64)
		out[string(p.Kind())] = p.Since(after)
	}
	writeJSON(w, http.StatusOK, out)
}

func (l *launcher) printPNG(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	for _, p := range l.printers {
		if string(p.Kind()) != r.PathValue("kind") {
			continue
		}
		for _, pr := range p.Since(id - 1) {
			if pr.ID == id && pr.PNG != nil {
				w.Header().Set("Content-Type", "image/png")
				w.Write(pr.PNG)
				return
			}
		}
	}
	http.NotFound(w, r)
}
