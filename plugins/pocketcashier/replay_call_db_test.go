package pocketcashier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// callFix adds stand-in routes for the real ones (the core API isn't mounted in
// this harness): pay books a row in what's seen, and answers with its id.
type callFix struct {
	*rfix
	seen []map[string]any
	who  []int64
}

func newCallFix(t *testing.T) *callFix {
	f := &callFix{rfix: newRfix(t, "10")}
	jwt := middleware.JWTAuth(testSecret)
	pay := func(c echo.Context) error {
		var b map[string]any
		if err := c.Bind(&b); err != nil {
			return err
		}
		if b["refuse"] == true {
			return apperr.ConflictCode("NO", "already returned")
		}
		b["path"] = c.Request().URL.Path
		f.seen = append(f.seen, b)
		f.who = append(f.who, middleware.CurrentUserID(c))
		return c.JSON(http.StatusCreated, map[string]any{"id": 1000 + len(f.seen)})
	}
	extraCallRoutes = []string{"POST /api/v1/till/t-pay", "POST /api/v1/till/t-pay/:id", "POST /api/v1/till/t-boss", "POST /api/v1/till/t-device", "POST /api/v1/till/t-both", "POST /api/v1/till/t-cust", "POST /api/v1/till/t-cancel", "POST /api/v1/till/t-boom", "POST /api/v1/till/t-nested"}
	t.Cleanup(func() { extraCallRoutes = nil })
	f.en.e.POST("/api/v1/till/t-pay", pay, jwt)
	f.en.e.POST("/api/v1/till/t-pay/:id", pay, jwt)
	f.en.e.POST("/api/v1/till/t-boss", pay, jwt, middleware.RequireRole(auth.RoleAdmin))
	f.en.e.POST("/api/v1/admin/t-pay", pay, jwt)
	return f
}

func (f *callFix) call(ref string, at time.Time, b CallBody) Action {
	raw, err := json.Marshal(b)
	if err != nil {
		f.en.t.Fatal(err)
	}
	return Action{Ref: f.ref(ref), Kind: "call", UserID: f.cashier, At: at, Body: raw, Handed: true}
}

func (f *callFix) expected() decimal.Decimal {
	f.en.t.Helper()
	sum, err := f.en.p.core.CashRegister.Summary(context.Background(), f.cashier)
	if err != nil {
		f.en.t.Fatal(err)
	}
	return sum.Expected
}

func (f *callFix) send(a ...Action) []ActionResult {
	f.en.t.Helper()
	rec := f.en.do(http.MethodPost, "/actions", ActionsRequest{Actions: a}, nil, f.secret)
	if rec.Code != http.StatusOK {
		f.en.t.Fatalf("send %d %s", rec.Code, rec.Body)
	}
	return decodeAs[ActionsReply](f.en.t, rec).Results
}

func TestCallAppliesThroughRoute(t *testing.T) {
	f := newCallFix(t)
	r := f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"amount":"50"}`), Label: "Credit"}))
	if r[0].Status != "applied" || r[0].ResultID != 1001 {
		t.Fatalf("result: %+v", r[0])
	}
	if len(f.seen) != 1 || f.seen[0]["amount"] != "50" || f.who[0] != f.cashier {
		t.Fatalf("the route saw %+v as %v", f.seen, f.who)
	}
}

func TestCallTwiceBooksOnce(t *testing.T) {
	f := newCallFix(t)
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`)})
	first, second := f.send(a), f.send(a)
	if len(f.seen) != 1 || second[0].Status != first[0].Status || second[0].ResultID != first[0].ResultID {
		t.Fatalf("booked %d; %+v then %+v", len(f.seen), first[0], second[0])
	}
}

func TestCallRunsAsThatPerson(t *testing.T) {
	f := newCallFix(t)
	r := f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-boss", Body: json.RawMessage(`{}`)}))
	if r[0].Status != "failed" || len(f.seen) != 0 {
		t.Fatalf("a cashier reached an admin's route: %+v", r[0])
	}
}

