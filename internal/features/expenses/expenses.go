// Package expenses records shop running costs (rent, salaries, utilities, ...).
package expenses

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

type Expense struct {
	ID          int64           `db:"id"           json:"id"`
	Category    string          `db:"category"     json:"category"`
	Amount      decimal.Decimal `db:"amount"       json:"amount"`
	Description *string         `db:"description"  json:"description,omitempty"`
	PaidBy      *int64          `db:"paid_by"      json:"paid_by,omitempty"`
	ExpenseDate time.Time       `db:"expense_date" json:"expense_date"`
	CreatedAt   time.Time       `db:"created_at"   json:"created_at"`
	// Unpaid is the part of Amount a bounced / cancelled cheque left unpaid.
	Unpaid     decimal.Decimal `db:"unpaid"       json:"unpaid"`
	PaidByName *string         `db:"paid_by_name" json:"paid_by_name,omitempty"`
	// ServiceProductID attributes an operating cost (a toner change, a machine
	// repair) to the service it belongs to. It is NOT per-unit consumption and
	// never enters COGS: the core P&L still counts it once, as an expense.
	ServiceProductID *int64  `db:"service_product_id" json:"service_product_id,omitempty"`
	ServiceName      *string `db:"service_name"       json:"service_name,omitempty"`
	// PaidFrom is the source label of the money move booked for it (List only);
	// nil = no money moved (paid from outside the shop).
	PaidFrom *string `db:"paid_from" json:"paid_from,omitempty"`
}

type CreateInput struct {
	Category    string  `json:"category"     form:"category"     validate:"required,min=1,max=80"`
	Amount      string  `json:"amount"       form:"amount"       validate:"required"`
	Description *string `json:"description"  form:"description"`
	ExpenseDate string  `json:"expense_date" form:"expense_date"`
	// ServiceProductID is optional; 0 or absent means "not attributed".
	ServiceProductID int64 `json:"service_product_id" form:"service_product_id"`
}

// Filter narrows the expense list: expense_date in [From, To) (To exclusive,
// as reports.ResolveRange gives it), an exact category, and words matched in
// the category / description. Limit 0 = 500.
type Filter struct {
	From     *time.Time
	To       *time.Time
	Category string
	Query    string
	Limit    int
	Offset   int
}

const filterWhere = `
		WHERE ($1::timestamptz IS NULL OR e.expense_date >= $1)
		  AND ($2::timestamptz IS NULL OR e.expense_date <  $2)
		  AND ($3 = '' OR e.category = $3)
		  AND ($4 = '' OR e.category ILIKE '%' || $4 || '%' OR e.description ILIKE '%' || $4 || '%')`

type Repository struct{ q db.Queryer }

func NewRepository(q db.Queryer) *Repository { return &Repository{q: q} }

func (r *Repository) List(ctx context.Context, f Filter) ([]Expense, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}
	var rows []Expense
	err := r.q.SelectContext(ctx, &rows, `
		SELECT e.*, u.name AS paid_by_name, sp.name AS service_name,
		       (SELECT from_label FROM money_receipts mr WHERE mr.ref_kind = 'expense' AND mr.ref_id = e.id ORDER BY mr.id LIMIT 1) AS paid_from
		FROM expenses e
		LEFT JOIN users u ON u.id = e.paid_by
		LEFT JOIN products sp ON sp.id = e.service_product_id`+filterWhere+`
		ORDER BY e.expense_date DESC, e.id DESC
		LIMIT $5 OFFSET $6`, f.From, f.To, f.Category, strings.TrimSpace(f.Query), limit, max(f.Offset, 0))
	return rows, err
}

// Sum totals every expense the filter matches (not a page of them).
func (r *Repository) Sum(ctx context.Context, f Filter) (decimal.Decimal, int, error) {
	var out struct {
		Total decimal.Decimal `db:"total"`
		Count int             `db:"count"`
	}
	err := r.q.GetContext(ctx, &out, `SELECT COALESCE(SUM(e.amount),0) AS total, COUNT(*) AS count FROM expenses e`+filterWhere,
		f.From, f.To, f.Category, strings.TrimSpace(f.Query))
	return out.Total, out.Count, err
}

