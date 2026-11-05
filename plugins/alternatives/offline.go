package alternatives

import (
	"context"
	"strconv"
)

// offlineData is the swap chip's answer for every product in a group, with the
// server out of reach.
// ponytail: one query per member; a single grouped query if big catalogues make it slow.
func (h handlers) offlineData(ctx context.Context) (any, error) {
	var ids []int64
	if err := h.p.store.db.SelectContext(ctx, &ids, `SELECT DISTINCT product_id FROM alt_members ORDER BY product_id`); err != nil {
		return nil, err
	}
	out := map[string][]AltDTO{}
	for _, id := range ids {
		alts, err := h.altsOf(ctx, id)
		if err != nil {
			return nil, err
		}
		if len(alts) > 0 {
			out[strconv.FormatInt(id, 10)] = alts
		}
	}
	return map[string]any{"byProduct": out}, nil
}