func TestCallAfterMovingToThePC(t *testing.T) {
	f := newCallFix(t)
	if err := f.en.p.core.Auth.ClaimDevice(t.Context(), f.cashier, "the-pc", "Counter PC"); err != nil {
		t.Fatal(err)
	}
	var dev string
	f.en.e.POST("/api/v1/till/t-device", func(c echo.Context) error {
		dev = c.Request().Header.Get("X-Ks-Device")
		return c.NoContent(http.StatusNoContent)
	}, middleware.JWTAuth(testSecret))
	r := f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-device", Body: json.RawMessage(`{}`)}))
	if r[0].Status != "applied" || dev != "the-pc" {
		t.Fatalf("a call made before the move must still go through: %+v, device %q", r[0], dev)
	}
}

func TestCallOutsideAPIRefused(t *testing.T) {
	f := newCallFix(t)
	for i, b := range []CallBody{
		{Method: "POST", Path: "/admin/x"},
		{Method: "POST", Path: "/api/v1/admin/t-pay"},
		{Method: "POST", Path: "/api/v1/auth/pin"},
		{Method: "POST", Path: "/api/v1/x/pocketcashier/phones/approve"},
		{Method: "POST", Path: "/api/v1/till/../admin/t-pay"},
		{Method: "PUT", Path: "/api/v1/x/repairs/config"},
		{Method: "POST", Path: "/api/v1/x/repairs/jobs/1/payouts"},
		{Method: "POST", Path: "/api/v1/till/suppliers/1/payment"},
		{Method: "GET", Path: "/api/v1/till/t-pay"},
	} {
		r := f.send(f.call(string(rune('a'+i)), time.Now(), b))
		if r[0].Status != "failed" || len(f.seen) != 0 {
			t.Fatalf("%s %s: %+v, seen %v", b.Method, b.Path, r[0], f.seen)
		}
	}
}

func TestCallTempRefResolved(t *testing.T) {
	f := newCallFix(t)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`), Label: "New patient"})
	temp := "temp:" + parent.Ref
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay/" + temp, Body: json.RawMessage(`{"patientId":"` + temp + `"}`)})
	r := f.send(parent, child)
	if r[1].Status != "applied" {
		t.Fatalf("child: %+v", r[1])
	}
	if got := f.seen[1]["patientId"]; got != float64(1001) {
		t.Fatalf("body: %v", got)
	}
	if got := f.seen[1]["path"]; got != "/api/v1/till/t-pay/1001" {
		t.Fatalf("path: %v", got)
	}
}

func TestCallTempRefSale(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	s := f.saleAct("s", ago(time.Minute), "1", "120")
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"saleId":"temp:` + s.Ref + `"}`)})
	r := f.send(s, child)
	if r[0].SaleID == 0 || r[1].Status != "applied" || f.seen[0]["saleId"] != float64(r[0].SaleID) {
		t.Fatalf("%+v, saw %v", r, f.seen)
	}
}

