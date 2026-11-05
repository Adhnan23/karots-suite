package combos

import "context"

// offlineData is what the till's Combos menu and cart hints read with the server
// out of reach.
func (h handlers) offlineData(ctx context.Context) (any, error) {
	cs, err := h.p.store.Till(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := h.p.store.Settings(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"till": dtos(cs), "settings": settings}, nil
}
