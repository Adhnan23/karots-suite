package catalogue

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type handlers struct{ p *Plugin }

// Status is the copy's state for the plugin's page and the pickers (CTL-4: no copy yet = type by hand).
type Status struct {
	State
	HasAddress bool `json:"hasAddress"`
}

func (h handlers) status(c echo.Context) error {
	st, err := h.p.store.State(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, Status{State: st, HasAddress: address() != ""})
}