func TestCallParentFailed(t *testing.T) {
	f := newCallFix(t)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), Label: "New patient Sunil"})
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"patientId":"temp:` + parent.Ref + `"}`)})
	r := f.send(parent, child)
	if r[1].Status != "failed" || !strings.Contains(r[1].Note, "New patient Sunil") || len(f.seen) != 0 {
		t.Fatalf("%+v, seen %v", r[1], f.seen)
	}
}

func TestCallLateIsMarked(t *testing.T) {
	f := newCallFix(t)
	at := ago(26 * time.Hour)
	r := f.send(f.call("a", at, CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`)}))
	if r[0].Status != "applied" || !strings.HasPrefix(r[0].Note, "Happened on the phone at") {
		t.Fatalf("%+v", r[0])
	}
	var stored time.Time
	if err := f.en.db.Get(&stored, `SELECT at FROM pocket_actions WHERE ref = $1`, f.ref("a")); err != nil || !stored.Equal(at) {
		t.Fatalf("when it happened must stay on record: %v %v", stored, err)
	}
	if r := f.send(f.call("b", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`)})); r[0].Note != "" {
		t.Fatalf("a call sent at once isn't late: %+v", r[0])
	}
}

func TestCallRefusedKeepsCash(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	f.send(f.saleAct("s", ago(time.Minute), "1", "120"))
	before := f.expected()
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), CashOut: "100", Label: "Return R-7"})
	r := f.send(a)
	if r[0].Status != "failed" || !strings.Contains(r[0].Note, "100") || !strings.Contains(r[0].Note, "already returned") {
		t.Fatalf("%+v", r[0])
	}
	want := before.Sub(decimal.NewFromInt(100))
	if after := f.expected(); !after.Equal(want) {
		t.Fatalf("the 100 that left the drawer isn't recorded: %s, want %s", after, want)
	}
	f.send(a)
	if after := f.expected(); !after.Equal(want) {
		t.Fatalf("a repeated send took the cash twice: %s", after)
	}
}

// A till closed meanwhile: as with a late sale, a till is opened for it, flagged.
func TestCallRefusedDrawerClosedOpensALateTill(t *testing.T) {
	f := newCallFix(t)
	r := f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), CashIn: "100", Label: "Credit"}))
	if r[0].Status != "failed" || !strings.Contains(r[0].Note, "booked") {
		t.Fatalf("%+v", r[0])
	}
	if got := f.expected(); !got.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("the 100 isn't on the books: %s", got)
	}
}

func TestCallRefusedBooksCashIn(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	before := f.expected()
	r := f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), CashIn: "70", Label: "Credit"}))
	if r[0].Status != "failed" || !strings.Contains(r[0].Note, "70") {
		t.Fatalf("%+v", r[0])
	}
	if after := f.expected(); !after.Equal(before.Add(decimal.NewFromInt(70))) {
		t.Fatalf("the 70 that came in isn't recorded: %s", after)
	}
}

func TestCallStaleBalanceFlagged(t *testing.T) {
	f := newCallFix(t)
	var cust int64
	if err := f.en.db.Get(&cust, `INSERT INTO customers (name, credit_limit, outstanding_balance) VALUES ($1, 0, 900) RETURNING id`, "T stale "+f.refStamp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.en.db.Exec(`DELETE FROM customers WHERE id = $1`, cust) }) //nolint:errcheck
	call := func(ref, saw string) ActionResult {
		return f.send(f.call(ref, time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`), Saw: &CallSaw{CustomerID: cust, Balance: saw}}))[0]
	}
	if r := call("same", "900.00"); r.Status != "applied" || r.Note != "" {
		t.Fatalf("the same balance needs no note: %+v", r)
	}
	r := call("stale", "500")
	if r.Status != "applied" || !strings.Contains(r.Note, "500") || !strings.Contains(r.Note, "900") {
		t.Fatalf("%+v", r)
	}
	var flagged int
	if err := f.en.db.Get(&flagged, `SELECT s.flagged FROM pocket_sends s JOIN pocket_actions a ON a.send_id = s.id WHERE a.ref = $1`, f.ref("stale")); err != nil || flagged != 1 {
		t.Fatalf("flagged %d, %v", flagged, err)
	}
}

