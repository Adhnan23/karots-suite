package aicatalog

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Load models / Test use the provider as typed; a blank key means the stored one.
func TestAPITypedSettings(t *testing.T) {
	_, do := testAPI(t, freshDB(t, "aicatalog_api_typed"))
	if rec := do("PUT", "/settings", `{"manualMode":true,"provider":"openai","baseUrl":"x","model":"m","apiKey":"stored"}`); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	srv, s := capture(t, 200, `{"data":[{"id":"m1"}]}`)
	ask := func(key string) int {
		return do("POST", "/settings/models", fmt.Sprintf(`{"provider":"openai","baseUrl":%q,"model":"m","apiKey":%q}`, srv.URL, key)).Code
	}
	if c := ask(""); c != 200 || s.auth != "Bearer stored" {
		t.Errorf("a blank key: %d %q", c, s.auth)
	}
	if c := ask(" typed "); c != 200 || s.auth != "Bearer typed" {
		t.Errorf("a typed key: %d %q", c, s.auth)
	}
	if rec := do("POST", "/settings/models", `[`); rec.Code != 422 {
		t.Errorf("a bad body: %d", rec.Code)
	}
	bad, _ := capture(t, 400, `{"error":{"message":"no such model"}}`)
	if rec := do("POST", "/settings/models", fmt.Sprintf(`{"baseUrl":%q,"apiKey":"k"}`, bad.URL)); rec.Code != 422 || !strings.Contains(rec.Body.String(), "no such model") {
		t.Errorf("models failing: %d %s", rec.Code, rec.Body)
	}
	ok, _ := capture(t, 200, `{"choices":[{"message":{"content":"ok"}}]}`)
	if rec := do("POST", "/settings/test", fmt.Sprintf(`{"baseUrl":%q,"apiKey":"k"}`, ok.URL)); rec.Code != 204 {
		t.Errorf("test answered: %d %s", rec.Code, rec.Body)
	}
	if rec := do("POST", "/settings/test", fmt.Sprintf(`{"baseUrl":%q,"apiKey":"k"}`, bad.URL)); rec.Code != 422 {
		t.Errorf("test failing: %d", rec.Code)
	}
	if rec := do("POST", "/settings/test", `[`); rec.Code != 422 {
		t.Errorf("test with a bad body: %d", rec.Code)
	}
}

// A clean-up answer keeps real changes only: a rename to the same or no name is dropped,
// a reparent to the top is kept, to itself or a missing category dropped. When the AI
// fails, the prompt comes back to copy. A rename of 80 characters applies, 81 doesn't.
func TestAPICleanupKeepsRealChanges(t *testing.T) {
	db := freshDB(t, "aicatalog_api_cleanup_filters")
	_, do := testAPI(t, db)
	top := cat(t, db, "Drinks", nil)
	sub := cat(t, db, "Cold", top)
	prod(t, db, "Coke 1L", sub)
	reply := fmt.Sprintf(`{"renames":[{"id":%d,"to":"Cold"},{"id":%d,"to":"  "},{"id":%d,"to":"Chilled"}],
		"reparents":[{"id":%d,"parent_id":0},{"id":%[4]d,"parent_id":%[4]d},{"id":%[4]d,"parent_id":999999}]}`, sub, sub, sub, sub)
	body, _ := json.Marshal(CleanupRequest{Reply: reply})
	r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", string(body)))
	if r.Result == nil {
		t.Fatalf("%+v", r)
	}
	pl := r.Result.Plan
	if len(pl.Renames) != 1 || pl.Renames[0].To != "Chilled" || len(pl.Reparents) != 1 || pl.Reparents[0].ParentID != 0 {
		t.Errorf("kept: renames %+v reparents %+v", pl.Renames, pl.Reparents)
	}
	apply := func(to string) string {
		var name string
		ap := fmt.Sprintf(`{"plan":{"renames":[{"id":%d,"to":%q}]}}`, top, to)
		if rec := do("POST", "/cleanup/apply", ap); rec.Code != 200 {
			t.Fatalf("apply: %d %s", rec.Code, rec.Body)
		}
		if err := db.Get(&name, `SELECT name FROM categories WHERE id = $1`, top); err != nil {
			t.Fatal(err)
		}
		return name
	}
	if n := strings.Repeat("a", 80); apply(n) != n {
		t.Errorf("an 80-character name wasn't applied")
	}
	if apply(strings.Repeat("b", 81)) == strings.Repeat("b", 81) {
		t.Errorf("an 81-character name was applied")
	}
	srv, _ := capture(t, 400, `{"error":{"message":"quota"}}`)
	if rec := do("PUT", "/settings", fmt.Sprintf(`{"manualMode":false,"provider":"openai","baseUrl":%q,"model":"m","apiKey":"k"}`, srv.URL)); rec.Code != 200 {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body)
	}
	if r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", `{"offset":0}`)); r.Result != nil || r.Prompt == "" || !strings.Contains(r.Error, "quota") {
		t.Errorf("the AI failing: %+v", r)
	}
}

