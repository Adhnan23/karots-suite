package combos

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// SettingsDTO is the shop's combo settings (mirrored in web/src/plugins/combos/types.ts).
type SettingsDTO struct {
	AutoApply bool `json:"autoApply"`
}

// Settings reads the shop's settings; no row (a full restore of an older
// backup empties the table) means the defaults.
func (s *Store) Settings(ctx context.Context) (SettingsDTO, error) {
	var v SettingsDTO
	err := s.db.GetContext(ctx, &v.AutoApply, `SELECT auto_apply FROM combos_settings WHERE id = 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return SettingsDTO{}, nil
	}
	return v, err
}

func (s *Store) SaveSettings(ctx context.Context, v SettingsDTO) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO combos_settings (id, auto_apply) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET auto_apply = EXCLUDED.auto_apply`, v.AutoApply)
	return err
}

func (h handlers) settings(c echo.Context) error {
	v, err := h.p.store.Settings(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, v)
}

func (h handlers) saveSettings(c echo.Context) error {
	var v SettingsDTO
	if err := c.Bind(&v); err != nil {
		return apperr.BadRequest("invalid body")
	}
	if err := h.p.store.SaveSettings(c.Request().Context(), v); err != nil {
		return err
	}
	h.audit(c, map[bool]string{true: "combo_auto_on", false: "combo_auto_off"}[v.AutoApply], 0, "")
	return c.JSON(http.StatusOK, v)
}
