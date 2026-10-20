package main

import (
	"net/http"

	"karots-suite/internal/folders"
)

// mountFolders is the page's folder browser for "Backups go to", "Also copy
// backups to" and the backup file for "Bring back a shop" (behind the page's
// token, like every route).
func mountFolders(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/folders", func(w http.ResponseWriter, r *http.Request) {
		out, err := folders.List(r.URL.Query().Get("path"), folderRoots())
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /api/folders/new", func(w http.ResponseWriter, r *http.Request) {
		var in struct{ Path, Name string }
		if !decode(w, r, &in) {
			return
		}
		p, err := folders.Make(in.Path, in.Name)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, map[string]string{"path": p})
	})
}

// folderRoots starts from the shop user's home and USB drives (Setup runs as root).
func folderRoots() []folders.Root {
	u := shopUser()
	home := ""
	if u != "" {
		home, _ = homeFn(u)
	}
	return folders.Roots(u, home)
}
