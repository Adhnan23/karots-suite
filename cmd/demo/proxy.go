package main

import (
	"bytes"
	"html"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// proxy forwards to the running shop and adds the demo chrome to every page it
// returns. The shop's server and SPA stay byte-for-byte a shop build's; only
// this layer knows it's a demo.
func (l *launcher) proxy(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(l.runner.URL())
	if err != nil || l.state().Phase != "ready" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	plugins := strings.Join(l.state().Shop.Plugins, ",")
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.Host = pr.In.Host
			pr.Out.Header.Del("Accept-Encoding") // so a page can be rewritten
		},
		ModifyResponse: func(resp *http.Response) error {
			if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
				return nil
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			body = inject(body, plugins)
			resp.Body = io.NopCloser(bytes.NewReader(body))
			resp.ContentLength = int64(len(body))
			resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
			return nil
		},
	}
	rp.ServeHTTP(w, r)
}

// inject puts the shop's plugin list (read by the SPA's plugin gate) and the
// chrome's script and styles just before </head>.
func inject(page []byte, plugins string) []byte {
	tags := `<meta name="ks-plugins" content="` + html.EscapeString(plugins) + `">` +
		`<link rel="stylesheet" href="/__demo/ui/overlay.css">` +
		`<script type="module" src="/__demo/ui/overlay.js"></script>`
	i := bytes.Index(page, []byte("</head>"))
	if i < 0 {
		return page
	}
	out := make([]byte, 0, len(page)+len(tags))
	out = append(out, page[:i]...)
	out = append(out, tags...)
	return append(out, page[i:]...)
}

func itoa(n int) string { return strconv.Itoa(n) }
