package plugin

import (
	"context"
	"errors"
	"testing"
)

// Plugins add lines to the dashboard's "Needs your attention"; one that fails
// is left out, never the whole dashboard.
func TestAttention(t *testing.T) {
	saved := attentionProviders
	t.Cleanup(func() { attentionProviders = saved })
	attentionProviders = hook[AttentionProvider]{}
	r := &Registry{}
	r.AddAttention(func(context.Context) ([]AttentionItem, error) {
		return []AttentionItem{{Tag: "Phones", Text: "1 phone is waiting", To: "/admin/x/p", Go: "Approve", Tone: "warn"}}, nil
	})
	r.AddAttention(func(context.Context) ([]AttentionItem, error) { return nil, errors.New("down") })
	got := Attention(context.Background())
	if len(got) != 1 || got[0].Text != "1 phone is waiting" {
		t.Fatalf("%+v", got)
	}
}