func (r *Repository) Create(ctx context.Context, category string, amount decimal.Decimal, desc *string, paidBy *int64, date time.Time, serviceID *int64) (*Expense, error) {
	var e Expense
	err := r.q.GetContext(ctx, &e, `
		INSERT INTO expenses (category, amount, description, paid_by, expense_date, service_product_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING *, NULL::varchar AS paid_by_name, NULL::varchar AS service_name`,
		category, amount, desc, paidBy, date, serviceID)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*Expense, error) {
	var e Expense
	err := r.q.GetContext(ctx, &e, `
		SELECT e.*, u.name AS paid_by_name, sp.name AS service_name
		FROM expenses e
		LEFT JOIN users u ON u.id = e.paid_by
		LEFT JOIN products sp ON sp.id = e.service_product_id
		WHERE e.id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) Update(ctx context.Context, id int64, category string, amount decimal.Decimal, desc *string, date time.Time, serviceID *int64) error {
	res, err := r.q.ExecContext(ctx, `
		UPDATE expenses SET category=$1, amount=$2, description=$3, expense_date=$4, service_product_id=$5
		WHERE id=$6`,
		category, amount, desc, date, serviceID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteReturning removes an expense and returns what it was, so a void can put
// exactly that amount back. sql.ErrNoRows when it is already gone (a second
// click waits on the row lock, then finds nothing — no double refund).
func (r *Repository) DeleteReturning(ctx context.Context, id int64) (*Expense, error) {
	var e Expense
	err := r.q.GetContext(ctx, &e, `DELETE FROM expenses WHERE id = $1 RETURNING *`, id)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Payment says where an expense's cash came from. Paid is true when a money
// move (CR- receipt) was booked for it; LockerID is the locker it left (0 when
// it left a till); From is the receipt's source label for display.
type Payment struct {
	Paid     bool
	LockerID int64
	From     string
}

// PaidFrom looks up the money move booked for an expense, over q (a tx or the db).
func PaidFrom(ctx context.Context, q db.Queryer, id int64) (Payment, error) {
	var p Payment
	err := q.GetContext(ctx, &p.From, `
		SELECT from_label FROM money_receipts
		WHERE ref_kind = 'expense' AND ref_id = $1 ORDER BY id LIMIT 1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, apperr.Internal("failed to load the expense payment", err)
	}
	p.Paid = true
	err = q.GetContext(ctx, &p.LockerID, `
		SELECT locker_id FROM locker_ledger
		WHERE ref_kind = 'expense' AND ref_id = $1 AND balance_delta < 0 ORDER BY id LIMIT 1`, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return p, apperr.Internal("failed to load the expense payment", err)
	}
	return p, nil
}

// TotalBetween sums expenses in [from, to]; used by the finance report.
func (r *Repository) TotalBetween(ctx context.Context, from, to time.Time) (decimal.Decimal, error) {
	var total decimal.Decimal
	err := r.q.GetContext(ctx, &total,
		`SELECT COALESCE(SUM(amount),0) FROM expenses WHERE expense_date >= $1 AND expense_date <= $2`,
		from, to)
	return total, err
}

type Service struct{ repo *Repository }

func NewService(db *sqlx.DB) *Service { return &Service{repo: NewRepository(db)} }

func (s *Service) List(ctx context.Context, f Filter) ([]Expense, error) {
	rows, err := s.repo.List(ctx, f)
	if err != nil {
		return nil, apperr.Internal("failed to list expenses", err)
	}
	return rows, nil
}

// parseCreate validates and normalizes a create form into its stored fields.
func parseCreate(in CreateInput, userID int64) (category string, amt decimal.Decimal, desc *string, paidBy *int64, date time.Time, err error) {
	amt, perr := money.Parse(in.Amount)
	if perr != nil || !amt.IsPositive() {
		return "", amt, nil, nil, date, apperr.Validation("amount must be greater than zero")
	}
	date = time.Now()
	if strings.TrimSpace(in.ExpenseDate) != "" {
		date, perr = time.Parse("2006-01-02", in.ExpenseDate)
		if perr != nil {
			return "", amt, nil, nil, date, apperr.Validation("date must be YYYY-MM-DD")
		}
	}
	if userID > 0 {
		paidBy = &userID
	}
	return strings.TrimSpace(in.Category), amt, in.Description, paidBy, date, nil
}

