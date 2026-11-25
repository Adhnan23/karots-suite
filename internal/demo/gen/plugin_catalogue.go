package gen

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
)

// cataloguesample is a small published catalogue (index.json + v1.json.gz, 204 real items near the demo shops'
// own products) so a demo shop with the Catalogue plugin has a copy to pick from. Rebuilt from the dev copy by
// hand; it never changes the plugin, which syncs it like the real one.
//
//go:embed cataloguesample
var catalogueSample embed.FS

// ServeCatalogueSample serves the sample on 127.0.0.1 for this process's shop and returns its address (the
// child sets KAROTS_CATALOGUE_URL to it before the server is built).
func ServeCatalogueSample() (string, error) {
	sub, err := fs.Sub(catalogueSample, "cataloguesample")
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go func() { _ = http.Serve(ln, http.FileServerFS(sub)) }()
	return "http://" + ln.Addr().String(), nil
}

// catalogueSetup: the owner presses Check now on the Catalogue page once, and the copy is there.
func catalogueSetup(g *gen) error {
	var st struct {
		Version int    `json:"version"`
		Count   int    `json:"count"`
		Error   string `json:"error"`
	}
	if err := g.admin.Do("POST", "/api/v1/x/catalogue/check", nil, &st); err != nil {
		return err
	}
	if st.Version == 0 || st.Count == 0 {
		return fmt.Errorf("the sample catalogue didn't load: %s", st.Error)
	}
	return nil
}
