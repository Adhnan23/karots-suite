package aicatalog

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"karots-suite/internal/features/categories"

	"github.com/jmoiron/sqlx"
)

const identifyListPrompt = `You identify the products of a shop in Sri Lanka, of any kind, from their names or codes.
Reply with STRICT JSON ONLY, one item per product, matching:
{"items":[{"id":0,"confident":true,"name":"","category":"","specs":"","explanation":""}]}
Look each item up to confirm it when web search is available. Codes are easily confused.
If you can't identify a product, set confident to false and leave its other fields empty. Never invent a name for it.
"name": its clear full name.
"category": uses " > " between levels. Keep it GENERAL and SHALLOW: usually one broad category, at most two levels.
Reuse an existing category's path exactly when one fits; only propose a new one when none does.
"specs": what a customer would ask to be sure it's the right item, as far as it applies: what it fits or works with
(models, sizes), its size or strength, type, ingredients, what it's for. Leave out what you can't confirm.
"explanation": one short sentence a non-expert understands: what the item is and what it's for.`

func identifyJob() Job {
	return Job{Key: "identify", Label: "Identify a whole list",
		Desc: "Names, categories and searchable hints for many products. Prices and stock never change.",
		Select: func(ctx context.Context, p *Plugin, in JobRequest) (Batch, int, error) {
			in.noHint = len(in.IDs) == 0 && in.Filter == "" && !in.Force // products without hints; a redo picks its own
			key := in.recordKey("identify")
			prods, total, err := selectProducts(ctx, p, key, in)
			b := Batch{Products: prods}
			if in.Force {
				b.State = key
			}
			return b, total, err
		},
		Prompt: identifyPrompt, Parse: identifyParse, Apply: identifyApply, Undo: identifyUndo}
}

func identifyPrompt(ctx context.Context, p *Plugin, b Batch) (string, string, error) {
	cats, err := p.store.Categories(ctx)
	if err != nil {
		return "", "", err
	}
	paths := pathsByID(cats)
	list := make([]string, 0, len(paths))
	for _, path := range paths {
		list = append(list, path)
	}
	sort.Strings(list)
	var u strings.Builder
	if len(list) > 0 {
		u.WriteString("EXISTING categories:\n- " + strings.Join(list, "\n- ") + "\n\n")
	}
	u.WriteString("PRODUCTS (id|name):\n")
	for _, x := range b.Products {
		u.WriteString(strconv.FormatInt(x.ID, 10) + "|" + x.Name + "\n")
	}
	system := identifyListPrompt
	return withNaming(system, b.Settings.NamingStyle), u.String(), nil
}

func hintText(specs, explanation string) string {
	return strings.TrimSpace(strings.TrimSpace(specs) + "\n" + strings.TrimSpace(explanation))
}

