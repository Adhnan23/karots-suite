package battle

import (
	"testing"

	"fmt"

	"github.com/shopspring/decimal"
	"pgregory.net/rapid"
)

// Random shop days moving money (docs/rules/money.md): sales and refunds at the
// till, withdrawals and deposits, expenses from the till / a locker / outside and
// their voids, transfers, bank charges, interest, Settle card, closes banked or
// left in the drawer. The machine keeps its own books for the till and every
// locker; after every step the auditor runs, every balance is held to the books
// (DRAW-1, LOCK-1), every move wrote one receipt (LOCK-4), a refused move wrote
// nothing, and the Cash flow page's net is what the shop holds (CF-1, CF-2).
func TestBattleMoney(t *testing.T) {
	t.Parallel()
	setRapid(t)
	rapid.Check(t, func(rt *rapid.T) {
		m := newMoneyMachine(rt)
		rt.Repeat(map[string]func(*rapid.T){
			"":           m.check,
			"Sell":       m.sell,
			"Refund":     m.refund,
			"Withdraw":   m.withdraw,
			"Deposit":    m.deposit,
			"Expense":    m.expense,
			"Void":       m.void,
			"Transfer":   m.transfer,
			"BankCharge": m.bankCharge,
			"Interest":   m.interest,
			"SettleCard": m.settleCard,
			"CloseOpen":  m.closeOpen,
		})
		m.s.agreeToday(rt)
	})
}

type mLocker struct {
	id       int64
	name     string
	cash     bool // a till may use it
	negative bool // may go below zero
	bal      decimal.Decimal
}

type mExpense struct {
	id       int64
	amount   decimal.Decimal
	paidFrom *mLocker // nil and !fromTill = outside
	fromTill bool
}

type moneyMachine struct {
	s        *shop
	till     int64 // till1's user id
	open     bool
	drawer   decimal.Decimal
	lockers  []*mLocker // safe, pocket, bank, card
	sales    []saleResult
	expenses []*mExpense
	receipts int // CR- receipts the books expect
}

func newMoneyMachine(t *rapid.T) *moneyMachine {
	s := newShop(t)
	m := &moneyMachine{s: s, till: s.userID(t, "0770000002")}
	mk := func(name, kind string, cash, neg bool, opening string) *mLocker {
		var l struct{ ID int64 }
		s.must(t, s.admin.Do("POST", "/api/v1/admin/lockers", map[string]any{"name": name, "kind": kind, "openingBalance": opening,
			"overdraft": neg, "cashierAccess": true}, &l))
		return &mLocker{id: l.ID, name: name, cash: cash, negative: neg, bal: dec(t, opening)}
	}
	m.lockers = []*mLocker{mk("Safe", "safe", true, false, "20000"), mk("Pocket", "pocket", true, true, "0"),
		mk("Bank", "bank", false, false, "50000"), {id: s.cardAcct, name: "card"}}
	m.receipts = m.count(t)
	return m
}

func (m *moneyMachine) count(t *rapid.T) int {
	var n int
	m.s.must(t, m.s.db.Get(&n, `SELECT count(*) FROM money_receipts`))
	return n
}

func (m *moneyMachine) safe() *mLocker          { return m.lockers[0] }
func (m *moneyMachine) bank() *mLocker          { return m.lockers[2] }
func (m *moneyMachine) card() *mLocker          { return m.lockers[3] }
func (m *moneyMachine) cashLockers() []*mLocker { return m.lockers[:2] }

// amount draws up to max rupees, in cents.
func amount(t *rapid.T, label string, max decimal.Decimal) decimal.Decimal {
	top := int(max.Shift(2).IntPart())
	if top < 1 {
		top = 1
	}
	return decimal.New(int64(rapid.IntRange(1, top).Draw(t, label)), -2)
}

// try sends a move the books expect to pass (ok) or be refused; refused, it must write nothing.
func (m *moneyMachine) try(t *rapid.T, ok bool, rule, what string, call func() error) bool {
	before := m.s.rows(t)
	err := call()
	switch {
	case ok && err != nil:
		t.Fatalf("%s %s refused: %v", rule, what, err)
	case !ok && err == nil:
		t.Fatalf("%s %s went through; the books say it can't", rule, what)
	case !ok && m.s.rows(t) != before:
		t.Fatalf("%s %s refused but wrote rows", rule, what)
	}
	return err == nil
}

