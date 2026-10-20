// Package folders lists the folders on this PC for a "Choose…" button: a
// browser's own picker never tells a page the real path, and the page may be
// on another device, so the program on the PC lists them itself. Setup's page
// and the Data page's copy folder both use it.
package folders

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Root is a place to start from (a drive, Home, USB drives).
type Root struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Listing is one folder: its subfolders and the backups in it (*.json.gz).
type Listing struct {
	Path    string   `json:"path"`
	Parent  string   `json:"parent"`
	Folders []string `json:"folders"`
	Files   []string `json:"files"`
	Roots   []Root   `json:"roots"`
}

// Roots are the drives on Windows; on Linux /, user's home, where USB drives
// appear for user, and /mnt.
func Roots(user, home string) []Root {
	var out []Root
	if runtime.GOOS == "windows" {
		for c := 'C'; c <= 'Z'; c++ {
			p := string(c) + `:\`
			if _, err := os.Stat(p); err == nil {
				out = append(out, Root{string(c) + ":", p})
			}
		}
		return out
	}
	out = append(out, Root{"/", "/"})
	if home != "" {
		out = append(out, Root{"Home", home})
	}
	if user != "" {
		for _, d := range []string{"/media/" + user, "/run/media/" + user} {
			if _, err := os.Stat(d); err == nil {
				out = append(out, Root{"USB drives", d})
			}
		}
	}
	if _, err := os.Stat("/mnt"); err == nil {
		out = append(out, Root{"/mnt", "/mnt"})
	}
	return out
}

// List lists p ("" = the first root); a folder not made yet opens its nearest parent.
func List(p string, roots []Root) (Listing, error) {
	out := Listing{Folders: []string{}, Files: []string{}, Roots: roots}
	if out.Roots == nil {
		out.Roots = []Root{}
	}
	if p == "" && len(roots) > 0 {
		p = roots[0].Path
	}
	p = filepath.Clean(p)
	for {
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
		return out, errors.New("can't open " + p)
	}
	out.Path = p
	if parent := filepath.Dir(p); parent != p {
		out.Parent = parent
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() && !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "$") && n != "System Volume Information" {
			out.Folders = append(out.Folders, n)
		} else if !e.IsDir() && strings.HasSuffix(n, ".json.gz") {
			out.Files = append(out.Files, n)
		}
	}
	sort.Slice(out.Folders, func(i, j int) bool { return strings.ToLower(out.Folders[i]) < strings.ToLower(out.Folders[j]) })
	sort.Strings(out.Files)
	return out, nil
}

// Make creates folder name inside dir and returns its path.
func Make(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:*?"<>|`) {
		return "", errors.New(`a folder name can't have / \ : * ? " < > |`)
	}
	p := filepath.Join(filepath.Clean(dir), name)
	return p, os.MkdirAll(p, 0o755)
}
