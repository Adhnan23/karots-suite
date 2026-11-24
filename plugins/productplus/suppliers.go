package productplus

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Supplier by field (PPF-10…13): the shop picks one text or dropdown field and a
// supplier per value; products with no own supplier follow it through core's
// plugin.RuleSupplier hook (SUP-12). Product Plus never writes a product's own
// supplier.

// usableTypes are the field types that can be the supplier field.
const usableTypes = `('text', 'select')`

// ruleRowsSQL is every product the rules give a supplier: the chosen field active
// and usable, the product in its scope, its value (own or the field's default)
// matching a rule. Put it after `WITH ` + covCTE.
const ruleRowsSQL = `, rule AS (
	SELECT p.id AS product_id, m.supplier_id, f.label || ': ' || btrim(COALESCE(v.value, f.default_value)) AS why
	FROM pp_supplier_field sf
	JOIN pp_fields f ON f.id = sf.field_id AND f.is_active AND f.type IN ` + usableTypes + `
	JOIN products p ON p.merged_into_id IS NULL AND ` + `%APPLIES%` + `
	LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = p.id
	JOIN pp_supplier_map m ON m.value_key = lower(btrim(COALESCE(v.value, f.default_value)))
)`

var ruleCTE = strings.Replace(ruleRowsSQL, "%APPLIES%", appliesSQL("f", "p.category_id"), 1)

func (s *Store) ruleSupplier() plugin.RuleSupplier {
	return plugin.RuleSupplier{For: s.rulesFor, Products: s.ruleProducts}
}

func (s *Store) rulesFor(ctx context.Context, ids []int64) (map[int64]plugin.RuleSupplierOf, error) {
	var rows []struct {
		ID  int64  `db:"product_id"`
		Sup int64  `db:"supplier_id"`
		Why string `db:"why"`
	}
	if err := appdb.On(ctx, s.db).SelectContext(ctx, &rows, `WITH `+covCTE+ruleCTE+`
		SELECT product_id, supplier_id, why FROM rule WHERE product_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	out := make(map[int64]plugin.RuleSupplierOf, len(rows))
	for _, r := range rows {
		out[r.ID] = plugin.RuleSupplierOf{SupplierID: r.Sup, Why: r.Why}
	}
	return out, nil
}

func (s *Store) ruleProducts(ctx context.Context, supplierID int64) ([]int64, error) {
	var ids []int64
	err := appdb.On(ctx, s.db).SelectContext(ctx, &ids, `WITH `+covCTE+ruleCTE+`
		SELECT product_id FROM rule WHERE supplier_id = $1`, supplierID)
	return ids, err
}

// moveRules points rules at the supplier whose products moved (PPF-13).
func moveRules(ctx context.Context, tx *sqlx.Tx, fromID, toID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE pp_supplier_map SET supplier_id = $2 WHERE supplier_id = $1`, fromID, toID)
	return err
}

// SupplierValue is one distinct value of the supplier field (PPF-12).
type SupplierValue struct {
	Value        string `db:"value"`
	Products     int    `db:"products"`
	SupplierID   int64  `db:"supplier_id"`
	SupplierName string `db:"supplier_name"`
}

// SupplierField is the chosen field's id, 0 when none.
func (s *Store) SupplierField(ctx context.Context) (int64, error) {
	var id int64
	err := appdb.On(ctx, s.db).GetContext(ctx, &id, `SELECT field_id FROM pp_supplier_field`)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// SupplierValues is the chosen field's value list: every distinct value (trimmed,
// any case; its most used spelling) of products in scope, plus a dropdown's unused
// options; no supplier first, then most products first (PPF-12).
func (s *Store) SupplierValues(ctx context.Context, fieldID int64) ([]SupplierValue, error) {
	out := []SupplierValue{}
	if fieldID == 0 {
		return out, nil
	}
	err := appdb.On(ctx, s.db).SelectContext(ctx, &out, `WITH `+covCTE+`, vals AS (
			SELECT btrim(COALESCE(v.value, f.default_value)) AS value, 1 AS n
			FROM pp_fields f
			JOIN products p ON p.merged_into_id IS NULL AND p.is_active AND `+appliesSQL("f", "p.category_id")+`
			LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = p.id
			WHERE f.id = $1
			UNION ALL
			SELECT btrim(o), 0 FROM pp_fields f, jsonb_array_elements_text(COALESCE(f.options, '[]')) o
			WHERE f.id = $1 AND f.type = 'select'
		), counted AS (
			SELECT lower(value) AS key, value, sum(n) AS n FROM vals WHERE value <> '' GROUP BY lower(value), value
		), grouped AS (
			SELECT key, (array_agg(value ORDER BY n DESC, value))[1] AS value, sum(n)::int AS products
			FROM counted GROUP BY key
		)
		SELECT g.value, g.products, COALESCE(m.supplier_id, 0) AS supplier_id, COALESCE(su.name, '') AS supplier_name
		FROM grouped g
		LEFT JOIN pp_supplier_map m ON m.value_key = g.key
		LEFT JOIN suppliers su ON su.id = m.supplier_id
		ORDER BY m.supplier_id IS NOT NULL, g.products DESC, g.value`, fieldID)
	return out, err
}

// ErrNeedsConfirm: switching the supplier field would clear its rules (PPF-10).
var ErrNeedsConfirm = errors.New("needs confirm")

// ErrUnusableField: only an active text or dropdown field can be chosen.
var ErrUnusableField = errors.New("unusable field")

// SetSupplierField chooses the field (0 = none). A different field while rules
// exist needs confirm, and clears them.
func (s *Store) SetSupplierField(ctx context.Context, fieldID int64, confirm bool) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if err := setSupplierField(ctx, tx, fieldID, confirm); err != nil {
		return err
	}
	return tx.Commit()
}

func setSupplierField(ctx context.Context, tx *sqlx.Tx, fieldID int64, confirm bool) error {
	var cur int64
	if err := tx.GetContext(ctx, &cur, `SELECT COALESCE((SELECT field_id FROM pp_supplier_field FOR UPDATE), 0)`); err != nil {
		return err
	}
	if cur == fieldID {
		return nil
	}
	if fieldID != 0 {
		var ok bool
		if err := tx.GetContext(ctx, &ok, `SELECT EXISTS (SELECT 1 FROM pp_fields WHERE id = $1 AND is_active AND type IN `+usableTypes+`)`, fieldID); err != nil {
			return err
		}
		if !ok {
			return ErrUnusableField
		}
	}
	var rules int
	if err := tx.GetContext(ctx, &rules, `SELECT count(*) FROM pp_supplier_map`); err != nil {
		return err
	}
	if rules > 0 && !confirm {
		return ErrNeedsConfirm
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM pp_supplier_map; DELETE FROM pp_supplier_field`); err != nil {
		return err
	}
	if fieldID == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pp_supplier_field (field_id) VALUES ($1)`, fieldID)
	return err
}

// SetSupplierRule sets the supplier for a value (0 removes the rule).
func (s *Store) SetSupplierRule(ctx context.Context, value string, supplierID int64) error {
	key := strings.ToLower(strings.TrimSpace(value))
	if supplierID == 0 {
		_, err := appdb.On(ctx, s.db).ExecContext(ctx, `DELETE FROM pp_supplier_map WHERE value_key = $1`, key)
		return err
	}
	_, err := appdb.On(ctx, s.db).ExecContext(ctx, `INSERT INTO pp_supplier_map (value_key, supplier_id) VALUES ($1, $2)
		ON CONFLICT (value_key) DO UPDATE SET supplier_id = EXCLUDED.supplier_id`, key, supplierID)
	return err
}
