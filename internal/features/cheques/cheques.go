// Package cheques keeps the shop's cheques: received from customers (held in the
// Cheques in hand place until they clear) and given to suppliers / for expenses
// (the bank keeps the money until they are cashed). Every money step goes through
// cashflow.MoveTx inside the caller's transaction; a status change is final.
package cheques

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/cashflow"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Line is one cheque as entered: a received one names the customer's Bank, a
// given one the shop's bank place it is drawn on (LockerID).
type Line struct {
	Bank     string
	LockerID int64
	Number   string
	Amount   decimal.Decimal
	Date     time.Time
	Note     string
}

// LineInput is a Line as it comes off the wire.
type LineInput struct {
	Bank     string
	LockerID int64
	Number   string
	Amount   string
	Date     string // YYYY-MM-DD
	Note     string
}

// Link is who a cheque is from / for and the record it paid.
type Link struct {
	CustomerID, SupplierID                                             int64
	SaleID, PaymentID, CustomerPaymentID, SupplierPaymentID, ExpenseID *int64
	Party                                                              string // name shown on the money receipt
}

// Cheque is one cheques row with the names it points at.
type Cheque struct {
	ID                int64           `db:"id"                  json:"id"`
	Direction         string          `db:"direction"           json:"direction"`
	Bank              string          `db:"bank"                json:"bank"`
	LockerID          *int64          `db:"locker_id"           json:"locker_id,omitempty"`
	Number            string          `db:"number"              json:"number"`
	Amount            decimal.Decimal `db:"amount"              json:"amount"`
	ChequeDate        time.Time       `db:"cheque_date"         json:"cheque_date"`
	Note              string          `db:"note"                json:"note"`
	Status            string          `db:"status"              json:"status"`
	StatusDate        *time.Time      `db:"status_date"         json:"status_date,omitempty"`
	StatusBy          *int64          `db:"status_by"           json:"status_by,omitempty"`
	CustomerID        *int64          `db:"customer_id"         json:"customer_id,omitempty"`
	SupplierID        *int64          `db:"supplier_id"         json:"supplier_id,omitempty"`
	SaleID            *int64          `db:"sale_id"             json:"sale_id,omitempty"`
	PaymentID         *int64          `db:"payment_id"          json:"payment_id,omitempty"`
	CustomerPaymentID *int64          `db:"customer_payment_id" json:"customer_payment_id,omitempty"`
	SupplierPaymentID *int64          `db:"supplier_payment_id" json:"supplier_payment_id,omitempty"`
	ExpenseID         *int64          `db:"expense_id"          json:"expense_id,omitempty"`
	CreatedBy         *int64          `db:"created_by"          json:"created_by,omitempty"`
	CreatedAt         time.Time       `db:"created_at"          json:"created_at"`
	CustomerName      *string         `db:"customer_name"       json:"customer_name,omitempty"`
	SupplierName      *string         `db:"supplier_name"       json:"supplier_name,omitempty"`
	LockerName        *string         `db:"locker_name"         json:"locker_name,omitempty"`
}

const selectCheque = `
	SELECT c.*, cu.name AS customer_name, s.name AS supplier_name, l.name AS locker_name
	FROM cheques c
	LEFT JOIN customers cu ON cu.id = c.customer_id
	LEFT JOIN suppliers s ON s.id = c.supplier_id
	LEFT JOIN lockers l ON l.id = c.locker_id`

