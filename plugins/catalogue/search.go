package catalogue

import (
	"context"
	"strings"

	"karots-suite/internal/plugin"

	"github.com/lib/pq"
)

// Have is the shop product a catalogue item already is (linked, or the same name): CTL-5.
type Have struct {
	ID   int64  `json:"id"   db:"id"`
	Name string `json:"name" db:"name"`
}

// Pick is a search result with what picking it fills (CTL-6): the shop's unit and the values of the shop's own
// fields (by attribute set key → {"values": {field key: value}}, the product form sections' shape).
type Pick struct {
	Item
	UnitID   int64                     `json:"unitId"`
	Sections map[string]map[string]any `json:"sections,omitempty"`
	Have     *Have                     `json:"have,omitempty"`
}

const maxResults = 20

// Search finds items by name and field values, every word somewhere, best first (CTL-5).
func (s *Store) Search(ctx context.Context, q string) ([]Pick, error) {
	words := strings.Fields(strings.ToLower(q))
	if len(words) == 0 {
		return []Pick{}, nil
	}
	pats := make([]string, len(words))
	for i, w := range words {
		pats[i] = "%" + likeEscape(w) + "%"
	}
	var rows []itemRow
	if err := s.db.SelectContext(ctx, &rows, `SELECT id, name, unit, kinds, fields FROM catalogue_items
		WHERE search LIKE ALL($1)
		ORDER BY lower(name) LIKE $2 DESC, similarity(lower(name), $3) DESC, length(name), id LIMIT $4`,
		pq.Array(pats), likeEscape(words[0])+"%", strings.ToLower(q), maxResults); err != nil {
		return nil, err
	}
	items := make([]Item, len(rows))
	for i, r := range rows {
		items[i] = r.item()
	}
	return s.picks(ctx, items)
}

// picks adds the fill and what the shop already has to items.
func (s *Store) picks(ctx context.Context, items []Item) ([]Pick, error) {
	f, err := s.filler(ctx)
	if err != nil {
		return nil, err
	}
	have, err := s.haves(ctx, items)
	if err != nil {
		return nil, err
	}
	out := make([]Pick, len(items))
	for i, it := range items {
		out[i] = Pick{Item: it, UnitID: f.unit(it.Unit), Sections: f.sections(it.Fields), Have: have[it.ID]}
	}
	return out, nil
}

// haves is the shop product each item already is: its link first, else an active product of the same name.
func (s *Store) haves(ctx context.Context, items []Item) (map[int64]*Have, error) {
	ids, names := make([]int64, len(items)), make([]string, len(items))
	for i, it := range items {
		ids[i], names[i] = it.ID, strings.ToLower(it.Name)
	}
	var linked []struct {
		Item int64 `db:"item_id"`
		Have
	}
	if err := s.db.SelectContext(ctx, &linked, `SELECT l.item_id, p.id, p.name FROM catalogue_links l
		JOIN products p ON p.id = l.product_id AND p.is_active WHERE l.item_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	var named []Have
	if err := s.db.SelectContext(ctx, &named, `SELECT id, name FROM products WHERE is_active AND lower(name) = ANY($1)`, pq.Array(names)); err != nil {
		return nil, err
	}
	out := map[int64]*Have{}
	byName := map[string]Have{}
	for _, h := range named {
		byName[strings.ToLower(h.Name)] = h
	}
	for _, it := range items {
		if h, ok := byName[strings.ToLower(it.Name)]; ok {
			out[it.ID] = &h
		}
	}
	for _, l := range linked {
		h := l.Have
		out[l.Item] = &h
	}
	return out, nil
}

// filler maps the catalogue's units and fields onto this shop's (CTL-6).
type filler struct {
	units       map[string]int64 // short name → unit id
	defaultUnit int64
	fields      map[string]fieldRef // normalised field name → the shop's field
}

type fieldRef struct{ set, key string }

func (s *Store) filler(ctx context.Context) (filler, error) {
	f := filler{units: map[string]int64{}, fields: map[string]fieldRef{}}
	var units []struct {
		ID      int64  `db:"id"`
		Abbr    string `db:"abbreviation"`
		Default bool   `db:"is_default"`
	}
	if err := s.db.SelectContext(ctx, &units, `SELECT id, abbreviation, is_default FROM units ORDER BY id`); err != nil {
		return f, err
	}
	for _, u := range units {
		if _, ok := f.units[strings.ToLower(u.Abbr)]; !ok {
			f.units[strings.ToLower(u.Abbr)] = u.ID
		}
		if u.Default || f.defaultUnit == 0 { // the shop's default, else its first unit
			f.defaultUnit = u.ID
		}
	}
	for _, set := range plugin.ProductAttributeSets() {
		attrs, err := set.Attributes(ctx)
		if err != nil {
			return f, err
		}
		for _, a := range attrs {
			if _, ok := f.fields[norm(a.Label)]; !ok {
				f.fields[norm(a.Label)] = fieldRef{set.Key, a.Key}
			}
		}
	}
	return f, nil
}

func (f filler) unit(short string) int64 {
	if id, ok := f.units[strings.ToLower(short)]; ok {
		return id
	}
	return f.defaultUnit
}

// sections are the values of the catalogue fields the shop has; one it doesn't have is left out.
func (f filler) sections(fields map[string]string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for name, v := range fields {
		ref, ok := f.fields[norm(name)]
		if !ok || v == "" {
			continue
		}
		if out[ref.set] == nil {
			out[ref.set] = map[string]any{"values": map[string]string{}}
		}
		out[ref.set]["values"].(map[string]string)[ref.key] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// norm makes "Generic  name" and "generic name" the same field (case and spaces ignored).
func norm(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
