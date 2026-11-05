package pocketcashier

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/middleware"

	"github.com/shopspring/decimal"
)

// CallBody is an action that goes through a real route, as the person who did it
// on the phone. No rule is written twice: the route decides, the same as at the
// counter.
type CallBody struct {
	Method string          `json:"method"`
	Path   string          `json:"path"`
	Body   json.RawMessage `json:"body"`
	// CashOut is cash the cashier already handed over. If the PC refuses the call,
	// that cash still left the drawer, so it is booked anyway.
	CashOut string   `json:"cashOut"`
	CashIn  string   `json:"cashIn"` // the same for cash the cashier took
	Label   string   `json:"label"`  // what this was, in a person's words, for notes
	Saw     *CallSaw `json:"saw"`    // what the phone showed, to flag when the PC knew better
	TempID  int64    `json:"tempId"` // the number the phone calls what this makes, until the PC gives the real one
	// TempFields: other numbers the phone gave what this makes, each read from that field of the reply.
	TempFields map[string]string `json:"tempFields"`
}

// CallSaw is the balance the phone showed when the cashier acted.
type CallSaw struct {
	CustomerID int64  `json:"customerId"`
	Balance    string `json:"balance"`
}

// lateAfter: older than this when it arrives, it says when it really happened.
const lateAfter = 5 * time.Minute

// callPrefixes are the routes a phone may replay: the till's own and the plugins'.
// A person's rights are still checked by the route; this keeps a phone's key away
// from admin pages, sign-in and the phones list, even for a manager.
var callPrefixes = []string{"/api/v1/till/", "/api/v1/x/"}

const callBarred = "/api/v1/x/pocketcashier/"

func callAllowed(method, path string) bool {
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
		return false
	}
	if strings.Contains(path, "..") || strings.HasPrefix(path, callBarred) {
		return false
	}
	for _, p := range callPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// tempRefs finds a temporary reference to something made earlier on the phone:
// quoted in a JSON body it becomes a number, bare in a path it becomes the id.
var (
	tempInBody = regexp.MustCompile(`"temp:([A-Za-z0-9-]+)"`)
	tempInPath = regexp.MustCompile(`temp:([A-Za-z0-9-]+)`)
	// Plugin screens only load a record whose id is a positive number, so what's made
	// offline is called by a number no real row reaches (phones pick them, 9e15 up).
	tempNumInBody = regexp.MustCompile(`([:\[,]\s*)(9\d{15})\b`)
	tempNumInPath = regexp.MustCompile(`/(9\d{15})(/|$)`)
)

// resolveTemp swaps every temporary reference for its real id. An error names
// what it depended on, so nothing lands on the wrong row.
func (h handlers) resolveTemp(ctx context.Context, re *regexp.Regexp, s string, repl func(id int64) string) (string, error) {
	var firstErr error
	out := re.ReplaceAllStringFunc(s, func(m string) string {
		ref := re.FindStringSubmatch(m)[1]
		id, err := h.tempID(ctx, ref)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return repl(id)
	})
	return out, firstErr
}

func (h handlers) tempID(ctx context.Context, ref string) (int64, error) {
	var row struct {
		Result *int64 `db:"result_id"`
		Sale   *int64 `db:"sale_id"`
		Kind   string `db:"kind"`
		Label  string `db:"label"`
	}
	err := h.p.core.DB.GetContext(ctx, &row, `SELECT result_id, sale_id, kind, COALESCE(body->>'label', '') AS label
		FROM pocket_actions WHERE ref = $1`, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("depends on something that never reached the PC")
	}
	if err != nil {
		return 0, err
	}
	if row.Result != nil {
		return *row.Result, nil
	}
	if row.Sale != nil {
		return *row.Sale, nil
	}
	name := row.Label
	if name == "" {
		name = "a " + row.Kind
	}
	return 0, fmt.Errorf("depends on %s, which didn't go through", name)
}

// tempField finds a number that named a second thing a call made. ok is false if
// no call on this phone gave out that number.
func (h handlers) tempField(ctx context.Context, p *Phone, num string) (int64, bool, error) {
	var row struct {
		ID    *string `db:"id"`
		Label string  `db:"label"`
	}
	err := h.p.core.DB.GetContext(ctx, &row, `SELECT result_reply->>(body->'tempFields'->>$2) AS id, COALESCE(body->>'label', '') AS label
		FROM pocket_actions WHERE phone_id = $1 AND jsonb_exists(body->'tempFields', $2) LIMIT 1`, p.ID, num)
	if err != nil {
		return 0, false, nil
	}
	if row.ID == nil {
		return 0, true, fmt.Errorf("depends on %s, which didn't go through", row.Label)
	}
	id, err := strconv.ParseInt(*row.ID, 10, 64)
	return id, true, err
}