// Clean-up goes 150 products a part; a run left past the shop's end starts again.
// Product names keep real changes only; of two duplicates with the same sales the older is kept;
// a product name of 150 characters applies.
func TestAPICleanupPartsNamesAndKeeps(t *testing.T) {
	db := freshDB(t, "aicatalog_api_cleanup_parts")
	_, do := testAPI(t, db)
	c := cat(t, db, "Bulbs", nil)
	var ids []int64
	for i := range 151 {
		ids = append(ids, prod(t, db, fmt.Sprintf("Bulb %03d", i), c))
	}
	part := func(off int) CleanupPreview {
		body, _ := json.Marshal(CleanupRequest{Offset: off, Reply: `{"renames":[]}`})
		r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", string(body)))
		if r.Result == nil {
			t.Fatalf("offset %d: %+v", off, r)
		}
		return *r.Result
	}
	if p := part(0); p.Chunk != 1 || p.Chunks != 2 || p.Next != 150 {
		t.Errorf("part 1: %d of %d, next %d", p.Chunk, p.Chunks, p.Next)
	}
	if p := part(150); p.Chunk != 2 || p.Next != -1 {
		t.Errorf("part 2: %d, next %d", p.Chunk, p.Next)
	}
	if p := part(151); p.Chunk != 1 {
		t.Errorf("an offset at the end starts again: part %d", p.Chunk)
	}
	a, b := ids[0], ids[1]
	reply := fmt.Sprintf(`{"names":[{"product_id":%d,"to":"Bulb 000"},{"product_id":%d,"to":" "},{"product_id":%d,"to":"LED Bulb 9W"}],
		"duplicates":[{"a":%d,"b":%d,"reason":"same bulb"}]}`, a, a, b, b, a)
	body, _ := json.Marshal(CleanupRequest{Reply: reply})
	r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", string(body)))
	if r.Result == nil {
		t.Fatalf("%+v", r)
	}
	if n := r.Result.Plan.Names; len(n) != 1 || n[0].To != "LED Bulb 9W" {
		t.Errorf("names kept: %+v", n)
	}
	if d := r.Result.Duplicates; len(d) != 1 || d[0].Keep.ID != a || d[0].Drop.ID != b {
		t.Errorf("same sales keeps the older: %+v", d)
	}
	long := strings.Repeat("n", 150)
	if rec := do("POST", "/cleanup/apply", fmt.Sprintf(`{"plan":{"names":[{"product_id":%d,"to":%q}]}}`, b, long)); rec.Code != 200 {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body)
	}
	var name string
	if err := db.Get(&name, `SELECT name FROM products WHERE id = $1`, b); err != nil || name != long {
		t.Errorf("a 150-character name wasn't applied: %q %v", name, err)
	}
}

// A product's hint through the API: none reads as blank, a saved one reads back capped
// at 2,000 characters, a missing product is a 404, a bad id or body a 400.
func TestAPIHints(t *testing.T) {
	db := freshDB(t, "aicatalog_api_hints")
	_, do := testAPI(t, db)
	pid := prod(t, db, "Bearing", cat(t, db, "Parts", nil))
	path := fmt.Sprintf("/items/%d", pid)
	if h := decode[HintBody](t, do("GET", path, "")); h.Specs != "" || h.Explanation != "" {
		t.Errorf("no hint yet: %+v", h)
	}
	exact, over := strings.Repeat("s", 2000), strings.Repeat("e", 2001)
	body, _ := json.Marshal(HintBody{Specs: exact, Explanation: over})
	if h := decode[HintBody](t, do("PUT", path, string(body))); h.Specs != exact || h.Explanation != over[:2000] {
		t.Errorf("saved: %d / %d characters", len(h.Specs), len(h.Explanation))
	}
	if h := decode[HintBody](t, do("GET", path, "")); h.Specs != exact || len(h.Explanation) != 2000 {
		t.Errorf("read back: %d / %d characters", len(h.Specs), len(h.Explanation))
	}
	for _, bad := range []string{"/items/0", "/items/-3", "/items/abc"} {
		if rec := do("GET", bad, ""); rec.Code != 400 {
			t.Errorf("GET %s: %d", bad, rec.Code)
		}
	}
	if rec := do("PUT", "/items/999999999", `{"specs":"x"}`); rec.Code != 404 {
		t.Errorf("a missing product: %d %s", rec.Code, rec.Body)
	}
	if rec := do("PUT", path, `[`); rec.Code != 400 {
		t.Errorf("a bad body: %d", rec.Code)
	}
	if rec := do("PUT", "/items/0", `{}`); rec.Code != 400 {
		t.Errorf("PUT a bad id: %d", rec.Code)
	}
}

