package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
)

// Client calls the shop's real API in-process, signed in as one person — the
// same requests a screen makes, so the data is what the app would write.
type Client struct {
	h      http.Handler
	device string
	cookie []*http.Cookie
}

// NewClient signs in by phone + PIN.
func NewClient(h http.Handler, phone, pin, device string) (*Client, error) {
	c := &Client{h: h, device: device}
	var out map[string]any
	if err := c.Do("POST", "/api/v1/auth/login", map[string]string{"phone": phone, "pin": pin}, &out); err != nil {
		return nil, fmt.Errorf("sign in %s: %w", phone, err)
	}
	return c, nil
}

// Do sends in as JSON and decodes the reply into out (nil = ignore). A reply
// outside 2xx is an error carrying the API's message.
func (c *Client) Do(method, path string, in, out any) error {
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
