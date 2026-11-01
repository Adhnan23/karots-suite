package plugin

import "context"

// AttentionItem is one line a plugin adds to the admin dashboard's "Needs your
// attention" (a phone waiting for approval…). To is the admin page that deals
// with it; Go the button's word; Tone is info | warn | crit.
type AttentionItem struct {
	Tag  string
	Text string
	To   string
	Go   string
	Tone string
}

// AttentionProvider lists what needs someone now (nil = nothing).
type AttentionProvider func(ctx context.Context) ([]AttentionItem, error)

var attentionProviders hook[AttentionProvider]

func (r *Registry) AddAttention(p AttentionProvider) { attentionProviders.add(p) }

// Attention collects every plugin's items; a provider that fails is left out.
func Attention(ctx context.Context) []AttentionItem {
	out := []AttentionItem{}
	for _, p := range attentionProviders.all() {
		if items, err := p(ctx); err == nil {
			out = append(out, items...)
		}
	}
	return out
}