// A merge into itself or a missing category, and a move to no category, are dropped;
// empty categories list and delete, and a delete naming none is refused.
func TestAPICleanupMergesAndEmpty(t *testing.T) {
	db := freshDB(t, "aicatalog_api_cleanup_merges")
	_, do := testAPI(t, db)
	a, b := cat(t, db, "Soap", nil), cat(t, db, "Soaps", nil)
	empty := cat(t, db, "Nothing here", nil)
	pid := prod(t, db, "Lux", a)
	prod(t, db, "Dove", b)
	reply := fmt.Sprintf(`{"merges":[{"from":%d,"into":%d},{"from":%[1]d,"into":%[1]d},{"from":%[1]d,"into":999999}],
		"uncategorized":[{"product_id":%[3]d,"category":""},{"product_id":%[3]d,"category":"Bath > Soap"}]}`, a, b, pid)
	body, _ := json.Marshal(CleanupRequest{Reply: reply})
	r := decode[AskReply[CleanupPreview]](t, do("POST", "/cleanup", string(body)))
	if r.Result == nil {
		t.Fatalf("%+v", r)
	}
	if m := r.Result.Plan.Merges; len(m) != 1 || m[0].From != a || m[0].Into != b {
		t.Errorf("merges kept: %+v", m)
	}
	if u := r.Result.Plan.Uncategorized; len(u) != 1 || u[0].Category != "Bath > Soap" {
		t.Errorf("moves out of no category kept: %+v", u)
	}
	list := decode[map[string][]EmptyCategory](t, do("GET", "/cleanup/empty-categories", ""))["categories"]
	listed := map[int64]bool{}
	for _, x := range list {
		listed[x.ID] = true
	}
	if !listed[empty] || listed[a] || listed[b] {
		t.Errorf("empty categories: %+v", list)
	}
	if rec := do("POST", "/cleanup/empty-categories", `{"ids":[]}`); rec.Code != 422 {
		t.Errorf("deleting none: %d", rec.Code)
	}
	got := decode[map[string]int64](t, do("POST", "/cleanup/empty-categories", fmt.Sprintf(`{"ids":[%d,%d]}`, empty, a)))
	if got["deleted"] != 1 {
		t.Errorf("deleted %d, want only the empty one", got["deleted"])
	}
}

// With the AI on: combos and a job's batch come back from it, or, when it fails, the
// prompt to copy with the reason. An OpenAI-style error, as a 400 or inside a 200, is a failure.
func TestAPIModeCombosAndJobs(t *testing.T) {
	db := freshDB(t, "aicatalog_api_mode")
	p, do := testAPI(t, db)
	p.jobs = []Job{fakeJob()}
	c := cat(t, db, "Baby", nil)
	a, b := prod(t, db, "Wipes", c), prod(t, db, "Powder", c)
	content := fmt.Sprintf(`{"combos":[{"name":"Baby set","product_ids":[%d,%d],"reason":"together"}]}`, a, b)
	msg, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
	use := func(status int, body string) {
		srv, _ := capture(t, status, body)
		if rec := do("PUT", "/settings", fmt.Sprintf(`{"manualMode":false,"provider":"openai","baseUrl":%q,"model":"m","apiKey":"k"}`, srv.URL)); rec.Code != 200 {
			t.Fatalf("settings: %d %s", rec.Code, rec.Body)
		}
	}
	type combos struct {
		Result *comboReply `json:"result"`
		Prompt string      `json:"prompt"`
		Error  string      `json:"error"`
	}
	use(200, string(msg))
	if r := decode[combos](t, do("POST", "/combo-suggest", `{}`)); r.Result == nil || len(r.Result.Ideas) != 1 || r.Prompt != "" {
		t.Errorf("combos from the AI: %+v", r)
	}
	for _, bad := range []struct {
		status int
		body   string
	}{{400, `{"error":{"message":"model gone"}}`}, {200, `{"error":{"message":"model gone"},"choices":[]}`}} {
		use(bad.status, bad.body)
		if r := decode[combos](t, do("POST", "/combo-suggest", `{}`)); r.Result != nil || r.Prompt == "" || !strings.Contains(r.Error, "model gone") {
			t.Errorf("combos, AI failing (%d): %+v", bad.status, r)
		}
		if r := decode[AskReply[JobPreview]](t, do("POST", "/jobs/fake/next", `{}`)); r.Result != nil || r.Prompt == "" || !strings.Contains(r.Error, "model gone") {
			t.Errorf("a job, AI failing (%d): %+v", bad.status, r)
		}
	}
	if r := decode[combos](t, do("POST", "/combo-suggest", fmt.Sprintf(`{"reply":%q}`, content))); r.Result == nil || len(r.Result.Ideas) != 1 {
		t.Errorf("a pasted answer is read even with the AI on: %+v", r)
	}
}
