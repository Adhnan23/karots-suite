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
	"sync"
	"sync/atomic"
	"time"

	_ "embed"

	"karots-suite/internal/features/cashflow"
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
	// ResultAt: where in the reply the id of what this made is, when it isn't at the top.
	ResultAt string `json:"resultAt"`
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

// callRoutesJSON is every route a phone may replay. A person's rights are still
// checked by the route; this keeps a phone's key to what the phone really does, away
// from admin and manager pages, sign-in and the phones list.
//
//go:embed call_routes.json
var callRoutesJSON []byte

var callRoutes = func() []string {
	var out []string
	if err := json.Unmarshal(callRoutesJSON, &out); err != nil {
		panic(err)
	}
	return out
}()

// extraCallRoutes is for tests' stand-in routes.
var extraCallRoutes []string

var callParam = regexp.MustCompile(`:[a-z]+`)

func callAllowed(method, path string) bool {
	if strings.Contains(path, "..") {
		return false
	}
	want := method + " " + path
	for _, list := range [][]string{callRoutes, extraCallRoutes} {
		for _, r := range list {
			if ok, _ := regexp.MatchString("^"+callParam.ReplaceAllString(regexp.QuoteMeta(r), "[^/]+")+"$", want); ok {
				return true
			}
		}
	}
	return false
}

// maxServerRetries: how many times the PC's own trouble (a 5xx) sends an action back
// to wait, about 5 s apart, before it counts as refused, so nothing blocks a phone for good.
var maxServerRetries = 20

// ponytail: counted in memory: a restart starts the count again, which only means more tries.
var serverRetries sync.Map

func keepTrying(ref string) bool {
	n, _ := serverRetries.LoadOrStore(ref, new(atomic.Int64))
	if n.(*atomic.Int64).Add(1) <= int64(maxServerRetries) {
		return true
	}
	serverRetries.Delete(ref)
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
	if b.CashIn != "" || b.CashOut != "" {
		if err := h.openLateTill(ctx, p, a.UserID); err != nil {
			return h.refused(ctx, a, b, err.Error())
		}
	}
	code, reply := h.dispatch(ctx, p, a.UserID, b.Method, path, body)
	if code >= 500 && keepTrying(a.Ref) {
		return ActionResult{Status: "retry", Note: replyMessage(code, reply)}
	}
	if code < 200 || code >= 300 {
		return h.refused(ctx, a, b, replyMessage(code, reply))
	}
	r := ActionResult{Status: "applied", ResultID: replyID(reply, b.ResultAt), Note: stale, Flagged: stale != "", Reply: reply}
	// ponytail: a replayed call counts on the day it arrives, marked late; give routes an "at" if owners need it dated back.
	if time.Since(a.At) > lateAfter {
		r.Note = strings.TrimSpace("Happened on the phone at " + a.At.Local().Format("15:04 on 2 Jan") + "; counted when it arrived. " + r.Note)
	}
	return r
}

// refused: the PC said no, so nothing changed. Cash is booked only if the cashier
// really handed it over: with the screen waiting they saw the refusal, and no money moved.
func (h handlers) refused(ctx context.Context, a Action, b CallBody, why string) ActionResult {
	if b.Label != "" {
		why = b.Label + ": " + why
	}
	if !a.Handed || (b.CashOut == "" && b.CashIn == "") {
		return ActionResult{Status: "failed", Note: why, Settled: a.Handed}
	}
	return ActionResult{Status: "failed", Note: h.bookCash(ctx, a, b, why), Settled: true}
}

