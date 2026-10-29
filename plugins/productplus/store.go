package productplus

import (
	"context"
	"encoding/json"
	"fmt"

	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

// Field is one admin-defined custom product field.
type Field struct {
	ID           int64  `db:"id"`
	Key          string `db:"key"`
	Label        string `db:"label"`
	Type         string `db:"type"` // text | textarea | number | bool | select | date
	DefaultValue string `db:"default_value"`
	Hint         string `db:"hint"`
	Required     bool   `db:"required"`
	Searchable   bool   `db:"searchable"`
	ShowAtTill   bool   `db:"show_at_till"`
	PrintOnLabel bool   `db:"print_on_label"`
	ShowInIntake bool   `db:"show_in_intake"`
	// OptionsRaw is the options JSONB as stored; Options is it decoded.
	OptionsRaw []byte   `db:"options"`
	Options    []string `db:"-"`
	SortOrder  int      `db:"sort_order"`
	IsActive   bool     `db:"is_active"`
	Scope      []Cat    `db:"-"` // chosen categories; none = all products
}

const fieldColumns = `id, key, label, type, default_value, hint, required, searchable,
	show_at_till, print_on_label, show_in_intake, options, sort_order, is_active`

type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

func (f *Field) decodeOptions() {
	f.Options = nil
	if len(f.OptionsRaw) > 0 {
		_ = json.Unmarshal(f.OptionsRaw, &f.Options)
	}
}

func optionsJSON(opts []string) any {
	if len(opts) == 0 {
		return nil
	}
	b, _ := json.Marshal(opts)
	return b
}

// Fields lists fields in display order with their scope.
func (s *Store) Fields(ctx context.Context, includeDisabled bool) ([]Field, error) {
	var rows []Field
	if err := s.db.SelectContext(ctx, &rows, `SELECT `+fieldColumns+` FROM pp_fields
		WHERE ($1 OR is_active) ORDER BY sort_order, id`, includeDisabled); err != nil {
		return nil, err
	}
	sc, err := s.scopes(ctx)
	for i := range rows {
		rows[i].decodeOptions()
		rows[i].Scope = sc[rows[i].ID]
	}
	return rows, err
}

func (s *Store) Field(ctx context.Context, id int64) (*Field, error) {
	var f Field
	if err := s.db.GetContext(ctx, &f, `SELECT `+fieldColumns+` FROM pp_fields WHERE id=$1`, id); err != nil {
		return nil, err
	}
	f.decodeOptions()
	sc, err := s.scopes(ctx)
	f.Scope = sc[id]
	return &f, err
}

// CreateField adds a field at the end of the order with a unique key slugged
// from its label, and its scope.
func (s *Store) CreateField(ctx context.Context, f Field, scope []int64) (int64, error) {
	var id int64
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		key, err := uniqueKey(ctx, tx, slugify(f.Label))
		if err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &id, `
			INSERT INTO pp_fields (key, label, type, default_value, hint, required, searchable,
				show_at_till, print_on_label, show_in_intake, options, sort_order, is_active)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,(SELECT count(*) FROM pp_fields),true) RETURNING id`,
			key, f.Label, f.Type, f.DefaultValue, f.Hint, f.Required, f.Searchable,
			f.ShowAtTill, f.PrintOnLabel, f.ShowInIntake, optionsJSON(f.Options)); err != nil {
			return err
		}
		return setScope(ctx, tx, id, scope)
	})
	return id, err
}

// UpdateOpts says what an edit may do to existing products. DefaultFor "new"
// (the default) writes the old default onto every product that read it, so only
// new products get the new one; "all" lets them follow. ConfirmLoss allows a
// type change that deletes values not fitting the new type.
type UpdateOpts struct {
	DefaultFor  string
	ConfirmLoss bool
}

// ErrDataLoss refuses a type change that would delete values, until confirmed.
type ErrDataLoss struct{ Lossy, Total int }

func (e *ErrDataLoss) Error() string {
	return fmt.Sprintf("%d of %d values don't fit the new type", e.Lossy, e.Total)
}