// resolveBody swaps temporary references in a JSON body.
func (h handlers) resolveBody(ctx context.Context, body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte("temp:")) {
		return body, nil
	}
	s, err := h.resolveTemp(ctx, tempInBody, string(body), func(id int64) string { return strconv.FormatInt(id, 10) })
	return []byte(s), err
}

// resolveBodyFor swaps both kinds of temporary reference in a body.
func (h handlers) resolveBodyFor(ctx context.Context, p *Phone, body []byte) ([]byte, error) {
	out, err := h.resolveBody(ctx, body)
	if err != nil {
		return nil, err
	}
	s, err := h.resolveNumbers(ctx, p, tempNumInBody, string(out))
	return []byte(s), err
}

// resolveNumbers swaps each number this phone gave something it made. A number
// nothing was given is just a number, and stays.
func (h handlers) resolveNumbers(ctx context.Context, p *Phone, re *regexp.Regexp, s string) (string, error) {
	var firstErr error
	out := re.ReplaceAllStringFunc(s, func(m string) string {
		g := re.FindStringSubmatch(m)
		num := g[1]
		if re == tempNumInBody {
			num = g[2]
		}
		var ref string
		err := h.p.core.DB.GetContext(ctx, &ref, `SELECT ref FROM pocket_actions WHERE phone_id = $1 AND body->>'tempId' = $2 LIMIT 1`, p.ID, num)
		var id int64
		switch {
		case err == nil:
			id, err = h.tempID(ctx, ref)
		default:
			var ok bool
			if id, ok, err = h.tempField(ctx, p, num); !ok {
				return m // not one of ours
			}
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			return m
		}
		return strings.Replace(m, num, strconv.FormatInt(id, 10), 1)
	})
	return out, firstErr
}

func (h handlers) runCall(ctx context.Context, p *Phone, a Action) ActionResult {
	var b CallBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return failed(err)
	}
	if !callAllowed(b.Method, b.Path) {
		return ActionResult{Status: "failed", Note: "a phone can't do this: " + b.Method + " " + b.Path}
	}
	path, err := h.resolveTemp(ctx, tempInPath, b.Path, func(id int64) string { return strconv.FormatInt(id, 10) })
	if err != nil {
		return h.refused(ctx, a, b, err.Error())
	}
	if path, err = h.resolveNumbers(ctx, p, tempNumInPath, path); err != nil {
		return h.refused(ctx, a, b, err.Error())
	}
	body, err := h.resolveBodyFor(ctx, p, b.Body)
	if err != nil {
		return h.refused(ctx, a, b, err.Error())
	}
	stale := h.staleBalance(ctx, b.Saw)
	code, reply := h.dispatch(ctx, p, a.UserID, b.Method, path, body)
	if code < 200 || code >= 300 {
		return h.refused(ctx, a, b, replyMessage(code, reply))
	}
	r := ActionResult{Status: "applied", ResultID: replyID(reply), Note: stale, Flagged: stale != ""}
	if len(b.TempFields) > 0 {
		r.Reply = reply
	}
	// ponytail: a replayed call counts on the day it arrives, marked late; give routes an "at" if owners need it dated back.
	if time.Since(a.At) > lateAfter {
		r.Note = strings.TrimSpace("Happened on the phone at " + a.At.Local().Format("15:04 on 2 Jan") + "; counted when it arrived. " + r.Note)
	}
	return r
}

// refused: the PC said no, so nothing changed, except that cash the cashier
// already handed over really left the drawer.
func (h handlers) refused(ctx context.Context, a Action, b CallBody, why string) ActionResult {
	label := b.Label
	if label != "" {
		why = label + ": " + why
	}
	if b.CashOut == "" && b.CashIn == "" {
		return ActionResult{Status: "failed", Note: why}
	}
	reason := "Taken on the phone but refused by the PC"
	if label != "" {
		reason += ": " + label
	}
	amount, moved, in := b.CashOut, "left", cashregister.MovementInput{Reason: reason, DeferDrawerKick: true}
	var err error
	if b.CashOut != "" {
		in.Amount = b.CashOut
		_, err = h.p.core.CashRegister.Withdraw(ctx, a.UserID, in)
	} else {
		amount, moved, in.Amount = b.CashIn, "came into", b.CashIn
		_, err = h.p.core.CashRegister.PayIn(ctx, a.UserID, in)
	}
	if err != nil {
		// ponytail: an untracked cash move is the only book entry without a locker; a shop that bars them or a closed till shows it at close, with this note.
		return ActionResult{Status: "failed", Note: fmt.Sprintf("%s. %s %s the drawer but couldn't be booked (%s): check the till", why, amount, moved, err.Error())}
	}
	return ActionResult{Status: "failed", Note: fmt.Sprintf("%s. %s %s the drawer and is booked as a cash move", why, amount, moved)}
}

