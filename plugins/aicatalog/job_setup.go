package aicatalog

import (
	"context"
	"strconv"
	"strings"

	"karots-suite/internal/features/products"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const setupPrompt = `Suggest how a shop should set up each product. Reply with STRICT JSON ONLY:
{"items":[{"id":0,"has_expiry":true,"pack_name":"","pack_size":0,"reason":""}]}
"has_expiry": does this kind of product carry an expiry date? null if you aren't sure.
"pack_name" and "pack_size": only if it's usually also sold in a bigger pack of whole units, such as "Card" and 10. Otherwise "" and 0.`

func setupJob() Job {
	return Job{Key: "setup", Label: "Setup details",
		Desc:   "Suggests which products carry an expiry date and what pack they come in. You fill in pack prices; a blank one follows the unit price.",
		Prompt: setupPromptFor, Parse: setupParse, Apply: setupApply, Undo: setupUndo}
}

func setupPromptFor(_ context.Context, _ *Plugin, b Batch) (string, string, error) {
	var u strings.Builder
	u.WriteString("PRODUCTS (id|name|category|details):\n")
	for _, x := range b.Products {
		u.WriteString(strconv.FormatInt(x.ID, 10) + "|" + x.Name + "|" + x.Category + "|" + strings.ReplaceAll(x.Hint(), "\n", " ") + "\n")
	}
	return setupPrompt, u.String(), nil
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func setupParse(ctx context.Context, p *Plugin, b Batch, reply string) ([]JobRow, error) {
	var r struct {
		Items []struct {
			ID        int64   `json:"id"`
			HasExpiry *bool   `json:"has_expiry"`
			PackName  string  `json:"pack_name"`
			PackSize  float64 `json:"pack_size"`
			Reason    string  `json:"reason"`
		} `json:"items"`
	}
	if err := decodeReply(reply, &r); err != nil {
		return nil, err
	}
	ids := make([]int64, len(b.Products))
	for i, x := range b.Products {
		ids[i] = x.ID
	}
	var cur []struct {
		ID      int64 `db:"id"`
		Expiry  bool  `db:"has_expiry"`
		Decimal bool  `db:"allow_decimal"`
	}
	if err := p.store.db.SelectContext(ctx, &cur, `SELECT p.id, p.has_expiry, u.allow_decimal FROM products p JOIN units u ON u.id = p.unit_id WHERE p.id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	expiry, decimals := map[int64]bool{}, map[int64]bool{}
	for _, c := range cur {
		expiry[c.ID], decimals[c.ID] = c.Expiry, c.Decimal
	}
	var packs []struct {
		ID   int64  `db:"product_id"`
		Name string `db:"name"`
	}
	if err := p.store.db.SelectContext(ctx, &packs, `SELECT product_id, name FROM product_packs WHERE product_id = ANY($1)`, pq.Array(ids)); err != nil {
		return nil, err
	}
	hasPack := map[string]bool{}
	for _, k := range packs {
		hasPack[strconv.FormatInt(k.ID, 10)+"|"+strings.ToLower(k.Name)] = true
	}
	asked := map[int64]bool{}
	for _, id := range ids {
		asked[id] = true
	}
	rows := []JobRow{}
	for _, it := range r.Items {
		if !asked[it.ID] {
			continue
		}
		// Any answer differs from a value the shop may have chosen, so it needs a tick.
		if it.HasExpiry != nil && *it.HasExpiry != expiry[it.ID] {
			rows = append(rows, JobRow{ProductID: it.ID, Kind: "expiry", Key: "expiry", Label: "Has an expiry date", Old: yesNo(expiry[it.ID]), New: yesNo(*it.HasExpiry), Reason: it.Reason, Change: true})
		}
		name := strings.TrimSpace(it.PackName)
		size := strconv.FormatFloat(it.PackSize, 'f', -1, 64)
		if name != "" && it.PackSize > 1 && it.PackSize == float64(int64(it.PackSize)) && !decimals[it.ID] && !hasPack[strconv.FormatInt(it.ID, 10)+"|"+strings.ToLower(name)] {
			rows = append(rows, JobRow{ProductID: it.ID, Kind: "pack", Key: "pack", Label: "Pack", New: name + " ×" + size, Reason: it.Reason, Ticked: !b.Redo, Extra: map[string]string{"name": name, "size": size}})
		}
	}
	return rows, nil
}

func setupApply(ctx context.Context, p *Plugin, tx *sqlx.Tx, r JobRow) ([]UndoOp, error) {
	switch r.Kind {
	case "expiry":
		var cur bool
		if err := tx.GetContext(ctx, &cur, `SELECT has_expiry FROM products WHERE id = $1`, r.ProductID); err != nil {
			return nil, err
		}
		if yesNo(cur) != r.Old {
			return nil, errStale
		}
		if _, err := tx.ExecContext(ctx, `UPDATE products SET has_expiry = $2 WHERE id = $1`, r.ProductID, r.New == "Yes"); err != nil {
			return nil, err
		}
		return []UndoOp{{RowKind: "expiry", ProductID: r.ProductID, Applied: r.New, Previous: r.Old}}, nil
	case "pack":
		k, err := p.core.Products.SavePack(ctx, r.ProductID, products.PackInput{Name: r.Extra["name"], Size: r.Extra["size"]})
		if err != nil {
			return nil, err
		}
		return []UndoOp{{RowKind: "pack", ProductID: r.ProductID, Key: strconv.FormatInt(k.ID, 10), Applied: k.Name + "|" + k.Size.String()}}, nil
	}
	return nil, errStale
}

func setupUndo(ctx context.Context, p *Plugin, tx *sqlx.Tx, op UndoOp) error {
	switch op.RowKind {
	case "expiry":
		res, err := tx.ExecContext(ctx, `UPDATE products SET has_expiry = $2 WHERE id = $1 AND has_expiry = $3`, op.ProductID, op.Previous == "Yes", op.Applied == "Yes")
		return staleIfNone(res, err)
	case "pack":
		id, err := strconv.ParseInt(op.Key, 10, 64)
		if err != nil {
			return err
		}
		// Only as it was made: a price, a barcode or a new size means someone has taken it on.
		name, size, _ := strings.Cut(op.Applied, "|")
		res, err := tx.ExecContext(ctx, `DELETE FROM product_packs k WHERE k.id = $1 AND k.name = $2 AND k.size = $3::numeric
			AND k.selling_price = 0 AND k.wholesale_price = 0
			AND NOT EXISTS (SELECT 1 FROM product_barcodes b WHERE b.pack_id = k.id)`, id, name, size)
		return staleIfNone(res, err)
	}
	return nil
}
