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
