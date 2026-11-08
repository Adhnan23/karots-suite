package aicatalog

import (
	"context"
	"slices"
	"strings"

	"karots-suite/internal/plugin"
)

// Settings → Identify: the Product Plus fields Identify also fills, both on the product
// form and in the Identify a whole list job.

// identifyFieldsRule rides every Identify prompt with fields ticked, the short one too,
// so a chat that already knows the rules still fills them.
const identifyFieldsRule = `Also fill the FIELDS below in "values" on each answer (best and every option), as
"values":{"<field>":"<value>"}. Fill a field only when it fits the category you give the item (some are only
for certain categories). Leave a field out if you aren't sure: never guess. Reuse an existing value exactly,
spelling and all, when one fits. A yes/no field is "yes" or "no". A date is YYYY-MM-DD. A dropdown must be one
of its options, exactly.`

// CandidateField is one field value an answer fills, in the shop's own spelling.
type CandidateField struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}

func splitKeys(s string) []string {
	out := []string{}
	for _, k := range strings.Split(s, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// fieldList is the Product Plus fields as a tick list; nil without the plugin.
func (p *Plugin) fieldList(ctx context.Context) ([]FieldOption, error) {
	set := p.attrSet("productplus")
	if set == nil {
		return nil, nil
	}
	attrs, err := set.Attributes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]FieldOption, 0, len(attrs))
	for _, a := range attrs {
		out = append(out, FieldOption{Key: a.Key, Label: a.Label, Type: a.Type, Scoped: len(a.CategoryIDs) > 0})
	}
	return out, nil
}

// keepFields is the ticked keys that are real fields, for saving.
func (p *Plugin) keepFields(ctx context.Context, keys []string) (string, error) {
	set := p.attrSet("productplus")
	if set == nil {
		return "", nil
	}
	attrs, err := pickAttrs(ctx, set, keys)
	out := make([]string, len(attrs))
	for i, a := range attrs {
		out[i] = a.Key
	}
	return strings.Join(out, ","), err
}

// pickAttrs is the set's fields whose keys are given, in the set's order.
func pickAttrs(ctx context.Context, set *plugin.ProductAttributeSet, keys []string) ([]plugin.ProductAttribute, error) {
	attrs, err := set.Attributes(ctx)
	if err != nil {
		return nil, err
	}
	var out []plugin.ProductAttribute
	for _, a := range attrs {
		if slices.Contains(keys, a.Key) {
			out = append(out, a)
		}
	}
	return out, nil
}

// identifyAttrs is the fields ticked in Settings → Identify (none without Product Plus).
func (p *Plugin) identifyAttrs(ctx context.Context, s Settings) ([]plugin.ProductAttribute, error) {
	set := p.attrSet("productplus")
	if set == nil || s.IdentifyFields == "" {
		return nil, nil
	}
	return pickAttrs(ctx, set, splitKeys(s.IdentifyFields))
}

// formFieldsBlock is the rule and the field list for the form's Identify ("" = none ticked).
func (p *Plugin) formFieldsBlock(ctx context.Context, s Settings) (string, []plugin.ProductAttribute, error) {
	ticked, err := p.identifyAttrs(ctx, s)
	if err != nil || len(ticked) == 0 {
		return "", nil, err
	}
	cats, err := p.store.Categories(ctx)
	if err != nil {
		return "", nil, err
	}
	var u strings.Builder
	u.WriteString("\n\n" + identifyFieldsRule + "\n")
	if err := writeFields(ctx, &u, p.attrSet("productplus"), ticked, pathsByID(cats)); err != nil {
		return "", nil, err
	}
	return strings.TrimRight(u.String(), "\n"), ticked, nil
}

// fillCandidates reads each answer's values into Fields: ticked, fitting the category it
// gives (an existing one; a new category takes only fields kept for every category),
// in the shop's spelling. The raw values aren't sent on.
func (p *Plugin) fillCandidates(ctx context.Context, r *IdentifyResult, ticked []plugin.ProductAttribute) error {
	set := p.attrSet("productplus")
	if set == nil || len(ticked) == 0 {
		r.Best.Values = nil
		for i := range r.Options {
			r.Options[i].Values = nil
		}
		return nil
	}
	cats, err := p.store.Categories(ctx)
	if err != nil {
		return err
	}
	byPath := map[string]int64{}
	for id, path := range pathsByID(cats) {
		byPath[strings.ToLower(path)] = id
	}
	known := map[string]map[string]string{}
	one := func(c *Candidate) error {
		catID := byPath[strings.ToLower(c.Category)]
		for _, a := range ticked {
			raw, ok := c.Values[a.Key]
			if !ok || !appliesTo(a, catID) {
				continue
			}
			row, ok, err := attrRow(ctx, set, a, raw, known)
			if err != nil {
				return err
			}
			if ok {
				c.Fields = append(c.Fields, CandidateField{Key: a.Key, Label: a.Label, Value: row.New})
			}
		}
		c.Values = nil
		return nil
	}
	if err := one(&r.Best); err != nil {
		return err
	}
	for i := range r.Options {
		if err := one(&r.Options[i]); err != nil {
			return err
		}
	}
	return nil
}
