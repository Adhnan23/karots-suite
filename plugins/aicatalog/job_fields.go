package aicatalog

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

const (
	knownValuesMax = 30
	knownCheckMax  = 2000
)

const fieldsPrompt = `Fill in details of a shop's products. Fill only the fields listed for each product.
Leave a value empty if you aren't sure: never guess. A field the shop alone can know stays empty.
Reuse an existing value exactly, spelling and all, when one fits; only write a new one when none does.
Reply with STRICT JSON ONLY:
{"items":[{"id":0,"values":[{"field":"","value":""}]}]}
A yes/no field is "yes" or "no". A date is YYYY-MM-DD. A dropdown must be one of its options, exactly.`

// FieldOption is a field in the page's tick list.
type FieldOption struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Type   string `json:"type"`
	Scoped bool   `json:"scoped"` // applies to chosen categories only
}

type fieldsOptions struct {
	Fields []string `json:"fields"`
}

func (p *Plugin) attrSet(key string) *plugin.ProductAttributeSet {
	if p.attrSets == nil {
		return nil
	}
	for _, s := range p.attrSets() {
		if s.Key == key {
			return &s
		}
	}
	return nil
}

func fieldsJob() Job {
	return Job{Key: "fields", Label: "Fill in details",
		Desc:      "Fills your own Product Plus fields, per category, reusing the values you already use. Tick only what a chatbot could know, not something like a rack number. Get categories right first: a wrong category asks for the wrong fields.",
		Available: func(p *Plugin) bool { return p.attrSet("productplus") != nil },
		Options: func(ctx context.Context, p *Plugin) (any, error) {
			attrs, err := p.attrSet("productplus").Attributes(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]FieldOption, 0, len(attrs))
			for _, a := range attrs {
				out = append(out, FieldOption{Key: a.Key, Label: a.Label, Type: a.Type, Scoped: len(a.CategoryIDs) > 0})
			}
			return out, nil
		},
		Select: fieldsSelect, Prompt: fieldsPromptFor, Parse: fieldsParse, Apply: fieldsApply, Undo: attrUndo("productplus")}
}

// tickedAttrs is the fields chosen on the page that still exist.
func tickedAttrs(ctx context.Context, set *plugin.ProductAttributeSet, raw json.RawMessage) ([]plugin.ProductAttribute, error) {
	var o fieldsOptions
	_ = json.Unmarshal(raw, &o)
	attrs, err := set.Attributes(ctx)
	if err != nil {
		return nil, err
	}
	var out []plugin.ProductAttribute
	for _, a := range attrs {
		if slices.Contains(o.Fields, a.Key) {
			out = append(out, a)
		}
	}
	return out, nil
}

func appliesTo(a plugin.ProductAttribute, categoryID int64) bool {
	return len(a.CategoryIDs) == 0 || slices.Contains(a.CategoryIDs, categoryID)
}

// fieldsSelect: the record is per set of ticked fields, so ticking another field later
// brings products back. A product is offered only if a ticked field in its category is
// still empty (a redo asks regardless).
func fieldsSelect(ctx context.Context, p *Plugin, in JobRequest) (Batch, int, error) {
	var b Batch
	set := p.attrSet("productplus")
	ticked, err := tickedAttrs(ctx, set, in.Options)
	if err != nil {
		return b, 0, err
	}
	if len(ticked) == 0 {
		return b, 0, apperr.Validation("tick the fields to fill first")
	}
	keys := make([]string, len(ticked))
	for i, a := range ticked {
		keys[i] = a.Key
	}
	sort.Strings(keys)
	b.State = "fields:" + strings.Join(keys, ",")
	if in.Force {
		b.State += ":redo"
	}
	size := in.Size
	if size <= 0 {
		size = jobBatchDefault
	}
	if len(in.IDs) > 0 {
		b.Products, _, err = selectProducts(ctx, p, b.State, in)
		return b, len(b.Products), err
	}
	in.noCap = true
	all, _, err := selectProducts(ctx, p, b.State, in)
	if err != nil {
		return b, 0, err
	}
	ids := make([]int64, len(all))
	for i, x := range all {
		ids[i] = x.ID
	}
	values, err := set.Values(ctx, ids)
	if err != nil {
		return b, 0, err
	}
	total := 0
	for _, x := range all {
		wanted := false
		for _, a := range ticked {
			if appliesTo(a, x.CategoryID) && (in.Force || values[x.ID][a.Key] == "") {
				wanted = true
				break
			}
		}
		if !wanted {
			continue
		}
		total++
		if len(b.Products) < min(size, jobBatchMax) {
			b.Products = append(b.Products, x)
		}
	}
	return b, total, nil
}