// UpdateField saves an edit (never the key or the order) in one tx: the type
// change (values that fit are normalised, the rest deleted — only with
// ConfirmLoss), the default change per DefaultFor, then the field and its scope.
func (s *Store) UpdateField(ctx context.Context, f Field, scope []int64, o UpdateOpts) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		var old Field
		if err := tx.GetContext(ctx, &old, `SELECT `+fieldColumns+` FROM pp_fields WHERE id=$1 FOR UPDATE`, f.ID); err != nil {
			return err
		}
		old.decodeOptions()
		if old.Type != f.Type {
			if err := convertValues(ctx, tx, f, o.ConfirmLoss); err != nil {
				return err
			}
		}
		if old.DefaultValue != f.DefaultValue && o.DefaultFor != "all" {
			// Keep what existing products show: they read the old default, so store it.
			if _, err := tx.ExecContext(ctx, `WITH `+covCTE+`
				INSERT INTO pp_values (field_id, product_id, value)
				SELECT f.id, p.id, f.default_value FROM pp_fields f, products p
				WHERE f.id = $1 AND NOT p.is_service AND `+appliesSQL("f", "p.category_id")+`
				ON CONFLICT (field_id, product_id) DO NOTHING`, f.ID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE pp_fields SET label=$2, type=$3, default_value=$4, hint=$5, required=$6,
			       searchable=$7, show_at_till=$8, print_on_label=$9, show_in_intake=$10, options=$11
			WHERE id=$1`,
			f.ID, f.Label, f.Type, f.DefaultValue, f.Hint, f.Required, f.Searchable,
			f.ShowAtTill, f.PrintOnLabel, f.ShowInIntake, optionsJSON(f.Options)); err != nil {
			return err
		}
		return setScope(ctx, tx, f.ID, scope)
	})
}

// convertValues re-reads every stored value of a field as its new type f.
func convertValues(ctx context.Context, tx *sqlx.Tx, f Field, confirm bool) error {
	var rows []struct {
		ProductID int64  `db:"product_id"`
		Value     string `db:"value"`
	}
	if err := tx.SelectContext(ctx, &rows, `SELECT product_id, value FROM pp_values WHERE field_id=$1`, f.ID); err != nil {
		return err
	}
	var drop []int64
	fit := map[int64]string{}
	for _, r := range rows {
		if v, err := coerce(f, r.Value, ""); err != nil {
			drop = append(drop, r.ProductID)
		} else if v != r.Value {
			fit[r.ProductID] = v
		}
	}
	if len(drop) > 0 && !confirm {
		return &ErrDataLoss{Lossy: len(drop), Total: len(rows)}
	}
	for _, pid := range drop {
		if _, err := tx.ExecContext(ctx, `DELETE FROM pp_values WHERE field_id=$1 AND product_id=$2`, f.ID, pid); err != nil {
			return err
		}
	}
	for pid, v := range fit {
		if _, err := tx.ExecContext(ctx, `UPDATE pp_values SET value=$3 WHERE field_id=$1 AND product_id=$2`, f.ID, pid, v); err != nil {
			return err
		}
	}
	return nil
}

// Impact is what an edit would do: Lossy of Total stored values don't fit
// newType; UsingDefault products (in scope, no value of their own) read the default.
type Impact struct {
	Lossy        int `json:"lossy"`
	Total        int `json:"total"`
	UsingDefault int `json:"usingDefault"`
}

func (s *Store) Impact(ctx context.Context, id int64, newType string, options []string) (Impact, error) {
	var im Impact
	f, err := s.Field(ctx, id)
	if err != nil {
		return im, err
	}
	var vals []string
	if err := s.db.SelectContext(ctx, &vals, `SELECT value FROM pp_values WHERE field_id=$1`, id); err != nil {
		return im, err
	}
	nf := *f
	nf.Type = newType
	if options != nil {
		nf.Options = options
	}
	im.Total = len(vals)
	for _, v := range vals {
		if _, err := coerce(nf, v, ""); err != nil {
			im.Lossy++
		}
	}
	st, err := s.Stats(ctx)
	im.UsingDefault = st[id].UsingDefault
	return im, err
}

// Stats per field over active products in its scope: how many read the default
// (no value of their own) and how many are Missing — a required field whose
// value (own or default) is blank. A yes/no is never missing.
type Stats struct{ UsingDefault, Missing int }

func (s *Store) Stats(ctx context.Context) (map[int64]Stats, error) {
	var rows []struct {
		ID           int64 `db:"id"`
		UsingDefault int   `db:"using_default"`
		Missing      int   `db:"missing"`
	}
	err := s.db.SelectContext(ctx, &rows, `WITH `+covCTE+`
		SELECT f.id,
			count(*) FILTER (WHERE v.value IS NULL) AS using_default,
			count(*) FILTER (WHERE f.required AND f.type <> 'bool' AND btrim(COALESCE(v.value, f.default_value)) = '') AS missing
		FROM pp_fields f
		JOIN products p ON p.is_active AND NOT p.is_service AND `+appliesSQL("f", "p.category_id")+`
		LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = p.id
		GROUP BY f.id`)
	out := make(map[int64]Stats, len(rows))
	for _, r := range rows {
		out[r.ID] = Stats{UsingDefault: r.UsingDefault, Missing: r.Missing}
	}
	return out, err
}

func (s *Store) SetFieldActive(ctx context.Context, id int64, active bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE pp_fields SET is_active=$2 WHERE id=$1`, id, active)
	return err
}

// MoveField shifts a field one place up or down, renumbering every field so
// ties resolve deterministically. No-op at the edges.
//
// ponytail: rewrites every row per move — fine for a short admin field list.
func (s *Store) MoveField(ctx context.Context, id int64, up bool) error {
	fields, err := s.Fields(ctx, true)
	if err != nil {
		return err
	}
	i := -1
	for k := range fields {
		if fields[k].ID == id {
			i = k
		}
	}
	j := i + 1
	if up {
		j = i - 1
	}
	if i < 0 || j < 0 || j >= len(fields) {
		return nil
	}
	fields[i], fields[j] = fields[j], fields[i]
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		for n, f := range fields {
			if _, err := tx.ExecContext(ctx, `UPDATE pp_fields SET sort_order=$2 WHERE id=$1`, f.ID, n); err != nil {
				return err
			}
		}
		return nil
	})
}

// uniqueKey returns base, or base_2, base_3, … until it finds a free key.
func uniqueKey(ctx context.Context, tx *sqlx.Tx, base string) (string, error) {
	key := base
	for i := 2; ; i++ {
		var n int
		if err := tx.GetContext(ctx, &n, `SELECT count(*) FROM pp_fields WHERE key=$1`, key); err != nil {
			return "", err
		}
		if n == 0 {
			return key, nil
		}
		key = fmt.Sprintf("%s_%d", base, i)
	}
}
