package productplus

import (
	"context"
	"encoding/json"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// mergeProduct moves a merged duplicate's field values to the kept product (core
// ProductMerger). Where both have a value for a field, the kept product's wins.
func mergeProduct(ctx context.Context, tx *sqlx.Tx, from, into int64) error {
	for _, q := range []string{
		`DELETE FROM pp_values d USING pp_values k
		  WHERE d.product_id = $1 AND k.product_id = $2 AND k.field_id = d.field_id`,
		`UPDATE pp_values SET product_id = $2 WHERE product_id = $1`,
	} {
		if _, err := tx.ExecContext(ctx, q, from, into); err != nil {
			return err
		}
	}
	return nil
}

// mergeCategory (core CategoryMerger): a field scoped to the merged category now
// also covers the one kept, so it keeps showing on the products that moved. The
// note lists the fields given the new scope row, so undo removes only those (a
// scope the shop had already set on the kept category stays).
func mergeCategory(ctx context.Context, tx *sqlx.Tx, from, into int64) (json.RawMessage, error) {
	var added []int64
	if err := tx.SelectContext(ctx, &added, `INSERT INTO pp_field_categories (field_id, category_id)
		SELECT field_id, $2 FROM pp_field_categories WHERE category_id = $1
		ON CONFLICT DO NOTHING RETURNING field_id`, from, into); err != nil {
		return nil, err
	}
	if len(added) == 0 {
		return nil, nil
	}
	return json.Marshal(categoryNote{Into: into, Fields: added})
}

type categoryNote struct {
	Into   int64   `json:"into"`
	Fields []int64 `json:"fields"`
}

// unmergeCategory removes the scope rows mergeCategory added. The merged
// category's own rows were never removed (no FK), so they match again once it is
// back with its old id.
func unmergeCategory(ctx context.Context, tx *sqlx.Tx, raw json.RawMessage) error {
	var n categoryNote
	if err := json.Unmarshal(raw, &n); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM pp_field_categories WHERE category_id = $1 AND field_id = ANY($2)`, n.Into, pq.Array(n.Fields))
	return err
}