func TestCallShowsItsLabelInReview(t *testing.T) {
	f := newCallFix(t)
	f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`), Label: "Credit from Sunil"}))
	var send int64
	if err := f.en.db.Get(&send, `SELECT send_id FROM pocket_actions WHERE ref = $1`, f.ref("a")); err != nil {
		t.Fatal(err)
	}
	_, admin := f.en.user("admin")
	rec := f.en.do(http.MethodGet, fmt.Sprintf("/sends/%d", send), nil, admin, "")
	got := decodeAs[SendDetail](t, rec)
	if len(got.Actions) != 1 || got.Actions[0].Label != "Credit from Sunil" {
		t.Fatalf("%d %+v", rec.Code, got)
	}
}

// Plugin screens only load a record whose id is a positive number, so what's made
// offline gets a very large one, which the PC swaps for the real id.
func TestCallTempNumberResolved(t *testing.T) {
	f := newCallFix(t)
	const temp = int64(9000000000012345)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`), Label: "New patient", TempID: temp})
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: fmt.Sprintf("/api/v1/till/t-pay/%d", temp),
		Body: json.RawMessage(fmt.Sprintf(`{"patientId":%d,"ids":[%d],"barcode":"%d","small":12}`, temp, temp, temp))})
	r := f.send(parent, child)
	if r[1].Status != "applied" {
		t.Fatalf("child: %+v", r[1])
	}
	if got := f.seen[1]["patientId"]; got != float64(1001) {
		t.Fatalf("body: %v", got)
	}
	if got := f.seen[1]["ids"].([]any)[0]; got != float64(1001) {
		t.Fatalf("list: %v", got)
	}
	if got := f.seen[1]["barcode"]; got != fmt.Sprint(temp) {
		t.Fatalf("a text that looks like one must stay as it is: %v", got)
	}
	if got := f.seen[1]["path"]; got != "/api/v1/till/t-pay/1001" {
		t.Fatalf("path: %v", got)
	}
}

func TestCallTempNumberNothingMadeLeftAlone(t *testing.T) {
	f := newCallFix(t)
	r := f.send(f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"qty":9000000000099999}`)}))
	if r[0].Status != "applied" || f.seen[0]["qty"] != float64(9000000000099999) {
		t.Fatalf("%+v %v", r[0], f.seen)
	}
}

func TestCallTempNumberParentFailed(t *testing.T) {
	f := newCallFix(t)
	const temp = int64(9000000000054321)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), Label: "New patient Sunil", TempID: temp})
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(fmt.Sprintf(`{"patientId":%d}`, temp))})
	r := f.send(parent, child)
	if r[1].Status != "failed" || !strings.Contains(r[1].Note, "New patient Sunil") || len(f.seen) != 0 {
		t.Fatalf("%+v, seen %v", r[1], f.seen)
	}
}

// One thing made offline can make two: a new patient is also a new customer.
func TestCallTempField(t *testing.T) {
	f := newCallFix(t)
	f.en.e.POST("/api/v1/till/t-both", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"id": 501, "customerId": 777})
	}, middleware.JWTAuth(testSecret))
	const temp, cust = int64(9000000000022222), int64(9000000000022223)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-both", Body: json.RawMessage(`{}`), TempID: temp, TempFields: map[string]string{fmt.Sprint(cust): "customerId"}})
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(fmt.Sprintf(`{"patientId":%d,"customerId":%d}`, temp, cust))})
	r := f.send(parent, child)
	if r[1].Status != "applied" || f.seen[0]["patientId"] != float64(501) || f.seen[0]["customerId"] != float64(777) {
		t.Fatalf("%+v, saw %v", r[1], f.seen)
	}
}

func TestSaleOnACustomerMadeOffline(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	var cust int64
	if err := f.en.db.Get(&cust, `INSERT INTO customers (name, credit_limit) VALUES ($1, 0) RETURNING id`, "T off "+f.refStamp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.en.db.Exec(`UPDATE sales SET customer_id = NULL WHERE customer_id = $1`, cust)
		f.en.db.Exec(`DELETE FROM customers WHERE id = $1`, cust)
	}) //nolint:errcheck
	f.en.e.POST("/api/v1/till/t-cust", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"id": 502, "customerId": cust})
	}, middleware.JWTAuth(testSecret))
	const temp, tempCust = int64(9000000000033333), int64(9000000000033334)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-cust", Body: json.RawMessage(`{}`), TempID: temp, TempFields: map[string]string{fmt.Sprint(tempCust): "customerId"}})
	sale := f.saleAct("s", ago(time.Second), "1", "120")
	var b map[string]any
	if err := json.Unmarshal(sale.Body, &b); err != nil {
		t.Fatal(err)
	}
	b["customerId"] = tempCust
	sale.Body, _ = json.Marshal(b)
	r := f.send(parent, sale)
	if r[1].Status != "applied" {
		t.Fatalf("%+v", r[1])
	}
	var got int64
	if err := f.en.db.Get(&got, `SELECT customer_id FROM sales WHERE id = $1`, r[1].SaleID); err != nil || got != cust {
		t.Fatalf("customer %d, %v", got, err)
	}
}

// C1: with the cashier watching the screen, a refusal means no money moved.
func TestCallRefusedWhileWatchedBooksNothing(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	f.send(f.saleAct("s", ago(time.Minute), "1", "120"))
	before := f.expected()
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), CashOut: "100", Label: "Return"})
	a.Handed = false
	r := f.send(a)
	if r[0].Status != "failed" || strings.Contains(r[0].Note, "booked") {
		t.Fatalf("%+v", r[0])
	}
	if after := f.expected(); !after.Equal(before) {
		t.Fatalf("cash moved on paper with none in hand: %s, was %s", after, before)
	}
}

// C1: the reply was lost, so the screen said "saved" and the cash was handed over.
// The retry says so, and the cash is booked, once.
func TestCallRefusedLostReplyBooksOnRetry(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	f.send(f.saleAct("s", ago(time.Minute), "1", "120"))
	before := f.expected()
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{"refuse":true}`), CashOut: "100", Label: "Return"})
	a.Handed = false
	f.send(a)
	a.Handed = true
	r := f.send(a)
	want := before.Sub(decimal.NewFromInt(100))
	if got := f.expected(); !got.Equal(want) {
		t.Fatalf("after the retry: %s, want %s", got, want)
	}
	if !strings.Contains(r[0].Note, "booked") {
		t.Fatalf("%+v", r[0])
	}
	f.send(a)
	if got := f.expected(); !got.Equal(want) {
		t.Fatalf("booked twice: %s", got)
	}
}

