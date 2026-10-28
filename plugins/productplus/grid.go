package productplus

import (
	"context"
	"strings"

	"github.com/lib/pq"
)

// GridQuery picks the Fill in rows: active products (in CategoryID's subtree when
// set, name containing Q), and with MissingOnly only those where one of the
// chosen fields is required, applies and reads blank.
type GridQuery struct {
	FieldIDs    []int64
	CategoryID  int64
	MissingOnly bool
	Q           string
	Limit       int
	Offset      int
}

// GridRow is one product with its cells: Values has a key for each chosen field
// that applies to it — nil when it reads the default, else its own value.
type GridRow struct {
	ID           int64             `json:"id"`
	Name         string            `json:"name"`
	CategoryName string            `json:"categoryName"`
	Values       map[int64]*string `json:"values"`
}

const gridFrom = `
	FROM products p JOIN categories c ON c.id = p.category_id
	WHERE p.is_active AND NOT p.is_service
	  AND ($2 = 0 OR p.category_id IN (SELECT id FROM sub))
	  AND ($3 = '' OR p.name ILIKE '%' || $3 || '%' OR p.name_local ILIKE '%' || $3 || '%')
	  AND EXISTS (SELECT 1 FROM pp_fields f WHERE f.id = ANY($1) AND ` + "%s" + `
	      AND (NOT $4 OR (f.required AND f.type <> 'bool' AND btrim(COALESCE(
	          (SELECT value FROM pp_values v WHERE v.field_id = f.id AND v.product_id = p.id), f.default_value)) = '')))`

const subCTE = `, sub(id) AS (
	SELECT $2::bigint UNION SELECT c.id FROM categories c JOIN sub ON c.parent_id = sub.id
)`

func (s *Store) Grid(ctx context.Context, q GridQuery) ([]GridRow, int, error) {
	if len(q.FieldIDs) == 0 {
		return []GridRow{}, 0, nil
	}
	from := strings.Replace(gridFrom, "%s", appliesSQL("f", "p.category_id"), 1)
	args := []any{pq.Array(q.FieldIDs), q.CategoryID, strings.TrimSpace(q.Q), q.MissingOnly}
	var total int
	if err := s.db.GetContext(ctx, &total, `WITH `+covCTE+subCTE+` SELECT count(*) `+from, args...); err != nil {
		return nil, 0, err
	}
	rows := []GridRow{}
	if err := s.db.SelectContext(ctx, &rows, `WITH `+covCTE+subCTE+`
		SELECT p.id, p.name, c.name AS categoryname `+from+` ORDER BY p.name, p.id LIMIT $5 OFFSET $6`,
		append(args, q.Limit, q.Offset)...); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return rows, total, nil
	}
	ids := make([]int64, len(rows))
	at := map[int64]int{}
	for i, r := range rows {
		ids[i], at[r.ID] = r.ID, i
		rows[i].Values = map[int64]*string{}
	}
	var cells []struct {
		ProductID int64   `db:"product_id"`
		FieldID   int64   `db:"field_id"`
		Value     *string `db:"value"`
	}
	if err := s.db.SelectContext(ctx, &cells, `WITH `+covCTE+`
		SELECT p.id AS product_id, f.id AS field_id, v.value FROM products p
		JOIN pp_fields f ON f.id = ANY($1) AND `+appliesSQL("f", "p.category_id")+`
		LEFT JOIN pp_values v ON v.field_id = f.id AND v.product_id = p.id
		WHERE p.id = ANY($2)`, pq.Array(q.FieldIDs), pq.Array(ids)); err != nil {
		return nil, 0, err
	}
	for _, c := range cells {
		rows[at[c.ProductID]].Values[c.FieldID] = c.Value
	}
	return rows, total, nil
}

// SheetColumns are the active fields' labels, the products sheet's extra columns.
func (s *Store) SheetColumns(ctx context.Context) ([]string, error) {
	fs, err := s.Fields(ctx, false)
	cols := make([]string, len(fs))
	for i, f := range fs {
		cols[i] = f.Label
	}
	return cols, err
}

// SheetExport gives each product's cells in SheetColumns order: the value it
// reads where the field applies (yes / no for a yes/no), blank elsewhere.
func (s *Store) SheetExport(ctx context.Context, ids []int64) (map[int64][]string, error) {
	fs, err := s.Fields(ctx, false)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	col := map[string]int{}
	for i, f := range fs {
		col[f.Label] = i
	}
	rows, err := s.effectiveValues(ctx, ids, "is_active")
	if err != nil {
		return nil, err
	}
	out := map[int64][]string{}
	for _, r := range rows {
		if out[r.ProductID] == nil {
			out[r.ProductID] = make([]string, len(fs))
		}
		v := r.Value
		if r.Type == "bool" {
			v = map[bool]string{true: "yes", false: "no"}[v == "1"]
		}
		out[r.ProductID][col[r.Label]] = v
	}
	return out, nil
}

// SheetImport stores one imported row's cells (keyed by column = field label)
// for the fields that apply to the product; a cell that doesn't fit is skipped
// and named in the returned note.
func (s *Store) SheetImport(ctx context.Context, productID, cat int64, cells map[string]string) (string, error) {
	shown, err := s.FormFields(ctx, productID, cat, "form")
	if err != nil {
		return "", err
	}
	var notes []string
	for _, ff := range shown {
		raw, ok := cells[ff.Field.Label]
		if !ok {
			continue
		}
		cur := ""
		if ff.Own {
			cur = ff.Value
		}
		v, err := coerce(ff.Field, raw, cur)
		if err != nil {
			notes = append(notes, err.Error())
			continue
		}
		if err := upsert(ctx, s.db, ff.Field.ID, productID, v); err != nil {
			return "", err
		}
	}
	return strings.Join(notes, "; "), nil
}
