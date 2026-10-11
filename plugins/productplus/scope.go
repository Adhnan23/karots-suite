package productplus

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// A field applies to all products when it has no pp_field_categories rows, else
// to products in those categories or any of their subcategories. Every read that
// shows, searches or counts a field goes through these two pieces of SQL, so the
// rule lives in one place.

// covCTE expands each scoped field's categories to their whole subtree. Put it
// first in a query (WITH RECURSIVE …) that then uses appliesSQL.
const covCTE = `RECURSIVE cov(field_id, category_id) AS (
	SELECT field_id, category_id FROM pp_field_categories
	UNION
	SELECT cov.field_id, c.id FROM categories c JOIN cov ON c.parent_id = cov.category_id
)`

// appliesSQL is true when field f applies to a product in category catExpr.
func appliesSQL(f, catExpr string) string {
	return `(NOT EXISTS (SELECT 1 FROM pp_field_categories s WHERE s.field_id = ` + f + `.id)
		OR EXISTS (SELECT 1 FROM cov WHERE cov.field_id = ` + f + `.id AND cov.category_id = ` + catExpr + `))`
}

// Cat is a category a field is scoped to.
type Cat struct {
	ID   int64  `db:"id"   json:"id"`
	Name string `db:"name" json:"name"`
}

// scopes returns each field's chosen categories (fields with none are absent).
func (s *Store) scopes(ctx context.Context) (map[int64][]Cat, error) {
	var rows []struct {
		FieldID int64 `db:"field_id"`
		Cat
	}
	err := s.db.SelectContext(ctx, &rows, `
		SELECT s.field_id, c.id, c.name FROM pp_field_categories s
		JOIN categories c ON c.id = s.category_id ORDER BY c.name`)
	out := map[int64][]Cat{}
	for _, r := range rows {
		out[r.FieldID] = append(out[r.FieldID], r.Cat)
	}
	return out, err
}

// setScope replaces a field's categories (none = all products).
func setScope(ctx context.Context, tx *sqlx.Tx, fieldID int64, cats []int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM pp_field_categories WHERE field_id=$1`, fieldID); err != nil {
		return err
	}
	if len(cats) == 0 {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO pp_field_categories (field_id, category_id)
		SELECT $1, unnest($2::bigint[]) ON CONFLICT DO NOTHING`, fieldID, pq.Array(cats))
	return err
}