// serviceRef turns the form's 0-means-none into a nullable reference.
func serviceRef(id int64) *int64 {
	if id <= 0 {
		return nil
	}
	return &id
}

func (s *Service) Create(ctx context.Context, in CreateInput, userID int64) (*Expense, error) {
	category, amt, desc, paidBy, date, err := parseCreate(in, userID)
	if err != nil {
		return nil, err
	}
	e, err := s.repo.Create(ctx, category, amt, desc, paidBy, date, serviceRef(in.ServiceProductID))
	if err != nil {
		return nil, apperr.Internal("failed to record expense", err)
	}
	return e, nil
}

// CreateInTx records an expense over an existing transaction, so the caller can
// book the matching cash move (cashflow.MoveTx) atomically in the same tx.
func (s *Service) CreateInTx(ctx context.Context, q db.Queryer, in CreateInput, userID int64) (*Expense, error) {
	category, amt, desc, paidBy, date, err := parseCreate(in, userID)
	if err != nil {
		return nil, err
	}
	e, err := NewRepository(q).Create(ctx, category, amt, desc, paidBy, date, serviceRef(in.ServiceProductID))
	if err != nil {
		return nil, apperr.Internal("failed to record expense", err)
	}
	return e, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Expense, error) {
	e, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("expense")
		}
		return nil, apperr.Internal("failed to load expense", err)
	}
	return e, nil
}

func (s *Service) Update(ctx context.Context, id int64, in CreateInput) error {
	amt, err := money.Parse(in.Amount)
	if err != nil || !amt.IsPositive() {
		return apperr.Validation("amount must be greater than zero")
	}
	date := time.Now()
	if strings.TrimSpace(in.ExpenseDate) != "" {
		date, err = time.Parse("2006-01-02", in.ExpenseDate)
		if err != nil {
			return apperr.Validation("date must be YYYY-MM-DD")
		}
	}
	// Once cash has left a till or locker for it, the amount is what was paid:
	// changing it here would leave the books disagreeing with the money moved.
	cur, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if !amt.Equal(cur.Amount) {
		pay, err := PaidFrom(ctx, s.repo.q, id)
		if err != nil {
			return err
		}
		if pay.Paid {
			return apperr.Validation("this expense was paid from " + pay.From + ", so its amount can't change — void it and record it again")
		}
	}
	err = s.repo.Update(ctx, id, strings.TrimSpace(in.Category), amt, in.Description, date, serviceRef(in.ServiceProductID))
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("expense")
	}
	if err != nil {
		return apperr.Internal("failed to update expense", err)
	}
	return nil
}

// DefaultCategories is the built-in expense category list, in the canonical
// order the combo box offers them. Learned categories from the DB are appended
// after these (see MergedCategories).
func DefaultCategories() []string {
	return []string{"Rent", "Electricity", "Salary", "Transport", "Water", "Maintenance", "Supplies", "Repairs"}
}

// DistinctCategories returns every category already used on an expense, so the
// combo box can suggest them next time without a category-management screen.
func (r *Repository) DistinctCategories(ctx context.Context) ([]string, error) {
	var rows []string
	err := r.q.SelectContext(ctx, &rows,
		`SELECT DISTINCT category FROM expenses WHERE category <> '' ORDER BY category`)
	return rows, err
}

// MergedCategories combines the built-in defaults with categories already used,
// preserving the defaults' canonical order first, then appending any DB category
// not already present (compared case-insensitively on the trimmed value) in the
// order given. Blank/whitespace DB rows are ignored.
func MergedCategories(distinct []string) []string {
	defaults := DefaultCategories()
	seen := make(map[string]bool, len(defaults)+len(distinct))
	for _, d := range defaults {
		seen[strings.ToLower(strings.TrimSpace(d))] = true
	}
	out := append([]string(nil), defaults...)
	for _, c := range distinct {
		key := strings.ToLower(strings.TrimSpace(c))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, strings.TrimSpace(c))
	}
	return out
}
