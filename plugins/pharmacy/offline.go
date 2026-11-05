package pharmacy

import (
	"context"
	"strconv"
)

// OfflinePart is what the till's Patient panel reads with the server out of reach:
// who the patients are, their prescriptions, and each medicine's flags.
type OfflinePart struct {
	Patients []PatientDTO             `json:"patients"`
	Scripts  map[int64][]ScriptDTO    `json:"scripts"` // by patient, those who have any
	Flags    map[string]MedicineFlags `json:"flags"`   // by product
	Doses    any                      `json:"doses"`
	Notes    any                      `json:"notes"`
}

// ponytail: one read per patient; only their active scripts' rows if a big pharmacy finds it slow.
func (h handlers) offlineData(ctx context.Context) (any, error) {
	out := OfflinePart{Patients: []PatientDTO{}, Scripts: map[int64][]ScriptDTO{}, Flags: map[string]MedicineFlags{}}
	var patients []Patient
	if err := h.p.store.db.SelectContext(ctx, &patients, patientSelect+` ORDER BY p.id`); err != nil {
		return nil, fail("load patients", err)
	}
	for _, p := range patients {
		out.Patients = append(out.Patients, patientDTO(p))
		scripts, err := h.scriptDTOs(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if len(scripts) > 0 {
			out.Scripts[p.ID] = scripts
		}
	}
	var ids []int64
	if err := h.p.store.db.SelectContext(ctx, &ids, `SELECT product_id FROM pharmacy_medicines`); err != nil {
		return nil, fail("load medicines", err)
	}
	flags, err := h.p.store.Flags(ctx, ids)
	if err != nil {
		return nil, fail("load medicine flags", err)
	}
	for id, f := range flags {
		out.Flags[strconv.FormatInt(id, 10)] = f
	}
	if out.Doses, err = h.p.store.Doses(ctx); err != nil {
		return nil, fail("list doses", err)
	}
	if out.Notes, err = h.p.store.Notes(ctx); err != nil {
		return nil, fail("list instructions", err)
	}
	return out, nil
}