// C1: Pay keeps the cart after a refusal, and each try is a new ref.
func TestSaleRefusedWhileWatchedBooksNothing(t *testing.T) {
	f := newCallFix(t)
	f.openAt(ago(time.Minute))
	before := f.expected()
	for _, ref := range []string{"x", "y"} {
		a := f.saleAct(ref, ago(time.Second), "1", "120")
		a.Body, _ = json.Marshal(SaleBody{SaleType: "retail", Items: []SaleItem{{ProductID: 999999999, Quantity: "1"}}, Payments: []SalePayment{{Method: "cash", Amount: "120"}}})
		if r := f.send(a); r[0].Status != "failed" {
			t.Fatalf("%+v", r[0])
		}
	}
	if after := f.expected(); !after.Equal(before) {
		t.Fatalf("%s, was %s", after, before)
	}
}

// C3: the phone goes away mid-batch: the work is still recorded, so a retry can't repeat it.
func TestCallSurvivesADroppedConnection(t *testing.T) {
	f := newCallFix(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	booked := 0
	f.en.e.POST("/api/v1/till/t-cancel", func(c echo.Context) error {
		booked++
		cancel()
		return c.JSON(http.StatusOK, map[string]any{"id": 900})
	}, middleware.JWTAuth(testSecret))
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-cancel", Body: json.RawMessage(`{}`)})
	raw, _ := json.Marshal(ActionsRequest{Actions: []Action{a}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/x/pocketcashier/actions", bytes.NewReader(raw)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(phoneHeader, f.secret)
	f.en.e.ServeHTTP(httptest.NewRecorder(), req)
	r := f.send(a)
	if booked != 1 || r[0].ResultID != 900 {
		t.Fatalf("booked %d times; %+v", booked, r[0])
	}
}

// I1: what a call made may sit inside its reply.
func TestCallResultInsideTheReply(t *testing.T) {
	f := newCallFix(t)
	f.en.e.POST("/api/v1/till/t-nested", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"package": map[string]any{"id": 321}, "created": true})
	}, middleware.JWTAuth(testSecret))
	const temp = int64(9000000000044444)
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-nested", Body: json.RawMessage(`{}`), TempID: temp, ResultAt: "package.id"})
	child := f.call("child", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(fmt.Sprintf(`{"pkg":%d}`, temp))})
	r := f.send(parent, child)
	if r[1].Status != "applied" || f.seen[0]["pkg"] != float64(321) {
		t.Fatalf("%+v, saw %v", r, f.seen)
	}
}