func identifyParse(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error) {
	var r struct {
		Items []struct {
			ID          int64  `json:"id"`
			Confident   bool   `json:"confident"`
			Name        string `json:"name"`
			Category    string `json:"category"`
			Specs       string `json:"specs"`
			Explanation string `json:"explanation"`
		} `json:"items"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	cats, err := p.store.Categories(ctx)
	if err != nil {
		return nil, err
	}
	paths := pathsByID(cats)
	known := map[string]bool{}
	for _, path := range paths {
		known[strings.ToLower(path)] = true
	}
	byID := map[int64]JobProduct{}
	for _, x := range b.Products {
		byID[x.ID] = x
	}
	rows := []JobRow{}
	seen := map[int64]bool{}
	for _, it := range r.Items {
		x, ok := byID[it.ID]
		if !ok || seen[it.ID] || !it.Confident { // not sure = no answer, whatever else it wrote
			continue
		}
		seen[it.ID] = true
		if hint := hintText(it.Specs, it.Explanation); hint != "" {
			old := hintText(x.Specs, x.Explain)
			if hint != old {
				rows = append(rows, JobRow{ProductID: it.ID, Kind: "hint", Key: "hint", Label: "Hint", Old: old, New: hint, Change: old != "", Ticked: old == "",
					Extra: map[string]string{"specs": capHint(it.Specs), "explanation": capHint(it.Explanation)}})
			}
		}
		if path := normPath(it.Category); path != "" && !strings.EqualFold(path, x.Category) {
			uncat := x.Category == "" || strings.EqualFold(x.Category, "uncategorized")
			rows = append(rows, JobRow{ProductID: it.ID, Kind: "category", Key: "category", Label: "Category", Old: x.Category, New: path,
				IsNew: !known[strings.ToLower(path)], Change: !uncat, Ticked: uncat})
		}
		if name := strings.TrimSpace(it.Name); name != "" && utf8.RuneCountInString(name) <= 150 && !strings.EqualFold(name, x.Name) {
			rows = append(rows, JobRow{ProductID: it.ID, Kind: "name", Key: "name", Label: "Name", Old: x.Name, New: name, Change: true})
		}
	}
	return rows, nil
}

func identifyApply(ctx context.Context, p *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error) {
	switch r.Kind {
	case "hint":
		var cur struct {
			Specs   string `db:"specs"`
			Explain string `db:"user_explanation"`
		}
		_ = tx.GetContext(ctx, &cur, `SELECT specs, user_explanation FROM aicatalog_items WHERE product_id = $1`, r.ProductID)
		if hintText(cur.Specs, cur.Explain) != r.Old {
			return nil, errStale
		}
		specs, explanation := capHint(r.Extra["specs"]), capHint(r.Extra["explanation"])
		if _, err := tx.ExecContext(ctx, `INSERT INTO aicatalog_items (product_id, resolved_name, specs, user_explanation, source)
			VALUES ($1, '', $2, $3, 'job') ON CONFLICT (product_id) DO UPDATE SET specs = $2, user_explanation = $3, updated_at = now()`,
			r.ProductID, specs, explanation); err != nil {
			return nil, err
		}
		return []UndoOp{{RowKind: "hint", ProductID: r.ProductID, Applied: hintText(specs, explanation),
			Extra: map[string]string{"specs": cur.Specs, "explanation": cur.Explain}}}, nil
	case "name":
		var old string
		if err := tx.GetContext(ctx, &old, `SELECT name FROM products WHERE id = $1`, r.ProductID); err != nil {
			return nil, err
		}
		if old != r.Old {
			return nil, errStale
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET name = $2 WHERE id = $1`, r.ProductID, r.New); err != nil {
			return nil, err
		}
		if err := p.store.AddOldName(ctx, tx, r.ProductID, old); err != nil {
			return nil, err
		}
		return []UndoOp{{RowKind: "name", ProductID: r.ProductID, Applied: r.New, Previous: old}}, nil
	case "category":
		var curID int64
		if err := tx.GetContext(ctx, &curID, `SELECT category_id FROM products WHERE id = $1`, r.ProductID); err != nil {
			return nil, err
		}
		cats, err := p.store.Categories(ctx)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(pathsByID(cats)[curID], r.Old) {
			return nil, errStale
		}
		var maxBefore int64
		if err := tx.GetContext(ctx, &maxBefore, `SELECT COALESCE(max(id), 0) FROM categories`); err != nil {
			return nil, err
		}
		cat, err := categories.NewService(tx).FindOrCreateByPath(ctx, normPath(r.New))
		if err != nil {
			return nil, err
		}
		var created []int64
		if err := tx.SelectContext(ctx, &created, `SELECT id FROM categories WHERE id > $1 ORDER BY id`, maxBefore); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET category_id = $2 WHERE id = $1`, r.ProductID, cat); err != nil {
			return nil, err
		}
		// Undo replays backwards: the product moves back first, then its new categories
		// go, child before parent.
		ops := make([]UndoOp, 0, len(created)+1)
		for _, id := range created {
			ops = append(ops, UndoOp{RowKind: "created_category", ProductID: r.ProductID, Applied: strconv.FormatInt(id, 10)})
		}
		return append(ops, UndoOp{RowKind: "category", ProductID: r.ProductID, Applied: strconv.FormatInt(cat, 10), Previous: strconv.FormatInt(curID, 10)}), nil
	}
	return nil, errStale
}

func identifyUndo(ctx context.Context, _ *Plugin, tx *sqlx.Tx, op UndoOp) error {
	switch op.RowKind {
	case "hint":
		res, err := tx.ExecContext(ctx, `UPDATE aicatalog_items SET specs = $2, user_explanation = $3
			WHERE product_id = $1 AND trim(both E'\n' FROM trim(specs) || E'\n' || trim(user_explanation)) = $4`,
			op.ProductID, op.Extra["specs"], op.Extra["explanation"], op.Applied)
		return staleIfNone(res, err)
	case "name":
		res, err := tx.ExecContext(ctx, `UPDATE products SET name = $2 WHERE id = $1 AND name = $3`, op.ProductID, op.Previous, op.Applied)
		return staleIfNone(res, err)
	case "category":
		res, err := tx.ExecContext(ctx, `UPDATE products SET category_id = $2 WHERE id = $1 AND category_id = $3`, op.ProductID, op.Previous, op.Applied)
		return staleIfNone(res, err)
	case "created_category":
		_, err := tx.ExecContext(ctx, `DELETE FROM categories c WHERE c.id = $1
			AND NOT EXISTS (SELECT 1 FROM products WHERE category_id = c.id)
			AND NOT EXISTS (SELECT 1 FROM categories WHERE parent_id = c.id)`, op.Applied)
		return err
	}
	return nil
}
