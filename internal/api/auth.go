package api

import (
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

func (h *handlers) LoginInfo(c echo.Context) error {
	info := LoginInfo{ServerTime: time.Now().UTC().Format(time.RFC3339), FileMode: h.d.Cfg.FileDataDir != ""}
	if cfg, err := h.d.Settings.Get(c.Request().Context()); err == nil && cfg != nil {
		info.InstallID = cfg.InstallID
		info.ShopName = cfg.ShopName
		info.Palette = settings.ValidSkin(cfg.Skin)
		info.Receipt = ShopHeader{Name: cfg.ShopName, Address: strOr(cfg.Address), Phone: strOr(cfg.Phone), Footer: strOr(cfg.ReceiptFooter)}
	}
	info.Notes, info.Coins = []string{}, []string{}
	if rows, err := h.t.denoms.List(c.Request().Context(), true); err == nil {
		for _, d := range rows {
			if d.IsNote {
				info.Notes = append(info.Notes, d.Value.String())
			} else {
				info.Coins = append(info.Coins, d.Value.String())
			}
		}
	}
	return c.JSON(http.StatusOK, info)
}

func (h *handlers) bindLogin(c echo.Context) (auth.LoginInput, error) {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return auth.LoginInput{}, apperr.BadRequest("invalid request body")
	}
	in := auth.LoginInput{Phone: req.Phone, PIN: req.Pin}
	if err := c.Validate(&in); err != nil {
		return in, apperr.Validation("Enter a valid phone number and 4–6 digit PIN")
	}
	return in, nil
}

func (h *handlers) Login(c echo.Context) error {
	in, err := h.bindLogin(c)
	if err != nil {
		return err
	}
	pair, err := h.d.Auth.Login(c.Request().Context(), in)
	if err != nil {
		return err
	}
	h.setCookie(c, pair.AccessToken)
	return c.JSON(http.StatusOK, LoginResponse{User: SessionUser{ID: pair.User.ID, Name: pair.User.Name, Role: pair.User.Role}})
}

// Logout refuses while the caller has an open till (or a plugin guard blocks),
// so the cash trail stays honest. Unreadable token → clean logout, never trapped.
func (h *handlers) Logout(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if ok && claims.UserID != 0 {
		ctx := c.Request().Context()
		for _, guard := range plugin.LogoutGuards() {
			if block, _, _ := guard(ctx, claims.UserID); block {
				return apperr.ConflictCode("LOGOUT_BLOCKED", "Finish your open work before signing out")
			}
		}
		if sess, err := h.d.CashRegister.Current(ctx, claims.UserID); err == nil && sess != nil {
			return apperr.ConflictCode("TILL_OPEN", "Close and count your till before signing out")
		}
	}
	h.presence.SignOut(device(c), time.Now())
	h.clearCookie(c)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) Lock(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if !ok || claims.UserID == 0 {
		return apperr.Unauthorized("")
	}
	token, err := h.d.Auth.ReissueLocked(claims, true)
	if err != nil {
		return err
	}
	h.setCookie(c, token)
	return c.NoContent(http.StatusNoContent)
}

// Unlock: the locked-in user OR any admin may unlock; identity is unchanged.
func (h *handlers) Unlock(c echo.Context) error {
	claims, ok := middleware.ParseClaims(c, h.d.Cfg.JWTSecret)
	if !ok || claims.UserID == 0 {
		return apperr.Unauthorized("")
	}
	in, err := h.bindLogin(c)
	if err != nil {
		return err
	}
	u, err := h.d.Auth.VerifyCredentials(c.Request().Context(), in)
	if err != nil {
		return apperr.Validation("Wrong phone number or PIN")
	}
	if u.ID != claims.UserID && u.Role != auth.RoleAdmin {
		return apperr.Forbidden("Only " + claims.Name + " or an admin can unlock this terminal")
	}
	token, err := h.d.Auth.ReissueLocked(claims, false)
	if err != nil {
		return err
	}
	h.setCookie(c, token)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) ChangePin(c echo.Context) error {
	ctx := c.Request().Context()
	if middleware.CurrentRole(c) == auth.RoleCashier && !middleware.MustChangePin(c) && !h.d.Auth.AllowCashierPinChange(ctx) {
		return apperr.Forbidden("PIN changes are turned off for cashiers")
	}
	var req ChangePinRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := auth.ChangeOwnPINInput{CurrentPIN: req.CurrentPin, NewPIN: req.NewPin, ConfirmPIN: req.ConfirmPin}
	if err := c.Validate(&in); err != nil {
		return apperr.Validation("PINs must be 4–6 digits")
	}
	u, err := h.d.Auth.ChangeOwnPIN(ctx, middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	token, err := h.d.Auth.AccessTokenFor(u)
	if err != nil {
		return err
	}
	h.setCookie(c, token)
	return c.NoContent(http.StatusNoContent)
}
