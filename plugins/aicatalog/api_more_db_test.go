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
