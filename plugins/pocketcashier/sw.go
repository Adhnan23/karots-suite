package pocketcashier

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"text/template"

	"karots-suite/internal/apperr"
	"karots-suite/internal/spa"

	"github.com/labstack/echo/v4"
)

//go:embed sw.js.tmpl
var swSource string

var swTmpl = template.Must(template.New("sw").Parse(swSource))

// sw is the installed app's service worker: every file of this build's SPA,
// cached on install, so the app opens on the phone with no server.
func (h handlers) sw(c echo.Context) error {
	files := []string{}
	_ = fs.WalkDir(spa.Dist(), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasPrefix(d.Name(), ".") && p != "index.html" {
			files = append(files, "/"+p)
		}
		return nil
	})
	list, _ := json.Marshal(files)
	ver, _ := json.Marshal(h.p.core.Cfg.Version)
	var buf bytes.Buffer
	if err := swTmpl.Execute(&buf, map[string]string{"Version": string(ver), "Files": string(list)}); err != nil {
		return apperr.Internal("failed to build the service worker", err)
	}
	c.Response().Header().Set("Service-Worker-Allowed", "/")
	c.Response().Header().Set("Cache-Control", "no-cache")
	return c.Blob(http.StatusOK, "text/javascript; charset=utf-8", buf.Bytes())
}

type manifestIcon struct {
	Src     string `json:"src"`
	Sizes   string `json:"sizes"`
	Type    string `json:"type"`
	Purpose string `json:"purpose,omitempty"`
}

// manifest makes the phone address installable as "Karots Pocket Cashier".
func (h handlers) manifest(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Content-Type", "application/manifest+json")
	return json.NewEncoder(c.Response()).Encode(map[string]any{
		"name": "Karots Pocket Cashier", "short_name": "Pocket Cashier",
		"start_url": "/x/pocketcashier/", "scope": "/", "display": "standalone", "orientation": "portrait",
		"background_color": "#16132B", "theme_color": "#16132B",
		"icons": []manifestIcon{
			{Src: "/icon-192.png", Sizes: "192x192", Type: "image/png"},
			{Src: "/icon-512.png", Sizes: "512x512", Type: "image/png"},
			{Src: "/icon-maskable-512.png", Sizes: "512x512", Type: "image/png", Purpose: "maskable"},
		},
	})
}
