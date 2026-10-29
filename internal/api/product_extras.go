package api

import (
	"context"

	"karots-suite/internal/apperr"
	"karots-suite/internal/plugin"
)

// checkExtras runs each sent plugin extra's Validate before the product is
// written; a key no plugin owns is refused. Nothing sent = nothing to do.
func checkExtras(ctx context.Context, req ProductRequest, x plugin.ExtraCtx) error {
	if len(req.Extra) == 0 {
		return nil
	}
	hooks := map[string]plugin.ProductExtras{}
	for _, h := range plugin.ProductExtrasHooks() {
		hooks[h.Key] = h
	}
	for key, raw := range req.Extra {
		h, ok := hooks[key]
		if !ok {
			return apperr.BadRequest("unknown product extra '" + key + "'")
		}
		if h.Validate != nil {
			if err := h.Validate(ctx, x, raw); err != nil {
				return err
			}
		}
	}
	return nil
}

// saveExtras hands each sent extra to its plugin after the product is written
// (checkExtras already refused unknown keys). The product stays saved if one
// fails; the error names the plugin's data.
func saveExtras(ctx context.Context, req ProductRequest, x plugin.ExtraCtx) error {
	for _, h := range plugin.ProductExtrasHooks() {
		raw, ok := req.Extra[h.Key]
		if !ok || h.Save == nil {
			continue
		}
		if err := h.Save(ctx, x, raw); err != nil {
			return apperr.Internal("the product was saved, but not its "+h.Key+" details", err)
		}
	}
	return nil
}