func (m *moneyMachine) needOpen(t *rapid.T) {
	if !m.open {
		t.Skip("till closed")
	}
}

func (m *moneyMachine) sell(t *rapid.T) {
	m.needOpen(t)
	qty := rapid.IntRange(1, 20).Draw(t, "qty")
	total := decimal.NewFromInt(int64(qty * 100))
	items := []line{{"productId": m.s.prod["plain"], "quantity": fmt.Sprint(qty)}}
	if rapid.IntRange(0, 3).Draw(t, "card") == 0 {
		m.sales = append(m.sales, m.s.sell(t, m.s.till1, "0", items, pay{"method": "card", "amount": total.String(), "accountId": m.card().id}))
		m.card().bal = m.card().bal.Add(total)
		m.receipts++ // LOCK-12: the takings' move
		return
	}
	given := total.Add(decimal.NewFromInt(int64(rapid.IntRange(0, 500).Draw(t, "extra"))))
	m.sales = append(m.sales, m.s.sell(t, m.s.till1, "0", items, cash(given.String())))
	m.drawer = m.drawer.Add(total)
}

// refund takes a whole sale back in cash: never stopped, even beyond the drawer (DRAW-3).
func (m *moneyMachine) refund(t *rapid.T) {
	m.needOpen(t)
	if len(m.sales) == 0 {
		t.Skip("nothing sold")
	}
	i := rapid.IntRange(0, len(m.sales)-1).Draw(t, "sale")
	r := m.sales[i]
	m.sales = append(m.sales[:i], m.sales[i+1:]...)
	v := m.s.sale(t, m.s.till1, r.ID)
	got := m.s.giveBack(t, m.s.till1, r.ID, v.Lines[0].ID, v.Lines[0].Quantity)
	if want := dec(t, r.Total); !got.Equal(want) {
		t.Fatalf("RET-15 refunded %s, want %s", got, want)
	}
	m.drawer = m.drawer.Sub(got)
	m.receipts++
}

func (m *moneyMachine) withdraw(t *rapid.T) {
	m.needOpen(t)
	amt := amount(t, "amount", decimal.Max(m.drawer, decimal.Zero).Add(decimal.NewFromInt(500)))
	var to *mLocker
	if rapid.Bool().Draw(t, "toLocker") {
		to = rapid.SampledFrom(m.cashLockers()).Draw(t, "locker")
	}
	body := map[string]any{"amount": amt.String(), "reason": "battle"}
	if to != nil {
		body["lockerId"] = to.id
	}
	if m.try(t, !amt.GreaterThan(m.drawer), "DRAW-3", "withdraw "+amt.String(), func() error {
		return m.s.till1.Do("POST", "/api/v1/till/drawer/withdraw", body, nil)
	}) {
		m.drawer = m.drawer.Sub(amt)
		if to != nil {
			to.bal = to.bal.Add(amt)
		}
		m.receipts++
	}
}

func (m *moneyMachine) deposit(t *rapid.T) {
	m.needOpen(t)
	amt := amount(t, "amount", decimal.NewFromInt(3000))
	var from *mLocker
	if rapid.Bool().Draw(t, "fromLocker") {
		from = rapid.SampledFrom(m.cashLockers()).Draw(t, "locker")
	}
	body := map[string]any{"amount": amt.String(), "reason": "battle"}
	ok := true
	if from != nil {
		body["lockerId"] = from.id
		ok = from.negative || !amt.GreaterThan(from.bal)
	}
	if m.try(t, ok, "LOCK-1", "deposit "+amt.String(), func() error {
		return m.s.till1.Do("POST", "/api/v1/till/drawer/deposit", body, nil)
	}) {
		m.drawer = m.drawer.Add(amt)
		if from != nil {
			from.bal = from.bal.Sub(amt)
		}
		m.receipts++
	}
}

