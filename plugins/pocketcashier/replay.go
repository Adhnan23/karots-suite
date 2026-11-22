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
	// Handed: the cashier has gone ahead (the sale is done, the cash handed over). False
	// only while a screen waits for this very answer: a refusal then moved no money.
	Handed bool `json:"handed"`
}

type ActionsRequest struct {
	SignedIn   *int64     `json:"signedIn"`   // who is signed in on the phone now (nil = nobody)
	SignedInAt *time.Time `json:"signedInAt"` // when they signed in on it
	Actions    []Action   `json:"actions"`
}

type ActionResult struct {
	Ref       string `json:"ref"`
	Status    string `json:"status"` // applied | skipped | failed
	SaleID    int64  `json:"saleId,omitempty"`
	ReceiptNo string `json:"receiptNo,omitempty"`
	HeldID    int64  `json:"heldId,omitempty"`
	ResultID  int64  `json:"resultId,omitempty"` // what a call made
	Flagged   bool   `json:"-"`                  // applied, but someone should look
	Settled   bool   `json:"-"`                  // its cash, if any, has been dealt with
	bookSale  bool   // a sale the PC refused: apply claims it, then books its handed cash
	// Reply is the route's own answer, for the screen waiting on it and for later
	// actions that read more than one id from it.
	Reply json.RawMessage `json:"reply,omitempty"`
	Note  string          `json:"note,omitempty"`
}

type ActionsReply struct {
	Results []ActionResult `json:"results"`
}

const maxBatch = 50

type phoneCtxKey struct{}

// phoneLocks serialise each phone's sends, so two at once (a retry while the
// first still applies) see each other's refs.
// The lock tells a send still working from one that stopped (a claim found under it is
// a stopped one); the claim row and sales.client_ref are the backstops in the database.
// ponytail: in-process locks — one server per shop.
var phoneLocks sync.Map

func (h handlers) actions(c echo.Context) error {
	var req ActionsRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if len(req.Actions) > maxBatch {
		return apperr.Validation("send at most 50 actions at a time")
	}
	// The phone may go away mid-batch. The work must still finish and be recorded, or
	// its retry would do it again (and a stop in between is caught by the claim, PKT-4).
	ctx := context.WithoutCancel(c.Request().Context())
	p := phoneFrom(c)
	ctx = context.WithValue(ctx, phoneCtxKey{}, p)
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
			if r.Status == "failed" && a.Handed {
				r = h.settleLate(ctx, a, r)
			}
			out.Results = append(out.Results, r)
			continue
		}
		if send == nil { // only a batch with something new gets a send row
			s, err := h.newSend(ctx, p.ID)
			if err != nil {
				return err
			}
			send = s
		}
		r, err := h.apply(ctx, p, send, a)
		if err != nil {
			return err
		}
		if r.Status == "retry" { // the PC's own trouble: this and what follows wait their turn
			break
		}
		out.Results = append(out.Results, r)
	}
	if req.SignedIn != nil {
		h.signedIn(ctx, p, *req.SignedIn, req.SignedInAt)
	}
	return c.JSON(http.StatusOK, out)
}

// sendRow is the batch's send; its counts go up with each action's row.
type sendRow struct{ id int64 }

func (h handlers) newSend(ctx context.Context, phoneID int64) (*sendRow, error) {
	s := &sendRow{}
	if err := h.p.core.DB.GetContext(ctx, &s.id, `INSERT INTO pocket_sends (phone_id) VALUES ($1) RETURNING id`, phoneID); err != nil {
		return nil, apperr.Internal("failed to record the send", err)
	}
	return s, nil
}

