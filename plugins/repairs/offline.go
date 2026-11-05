package repairs

import (
	"context"
	"strconv"
)

// OfflinePart is what the till's Repairs panel reads with the server out of reach:
// the jobs not yet collected, their bills, and what the form suggests.
type OfflinePart struct {
	Config  any                 `json:"config"`
	Suggest any                 `json:"suggest"`
	Labour  int64               `json:"labour"`
	Jobs    []*JobDTO           `json:"jobs"`
	Bills   map[string]*BillDTO `json:"bills"` // by job, those with something to bill
}

func (h handlers) offlineData(ctx context.Context) (any, error) {
	out := OfflinePart{Jobs: []*JobDTO{}, Bills: map[string]*BillDTO{}}
	cfg, err := h.p.store.Config(ctx)
	if err != nil {
		return nil, err
	}
	out.Config = map[string]any{"warrantyMode": cfg.WarrantyMode, "defaultWarrantyDays": cfg.DefaultWarrantyDays}
	if out.Suggest, err = h.p.store.Suggestions(ctx); err != nil {
		return nil, err
	}
	if out.Labour, err = h.p.store.LabourID(ctx); err != nil {
		return nil, err
	}
	rows, err := h.p.store.List(ctx, ListFilter{Show: "open", Limit: 1000})
	if err != nil {
		return nil, err
	}
	for _, j := range rows {
		dto, err := h.jobDTO(ctx, j.ID)
		if err != nil {
			return nil, err
		}
		out.Jobs = append(out.Jobs, dto)
		d, err := h.p.store.GetJob(ctx, j.ID)
		if err != nil {
			return nil, err
		}
		if len(d.Parts) == 0 && len(d.Charges) == 0 {
			continue
		}
		bill, err := h.billOf(ctx, d)
		if err != nil {
			return nil, err
		}
		out.Bills[strconv.FormatInt(j.ID, 10)] = bill
	}
	return out, nil
}
