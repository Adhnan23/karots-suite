package aicatalog

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseIdentifyTolerant(t *testing.T) {
	fenced := "Sure! Here is what I found:\n```json\n{\"confident\":true,\"best\":{\"name\":\"6201-2Z Ball Bearing\",\"category\":\"Bearings / Ball Bearings\",\"specs\":\"12x32x10 mm\",\"explanation\":\"A small sealed bearing.\"},\"options\":[]}\n```\nHope that helps."
	r, err := parseIdentify(fenced)
	if err != nil || !r.Confident || r.Best.Name != "6201-2Z Ball Bearing" || r.Best.Category != "Bearings > Ball Bearings" {
		t.Fatalf("fenced + prose: %+v %v", r, err)
	}
	five := `{"confident":false,"options":[{"name":"A"},{"name":"B"},{"name":""},{"name":"C"},{"name":"D"},{"name":"E"}]}`
	if r, err := parseIdentify(five); err != nil || len(r.Options) != 4 || r.Options[3].Name != "D" {
		t.Fatalf("options capped at 4, blanks dropped: %+v %v", r, err)
	}
	// "confident" with no best falls back to options; a lone best becomes one option.
	if r, err := parseIdentify(`{"confident":false,"best":{"name":"X","category":"1/2 inch"}}`); err != nil || len(r.Options) != 1 || r.Options[0].Category != "1/2 inch" {
		t.Fatalf("lone best: %+v %v", r, err)
	}
	for _, bad := range []string{"", "I couldn't find that item, sorry.", "{not json}", `{"confident":false,"options":[]}`} {
		if _, err := parseIdentify(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestParseCleanup(t *testing.T) {
	raw := "```json\n" + `{"renames":[{"id":1,"to":"Drinks","reason":"plural"}],"merges":[],"moves":[],"reparents":[],
	"uncategorized":[{"product_id":7,"category":"Snacks /  Biscuits","reason":"r"}],"names":[],"duplicates":[{"a":3,"b":4,"reason":"same"}]}` + "\n```"
	p, err := parseCleanup(raw)
	if err != nil || len(p.Renames) != 1 || p.Uncategorized[0].Category != "Snacks > Biscuits" || p.Duplicates[0].B != 4 {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := parseCleanup("no plan here"); err != errNoAnswer {
		t.Fatalf("want errNoAnswer, got %v", err)
	}
}

func TestPrompts(t *testing.T) {
	s := Settings{NamingStyle: "Brand Model Size", CustomPrompt: "Use British spelling"}
	full := IdentifyPrompt("6201 ZZ", "8901234", "", []string{"Bearings"}, s)
	for _, want := range []string{"Identify this item: 6201 ZZ", "Barcode: 8901234", "Brand Model Size", "Use British spelling", "- Bearings", jsonOnly} {
		if !strings.Contains(full, want) {
			t.Errorf("full prompt lacks %q", want)
		}
	}
	short := IdentifyFollowUp("6202", "", "it is blue")
	if !strings.HasPrefix(short, "Next item: 6202") || !strings.Contains(short, "it is blue") || strings.Contains(short, "EXISTING") {
		t.Errorf("short prompt: %q", short)
	}
	pid := int64(1)
	cp := CleanupPrompt([]CatRow{{ID: 2, Name: "Ball", ParentID: &pid, Count: 3}}, map[int64]string{2: "Bearings > Ball"},
		[]CleanupProduct{{ID: 9, Name: "Coke", Uncat: true}}, Settings{})
	if !strings.Contains(cp, "2|Ball|1|3|Bearings > Ball") || !strings.Contains(cp, "9|Coke|0|U") || !strings.Contains(cp, namesNone) {
		t.Errorf("cleanup prompt:\n%s", cp)
	}
	if !strings.Contains(CleanupPrompt(nil, nil, nil, s), "Brand Model Size") {
		t.Error("cleanup prompt should carry the naming style for names")
	}
}

func fake(t *testing.T, status int, body string, gotPath *string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body) //nolint:errcheck
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		w.WriteHeader(status)
		io.WriteString(w, body) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

const answer = `{"confident":true,"best":{"name":"Coca-Cola 1L","category":"Drinks"}}`

func TestClientOpenAICompat(t *testing.T) {
	var path string
	srv := fake(t, 200, `{"choices":[{"message":{"role":"assistant","content":"`+strings.ReplaceAll(answer, `"`, `\"`)+`"}}]}`, &path)
	r, err := NewClient(Settings{Provider: "openai", BaseURL: srv.URL + "/v1", APIKey: "k"}).Identify(context.Background(), "coke", "", "", nil)
	if err != nil || r.Best.Name != "Coca-Cola 1L" || path != "/v1/chat/completions" {
		t.Fatalf("%+v %v %s", r, err, path)
	}
}

func TestClientGeminiNative(t *testing.T) {
	var path string
	srv := fake(t, 200, `{"candidates":[{"content":{"parts":[{"text":"{\"confident\":true,"},{"text":"\"best\":{\"name\":\"Coca-Cola 1L\"}}"}]}}]}`, &path)
	r, err := NewClient(Settings{Provider: "gemini", BaseURL: srv.URL + "/v1beta/openai", Model: "m", APIKey: "k"}).Identify(context.Background(), "coke", "", "", nil)
	if err != nil || r.Best.Name != "Coca-Cola 1L" || path != "/v1beta/models/m:generateContent" {
		t.Fatalf("%+v %v %s", r, err, path)
	}
}

func TestClientErrorsSurface(t *testing.T) {
	ctx := context.Background()
	srv := fake(t, 401, `{"error":{"message":"Invalid API key"}}`, nil)
	if err := NewClient(Settings{BaseURL: srv.URL, APIKey: "k"}).Ping(ctx); err == nil || !strings.Contains(err.Error(), "Invalid API key") {
		t.Errorf("openai error: %v", err)
	}
	srv = fake(t, 429, `[{"error":{"message":"Quota exceeded"}}]`, nil)
	if err := NewClient(Settings{Provider: "gemini", BaseURL: srv.URL, APIKey: "k"}).Ping(ctx); err == nil || !strings.Contains(err.Error(), "Quota exceeded") {
		t.Errorf("gemini array error: %v", err)
	}
	if err := NewClient(Settings{BaseURL: "https://api.groq.com/openai/v1"}).Ping(ctx); err == nil || !strings.Contains(err.Error(), "key") {
		t.Errorf("no key, not local: %v", err)
	}
}
