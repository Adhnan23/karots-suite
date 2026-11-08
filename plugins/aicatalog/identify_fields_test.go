package aicatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"karots-suite/internal/plugin"
)

// The form's Identify fills the fields ticked in Settings → Identify.
func formFieldEnv(t *testing.T, name string) (fieldEnvT, func(method, path, body string) *httptest.ResponseRecorder) {
	e := fieldEnv(t, name)
	p, do := testAPI(t, e.p.store.db)
	p.attrSets = e.p.attrSets
	e.p = p
	return e, do
}

func TestSettingsIdentifyFields(t *testing.T) {
	e, do := formFieldEnv(t, "aicatalog_settings_fields")
	s := decode[SettingsDTO](t, do("GET", "/settings", ""))
	if len(s.FieldOptions) != 6 || len(s.IdentifyFields) != 0 {
		t.Fatalf("offered, nothing ticked: %+v", s)
	}
	s = decode[SettingsDTO](t, do("PUT", "/settings", `{"manualMode":true,"identifyFields":["generic_name","nope"]}`))
	if strings.Join(s.IdentifyFields, ",") != "generic_name" {
		t.Fatalf("a field that doesn't exist isn't kept: %+v", s.IdentifyFields)
	}
	e.p.attrSets = func() []plugin.ProductAttributeSet { return nil }
	if s = decode[SettingsDTO](t, do("GET", "/settings", "")); s.FieldOptions != nil {
		t.Fatalf("no Product Plus, nothing offered: %+v", s)
	}
}

func TestFormIdentifyAsksTickedFieldsInBothPrompts(t *testing.T) {
	_, do := formFieldEnv(t, "aicatalog_form_fields_prompt")
	r := decode[AskReply[IdentifyResult]](t, do("POST", "/identify", `{"query":"panadl"}`))
	if strings.Contains(r.Prompt, "FIELDS") {
		t.Fatalf("nothing ticked: %s", r.Prompt)
	}
	do("PUT", "/settings", `{"manualMode":true,"identifyFields":["generic_name","form"]}`)
	for _, short := range []bool{false, true} {
		r = decode[AskReply[IdentifyResult]](t, do("POST", "/identify", fmt.Sprintf(`{"query":"panadl","short":%v}`, short)))
		for _, want := range []string{"generic_name (Generic name, text), only for categories: Medicine", "pick one only: Tablet, Capsule", `"values"`} {
			if !strings.Contains(r.Prompt, want) {
				t.Errorf("short=%v: no %q in %s", short, want, r.Prompt)
			}
		}
	}
}

func TestFormIdentifyReadsFieldValues(t *testing.T) {
	e, do := formFieldEnv(t, "aicatalog_form_fields_reply")
	e.f.known["generic_name"] = []string{"Paracetamol"}
	do("PUT", "/settings", `{"manualMode":true,"identifyFields":["generic_name","form","manufacturer"]}`)
	reply := `{"confident":false,"options":[
		{"name":"Panadol 500mg","category":"Medicine","values":{"generic_name":"paracetamol","form":"Syrup","manufacturer":"GSK","rack":"A1"}},
		{"name":"Panadol Spark","category":"Parts","values":{"generic_name":"Paracetamol","manufacturer":"GSK"}}]}`
	body, _ := json.Marshal(map[string]string{"query": "panadl", "reply": reply})
	r := decode[AskReply[IdentifyResult]](t, do("POST", "/identify", string(body)))
	if r.Result == nil || len(r.Result.Options) != 2 {
		t.Fatalf("%+v", r)
	}
	str := func(c Candidate) string {
		var s []string
		for _, f := range c.Fields {
			s = append(s, f.Key+"="+f.Value)
		}
		return strings.Join(s, ",")
	}
	if got := str(r.Result.Options[0]); got != "generic_name=Paracetamol,manufacturer=GSK" {
		t.Errorf("shop's spelling, off-list dropdown and unticked dropped: %s", got)
	}
	if got := str(r.Result.Options[1]); got != "manufacturer=GSK" {
		t.Errorf("a Medicine field on a part is dropped: %s", got)
	}
	if r.Result.Options[0].Values != nil || r.Result.Options[0].Fields[0].Label != "Generic name" {
		t.Errorf("raw values aren't sent on: %+v", r.Result.Options[0])
	}
}

func TestJobIdentifyUsesSettingsTicks(t *testing.T) {
	e, a := identifyFieldEnv(t, "aicatalog_identify_settings_ticks")
	b := Batch{Settings: Settings{IdentifyFields: "generic_name"}, Products: []JobProduct{{ID: a, Name: "panadol 500"}}}
	_, user, err := identifyPrompt(context.Background(), e.p, b)
	if err != nil || !strings.Contains(user, "generic_name (Generic name") {
		t.Fatalf("%s %v", user, err)
	}
}
