package documents

import (
	"context"

	"karots-suite/internal/features/recipes"
)

// OfflinePart is what the till's Print & Copy menu reads with the server out of
// reach: the menu, and what's needed to price a job and list what it uses. No costs.
type OfflinePart struct {
	Services []TillService             `json:"services"`
	Prices   map[int64][]OfflinePrice  `json:"prices"`  // by service
	Paper    map[int64][]OfflinePaper  `json:"paper"`   // by service
	Recipes  map[int64][]OfflineRecipe `json:"recipes"` // by the service's product
}

type OfflinePrice struct {
	Size      *string `json:"size"`
	Material  string  `json:"material"`
	Color     bool    `json:"color"`
	Double    bool    `json:"double"`
	MinQty    int     `json:"minQty"`
	UnitPrice string  `json:"unitPrice"`
}

type OfflinePaper struct {
	Size       *string `json:"size"`
	Material   string  `json:"material"`
	ProductID  int64   `json:"productId"`
	QtyPerUnit string  `json:"qtyPerUnit"`
}

type OfflineRecipe struct {
	ProductID  int64   `json:"productId"`
	QtyPerUnit *string `json:"qtyPerUnit"`
	YieldUnits *string `json:"yieldUnits"`
	WholeUnits bool    `json:"wholeUnits"`
}

func (h handlers) offlineData(ctx context.Context) (any, error) {
	services, err := h.tillServices(ctx)
	if err != nil {
		return nil, err
	}
	out := OfflinePart{Services: services, Prices: map[int64][]OfflinePrice{}, Paper: map[int64][]OfflinePaper{}, Recipes: map[int64][]OfflineRecipe{}}
	for _, s := range services {
		if s.Kind != "metered" {
			continue
		}
		prices, err := h.p.store.Prices(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range prices {
			out.Prices[s.ID] = append(out.Prices[s.ID], OfflinePrice{Size: p.Size, Material: p.Material, Color: p.Color, Double: p.DoubleSide, MinQty: p.MinQty, UnitPrice: p.UnitPrice.StringFixed(2)})
		}
		paper, err := h.p.store.Paper(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		for _, p := range paper {
			out.Paper[s.ID] = append(out.Paper[s.ID], OfflinePaper{Size: p.Size, Material: p.Material, ProductID: p.ProductID, QtyPerUnit: p.QtyPerUnit.String()})
		}
		recipe, err := h.p.store.Recipe(ctx, s.ProductID)
		if err != nil {
			return nil, err
		}
		for _, r := range recipe {
			out.Recipes[s.ProductID] = append(out.Recipes[s.ProductID], offlineRecipe(r))
		}
	}
	return out, nil
}

func offlineRecipe(r recipes.Component) OfflineRecipe {
	o := OfflineRecipe{ProductID: r.ComponentProductID, WholeUnits: r.WholeUnits}
	if r.QtyPerUnit.Valid {
		v := r.QtyPerUnit.Decimal.String()
		o.QtyPerUnit = &v
	}
	if r.YieldUnits.Valid {
		v := r.YieldUnits.Decimal.String()
		o.YieldUnits = &v
	}
	return o
}
