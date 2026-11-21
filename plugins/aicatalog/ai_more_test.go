package aicatalog

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type seen struct{ body, auth string }

// capture answers status/body and records each request's body and Authorization header.
func capture(t *testing.T, status int, body string) (*httptest.Server, *seen) {
	s := &seen{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.body, s.auth = string(b), r.Header.Get("Authorization")
		w.WriteHeader(status)
		io.WriteString(w, body) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv, s
}

// The instructions go as a system message (OpenAI) or system instruction (Gemini), and
// only when there are some: a Ping sends none.
func TestClientSendsSystemOnlyWhenSet(t *testing.T) {
	ctx := context.Background()
	srv, s := capture(t, 200, `{"choices":[{"message":{"content":"ok"}}]}`)
	c := NewClient(Settings{BaseURL: srv.URL, APIKey: "k"})
	if _, err := c.complete(ctx, "be brief", "hi", false); err != nil || !strings.Contains(s.body, `"role":"system"`) || s.auth != "Bearer k" {
		t.Errorf("openai with a system prompt: %v %s %q", err, s.body, s.auth)
	}
	if err := c.Ping(ctx); err != nil || strings.Contains(s.body, `"role":"system"`) {
		t.Errorf("openai ping: %v %s", err, s.body)
	}
	gsrv, g := capture(t, 200, `{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`)
	gc := NewClient(Settings{Provider: "gemini", BaseURL: gsrv.URL, Model: "m", APIKey: "k"})
	if _, err := gc.complete(ctx, "be brief", "hi", false); err != nil || !strings.Contains(g.body, "system_instruction") {
		t.Errorf("gemini with a system prompt: %v %s", err, g.body)
	}
	if err := gc.Ping(ctx); err != nil || strings.Contains(g.body, "system_instruction") {
		t.Errorf("gemini ping: %v %s", err, g.body)
	}
}

// HTTP 400 is a failure on both paths; an unreadable answer is cut to 300 characters.
func TestClientFailuresAndSnippets(t *testing.T) {
	ctx := context.Background()
	srv, _ := capture(t, 400, `{"error":{"message":"bad model"}}`)
	if err := NewClient(Settings{Provider: "gemini", BaseURL: srv.URL, APIKey: "k"}).Ping(ctx); err == nil || !strings.Contains(err.Error(), "bad model") {
		t.Errorf("gemini 400: %v", err)
	}
	long := strings.Repeat("x", 400)
	srv, _ = capture(t, 200, long)
	err := NewClient(Settings{Provider: "gemini", BaseURL: srv.URL, APIKey: "k"}).Ping(ctx)
	if err == nil || strings.Contains(err.Error(), long) || !strings.Contains(err.Error(), strings.Repeat("x", 300)+"…") {
		t.Errorf("a long unreadable answer: %v", err)
	}
}

// The model list: the key as a bearer header, ids sorted, blanks dropped; failures say why.
func TestListModels(t *testing.T) {
	ctx := context.Background()
	srv, s := capture(t, 200, `{"data":[{"id":"b"},{"id":" "},{"id":"a"}]}`)
	ms, err := NewClient(Settings{BaseURL: srv.URL, APIKey: "k"}).ListModels(ctx)
	if err != nil || strings.Join(ms, ",") != "a,b" || s.auth != "Bearer k" {
		t.Errorf("models %v %v %q", ms, err, s.auth)
	}
	srv, s = capture(t, 200, `{"data":[]}`)
	srv.URL = strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	if _, err := NewClient(Settings{BaseURL: srv.URL}).ListModels(ctx); err != nil || s.auth != "" {
		t.Errorf("a local server with no key: %v %q", err, s.auth)
	}
	srv, _ = capture(t, 400, `{"error":{"message":"nope"}}`)
	if _, err := NewClient(Settings{BaseURL: srv.URL, APIKey: "k"}).ListModels(ctx); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("a 400: %v", err)
	}
	srv, _ = capture(t, 200, `not json`)
	if _, err := NewClient(Settings{BaseURL: srv.URL, APIKey: "k"}).ListModels(ctx); err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Errorf("an unreadable list: %v", err)
	}
	if _, err := NewClient(Settings{BaseURL: "http://127.0.0.1:1", APIKey: "k"}).ListModels(ctx); err == nil {
		t.Errorf("nobody listening: no error")
	}
	if _, err := NewClient(Settings{BaseURL: "http://bad host", APIKey: "k"}).ListModels(ctx); err == nil {
		t.Errorf("a bad address: no error")
	}
}