// refusedSale: the PC can't take a sale at all, but the customer's cash really
// went into the drawer (or a payout came out of it), so that's booked anyway.
func (h handlers) refusedSale(ctx context.Context, a Action, why string) ActionResult {
	var b SaleBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return ActionResult{Status: "failed", Note: why}
	}
	paid, cash := decimal.Zero, decimal.Zero
	for _, p := range b.Payments {
		amt, err := decimal.NewFromString(p.Amount)
		if err != nil {
			continue
		}
		paid = paid.Add(amt)
		if p.Method == "cash" {
			cash = cash.Add(amt)
		}
	}
	change := decimal.Zero
	if c, err := decimal.NewFromString(b.ChangeGiven); err == nil {
		change = c
	} else if t, err := decimal.NewFromString(b.Total); err == nil {
		change = decimal.Max(paid.Sub(t), decimal.Zero)
	}
	net := cash.Sub(change)
	call := CallBody{Label: "A sale"}
	switch {
	case net.IsPositive():
		call.CashIn = net.StringFixed(2)
	case net.IsNegative():
		call.CashOut = net.Neg().StringFixed(2)
	}
	return h.refused(ctx, a, call, why)
}

// staleBalance compares what the phone showed with what the PC has now.
func (h handlers) staleBalance(ctx context.Context, saw *CallSaw) string {
	if saw == nil {
		return ""
	}
	var now decimal.Decimal
	if err := h.p.core.DB.GetContext(ctx, &now, `SELECT outstanding_balance FROM customers WHERE id = $1`, saw.CustomerID); err != nil {
		return ""
	}
	shown, err := decimal.NewFromString(saw.Balance)
	if err != nil || shown.Equal(now) {
		return ""
	}
	return fmt.Sprintf("The phone showed a balance of %s; the PC had %s", shown.StringFixed(2), now.StringFixed(2))
}

// dispatch runs the route in this process, signed in as that person. It carries
// their current device: work done on the phone before they moved to a PC must
// still go through.
func (h handlers) dispatch(ctx context.Context, p *Phone, userID int64, method, path string, body []byte) (int, []byte) {
	u, err := h.p.core.Auth.GetUser(ctx, userID)
	if err != nil {
		return http.StatusUnauthorized, []byte(`{"error":{"message":"no such person"}}`)
	}
	tok, err := h.p.core.Auth.AccessTokenFor(u)
	if err != nil {
		return http.StatusInternalServerError, []byte(`{"error":{"message":"could not sign in as that person"}}`)
	}
	device := p.Device
	if active, _, err := h.p.core.Auth.DeviceOf(ctx, userID); err == nil && active != "" {
		device = active
	}
	req, err := http.NewRequestWithContext(ctx, method, path, bytes.NewReader(body))
	if err != nil {
		return http.StatusBadRequest, []byte(`{"error":{"message":"bad path"}}`)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ks-Device", device)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: tok})
	rec := &callReply{header: http.Header{}, code: http.StatusOK}
	h.p.echo.ServeHTTP(rec, req)
	return rec.code, rec.buf.Bytes()
}

// callReply catches a route's answer (net/http/httptest isn't for production builds).
type callReply struct {
	header http.Header
	code   int
	buf    bytes.Buffer
}

func (r *callReply) Header() http.Header         { return r.header }
func (r *callReply) Write(b []byte) (int, error) { return r.buf.Write(b) }
func (r *callReply) WriteHeader(code int)        { r.code = code }

func replyMessage(code int, reply []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(reply, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	return "the PC answered " + strconv.Itoa(code)
}

// replyID is what the route made, from the field names the routes use.
func replyID(reply []byte) int64 {
	var r map[string]any
	if json.Unmarshal(reply, &r) != nil {
		return 0
	}
	for _, k := range []string{"id", "paymentId", "claimId", "saleId", "receiptId"} {
		if n, ok := r[k].(float64); ok {
			return int64(n)
		}
	}
	return 0
}