func (m *moneyMachine) expense(t *rapid.T) {
	amt := amount(t, "amount", decimal.NewFromInt(4000))
	e := &mExpense{amount: amt}
	source, ok := "", true
	switch rapid.IntRange(0, 2).Draw(t, "from") {
	case 0:
		if !m.open {
			t.Skip("till closed")
		}
		e.fromTill, source, ok = true, fmt.Sprintf("till:%d", m.till), !amt.GreaterThan(m.drawer)
	case 1:
		l := rapid.SampledFrom(m.lockers[:3]).Draw(t, "locker")
		e.paidFrom, source, ok = l, fmt.Sprintf("locker:%d", l.id), l.negative || !amt.GreaterThan(l.bal)
	}
	if !m.try(t, ok, "EXP-1", "expense "+amt.String()+" from "+source, func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/expenses", map[string]any{"category": "Battle", "amount": amt.String(), "source": source}, nil)
	}) {
		return
	}
	m.s.must(t, m.s.db.Get(&e.id, `SELECT max(id) FROM expenses`))
	switch {
	case e.fromTill:
		m.drawer = m.drawer.Sub(amt)
		m.receipts++
	case e.paidFrom != nil:
		e.paidFrom.bal = e.paidFrom.bal.Sub(amt)
		m.receipts++
	}
	m.expenses = append(m.expenses, e)
}

// void puts back what left for an expense, into a cash locker (EXP-2).
func (m *moneyMachine) void(t *rapid.T) {
	if len(m.expenses) == 0 {
		t.Skip("no expense")
	}
	i := rapid.IntRange(0, len(m.expenses)-1).Draw(t, "expense")
	e := m.expenses[i]
	to := rapid.SampledFrom(m.cashLockers()).Draw(t, "to")
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/expenses/%d/void", e.id), map[string]any{"dest": fmt.Sprintf("locker:%d", to.id)}, nil))
	if e.fromTill || e.paidFrom != nil {
		to.bal = to.bal.Add(e.amount)
		m.receipts++
	}
	m.expenses = append(m.expenses[:i], m.expenses[i+1:]...)
}

func (m *moneyMachine) transfer(t *rapid.T) {
	from := rapid.SampledFrom(m.lockers[:3]).Draw(t, "from")
	to := rapid.SampledFrom(m.lockers[:3]).Draw(t, "to")
	if from == to {
		t.Skip("same locker")
	}
	amt := amount(t, "amount", decimal.Max(from.bal, decimal.Zero).Add(decimal.NewFromInt(1000)))
	if m.try(t, from.negative || !amt.GreaterThan(from.bal), "LOCK-1", "transfer "+amt.String()+" "+from.name+"→"+to.name, func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/lockers/transfer", map[string]any{"fromId": from.id, "toId": to.id, "amount": amt.String()}, nil)
	}) {
		from.bal, to.bal = from.bal.Sub(amt), to.bal.Add(amt)
		m.receipts++
	}
}

func (m *moneyMachine) bankCharge(t *rapid.T) {
	amt := amount(t, "amount", decimal.NewFromInt(500))
	if m.try(t, !amt.GreaterThan(m.bank().bal), "LOCK-8", "bank charge "+amt.String(), func() error {
		return m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/lockers/%d/adjust", m.bank().id), map[string]any{"type": "bank_charge", "amount": amt.String(), "note": "battle"}, nil)
	}) {
		m.bank().bal = m.bank().bal.Sub(amt)
		m.receipts++
	}
}

func (m *moneyMachine) interest(t *rapid.T) {
	amt := amount(t, "amount", decimal.NewFromInt(500))
	m.s.must(t, m.s.admin.Do("POST", fmt.Sprintf("/api/v1/admin/lockers/%d/adjust", m.bank().id), map[string]any{"type": "interest", "amount": amt.String(), "note": "battle"}, nil))
	m.bank().bal = m.bank().bal.Add(amt)
	m.receipts++
}

// settleCard banks some of the card account: the fee leaves as a bank charge, the rest goes to the bank (LOCK-7).
func (m *moneyMachine) settleCard(t *rapid.T) {
	if !m.card().bal.IsPositive() {
		t.Skip("nothing on the card account")
	}
	gross := amount(t, "gross", m.card().bal.Add(decimal.NewFromInt(100)))
	fee := decimal.Zero
	if gross.GreaterThan(decimal.NewFromInt(1)) && rapid.Bool().Draw(t, "fee") {
		fee = amount(t, "fee", gross.Div(decimal.NewFromInt(10)).Floor())
	}
	if m.try(t, !gross.GreaterThan(m.card().bal), "LOCK-7", "settle "+gross.String(), func() error {
		return m.s.admin.Do("POST", "/api/v1/admin/lockers/settle-card", map[string]any{"cardId": m.card().id, "bankId": m.bank().id, "gross": gross.String(), "fee": fee.String()}, nil)
	}) {
		m.card().bal = m.card().bal.Sub(gross)
		m.bank().bal = m.bank().bal.Add(gross.Sub(fee))
		m.receipts++
		if fee.IsPositive() {
			m.receipts++
		}
	}
}

