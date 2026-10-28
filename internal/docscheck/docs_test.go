// Package docscheck keeps the user docs and the legal pack in step with the
// plugins: adding a plugin without its guide page, screenshot, price and Order
// Form tick box fails `make test`. It reads plugins/*/plugin.json, so it names
// no plugin itself.
package docscheck

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const root = "../.."

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEveryPluginIsDocumentedAndPriced(t *testing.T) {
	dirs, _ := filepath.Glob(filepath.Join(root, "plugins", "*", "plugin.json"))
	if len(dirs) == 0 {
		t.Fatal("no plugins found")
	}
	readme := read(t, "README.md")
	index := read(t, "docs/guide/plugins/README.md")
	shots := read(t, "web/docs-shots/shots.spec.ts")
	form := read(t, "docs/legal/agreement/order-form.html")
	rules := read(t, "docs/legal/agreement/shop-rules.html")
	for _, f := range dirs {
		var p struct{ Name string }
		b, _ := os.ReadFile(f)
		if err := json.Unmarshal(b, &p); err != nil || p.Name == "" {
			t.Fatalf("%s: no name", f)
		}
		key := filepath.Base(filepath.Dir(f))
		name := html.EscapeString(p.Name)
		checks := []struct {
			ok   bool
			what string
		}{
			{strings.Contains(readme, "docs/guide/plugins/"+key+".md"), "a row in README.md's plugin table"},
			{exists("docs/guide/plugins/" + key + ".md"), "docs/guide/plugins/" + key + ".md"},
			{strings.Contains(index, "("+key+".md)"), "a row in docs/guide/plugins/README.md"},
			{strings.Contains(shots, "'"+key+"'"), "a place in web/docs-shots/shots.spec.ts's plugin list"},
			{exists("docs/images/plugins/" + key + ".jpg"), "docs/images/plugins/" + key + ".jpg (make docs-shots)"},
			{strings.Contains(form, "☐ "+name), "a tick box in the First Schedule (docs/legal/agreement/order-form.html)"},
			{strings.Contains(rules, "<tr><td>"+name), "a price in the Shop Rules (docs/legal/agreement/shop-rules.html §15)"},
		}
		for _, c := range checks {
			if !c.ok {
				t.Errorf("plugin %s (%s) needs %s", key, p.Name, c.what)
			}
		}
	}
	n := len(dirs)
	if !strings.Contains(readme, fmt.Sprintf("plugins-%d-", n)) {
		t.Errorf("README.md's plugins badge should say %d", n)
	}
	if w := words[n]; w == "" || !strings.Contains(rules, "all "+w+" plugins") {
		t.Errorf(`Shop Rules "Everything" row should say "all %s plugins"`, words[n])
	}
}

var words = map[int]string{10: "ten", 11: "eleven", 12: "twelve", 13: "thirteen", 14: "fourteen", 15: "fifteen", 16: "sixteen"}

func exists(path string) bool {
	_, err := os.Stat(filepath.Join(root, path))
	return err == nil
}