// ParseLines reads the wire lines of one cheque payment (dir "in" | "out") and
// returns them with their sum. At least one line; a received cheque needs its
// bank, a given one the bank place it is drawn on.
func ParseLines(in []LineInput, dir string) ([]Line, decimal.Decimal, error) {
	if len(in) == 0 {
		return nil, decimal.Zero, apperr.Validation("add the cheque details")
	}
	out := make([]Line, 0, len(in))
	sum := decimal.Zero
	for _, li := range in {
		l := Line{Bank: strings.TrimSpace(li.Bank), LockerID: li.LockerID, Number: strings.TrimSpace(li.Number), Note: strings.TrimSpace(li.Note)}
		if dir == "in" && l.Bank == "" {
			return nil, decimal.Zero, apperr.Validation("pick the cheque's bank")
		}
		if dir == "out" && l.LockerID <= 0 {
			return nil, decimal.Zero, apperr.Validation("pick the bank place the cheque is drawn on")
		}
		if l.Number == "" {
			return nil, decimal.Zero, apperr.Validation("enter the cheque number")
		}
		amt, err := decimal.NewFromString(strings.TrimSpace(li.Amount))
		if err != nil || !amt.IsPositive() {
			return nil, decimal.Zero, apperr.Validation("cheque " + l.Number + ": amount must be greater than zero")
		}
		d, err := time.Parse("2006-01-02", strings.TrimSpace(li.Date))
		if err != nil {
			return nil, decimal.Zero, apperr.Validation("cheque " + l.Number + ": enter the cheque date")
		}
		l.Amount, l.Date = amt.Round(2), d
		sum = sum.Add(l.Amount)
		out = append(out, l)
	}
	return out, sum, nil
}

// Service changes and reads cheques.
type Service struct {
	db *sqlx.DB
	cf *cashflow.Service
}

func NewService(db *sqlx.DB, cf *cashflow.Service) *Service { return &Service{db: db, cf: cf} }

