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
)

// pickExtra is what a picker sends as extra.catalogue.
type pickExtra struct {
	ItemID int64 `json:"itemId"`
}

// took is what a linked product took from the catalogue (CTL-7), compared on later syncs (CTL-8).
type took struct {
	Name   string            `json:"name"`
	Unit   string            `json:"unit"`
	Fields map[string]string `json:"fields"`
}

func (s *Store) item(ctx context.Context, id int64) (Item, error) {
	var r itemRow
	err := appdb.On(ctx, s.db).GetContext(ctx, &r, `SELECT id, name, unit, kinds, fields FROM catalogue_items WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, apperr.Validation("that catalogue product isn't on this PC")
	}
	return r.item(), err
}

// extras is the pick's product extra: refused when the item isn't in the copy (PLG-3), then the link and the
// shop's empty fields filled (CTL-6, CTL-7).
func (s *Store) extras() plugin.ProductExtras {
	parse := func(raw json.RawMessage) (pickExtra, error) {
		var e pickExtra
		if err := json.Unmarshal(raw, &e); err != nil || e.ItemID <= 0 {
			return e, apperr.BadRequest("a catalogue pick needs its item")
		}
		return e, nil
	}
	return plugin.ProductExtras{Key: "catalogue",
		Validate: func(ctx context.Context, _ plugin.ExtraCtx, raw json.RawMessage) error {
			e, err := parse(raw)
			if err != nil {
				return err
			}
			_, err = s.item(ctx, e.ItemID)
			return err
		},
		Save: func(ctx context.Context, x plugin.ExtraCtx, raw json.RawMessage) error {
			e, err := parse(raw)
			if err != nil {
				return err
			}
			return s.link(ctx, x.ProductID, e.ItemID, x.Whole)
		},
	}
}

// link records the pick and fills the shop's fields the person left empty; typed values win (CTL-6). A pick from
// a whole product form is already reviewed; a quick one waits on Items to review (CTL-7).
func (s *Store) link(ctx context.Context, productID, itemID int64, whole bool) error {
	it, err := s.item(ctx, itemID)
	if err != nil {
		return err
	}
	t, _ := json.Marshal(took{Name: it.Name, Unit: it.Unit, Fields: it.Fields})
	if _, err := s.db.ExecContext(ctx, `INSERT INTO catalogue_links (product_id, item_id, took, reviewed) VALUES ($1, $2, $3, $4)
		ON CONFLICT (product_id) DO UPDATE SET item_id = EXCLUDED.item_id, took = EXCLUDED.took, reviewed = EXCLUDED.reviewed`,
		productID, itemID, t, whole); err != nil {
		return err
	}
	f, err := s.filler(ctx)
	if err != nil {
		return err
	}
	for set, sec := range f.sections(it.Fields) {
		vals := sec["values"].(map[string]string)
		for _, a := range plugin.ProductAttributeSets() {
			if a.Key != set || a.Set == nil {
				continue
			}
			have := map[int64]map[string]string{}
			if a.Values != nil {
				if have, err = a.Values(ctx, []int64{productID}); err != nil {
					return err
				}
			}
			for key, v := range vals {
				if strings.TrimSpace(have[productID][key]) != "" {
					continue
				}
				if err := a.Set(ctx, productID, key, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// reviewSource puts picked products and later catalogue changes on Items to review (REV-1, CTL-7, CTL-8).
func (s *Store) reviewSource() plugin.ReviewSource {
	return plugin.ReviewSource{Key: "catalogue",
		Notes: func(ctx context.Context) (map[int64][]string, error) {
			var rows []struct {
				ID   int64  `db:"product_id"`
				Note string `db:"note"`
			}
			if err := s.db.SelectContext(ctx, &rows, `
				SELECT product_id, 'From the catalogue' AS note FROM catalogue_links WHERE NOT reviewed
				UNION ALL
				SELECT product_id, 'The catalogue changed: ' || (SELECT string_agg(c->>'what' || ' ' || coalesce(nullif(c->>'then', ''), '—') || ' → ' || coalesce(nullif(c->>'now', ''), '—'), '; ')
					FROM jsonb_array_elements(changes) c) FROM catalogue_reviews`); err != nil {
				return nil, err
			}
			out := map[int64][]string{}
			for _, r := range rows {
				out[r.ID] = append(out[r.ID], r.Note)
			}
			return out, nil
		},
		// Mark reviewed: the pick is reviewed and a waiting change is kept as the shop has it ("Keep mine", CTL-9).
		Clear: func(ctx context.Context, productID int64) error { return s.keepMine(ctx, productID) },
	}
}
