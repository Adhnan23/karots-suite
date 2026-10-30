package pocketcashier

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// A phone pairs with a key of its own (a long random secret it keeps, sent as
// X-Ks-Phone); only its SHA-256 is stored. An admin approves the phone by the
// six-digit code it shows. The key works whoever is signed in on the phone —
// it is what lets the copy refresh on the sign-in screen.
const phoneHeader = "X-Ks-Phone"

// Phone is a paired phone.
type Phone struct {
	ID       int64      `db:"id"`
	Name     string     `db:"name"`
	Device   string     `db:"device"`
	Status   string     `db:"status"`
	UserID   *int64     `db:"user_id"`
	LastSeen *time.Time `db:"last_seen"`
}

type PairRequest struct {
	Name   string `json:"name"`
	Device string `json:"device"`
	Secret string `json:"secret"`
}

type PairReply struct {
	Code string `json:"code"`
}

type PairStatus struct {
	Status string `json:"status"` // pending | active | blocked
	Name   string `json:"name"`
}

func tokenHash(secret string) []byte {
	h := sha256.Sum256([]byte(secret))
	return h[:]
}

func pairCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n)
}

func (h handlers) pair(c echo.Context) error {
	var req PairRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	req.Name, req.Device = strings.TrimSpace(req.Name), strings.TrimSpace(req.Device)
	if req.Name == "" || len(req.Name) > 40 || req.Device == "" || len(req.Device) > 64 || len(req.Secret) < 32 || len(req.Secret) > 200 {
		return apperr.Validation("give the phone a name (up to 40 letters)")
	}
	code := pairCode()
	// A repeat from the same phone while it waits replaces its request; a phone
	// already paired or blocked has to be removed in Phones first.
	res, err := h.p.core.DB.ExecContext(c.Request().Context(), `
		INSERT INTO pocket_phones (name, device, token_hash, pair_code) VALUES ($1, $2, $3, $4)
		ON CONFLICT (device) DO UPDATE SET name = EXCLUDED.name, token_hash = EXCLUDED.token_hash, pair_code = EXCLUDED.pair_code,
			created_at = now() WHERE pocket_phones.status = 'pending'`, req.Name, req.Device, tokenHash(req.Secret), code)
	if err != nil {
		return apperr.Internal("failed to pair the phone", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.ConflictCode("PHONE_PAIRED", "This phone is already paired — remove it in Phones to pair it again")
	}
	return c.JSON(http.StatusOK, PairReply{Code: code})
}

func (s *Store) phoneBySecret(ctx context.Context, secret string) (*Phone, error) {
	if secret == "" {
		return nil, nil
	}
	var p Phone
	err := s.db.GetContext(ctx, &p, `SELECT id, name, device, status, user_id, last_seen FROM pocket_phones WHERE token_hash = $1`, tokenHash(secret))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &p, err
}

func (h handlers) pairStatus(c echo.Context) error {
	p, err := h.p.store.phoneBySecret(c.Request().Context(), c.Request().Header.Get(phoneHeader))
	if err != nil {
		return apperr.Internal("failed to read the phone", err)
	}
	if p == nil {
		return apperr.ConflictCode("PHONE_UNKNOWN", "This phone isn't paired")
	}
	return c.JSON(http.StatusOK, PairStatus{Status: p.Status, Name: p.Name})
}

const ctxPhone = "pocketcashier.phone"

// phoneAuth lets an approved phone in by its key (the header, or ?t= for an
// EventSource, which can't send headers).
func (h handlers) phoneAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		secret := c.Request().Header.Get(phoneHeader)
		if secret == "" {
			secret = c.QueryParam("t")
		}
		ctx := c.Request().Context()
		p, err := h.p.store.phoneBySecret(ctx, secret)
		if err != nil {
			return apperr.Internal("failed to read the phone", err)
		}
		switch {
		case p == nil:
			return &apperr.AppError{Status: http.StatusUnauthorized, Code: "PHONE_UNKNOWN", Message: "This phone isn't paired"}
		case p.Status == "blocked":
			return &apperr.AppError{Status: http.StatusForbidden, Code: "PHONE_BLOCKED", Message: "This phone was blocked by the shop"}
		case p.Status != "active":
			return &apperr.AppError{Status: http.StatusForbidden, Code: "PHONE_PENDING", Message: "This phone waits for an admin to approve it"}
		}
		if p.LastSeen == nil || time.Since(*p.LastSeen) > 10*time.Second {
			_, _ = h.p.core.DB.ExecContext(ctx, `UPDATE pocket_phones SET last_seen = now() WHERE id = $1`, p.ID)
		}
		c.Set(ctxPhone, p)
		return next(c)
	}
}

func phoneFrom(c echo.Context) *Phone {
	p, _ := c.Get(ctxPhone).(*Phone)
	return p
}

type WhoAmI struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (h handlers) whoami(c echo.Context) error {
	p := phoneFrom(c)
	return c.JSON(http.StatusOK, WhoAmI{ID: p.ID, Name: p.Name})
}
