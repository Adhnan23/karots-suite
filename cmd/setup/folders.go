package main

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// mountFolders is the page's folder browser for "Backups go to": a browser's
// own folder picker never tells a page the real path, but Setup runs on this
// PC, so it lists folders itself (behind the page's token, like every route).
func mountFolders(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/folders", func(w http.ResponseWriter, r *http.Request) {
		type root struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}
		out := struct {
			Path    string   `json:"path"`
			Parent  string   `json:"parent"`
			Folders []string `json:"folders"`
			Roots   []root   `json:"roots"`
		}{Folders: []string{}, Roots: []root{}}
		for _, rt := range folderRoots() {
			out.Roots = append(out.Roots, root{rt[0], rt[1]})
		}
		p := r.URL.Query().Get("path")
		if p == "" && len(out.Roots) > 0 {
			p = out.Roots[0].Path
		}
		p = filepath.Clean(p)
		for { // not made yet (e.g. inside the install folder): open its nearest parent
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				break
			}
			parent := filepath.Dir(p)
			if parent == p {
				break
			}
			p = parent
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "can't open "+p)
			return
		}
		out.Path = p
		if parent := filepath.Dir(p); parent != p {
			out.Parent = parent
		}
		for _, e := range entries {
			n := e.Name()
			if e.IsDir() && !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "$") && n != "System Volume Information" {
				out.Folders = append(out.Folders, n)
			}
		}
		sort.Slice(out.Folders, func(i, j int) bool { return strings.ToLower(out.Folders[i]) < strings.ToLower(out.Folders[j]) })
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /api/folders/new", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Path, Name string }
		if !decode(w, r, &in) {
			return
		}
		name := strings.TrimSpace(in.Name)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:*?"<>|`) {
			writeErr(w, http.StatusBadRequest, "a folder name can't have / \\ : * ? \" < > |")
			return
		}
		p := filepath.Join(filepath.Clean(in.Path), name)
		if err := os.MkdirAll(p, 0o755); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"path": p})
	})
}

// folderRoots are the places to start from: the drives on Windows; on Linux /,
// the shop user's home and where USB drives appear.
func folderRoots() [][2]string {
	var out [][2]string
	if runtime.GOOS == "windows" {
		for c := 'C'; c <= 'Z'; c++ {
			p := string(c) + `:\`
			if _, err := os.Stat(p); err == nil {
				out = append(out, [2]string{string(c) + ":", p})
			}
		}
		return out
	}
	out = append(out, [2]string{"/", "/"})
	if u := shopUser(); u != "" {
		if home, err := homeFn(u); err == nil {
			out = append(out, [2]string{"Home", home})
		}
		for _, d := range []string{"/media/" + u, "/run/media/" + u} {
			if _, err := os.Stat(d); err == nil {
				out = append(out, [2]string{"USB drives", d})
			}
		}
	}
	if _, err := os.Stat("/mnt"); err == nil {
		out = append(out, [2]string{"/mnt", "/mnt"})
	}
	return out
}
