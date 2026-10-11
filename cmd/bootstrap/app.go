package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sync"
)

//go:embed app.html
var appPage []byte

// The build stream's last line starts with one of these, so the page can tell
// the log from the outcome.
const (
	resultMark = "@@RESULT "
	errorMark  = "@@ERROR "
)

type appPlugin struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type appState struct {
	Plugins   []appPlugin `json:"plugins"`
	OS        string      `json:"os"`
	Arch      string      `json:"arch"`
	Name      string      `json:"name"`
	Out       string      `json:"out"`
	HasSecret bool        `json:"hasSecret"` // POS_SUPPORT_SECRET set → each shop gets its own support PIN
}

type appBuild struct {
	Plugins   []string `json:"plugins"`
	OS        string   `json:"os"`
	Arch      string   `json:"arch"`
	Name      string   `json:"name"`
	InstallID string   `json:"installId"`
}

// serveApp runs the bootstrap app on addr (loopback only) and opens it in the
// browser. It serves until the process is stopped (Ctrl+C).
func serveApp(addr string, all []manifest, b func(buildOpts, io.Writer) (buildResult, error)) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w (another bootstrap running? pass -addr)", addr, err)
	}
	url := "http://" + ln.Addr().String()
	fmt.Printf("Karots Business Suite — bootstrap\n  open %s   (Ctrl+C to stop)\n", url)
	openBrowser(url)
	return http.Serve(ln, appHandler(all, b))
}

func appHandler(all []manifest, b func(buildOpts, io.Writer) (buildResult, error)) http.Handler {
	byKey := map[string]manifest{}
	st := appState{Plugins: []appPlugin{}, OS: runtime.GOOS, Arch: "amd64", Name: "karots-suite", Out: "dist", HasSecret: os.Getenv("POS_SUPPORT_SECRET") != ""}
	for _, m := range all {
		byKey[m.Key] = m
		st.Plugins = append(st.Plugins, appPlugin{m.Key, m.Name, m.Version, m.Description})
	}
	var busy sync.Mutex // one build at a time: both write the same plugin lists
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(appPage) //nolint:errcheck
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, st) })
	mux.HandleFunc("POST /api/build", func(w http.ResponseWriter, r *http.Request) {
		var in appBuild
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&in); err != nil {
			writeErr(w, http.StatusBadRequest, "bad request: "+err.Error())
			return
		}
		o := buildOpts{OS: in.OS, Arch: in.Arch, Name: in.Name, Out: st.Out, InstallID: in.InstallID}
		for _, k := range in.Plugins {
			m, ok := byKey[k]
			if !ok {
				writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown plugin %q", k))
				return
			}
			o.Plugins = append(o.Plugins, m)
		}
		if err := o.check(); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if !busy.TryLock() {
			writeErr(w, http.StatusConflict, "a build is already running")
			return
		}
		defer busy.Unlock()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		log := flushWriter{w}
		res, err := b(o, log)
		if err != nil {
			fmt.Fprintf(log, "\n%s%s\n", errorMark, err)
			return
		}
		j, _ := json.Marshal(res)
		fmt.Fprintf(log, "\n%s%s\n", resultMark, j)
	})
	return mux
}

// flushWriter pushes each write to the browser so the log streams live.
type flushWriter struct{ w http.ResponseWriter }

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if fl, ok := f.w.(http.Flusher); ok {
		fl.Flush()
	}
	return n, err
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// openBrowser is best-effort: the URL is printed either way.
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