// closeOpen closes the till (counted right, over or short; banked or left in it)
// and opens it again (a float from a cash locker or from nowhere).
func (m *moneyMachine) closeOpen(t *rapid.T) {
	if m.open {
		counted := decimal.Max(decimal.Zero, m.drawer.Add(decimal.New(int64(rapid.IntRange(-500, 500).Draw(t, "overShort")), -2)))
		body := map[string]any{"breakdown": map[string]int{}}
		if counted.IsPositive() {
			body["breakdown"] = map[string]int{counted.String(): 1}
		}
		var to *mLocker
		if rapid.Bool().Draw(t, "bank") {
			to = rapid.SampledFrom(m.cashLockers()).Draw(t, "to")
			body["destLockerId"] = to.id
		}
		var res struct{ ExpectedCash, Difference string }
		m.s.must(t, m.s.till1.Do("POST", "/api/v1/till/drawer/close", body, &res))
		if !dec(t, res.ExpectedCash).Equal(m.drawer) || !dec(t, res.Difference).Equal(counted.Sub(m.drawer)) {
			t.Fatalf("DRAW-1 close expected %s / difference %s, the books say %s / %s", res.ExpectedCash, res.Difference, m.drawer, counted.Sub(m.drawer))
		}
		if to != nil && counted.IsPositive() {
			to.bal = to.bal.Add(counted)
		}
		if counted.IsPositive() {
			m.receipts++
		}
		m.open = false
		return
	}
	float := decimal.NewFromInt(int64(rapid.IntRange(0, 50).Draw(t, "float") * 100))
	body := map[string]any{"breakdown": map[string]int{}}
	if float.IsPositive() {
		body["breakdown"] = map[string]int{"100": int(float.IntPart() / 100)}
	}
	var from *mLocker
	if rapid.Bool().Draw(t, "fromLocker") {
		from = rapid.SampledFrom(m.cashLockers()).Draw(t, "from")
		body["sourceLockerId"] = from.id
	}
	ok := from == nil || !float.IsPositive() || from.negative || !float.GreaterThan(from.bal)
	if m.try(t, ok, "LOCK-1", "open with "+float.String(), func() error {
		return m.s.till1.Do("POST", "/api/v1/till/drawer/open", body, nil)
	}) {
		m.open, m.drawer = true, float
		if from != nil {
			from.bal = from.bal.Sub(float)
		}
		if float.IsPositive() {
			m.receipts++
		}
	}
}

func (m *moneyMachine) check(t *rapid.T) {
	m.s.audit(t)
	m.s.agreeSometimes(t)
	held := decimal.Zero
	for _, l := range m.lockers {
		var bal decimal.Decimal
		m.s.must(t, m.s.db.Get(&bal, `SELECT COALESCE(SUM(balance_delta), 0) FROM locker_ledger WHERE locker_id = $1`, l.id))
		if !bal.Equal(l.bal) {
			t.Fatalf("LOCK-1 %s holds %s, the books say %s", l.name, bal, l.bal)
		}
		if !l.negative && bal.IsNegative() {
			t.Fatalf("LOCK-1 %s below zero: %s", l.name, bal)
		}
		held = held.Add(bal)
	}
	if m.open {
		if got := dec(t, m.s.expected(t, m.s.till1)); !got.Equal(m.drawer) {
			t.Fatalf("DRAW-1 drawer expects %s, the books say %s", got, m.drawer)
		}
		held = held.Add(m.drawer)
	}
	if got := m.count(t); got != m.receipts {
		t.Fatalf("LOCK-4 %d receipts, the books say %d", got, m.receipts)
	}
	var cf struct{ CashOnHand, Net string }
	m.s.must(t, m.s.admin.Do("GET", "/api/v1/admin/cashflow", nil, &cf))
	if got := dec(t, cf.Net); !got.Equal(held) {
		t.Fatalf("CF-2 today's net %s, the shop holds %s (it held nothing this morning)", got, held)
	}
}
