package productplus

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// A product's value for a field is its pp_values row, else the field's default.
// A row is kept for every value someone set — even one equal to the default — so
// a later default change ("only new products") can't move a deliberate choice.

// coerce checks raw against the field's type and returns the value to store:
// yes/no words → "1" / "", a dropdown option matched case-insensitively to its
// spelling, numbers and YYYY-MM-DD dates as typed. current (the product's stored
// value) is accepted as is, so a dropdown option since removed survives a save.
func coerce(f Field, raw, current string) (string, error) {
	v := strings.TrimSpace(raw)
	if f.Type == "bool" {
		switch strings.ToLower(v) {
		case "1", "yes", "y", "true":
			return "1", nil
		case "", "0", "no", "n", "false":
			return "", nil
		}
		return "", errors.New(f.Label + ": yes or no")
	}
	if v == "" || (current != "" && v == current) {
		return v, nil
	}
	switch f.Type {
	case "number":
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", errors.New(f.Label + ": not a number")
		}
	case "date":
		if _, err := time.Parse("2006-01-02", v); err != nil {
			return "", errors.New(f.Label + ": not a date (YYYY-MM-DD)")
		}
	case "select":
		i := slices.IndexFunc(f.Options, func(o string) bool { return strings.EqualFold(o, v) })
		if i < 0 {
			return "", errors.New(f.Label + ": " + v + " is not an option")
		}
		return f.Options[i], nil
	}
	return v, nil
}

// FormField is a field shown for a product with the value to show.
type FormField struct {
	Field   Field
	Value   string
	Own     bool // the product has its own value (false = reads the default)
	Removed bool // a dropdown value whose option was removed
}

// FormFields lists the active fields that apply to category cat (where "intake":
// only those marked for intake), each with productID's value (0 = new: defaults).
func (s *Store) FormFields(ctx context.Context, productID, cat int64, where string) ([]FormField, error) {
	var rows []struct {
		Field
		Value *string `db:"value"`
	}
	if err := s.db.SelectContext(ctx, &rows, `WITH `+covCTE+`
		SELECT `+prefixed("f")+`, v.value FROM pp_fields f
		LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = $1
		WHERE f.is_active AND ($3 <> 'intake' OR f.show_in_intake) AND `+appliesSQL("f", "$2::bigint")+`
		ORDER BY f.sort_order, f.id`, productID, cat, where); err != nil {
		return nil, err
	}
	out := make([]FormField, 0, len(rows))
	for _, r := range rows {
		r.Field.decodeOptions()
		ff := FormField{Field: r.Field, Value: r.Field.DefaultValue, Own: r.Value != nil}
		if r.Value != nil {
			ff.Value = *r.Value
		}
		ff.Removed = r.Field.Type == "select" && ff.Value != "" && !slices.Contains(r.Field.Options, ff.Value)
		out = append(out, ff)
	}
	return out, nil
}

// prefixed is fieldColumns qualified by a table alias.
func prefixed(a string) string {
	cols := strings.Split(strings.Join(strings.Fields(fieldColumns), ""), ",")
	for i, c := range cols {
		cols[i] = a + "." + c
	}
	return strings.Join(cols, ", ")
}

// Cell is one value to set; CellError says why one wasn't.
type Cell struct {
	ProductID int64  `json:"productId"`
	FieldID   int64  `json:"fieldId"`
	Value     string `json:"value"`
}
type CellError struct {
	ProductID int64  `json:"productId"`
	FieldID   int64  `json:"fieldId"`
	Message   string `json:"message"`
}

// SetValues stores each cell that fits its field; a bad one is reported and the
// good ones still save (one tx). Fields must exist; scope isn't checked here
// (Fill in only offers cells in scope).
func (s *Store) SetValues(ctx context.Context, cells []Cell) (int, []CellError, error) {
	fields, err := s.fieldsByID(ctx)
	if err != nil {
		return 0, nil, err
	}
	var bad []CellError
	saved := 0
	err = appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		for _, c := range cells {
			f, ok := fields[c.FieldID]
			if !ok {
				bad = append(bad, CellError{c.ProductID, c.FieldID, "no such field"})
				continue
			}
			var cur string
			_ = tx.GetContext(ctx, &cur, `SELECT value FROM pp_values WHERE field_id=$1 AND product_id=$2`, f.ID, c.ProductID)
			v, err := coerce(f, c.Value, cur)
			if err != nil {
				bad = append(bad, CellError{c.ProductID, c.FieldID, err.Error()})
				continue
			}
			if err := upsert(ctx, tx, f.ID, c.ProductID, v); err != nil {
				return err
			}
			saved++
		}
		return nil
	})
	return saved, bad, err
}

func upsert(ctx context.Context, q appdb.Queryer, fieldID, productID int64, v string) error {
	_, err := q.ExecContext(ctx, `INSERT INTO pp_values (field_id, product_id, value) VALUES ($1,$2,$3)
		ON CONFLICT (field_id, product_id) DO UPDATE SET value = EXCLUDED.value`, fieldID, productID, v)
	return err
}

func (s *Store) fieldsByID(ctx context.Context) (map[int64]Field, error) {
	all, err := s.Fields(ctx, true)
	m := make(map[int64]Field, len(all))
	for _, f := range all {
		m[f.ID] = f
	}
	return m, err
}