// bookCash puts cash that really moved on the books, and says so.
func (h handlers) bookCash(ctx context.Context, a Action, b CallBody, why string) string {
	reason := "Taken on the phone but refused by the PC"
	if b.Label != "" {
		reason += ": " + b.Label
	}
	till, outside := cashflow.Till(a.UserID), cashflow.External()
	in := cashflow.MoveInput{From: outside, To: till, Reason: reason, ActorID: a.UserID, Recorded: true, ReceiptKind: "refused_on_phone"}
	raw, moved := b.CashIn, "came into"
	if b.CashOut != "" {
		in.From, in.To = till, outside
		raw, moved = b.CashOut, "left"
	}
	amount, err := decimal.NewFromString(raw)
	if err == nil {
		phone, _ := ctx.Value(phoneCtxKey{}).(*Phone) // set for every batch
		err = h.openLateTill(ctx, phone, a.UserID)
	}
	if err == nil {
		in.Amount = amount
		_, err = h.p.core.Cashflow.Move(ctx, in)
	}
	if err != nil {
		return fmt.Sprintf("%s. %s %s the drawer but couldn't be booked (%s): check the till", why, raw, moved, err.Error())
	}
	return fmt.Sprintf("%s. %s %s the drawer and is booked as a cash move", why, raw, moved)
}

// openLateTill: cash work needs an open till. Closed meanwhile, one is opened for it
// and marked for review, as for a late sale.
func (h handlers) openLateTill(ctx context.Context, p *Phone, userID int64) error {
	sess, err := h.p.core.CashRegister.Current(ctx, userID)
	if err != nil || sess != nil {
		return err
	}
	_, err = h.p.core.CashRegister.Open(ctx, userID, cashregister.OpenInput{OpeningCash: "0", Remote: true, ReviewNote: "opened for late work from " + p.Name})
	return err
}

// settleLate: an action refused with the cashier watching, whose reply never reached
// them. The screen then said "saved" and the cash was handed over, and this retry says so.
func (h handlers) settleLate(ctx context.Context, a Action, r ActionResult) ActionResult {
	var settled bool
	if err := h.p.core.DB.GetContext(ctx, &settled, `SELECT cash_settled FROM pocket_actions WHERE ref = $1`, a.Ref); err != nil || settled {
		return r
	}
	b := CallBody{}
	if a.Kind == "sale" {
		b = saleCash(a)
	} else if err := json.Unmarshal(a.Body, &b); err != nil {
		return r
	}
	if b.CashOut != "" || b.CashIn != "" {
		r.Note = h.bookCash(ctx, a, b, r.Note)
	}
	_, _ = h.p.core.DB.ExecContext(ctx, `UPDATE pocket_actions SET cash_settled = true, note = $2 WHERE ref = $1`, a.Ref, r.Note)
	return r
}

// refusedSale: the PC can't take a sale at all, but a customer's cash may really be
// in the drawer (or a payout out of it).
func (h handlers) refusedSale(ctx context.Context, a Action, why string) ActionResult {
	return h.refused(ctx, a, saleCash(a), why)
}

// saleCash is the cash a sale moved, as a call would say it.
func saleCash(a Action) CallBody {
	call := CallBody{Label: "A sale"}
	var b SaleBody
	if err := json.Unmarshal(a.Body, &b); err != nil {
		return call
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
	switch {
	case net.IsPositive():
		call.CashIn = net.StringFixed(2)
	case net.IsNegative():
		call.CashOut = net.Neg().StringFixed(2)
	}
	return call
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

// replyID is what the route made: at resultAt ("package.id") if given, otherwise from
// the field names the routes use.
func replyID(reply []byte, resultAt string) int64 {
	var r map[string]any
	if json.Unmarshal(reply, &r) != nil {
		return 0
	}
	if resultAt != "" {
		var at any = r
		for _, k := range strings.Split(resultAt, ".") {
			m, ok := at.(map[string]any)
			if !ok {
				return 0
			}
			at = m[k]
		}
		n, _ := at.(float64)
		return int64(n)
	}
	for _, k := range []string{"id", "paymentId", "claimId", "saleId", "receiptId"} {
		if n, ok := r[k].(float64); ok {
			return int64(n)
		}
	}
	return 0
}
