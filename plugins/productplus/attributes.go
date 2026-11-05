package productplus

import (
	"context"
	"errors"

	"karots-suite/internal/plugin"

	"github.com/lib/pq"
)

// attributes offers the shop's fields through the generic hook. Saving goes through
// SetValues, so a field's own type and options still decide what's valid.
func (s *Store) attributes() plugin.ProductAttributeSet {
	return plugin.ProductAttributeSet{Key: "productplus", Attributes: s.attrList, Values: s.attrValues, Known: s.attrKnown, Set: s.attrSet}
}

func (s *Store) attrList(ctx context.Context) ([]plugin.ProductAttribute, error) {
	fields, err := s.Fields(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		FieldID int64 `db:"field_id"`
		Cat     int64 `db:"category_id"`
	}
	if err := s.db.SelectContext(ctx, &rows, `WITH `+covCTE+` SELECT field_id, category_id FROM cov`); err != nil {
		return nil, err
	}
	cats := map[int64][]int64{}
	for _, r := range rows {
		cats[r.FieldID] = append(cats[r.FieldID], r.Cat)
	}
	out := make([]plugin.ProductAttribute, 0, len(fields))
	for _, f := range fields {
		out = append(out, plugin.ProductAttribute{Key: f.Key, Label: f.Label, Type: f.Type, Options: f.Options, CategoryIDs: cats[f.ID]})
	}
	return out, nil
}

func (s *Store) attrValues(ctx context.Context, ids []int64) (map[int64]map[string]string, error) {
	var rows []struct {
		ID    int64  `db:"product_id"`
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT v.product_id, f.key, v.value FROM pp_values v
		JOIN pp_fields f ON f.id = v.field_id WHERE v.product_id = ANY($1) AND v.value <> ''`, pq.Array(ids)); err != nil {
		return nil, err
	}
	out := map[int64]map[string]string{}
	for _, r := range rows {
		if out[r.ID] == nil {
			out[r.ID] = map[string]string{}
		}
		out[r.ID][r.Key] = r.Value
	}
	return out, nil
}

func (s *Store) attrKnown(ctx context.Context, attr string, limit int) ([]plugin.KnownValue, error) {
	var vals []string
	if err := s.db.SelectContext(ctx, &vals, `SELECT v.value FROM pp_values v JOIN pp_fields f ON f.id = v.field_id
		WHERE f.key = $1 AND v.value <> '' GROUP BY v.value ORDER BY COUNT(*) DESC, v.value LIMIT $2`, attr, limit); err != nil {
		return nil, err
	}
	out := make([]plugin.KnownValue, len(vals))
	for i, v := range vals {
		out[i].Value = v
	}
	return out, nil
}

func (s *Store) attrSet(ctx context.Context, productID int64, attr, value string) error {
	var fieldID int64
	if err := s.db.GetContext(ctx, &fieldID, `SELECT id FROM pp_fields WHERE key = $1 AND is_active`, attr); err != nil {
		return errors.New("that field is gone")
	}
	_, bad, err := s.SetValues(ctx, []Cell{{ProductID: productID, FieldID: fieldID, Value: value}})
	if err != nil {
		return err
	}
	if len(bad) > 0 {
		return errors.New(bad[0].Message)
	}
	return nil
}
