package battle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
)

// client calls the shop's real API in-process, signed in as one person — the
// requests a screen makes. (Its own, not the Demo Room's: core never imports it.)
type client struct {
	h      http.Handler
	device string
	cookie []*http.Cookie
}

func newClient(h http.Handler, phone, pin, device string) (*client, error) {
	c := &client{h: h, device: device}
	if err := c.Do("POST", "/api/v1/auth/login", map[string]string{"phone": phone, "pin": pin}, nil); err != nil {
		return nil, fmt.Errorf("sign in %s: %w", phone, err)
	}
	return c, nil
}

// Do sends in as JSON and decodes the reply into out (nil = ignore); a reply
// outside 2xx is an error carrying the API's message.
func (c *client) Do(method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, body)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ks-Device", c.device)
	for _, k := range c.cookie {
		req.AddCookie(k)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	if set := w.Result().Cookies(); len(set) > 0 {
		c.cookie = set
	}
	if w.Code < 200 || w.Code > 299 {
		return fmt.Errorf("%s %s: %d %s", method, path, w.Code, bytes.TrimSpace(w.Body.Bytes()))
	}
	if out == nil || w.Body.Len() == 0 {
		return nil
	}
	return json.Unmarshal(w.Body.Bytes(), out)
}