func get(ctx context.Context, q appdb.Queryer, id int64) (*Cheque, error) {
	var c Cheque
	err := q.GetContext(ctx, &c, selectCheque+` WHERE c.id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("cheque")
	}
	if err != nil {
		return nil, apperr.Internal("failed to load cheque", err)
	}
	return &c, nil
}

// lockPending locks a cheque row and refuses one that is no longer pending.
func lockPending(ctx context.Context, tx *sqlx.Tx, id int64) (*Cheque, error) {
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM cheques WHERE id = $1 FOR UPDATE`, id); err != nil {
		return nil, apperr.Internal("failed to lock cheque", err)
	}
	c, err := get(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if c.Status != "pending" {
		return nil, apperr.Conflict("this cheque is already " + c.Status)
	}
	return c, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Cheque, error) { return get(ctx, s.db, id) }

// Filter narrows List. Zero values match everything.
type Filter struct {
	Direction, Status                string
	From, To                         *time.Time // cheque_date, inclusive
	CustomerID, SupplierID, LockerID int64
	Query                            string // number, bank or who
	Limit, Offset                    int
}

func (s *Service) List(ctx context.Context, f Filter) ([]Cheque, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows := []Cheque{}
	err := s.db.SelectContext(ctx, &rows, selectCheque+`
		WHERE ($1 = '' OR c.direction = $1)
		  AND ($2 = '' OR c.status = $2)
		  AND ($3::date IS NULL OR c.cheque_date >= $3)
		  AND ($4::date IS NULL OR c.cheque_date <= $4)
		  AND ($5 = 0 OR c.customer_id = $5)
		  AND ($6 = 0 OR c.supplier_id = $6)
		  AND ($7 = 0 OR c.locker_id = $7)
		  AND ($8 = '' OR c.number ILIKE '%' || $8 || '%' OR c.bank ILIKE '%' || $8 || '%'
		       OR cu.name ILIKE '%' || $8 || '%' OR s.name ILIKE '%' || $8 || '%')
		ORDER BY (c.status = 'pending') DESC, c.cheque_date, c.id
		LIMIT $9 OFFSET $10`,
		f.Direction, f.Status, f.From, f.To, f.CustomerID, f.SupplierID, f.LockerID, strings.TrimSpace(f.Query), f.Limit, f.Offset)
	if err != nil {
		return nil, apperr.Internal("failed to list cheques", err)
	}
	return rows, nil
}

// CountTotal is how many cheques and their sum.
type CountTotal struct {
	Count int             `db:"count" json:"count"`
	Total decimal.Decimal `db:"total" json:"total"`
}

// BankAfter is a bank place whose pending given cheques take it below zero.
type BankAfter struct {
	LockerID int64           `db:"locker_id" json:"locker_id"`
	Name     string          `db:"name"      json:"name"`
	After    decimal.Decimal `db:"after"     json:"after"`
}

// Summary is the Cheques page header and the dashboard reminders.
type Summary struct {
	InHand    decimal.Decimal `json:"in_hand"`  // pending received
	Given     decimal.Decimal `json:"given"`    // pending given
	DueWeek   decimal.Decimal `json:"due_week"` // pending given dated within the next 7 days (or past)
	ToDeposit CountTotal      `json:"to_deposit"`
	GivenDue  CountTotal      `json:"given_due"`
	Short     []BankAfter     `json:"short"`
}

func (s *Service) Summary(ctx context.Context, today time.Time) (Summary, error) {
	day := today.Format("2006-01-02")
	var sm Summary
	err := s.db.QueryRowxContext(ctx, `
		SELECT COALESCE(SUM(amount) FILTER (WHERE direction = 'in'), 0),
		       COALESCE(SUM(amount) FILTER (WHERE direction = 'out'), 0),
		       COALESCE(SUM(amount) FILTER (WHERE direction = 'out' AND cheque_date <= $1::date + 7), 0),
		       COUNT(*) FILTER (WHERE direction = 'in' AND cheque_date <= $1::date),
		       COALESCE(SUM(amount) FILTER (WHERE direction = 'in' AND cheque_date <= $1::date), 0),
		       COUNT(*) FILTER (WHERE direction = 'out' AND cheque_date <= $1::date + 3),
		       COALESCE(SUM(amount) FILTER (WHERE direction = 'out' AND cheque_date <= $1::date + 3), 0)
		FROM cheques WHERE status = 'pending'`, day).
		Scan(&sm.InHand, &sm.Given, &sm.DueWeek, &sm.ToDeposit.Count, &sm.ToDeposit.Total, &sm.GivenDue.Count, &sm.GivenDue.Total)
	if err != nil {
		return sm, apperr.Internal("failed to total cheques", err)
	}
	sm.Short = []BankAfter{}
	err = s.db.SelectContext(ctx, &sm.Short, `
		SELECT l.id AS locker_id, l.name,
		       COALESCE((SELECT SUM(balance_delta) FROM locker_ledger WHERE locker_id = l.id), 0) - p.given AS after
		FROM lockers l
		JOIN (SELECT locker_id, SUM(amount) AS given FROM cheques
		      WHERE direction = 'out' AND status = 'pending' GROUP BY locker_id) p ON p.locker_id = l.id
		WHERE COALESCE((SELECT SUM(balance_delta) FROM locker_ledger WHERE locker_id = l.id), 0) - p.given < 0
		ORDER BY l.name`)
	if err != nil {
		return sm, apperr.Internal("failed to check bank places", err)
	}
	return sm, nil
}

// AfterCheques maps each bank place to the total of its pending given cheques
// (After cheques = balance − this).
func (s *Service) AfterCheques(ctx context.Context) (map[int64]decimal.Decimal, error) {
	var rows []struct {
		LockerID int64           `db:"locker_id"`
		Given    decimal.Decimal `db:"given"`
	}
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT locker_id, SUM(amount) AS given FROM cheques
		WHERE direction = 'out' AND status = 'pending' GROUP BY locker_id`); err != nil {
		return nil, apperr.Internal("failed to total given cheques", err)
	}
	out := make(map[int64]decimal.Decimal, len(rows))
	for _, r := range rows {
		out[r.LockerID] = r.Given
	}
	return out, nil
}

func idStr(id int64) string { return strconv.FormatInt(id, 10) }
