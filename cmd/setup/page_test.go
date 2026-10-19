package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const tok = "0123456789abcdef0123456789abcdef"

func testPage(steps func(*Job) []Step) http.Handler {
	mk := func(ch Choice) (*Job, error) {
		j, err := applyChoice(conf("linux", "file", false), "", ch)
		if j != nil {
			j.Run, j.Log = &rec{}, io.Discard
		}
		return j, err
	}
	return pageHandler(tok, mk, steps, func() {})
}

func call(h http.Handler, method, path, body string, token bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token {
		req.Header.Set("X-Setup-Token", tok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPageNeedsToken(t *testing.T) {
	h := testPage(Steps)
	for _, p := range []string{"/", "/api/plan?role=server"} {
		if rec := call(h, "GET", p, "", false); rec.Code != http.StatusForbidden {
			t.Errorf("%s without token: %d", p, rec.Code)
		}
		if rec := call(h, "GET", p+"&t=wrong", "", false); rec.Code != http.StatusForbidden && p != "/" {
			t.Errorf("%s wrong token: %d", p, rec.Code)
		}
	}
	if rec := call(h, "GET", "/?t="+tok, "", false); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>") {
		t.Fatalf("page with token: %d", rec.Code)
	}
}

func TestPagePlan(t *testing.T) {
	rec := call(testPage(Steps), "GET", "/api/plan?role=server", "", true)
	var p struct {
		Shop  string
		Steps []struct{ ID, Title, Detail string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Shop != "Al Raj" || len(p.Steps) == 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, s := range p.Steps {
		if s.ID == "database" {
			t.Fatal("file mode plans a database step")
		}
		if s.Detail == "" {
			t.Fatalf("step %s has no detail", s.ID)
		}
	}
}

func TestPageRunStreams(t *testing.T) {
	steps := func(*Job) []Step {
		return []Step{
			{ID: "a", Title: "A", Do: func(j *Job) error { fmt.Fprintln(j.Log, "doing a"); return nil }},
			{ID: "b", Title: "B", Do: func(*Job) error { return nil }},
		}
	}
	body := call(testPage(steps), "POST", "/api/run", `{"role":"server"}`, true).Body.String()
	for _, want := range []string{"@@STEP a run", "doing a", "@@STEP a ok", "@@STEP b ok", "@@DONE"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	need := func(*Job) []Step {
		return []Step{{ID: "database", Title: "Database", Do: func(*Job) error { return ErrNeedSuperPassword }}}
	}
	body = call(testPage(need), "POST", "/api/run", `{"role":"server"}`, true).Body.String()
	if !strings.Contains(body, "@@STEP database fail") || !strings.Contains(body, "@@NEED superPassword") || strings.Contains(body, "@@DONE") {
		t.Fatalf("need:\n%s", body)
	}
}

func TestPageRefusesTextPlain(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/run", strings.NewReader(`{"role":"server"}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Setup-Token", tok)
	rec := httptest.NewRecorder()
	testPage(Steps).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("%d", rec.Code)
	}
}

// The person at the PC picks kiosk or background; the plan follows it.
func TestPagePlanOpenChoice(t *testing.T) {
	var p struct {
		Open  string
		Steps []struct{ ID, Detail string }
	}
	rec := call(testPage(Steps), "GET", "/api/plan?role=server&open=background", "", true)
	json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Open != "background" {
		t.Fatalf("open %q: %s", p.Open, rec.Body)
	}
	for _, s := range p.Steps {
		if s.ID == "open" && !strings.Contains(s.Detail, "background") {
			t.Fatalf("open step: %q", s.Detail)
		}
	}
}