// CheckProduct validates a product form's values (by field key) for category
// cat: only the keys sent are checked — a field the form didn't show is left as
// it is — and a required one sent blank is refused.
func (s *Store) CheckProduct(ctx context.Context, productID, cat int64, where string, values map[string]string) error {
	shown, err := s.FormFields(ctx, productID, cat, where)
	if err != nil {
		return err
	}
	for _, ff := range shown {
		raw, sent := values[ff.Field.Key]
		if !sent {
			continue
		}
		cur := ""
		if ff.Own {
			cur = ff.Value
		}
		v, err := coerce(ff.Field, raw, cur)
		if err != nil {
			return apperr.Validation(err.Error())
		}
		if ff.Field.Required && ff.Field.Type != "bool" && v == "" {
			return apperr.Validation(ff.Field.Label + " is required")
		}
	}
	return nil
}

// SaveProduct stores a product form's values (by field key) for the fields that
// apply to its category; keys for other fields are ignored.
func (s *Store) SaveProduct(ctx context.Context, productID, cat int64, where string, values map[string]string) error {
	shown, err := s.FormFields(ctx, productID, cat, where)
	if err != nil {
		return err
	}
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		for _, ff := range shown {
			raw, sent := values[ff.Field.Key]
			if !sent {
				continue
			}
			cur := ""
			if ff.Own {
				cur = ff.Value
			}
			v, err := coerce(ff.Field, raw, cur)
			if err != nil {
				return apperr.Validation(err.Error())
			}
			if err := upsert(ctx, tx, ff.Field.ID, productID, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// effective is one product × active applying field with its value (own or default).
type effective struct {
	ProductID int64  `db:"product_id"`
	Label     string `db:"label"`
	Type      string `db:"type"`
	Value     string `db:"value"`
}

// effectiveValues reads every active field that applies to the given products,
// with the value each reads, in field order. flag (a trusted column name or
// "true") narrows the fields.
func (s *Store) effectiveValues(ctx context.Context, ids []int64, flag string) ([]effective, error) {
	var rows []effective
	err := s.db.SelectContext(ctx, &rows, `WITH `+covCTE+`
		SELECT p.id AS product_id, f.label, f.type, COALESCE(v.value, f.default_value) AS value
		FROM products p JOIN pp_fields f ON f.is_active AND f.`+flag+` AND `+appliesSQL("f", "p.category_id")+`
		LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = p.id
		WHERE p.id = ANY($1)
		ORDER BY p.id, f.sort_order, f.id`, pq.Array(ids))
	return rows, err
}

// MetaFor is the admin list line per product ("Model: XJ-9 · Waterproof"):
// blanks and No skipped, a Yes shows its label, long text cut at 40.
func (s *Store) MetaFor(ctx context.Context, ids []int64) (map[int64]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.effectiveValues(ctx, ids, "is_active")
	if err != nil {
		return nil, err
	}
	parts := map[int64][]string{}
	for _, r := range rows {
		if t := shown(r, 40); t.Label != "" {
			if t.Value == "" {
				parts[r.ProductID] = append(parts[r.ProductID], t.Label)
			} else {
				parts[r.ProductID] = append(parts[r.ProductID], t.Label+": "+t.Value)
			}
		}
	}
	out := make(map[int64]string, len(parts))
	for id, ps := range parts {
		out[id] = strings.Join(ps, " · ")
	}
	return out, nil
}

// shown turns a value into a display row: an empty Label = show nothing; a Yes
// has an empty Value (the label says it).
func shown(r effective, max int) plugin.DetailRow {
	if r.Type == "bool" {
		if r.Value != "1" {
			return plugin.DetailRow{}
		}
		return plugin.DetailRow{Label: r.Label}
	}
	if strings.TrimSpace(r.Value) == "" {
		return plugin.DetailRow{}
	}
	return plugin.DetailRow{Label: r.Label, Value: truncate(r.Value, max)}
}

// TillRows are the till ⓘ rows (fields marked "show at till"); LabelRows the
// label top / bottom line options (fields marked "on label").
func (s *Store) TillRows(ctx context.Context, id int64) ([]plugin.DetailRow, error) {
	return s.detail(ctx, id, "show_at_till", 120)
}
func (s *Store) LabelRows(ctx context.Context, id int64) ([]plugin.DetailRow, error) {
	return s.detail(ctx, id, "print_on_label", 40)
}

func (s *Store) detail(ctx context.Context, id int64, flag string, max int) ([]plugin.DetailRow, error) {
	rows, err := s.effectiveValues(ctx, []int64{id}, flag)
	if err != nil {
		return nil, err
	}
	out := []plugin.DetailRow{}
	for _, r := range rows {
		if t := shown(r, max); t.Label != "" {
			if r.Type == "bool" {
				t.Value = "Yes"
			}
			out = append(out, t)
		}
	}
	return out, nil
}

// MatchProductIDs finds products whose searchable field reads a value containing
// q (own or default), or — for a yes/no reading Yes — whose label contains q.
// Only fields that apply to the product count.
//
// ponytail: substring match only; a typed column + index if searchable numbers
// ever need ranges.
func (s *Store) MatchProductIDs(ctx context.Context, q string) ([]int64, error) {
	var ids []int64
	err := s.db.SelectContext(ctx, &ids, `WITH `+covCTE+`
		SELECT DISTINCT p.id FROM products p
		JOIN pp_fields f ON f.is_active AND f.searchable AND `+appliesSQL("f", "p.category_id")+`
		LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = p.id
		WHERE (f.type <> 'bool' AND COALESCE(v.value, f.default_value) ILIKE '%' || $1 || '%')
		   OR (f.type = 'bool' AND COALESCE(v.value, f.default_value) = '1' AND f.label ILIKE '%' || $1 || '%')`, q)
	return ids, err
}

// truncate shortens a value for a compact line (rune-safe, with an ellipsis).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

// slugify turns a label into a stable key (lowercase, alnum + "_").
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('_')
		}
	}
	if out := strings.Trim(b.String(), "_"); out != "" {
		return out
	}
	return "field"
}
