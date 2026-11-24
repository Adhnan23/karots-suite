package catalogue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// keepMine reviews the pick and keeps the product as the shop has it: the link takes the catalogue's values as
// they are now, so the same change isn't asked again, and a waiting change goes (CTL-9, REV-2).
func (s *Store) keepMine(ctx context.Context, productID int64) error {
	return appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE catalogue_links l SET reviewed = true,
			took = coalesce((SELECT jsonb_build_object('name', i.name, 'unit', i.unit, 'fields', i.fields)
				FROM catalogue_items i WHERE i.id = l.item_id), took)
			WHERE product_id = $1`, productID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM catalogue_reviews WHERE product_id = $1`, productID)
		return err
	})
}

// change is one thing the catalogue changed on a linked product: what, the catalogue's value then and now.
type change struct {
	What string `json:"what"`
	Then string `json:"then"`
	Now  string `json:"now"`
}

type linkRow struct {
	ProductID int64  `db:"product_id"`
	ItemID    int64  `db:"item_id"`
	Took      []byte `db:"took"`
	Name      string `db:"name"`
	Unit      string `db:"unit"`
}

// afterChange runs inside a sync: a gone item's links follow it to the item it went into (CTL-12), and a linked
// product whose item changed something the shop has waits on Items to review, unless it already has the new
// values (CTL-8); otherwise the link quietly takes the new values.
func (s *Store) afterChange(ctx context.Context, tx *sqlx.Tx, changed map[int64]Item, gone []gone) error {
	moved := []int64{}
	for _, g := range gone {
		if g.Into <= 0 {
			continue // dropped with nothing in its place: the link stays, nothing to review
		}
		if _, err := tx.ExecContext(ctx, `UPDATE catalogue_links SET item_id = $2 WHERE item_id = $1`, g.ID, g.Into); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE catalogue_reviews SET item_id = $2 WHERE item_id = $1`, g.ID, g.Into); err != nil {
			return err
		}
		moved = append(moved, g.Into)
	}
	ids := append([]int64{}, moved...)
	for id := range changed {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}
	var links []linkRow
	if err := tx.SelectContext(ctx, &links, `SELECT l.product_id, l.item_id, l.took, p.name, lower(u.abbreviation) AS unit
		FROM catalogue_links l JOIN products p ON p.id = l.product_id JOIN units u ON u.id = p.unit_id
		WHERE l.item_id = ANY($1)`, pq.Array(ids)); err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	f, err := s.filler(ctx)
	if err != nil {
		return err
	}
	pids := make([]int64, len(links))
	for i, l := range links {
		pids[i] = l.ProductID
	}
	vals, err := shopValues(ctx, pids)
	if err != nil {
		return err
	}
	for _, l := range links {
		var r itemRow
		if err := tx.GetContext(ctx, &r, `SELECT id, name, unit, kinds, fields FROM catalogue_items WHERE id = $1`, l.ItemID); err != nil {
			return err
		}
		now := r.item()
		var then took
		_ = json.Unmarshal(l.Took, &then)
		var cs []change
		if then.Name != now.Name && !strings.EqualFold(strings.TrimSpace(l.Name), strings.TrimSpace(now.Name)) {
			cs = append(cs, change{"Name", then.Name, now.Name})
		}
		if then.Unit != now.Unit && l.Unit != strings.ToLower(now.Unit) {
			cs = append(cs, change{"Unit", then.Unit, now.Unit})
		}
		for name := range unionKeys(then.Fields, now.Fields) {
			ref, has := f.fields[norm(name)]
			if !has || then.Fields[name] == now.Fields[name] {
				continue // a field the shop doesn't have is never a review
			}
			if vals[l.ProductID][ref.set+"/"+ref.key] == now.Fields[name] {
				continue // the product already has it
			}
			cs = append(cs, change{name, then.Fields[name], now.Fields[name]})
		}
		if len(cs) == 0 {
			t, _ := json.Marshal(took{Name: now.Name, Unit: now.Unit, Fields: now.Fields})
			if _, err := tx.ExecContext(ctx, `UPDATE catalogue_links SET took = $2 WHERE product_id = $1`, l.ProductID, t); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM catalogue_reviews WHERE product_id = $1`, l.ProductID); err != nil {
				return err
			}
			continue
		}
		b, _ := json.Marshal(cs)
		if _, err := tx.ExecContext(ctx, `INSERT INTO catalogue_reviews (product_id, item_id, changes) VALUES ($1, $2, $3)
			ON CONFLICT (product_id) DO UPDATE SET item_id = EXCLUDED.item_id, changes = EXCLUDED.changes, created_at = now()`,
			l.ProductID, l.ItemID, b); err != nil {
			return err
		}
	}
	return nil
}

// shopValues is every product's own field values, "set/key" → value.
func shopValues(ctx context.Context, ids []int64) (map[int64]map[string]string, error) {
	out := map[int64]map[string]string{}
	for _, a := range plugin.ProductAttributeSets() {
		if a.Values == nil {
			continue
		}
		m, err := a.Values(ctx, ids)
		if err != nil {
			return nil, err
		}
		for id, kv := range m {
			if out[id] == nil {
				out[id] = map[string]string{}
			}
			for k, v := range kv {
				out[id][a.Key+"/"+k] = v
			}
		}
	}
	return out, nil
}

func unionKeys(a, b map[string]string) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}

// Waiting is a product's waiting catalogue change, for its Items to review buttons.
func (s *Store) Waiting(ctx context.Context, productID int64) ([]change, error) {
	var raw []byte
	err := s.db.GetContext(ctx, &raw, `SELECT changes FROM catalogue_reviews WHERE product_id = $1`, productID)
	if errors.Is(err, sql.ErrNoRows) {
		return []change{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cs []change
	return cs, json.Unmarshal(raw, &cs)
}

// useCatalogue writes the waiting change's name and fields (never the unit: stock is counted in it), then keeps
// the link as the catalogue has it now (CTL-9). rename is core's rename.
func (s *Store) useCatalogue(ctx context.Context, productID int64, rename func(ctx context.Context, id int64, name string) error) error {
	cs, err := s.Waiting(ctx, productID)
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		return apperr.Validation("nothing from the catalogue is waiting on this product")
	}
	f, err := s.filler(ctx)
	if err != nil {
		return err
	}
	for _, c := range cs {
		switch {
		case c.What == "Name":
			if err := rename(ctx, productID, c.Now); err != nil {
				return err
			}
		case c.What == "Unit":
		default:
			ref, ok := f.fields[norm(c.What)]
			if !ok {
				continue
			}
			for _, a := range plugin.ProductAttributeSets() {
				if a.Key == ref.set && a.Set != nil {
					if err := a.Set(ctx, productID, ref.key, c.Now); err != nil {
						return err
					}
				}
			}
		}
	}
	return s.keepMine(ctx, productID)
}
