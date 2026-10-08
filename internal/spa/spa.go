package spa

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

// The web app manifest (installable app) has no type in Go's table.
func init() { _ = mime.AddExtensionType(".webmanifest", "application/manifest+json") }

// Handler serves real files; any other path that is not an asset gets
// index.html so client-side routes survive a reload. /api/* never reaches here
// (Echo matches the more specific /api/* route first).
func Handler(fsys fs.FS) echo.HandlerFunc {
	files := http.FileServer(http.FS(fsys))
	return func(c echo.Context) error {
		p := strings.TrimPrefix(path.Clean(c.Request().URL.Path), "/")
		if p != "" && p != "." {
			if st, err := fs.Stat(fsys, p); err == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(c.Response(), c.Request())
				return nil
			}
			if strings.HasPrefix(p, "assets/") || path.Ext(p) != "" {
				return echo.NewHTTPError(http.StatusNotFound, "Not Found")
			}
		}
		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "frontend not built — run `make web`")
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.HTMLBlob(http.StatusOK, index)
	}
}