func fieldsPromptFor(ctx context.Context, p *Plugin, b Batch) (string, string, error) {
	set := p.attrSet("productplus")
	ticked, err := tickedAttrs(ctx, set, b.Options)
	if err != nil {
		return "", "", err
	}
	var u strings.Builder
	u.WriteString("FIELDS:\n")
	for _, a := range ticked {
		kind := a.Type
		switch a.Type {
		case "select":
			kind = "dropdown, pick one only: " + strings.Join(a.Options, ", ")
		case "bool":
			kind = "yes/no"
		case "textarea":
			kind = "text"
		}
		u.WriteString(a.Key + " (" + a.Label + ", " + kind + ")")
		if a.Type == "text" || a.Type == "textarea" {
			known, err := set.Known(ctx, a.Key, knownValuesMax)
			if err != nil {
				return "", "", err
			}
			if len(known) > 0 {
				vals := make([]string, len(known))
				for i, k := range known {
					vals[i] = k.Value
				}
				u.WriteString(", existing: " + strings.Join(vals, ", "))
			}
		}
		u.WriteString("\n")
	}
	u.WriteString("\nPRODUCTS (id|name|category|details|fields to fill):\n")
	for _, x := range b.Products {
		var want []string
		for _, a := range ticked {
			if appliesTo(a, x.CategoryID) {
				want = append(want, a.Key)
			}
		}
		u.WriteString(strconv.FormatInt(x.ID, 10) + "|" + x.Name + "|" + x.Category + "|" + strings.ReplaceAll(x.Hint(), "\n", " ") + "|" + strings.Join(want, ", ") + "\n")
	}
	return fieldsPrompt, u.String(), nil
}