// stored is the result a ref already got.
func (h handlers) stored(ctx context.Context, ref string) (ActionResult, bool, error) {
	var row struct {
		Status    string  `db:"status"`
		SaleID    *int64  `db:"sale_id"`
		HeldID    *int64  `db:"held_id"`
		ResultID  *int64  `db:"result_id"`
		Reply     []byte  `db:"result_reply"`
		Note      string  `db:"note"`
		ReceiptNo *string `db:"receipt_no"`
	}
	err := h.p.core.DB.GetContext(ctx, &row, `SELECT a.status, a.sale_id, a.held_id, a.result_id, a.result_reply, a.note, s.receipt_no
		FROM pocket_actions a LEFT JOIN sales s ON s.id = a.sale_id WHERE a.ref = $1`, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return ActionResult{}, false, nil
	}
	if err != nil {
		return ActionResult{}, false, apperr.Internal("failed to read the action", err)
	}
	if row.Status == "running" {
		return h.stopped(ctx, ref)
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
	r.Reply = row.Reply
	return r, true, nil
}

// stoppedNote: what a claimed action says when the PC stopped part way through it.
const stoppedNote = "the PC stopped part way through this; it may not have gone through — check before doing it again"

// stopped: an action claimed but never recorded. It may have been done, so it is
// never run again (PKT-4) and no cash is booked for it; it waits on the review screen.
func (h handlers) stopped(ctx context.Context, ref string) (ActionResult, bool, error) {
	_, err := h.p.core.DB.ExecContext(ctx, `WITH a AS (
		UPDATE pocket_actions SET status = 'failed', note = $2, cash_settled = true WHERE ref = $1 AND status = 'running' RETURNING send_id)
		UPDATE pocket_sends SET actions = actions + 1, failed = failed + 1 WHERE id IN (SELECT send_id FROM a)`, ref, stoppedNote)
	if err != nil {
		return ActionResult{}, false, apperr.Internal("failed to record the action", err)
	}
	return ActionResult{Ref: ref, Status: "failed", Note: stoppedNote}, true, nil
}

// apply runs one new action and records its result under its ref. Anything but a
// sale is claimed first: a repeat finding the claim never runs it twice (PKT-4).
func (h handlers) apply(ctx context.Context, p *Phone, s *sendRow, a Action) (ActionResult, error) {
	body := a.Body
	if !json.Valid(body) {
		body = json.RawMessage(`null`)
	}
	var uid *int64
	if a.UserID != 0 {
		uid = &a.UserID
	}
	claimIt := func() (bool, error) {
		res, err := h.p.core.DB.ExecContext(ctx, `INSERT INTO pocket_actions (ref, phone_id, send_id, kind, user_id, at, body, status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'running') ON CONFLICT (ref) DO NOTHING`, a.Ref, p.ID, s.id, a.Kind, uid, a.At, body)
		if err != nil {
			return false, apperr.Internal("failed to record what the phone did", err)
		}
		n, _ := res.RowsAffected() // 0: another send has it: this one waits, and gets its answer next time
		return n > 0, nil
	}
	claim := a.Kind != "sale" // a sale's own ref (sales.client_ref) already stops it selling twice
	if claim {
		if ok, err := claimIt(); err != nil || !ok {
			return ActionResult{Ref: a.Ref, Status: "retry"}, err
		}
	}
	r := h.run(ctx, p, a)
	if r.bookSale && a.Handed {
		// Its cash is booked on its own: claimed first, so a stop between the booking and
		// the record never books it again (PKT-4, PKT-7).
		if ok, err := claimIt(); err != nil || !ok {
			return ActionResult{Ref: a.Ref, Status: "retry"}, err
		}
		claim = true
	}
	if r.bookSale {
		r = h.refusedSale(ctx, a, r.Note)
	}
	r.Ref = a.Ref
	if r.Status == "retry" {
		if claim { // nothing was done: the next try runs it afresh
			if _, err := h.p.core.DB.ExecContext(ctx, `DELETE FROM pocket_actions WHERE ref = $1 AND status = 'running'`, a.Ref); err != nil {
				return r, apperr.Internal("failed to record what the phone did", err)
			}
		}
		return r, nil
	}
	// The row and its send's counts in one statement: a cut between them would leave
	// What came in saying less than came (PKT-12).
	_, err := h.p.core.DB.ExecContext(ctx, `WITH a AS (
		INSERT INTO pocket_actions (ref, phone_id, send_id, kind, user_id, at, body, status, sale_id, held_id, result_id, note, result_reply, cash_settled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9, 0),NULLIF($10, 0),NULLIF($11, 0),$12,$13,$14)
		ON CONFLICT (ref) DO UPDATE SET status = EXCLUDED.status, sale_id = EXCLUDED.sale_id, held_id = EXCLUDED.held_id,
			result_id = EXCLUDED.result_id, note = EXCLUDED.note, result_reply = EXCLUDED.result_reply, cash_settled = EXCLUDED.cash_settled
			WHERE pocket_actions.status = 'running'
		RETURNING sale_id)
		UPDATE pocket_sends SET actions = actions + 1, failed = failed + ($8 = 'failed')::int,
			flagged = flagged + ($15 OR EXISTS (SELECT 1 FROM sale_flags f JOIN a ON f.sale_id = a.sale_id))::int
		WHERE id = $3 AND EXISTS (SELECT 1 FROM a)`,
		a.Ref, p.ID, s.id, a.Kind, uid, a.At, body, r.Status, r.SaleID, r.HeldID, r.ResultID, r.Note, replyJSON(r.Reply), r.Settled, r.Flagged)
	if err != nil {
		return r, apperr.Internal("failed to record what the phone did", err)
	}
	return r, nil
}

// trouble: the PC's own (its database away, a 5xx), not a refusal.
func trouble(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := apperr.As(err)
	return !ok || ae.Status >= 500
}

// fail: the PC's own trouble sends the action back to wait its turn, a few times
// over, before anything is refused or booked for it (PKT-5); anything else is a refusal.
func fail(a Action, err error) ActionResult {
	if trouble(err) && keepTrying(a.Ref) {
		return ActionResult{Status: "retry", Note: err.Error()}
	}
	return failed(err)
}

func failed(err error) ActionResult {
	msg := err.Error()
	if ae, ok := apperr.As(err); ok {
		msg = ae.Message
	}
	return ActionResult{Status: "failed", Note: msg}
}

// signedIn: the person on the phone now holds it as their one device.
func replyJSON(b []byte) any {
	if json.Valid(b) && len(b) > 0 {
		return string(b)
	}
	return nil
}

// signedIn keeps the phone holding the person it shows, unless they have since
// signed in somewhere else: whichever sign-in came last holds them. A send never
// takes a cashier back from the PC they moved to.
func (h handlers) signedIn(ctx context.Context, p *Phone, uid int64, at *time.Time) {
	var who struct {
		Role     string     `db:"role"`
		System   bool       `db:"is_system"`
		Active   *string    `db:"active_device"`
		ActiveAt *time.Time `db:"active_device_at"`
	}
	if err := h.p.core.DB.GetContext(ctx, &who, `SELECT role::text AS role, is_system, active_device, active_device_at FROM users WHERE id = $1`, uid); err != nil ||
		who.System || who.Role == "admin" {
		return
	}
	free := who.Active == nil || *who.Active == "" || *who.Active == p.Device
	later := at != nil && (who.ActiveAt == nil || at.After(*who.ActiveAt))
	if !free && !later {
		return
	}
	if err := h.p.core.Auth.ClaimDevice(ctx, uid, p.Device, p.Name); err == nil {
		_, _ = h.p.core.DB.ExecContext(ctx, `UPDATE pocket_phones SET user_id = $2 WHERE id = $1`, p.ID, uid)
	}
}
