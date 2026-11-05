package pharmacy

import (
	"context"
	"errors"
	"strings"

	"karots-suite/internal/plugin"

	"github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// attributes offers a medicine's marks and usual dose through the generic hook.
// Saving goes through SetFlags, so a controlled drug stays prescription-only and a
// used dose joins the list.
func (s *Store) attributes() plugin.ProductAttributeSet {
	return plugin.ProductAttributeSet{Key: "pharmacy",
		Attributes: func(context.Context) ([]plugin.ProductAttribute, error) {
			return []plugin.ProductAttribute{
				{Key: "rx_only", Label: "Needs a prescription", Type: "bool"},
				{Key: "controlled", Label: "Controlled drug", Type: "bool"},
				{Key: "dose", Label: "Usual dose", Type: "text"},
			}, nil
		},
		Values: s.attrValues, Known: s.attrKnown, Set: s.attrSet, AddKnown: s.attrAddKnown, RemoveKnown: s.attrRemoveKnown}
}

func (s *Store) attrValues(ctx context.Context, ids []int64) (map[int64]map[string]string, error) {
	var rows []struct {
		ID         int64  `db:"product_id"`
		Controlled bool   `db:"controlled"`
		RxOnly     bool   `db:"rx_only"`
		Dose       string `db:"dose"`
	}
	if err := s.db.SelectContext(ctx, &rows, `SELECT p.id AS product_id, c.product_id IS NOT NULL AS controlled,
		COALESCE(m.rx_only, false) AS rx_only, COALESCE(m.dose, '') AS dose
		FROM unnest($1::bigint[]) AS p(id)
		LEFT JOIN pharmacy_controlled_products c ON c.product_id = p.id
		LEFT JOIN pharmacy_medicines m ON m.product_id = p.id`, pq.Array(ids)); err != nil {
		return nil, err
	}
	out := make(map[int64]map[string]string, len(rows))
	for _, r := range rows {
		v := map[string]string{}
		if r.Controlled {
			v["controlled"] = "yes"
		}
		if r.RxOnly || r.Controlled {
			v["rx_only"] = "yes"
		}
		if r.Dose != "" {
			v["dose"] = r.Dose
		}
		out[r.ID] = v
	}
	return out, nil
}

func (s *Store) attrKnown(ctx context.Context, attr string, limit int) ([]plugin.KnownValue, error) {
	if attr != "dose" {
		return nil, nil
	}
	doses, err := s.Doses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]plugin.KnownValue, 0, min(limit, len(doses)))
	for _, d := range doses {
		if len(out) == limit {
			break
		}
		out = append(out, plugin.KnownValue{Value: d.Dose, Meaning: d.Words, Extra: map[string]string{"perDay": d.PerDay.String()}})
	}
	return out, nil
}

// attrSet changes one mark and keeps the others, as the product form does.
func (s *Store) attrSet(ctx context.Context, productID int64, attr, value string) error {
	flags, err := s.Flags(ctx, []int64{productID})
	if err != nil {
		return err
	}
	f := flags[productID]
	on := strings.EqualFold(value, "yes")
	switch attr {
	case "rx_only":
		f.RxOnly = on
	case "controlled":
		f.Controlled = on
	case "dose":
		f.Dose = value
	default:
		return errors.New("no such mark")
	}
	return s.SetFlags(ctx, productID, f)
}

func (s *Store) attrAddKnown(ctx context.Context, attr string, v plugin.KnownValue) error {
	if attr != "dose" {
		return nil
	}
	perDay, err := decimal.NewFromString(strings.TrimSpace(v.Extra["perDay"]))
	if err != nil {
		perDay = decimal.Zero
	}
	_, err = s.CreateDose(ctx, DoseInput{Dose: v.Value, Words: v.Meaning, PerDay: perDay})
	return err
}

func (s *Store) attrRemoveKnown(ctx context.Context, attr, value string) error {
	if attr != "dose" {
		return nil
	}
	doses, err := s.Doses(ctx)
	if err != nil {
		return err
	}
	for _, d := range doses {
		if strings.EqualFold(strings.TrimSpace(d.Dose), strings.TrimSpace(value)) {
			if d.Uses > 0 {
				return nil
			}
			return s.DeleteDose(ctx, d.ID)
		}
	}
	return nil
}