func fieldsParse(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error) {
	var r struct {
		Items []struct {
			ID     int64 `json:"id"`
			Values []struct {
				Field string `json:"field"`
				Value any    `json:"value"`
			} `json:"values"`
		} `json:"items"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	set := p.attrSet("productplus")
	ticked, err := tickedAttrs(ctx, set, b.Options)
	if err != nil {
		return nil, err
	}
	byKey := map[string]plugin.ProductAttribute{}
	for _, a := range ticked {
		byKey[a.Key] = a
	}
	byID, ids := map[int64]JobProduct{}, []int64{}
	for _, x := range b.Products {
		byID[x.ID] = x
		ids = append(ids, x.ID)
	}
	current, err := set.Values(ctx, ids)
	if err != nil {
		return nil, err
	}
	known := map[string]map[string]string{}
	rows := []JobRow{}
	for _, it := range r.Items {
		x, ok := byID[it.ID]
		if !ok {
			continue
		}
		seen := map[string]bool{}
		for _, v := range it.Values {
			a, ok := byKey[v.Field]
			if !ok || seen[v.Field] || !appliesTo(a, x.CategoryID) { // not ticked, or not this category's
				continue
			}
			val, ok := attrValueOf(a, v.Value)
			if !ok {
				continue
			}
			seen[v.Field] = true
			if a.Type == "text" || a.Type == "textarea" {
				if known[a.Key] == nil {
					list, err := set.Known(ctx, a.Key, knownCheckMax)
					if err != nil {
						return nil, err
					}
					known[a.Key] = map[string]string{}
					for _, k := range list {
						known[a.Key][strings.ToLower(k.Value)] = k.Value
					}
				}
			}
			row := JobRow{ProductID: it.ID, Kind: "attr", Key: a.Key, Label: a.Label, New: val}
			if a.Type == "text" || a.Type == "textarea" {
				if same, ok := known[a.Key][strings.ToLower(val)]; ok {
					row.New = same
				} else {
					row.IsNew = true
				}
			}
			old := current[it.ID][a.Key]
			if old != "" && !b.Redo {
				continue // never over a value the shop has, unless asked to redo it
			}
			if strings.EqualFold(old, row.New) {
				continue
			}
			row.Old, row.Change, row.Ticked = old, old != "", old == ""
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// attrValueOf reads a chatbot's value for an attribute; ok is false for no suggestion.
func attrValueOf(a plugin.ProductAttribute, raw any) (string, bool) {
	var v string
	switch x := raw.(type) {
	case string:
		v = strings.TrimSpace(x)
	case bool:
		v = "no"
		if x {
			v = "yes"
		}
	case float64:
		v = strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return "", false
	}
	if v == "" {
		return "", false
	}
	if a.Type == "select" {
		i := slices.IndexFunc(a.Options, func(o string) bool { return strings.EqualFold(o, v) })
		if i < 0 {
			return "", false
		}
		return a.Options[i], true
	}
	if a.Type == "bool" {
		switch strings.ToLower(v) {
		case "yes", "true", "1":
			return "yes", true
		case "no", "false", "0":
			return "no", true
		}
		return "", false
	}
	return v, true
}

// fieldsApply checks again that the field applies: the plugin's save doesn't, and the
// product's category may have changed since the preview.
func fieldsApply(ctx context.Context, p *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error) {
	set := p.attrSet("productplus")
	if set == nil {
		return nil, errStale
	}
	attrs, err := set.Attributes(ctx)
	if err != nil {
		return nil, err
	}
	var cat int64
	if err := tx.GetContext(ctx, &cat, `SELECT category_id FROM products WHERE id = $1`, r.ProductID); err != nil {
		return nil, err
	}
	i := slices.IndexFunc(attrs, func(a plugin.ProductAttribute) bool { return a.Key == r.Key })
	if i < 0 || !appliesTo(attrs[i], cat) {
		return nil, errStale
	}
	return attrApply("productplus")(ctx, p, tx, r)
}

// attrApply saves through the plugin, if the value hasn't changed since the preview.
func attrApply(setKey string) func(context.Context, *Plugin, *sqlx.Tx, JobRow) ([]UndoOp, error) {
	return func(ctx context.Context, p *Plugin, _ *sqlx.Tx, r JobRow) ([]UndoOp, error) {
		set := p.attrSet(setKey)
		if set == nil {
			return nil, errStale
		}
		cur, err := set.Values(ctx, []int64{r.ProductID})
		if err != nil {
			return nil, err
		}
		old := cur[r.ProductID][r.Key]
		if old != r.Old {
			return nil, errStale
		}
		if err := set.Set(ctx, r.ProductID, r.Key, r.New); err != nil {
			return nil, err
		}
		after, err := set.Values(ctx, []int64{r.ProductID})
		if err != nil {
			return nil, err
		}
		// A plugin's own rule may change another value too (a controlled drug is
		// prescription-only). Those undo as well, after the row itself: replay runs backwards.
		var ops []UndoOp
		keys := make([]string, 0, len(after[r.ProductID]))
		for k := range after[r.ProductID] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if k != r.Key && after[r.ProductID][k] != cur[r.ProductID][k] {
				ops = append(ops, UndoOp{RowKind: "attr", ProductID: r.ProductID, Key: k, Applied: after[r.ProductID][k], Previous: cur[r.ProductID][k]})
			}
		}
		return append(ops, UndoOp{RowKind: "attr", ProductID: r.ProductID, Key: r.Key, Applied: after[r.ProductID][r.Key], Previous: old}), nil
	}
}

func attrUndo(setKey string) func(context.Context, *Plugin, *sqlx.Tx, UndoOp) error {
	return func(ctx context.Context, p *Plugin, _ *sqlx.Tx, op UndoOp) error {
		set := p.attrSet(setKey)
		if set == nil {
			return nil
		}
		cur, err := set.Values(ctx, []int64{op.ProductID})
		if err != nil {
			return err
		}
		if cur[op.ProductID][op.Key] != op.Applied {
			return errStale
		}
		return set.Set(ctx, op.ProductID, op.Key, op.Previous)
	}
}
