package pocketcashier

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"karots-suite/internal/apperr"

	"github.com/labstack/echo/v4"
)

// Everything a phone does — online or offline — is an action with a one-time
// ref, sent oldest first to one endpoint and applied through the core services.
// A ref already seen gets its stored result back unchanged, so a retried or
// doubled send can never book twice. Nothing is refused after the fact: what
// can't be applied is stored `failed` with its reason, acknowledged (never
// resent) and shown on the review screen.

type Action struct {
	Ref    string          `json:"ref"`
	Kind   string          `json:"kind"` // open | sale | hold | unhold | withdraw | deposit | close | call
	UserID int64           `json:"userId"`
	At     time.Time       `json:"at"`
	Body   json.RawMessage `json:"body"`
}

type ActionsRequest struct {
	SignedIn *int64   `json:"signedIn"` // who is signed in on the phone now (nil = nobody)
	Actions  []Action `json:"actions"`
}

type ActionResult struct {
	Ref       string `json:"ref"`
	Status    string `json:"status"` // applied | skipped | failed
	SaleID    int64  `json:"saleId,omitempty"`
	ReceiptNo string `json:"receiptNo,omitempty"`
	HeldID    int64  `json:"heldId,omitempty"`
	ResultID  int64  `json:"resultId,omitempty"` // what a call made
	Flagged   bool   `json:"-"`                  // applied, but someone should look
	Note      string `json:"note,omitempty"`
}

type ActionsReply struct {
	Results []ActionResult `json:"results"`
}

const maxBatch = 50

// phoneLocks serialise each phone's sends, so two at once (a retry while the
// first still applies) see each other's refs.
// ponytail: in-process locks — one server per shop; sales.client_ref is unique in the DB as the backstop.
var phoneLocks sync.Map

func (h handlers) actions(c echo.Context) error {
	var req ActionsRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if len(req.Actions) > maxBatch {
		return apperr.Validation("send at most 50 actions at a time")
	}
	ctx := c.Request().Context()
	p := phoneFrom(c)
	mu, _ := phoneLocks.LoadOrStore(p.ID, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	out := ActionsReply{Results: make([]ActionResult, 0, len(req.Actions))}
	var send *sendRow
	for _, a := range req.Actions {
		if a.Ref == "" || len(a.Ref) > 100 {
			out.Results = append(out.Results, ActionResult{Ref: a.Ref, Status: "failed", Note: "no ref"})
			continue
		}
		if r, ok, err := h.stored(ctx, a.Ref); err != nil {
			return err
		} else if ok {
			out.Results = append(out.Results, r)
			continue
		}
		if send == nil { // only a batch with something new gets a send row
			s, err := h.newSend(ctx, p.ID)
			if err != nil {
				return err
			}
			send = s
			defer h.closeSend(send)
		}
		out.Results = append(out.Results, h.apply(ctx, p, send, a))
	}
	if req.SignedIn != nil {
		h.signedIn(ctx, p, *req.SignedIn)
	}
	return c.JSON(http.StatusOK, out)
}

// sendRow counts one batch's new actions; written when the batch ends.
type sendRow struct{ id, actions, failed, flagged int64 }

func (h handlers) newSend(ctx context.Context, phoneID int64) (*sendRow, error) {
	s := &sendRow{}
	if err := h.p.core.DB.GetContext(ctx, &s.id, `INSERT INTO pocket_sends (phone_id) VALUES ($1) RETURNING id`, phoneID); err != nil {
		return nil, apperr.Internal("failed to record the send", err)
	}
	return s, nil
}

func (h handlers) closeSend(s *sendRow) {
	_, _ = h.p.core.DB.Exec(`UPDATE pocket_sends SET actions = $2, failed = $3, flagged = $4 WHERE id = $1`, s.id, s.actions, s.failed, s.flagged)
}

// stored is the result a ref already got.
func (h handlers) stored(ctx context.Context, ref string) (ActionResult, bool, error) {
	var row struct {
		Status    string  `db:"status"`
		SaleID    *int64  `db:"sale_id"`
		HeldID    *int64  `db:"held_id"`
		ResultID  *int64  `db:"result_id"`
		Note      string  `db:"note"`
		ReceiptNo *string `db:"receipt_no"`
	}
	err := h.p.core.DB.GetContext(ctx, &row, `SELECT a.status, a.sale_id, a.held_id, a.result_id, a.note, s.receipt_no
		FROM pocket_actions a LEFT JOIN sales s ON s.id = a.sale_id WHERE a.ref = $1`, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return ActionResult{}, false, nil
	}
	if err != nil {
		return ActionResult{}, false, apperr.Internal("failed to read the action", err)
	}
	r := ActionResult{Ref: ref, Status: row.Status, Note: row.Note}
	if row.SaleID != nil {
		r.SaleID = *row.SaleID
	}
	if row.HeldID != nil {
		r.HeldID = *row.HeldID
	}
	if row.ReceiptNo != nil {
		r.ReceiptNo = *row.ReceiptNo
	}
	if row.ResultID != nil {
		r.ResultID = *row.ResultID
	}
	return r, true, nil
}

// apply runs one new action and records its result under its ref.
func (h handlers) apply(ctx context.Context, p *Phone, s *sendRow, a Action) ActionResult {
	r := h.run(ctx, p, a)
	r.Ref = a.Ref
	s.actions++
	if r.Status == "failed" {
		s.failed++
	}
	if r.Flagged {
		s.flagged++
	}
	if r.SaleID != 0 {
		var n int
		_ = h.p.core.DB.GetContext(ctx, &n, `SELECT count(*) FROM sale_flags WHERE sale_id = $1`, r.SaleID)
		if n > 0 {
			s.flagged++
		}
	}
	body := a.Body
	if !json.Valid(body) {
		body = json.RawMessage(`null`)
	}
	var uid *int64
	if a.UserID != 0 {
		uid = &a.UserID
	}
	_, _ = h.p.core.DB.ExecContext(ctx, `INSERT INTO pocket_actions (ref, phone_id, send_id, kind, user_id, at, body, status, sale_id, held_id, result_id, note)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9, 0),NULLIF($10, 0),NULLIF($11, 0),$12) ON CONFLICT (ref) DO NOTHING`,
		a.Ref, p.ID, s.id, a.Kind, uid, a.At, body, r.Status, r.SaleID, r.HeldID, r.ResultID, r.Note)
	return r
}

func failed(err error) ActionResult {
	msg := err.Error()
	if ae, ok := apperr.As(err); ok {
		msg = ae.Message
	}
	return ActionResult{Status: "failed", Note: msg}
}

// signedIn: the person on the phone now holds it as their one device.
func (h handlers) signedIn(ctx context.Context, p *Phone, uid int64) {
	var role string
	var system bool
	if err := h.p.core.DB.QueryRowxContext(ctx, `SELECT role::text, is_system FROM users WHERE id = $1`, uid).Scan(&role, &system); err != nil ||
		system || role == "admin" {
		return
	}
	if err := h.p.core.Auth.ClaimDevice(ctx, uid, p.Device, p.Name); err == nil {
		_, _ = h.p.core.DB.ExecContext(ctx, `UPDATE pocket_phones SET user_id = $2 WHERE id = $1`, p.ID, uid)
	}
}