// C2: the screen gets the route's own reply, on the send and on a repeat of it.
func TestCallGivesBackTheRoutesReply(t *testing.T) {
	f := newCallFix(t)
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`)})
	for i := 0; i < 2; i++ {
		var got map[string]any
		if err := json.Unmarshal(f.send(a)[0].Reply, &got); err != nil || got["id"] != float64(1001) {
			t.Fatalf("try %d: %v %v", i, got, err)
		}
	}
}

// I3: the PC's own trouble isn't a refusal: the action stays with the phone and goes again.
func TestCallServerErrorIsTriedAgain(t *testing.T) {
	f := newCallFix(t)
	fail := true
	booked := 0
	f.en.e.POST("/api/v1/till/t-boom", func(c echo.Context) error {
		if fail {
			return apperr.Internal("database hiccup", errors.New("boom"))
		}
		booked++
		return c.JSON(http.StatusOK, map[string]any{"id": 1})
	}, middleware.JWTAuth(testSecret))
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-boom", Body: json.RawMessage(`{}`), CashIn: "50"})
	after := f.call("b", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-pay", Body: json.RawMessage(`{}`)})
	if r := f.send(a, after); len(r) != 0 || len(f.seen) != 0 {
		t.Fatalf("nothing may be answered, and the later action must wait its turn: %+v, %v", r, f.seen)
	}
	fail = false
	if r := f.send(a, after); len(r) != 2 || r[0].Status != "applied" || booked != 1 {
		t.Fatalf("%+v, booked %d", r, booked)
	}
}

// I3: something the PC can never do mustn't block the phone for good.
func TestCallServerErrorGivesUpInTheEnd(t *testing.T) {
	f := newCallFix(t)
	was := maxServerRetries
	maxServerRetries = 2
	t.Cleanup(func() { maxServerRetries = was })
	f.en.e.POST("/api/v1/till/t-boom", func(c echo.Context) error {
		return apperr.Internal("always", errors.New("boom"))
	}, middleware.JWTAuth(testSecret))
	a := f.call("a", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-boom", Body: json.RawMessage(`{}`)})
	f.send(a)
	f.send(a)
	if r := f.send(a); len(r) != 1 || r[0].Status != "failed" {
		t.Fatalf("%+v", r)
	}
}

func TestHoldOnACustomerMadeOffline(t *testing.T) {
	f := newCallFix(t)
	const temp, tempCust = int64(9000000000055555), int64(9000000000055556)
	var cust int64
	if err := f.en.db.Get(&cust, `INSERT INTO customers (name, credit_limit) VALUES ($1, 0) RETURNING id`, "T hold "+f.refStamp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.en.db.Exec(`DELETE FROM held_sales WHERE cashier_id = $1`, f.cashier)
		f.en.db.Exec(`DELETE FROM customers WHERE id = $1`, cust)
	}) //nolint:errcheck
	f.en.e.POST("/api/v1/till/t-cust", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"id": 502, "customerId": cust})
	}, middleware.JWTAuth(testSecret))
	parent := f.call("parent", time.Now(), CallBody{Method: "POST", Path: "/api/v1/till/t-cust", Body: json.RawMessage(`{}`), TempID: temp, TempFields: map[string]string{fmt.Sprint(tempCust): "customerId"}})
	hold := act(f.ref("h"), "hold", f.cashier, time.Now(), map[string]any{"label": "x", "saleType": "retail", "customerId": tempCust, "discount": "0", "cart": `{"v":2,"lines":[],"customerId":` + fmt.Sprint(tempCust) + `}`, "itemCount": 0, "total": "0"})
	r := f.send(parent, hold)
	if r[1].Status != "applied" {
		t.Fatalf("%+v", r[1])
	}
	var got int64
	if err := f.en.db.Get(&got, `SELECT customer_id FROM held_sales WHERE id = $1`, r[1].HeldID); err != nil || got != cust {
		t.Fatalf("customer %d, %v", got, err)
	}
}
