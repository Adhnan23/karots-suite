// Package customers manages registered customers, especially credit ("vade")
// buyers. Walk-in sales leave customer_id NULL.
package customers

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

type Customer struct {
	ID                 int64           `db:"id"                  json:"id"`
	Name               string          `db:"name"                json:"name"`
	Phone              *string         `db:"phone"               json:"phone,omitempty"`
	Address            *string         `db:"address"             json:"address,omitempty"`
	CreditLimit        decimal.Decimal `db:"credit_limit"        json:"credit_limit"`
	OutstandingBalance decimal.Decimal `db:"outstanding_balance" json:"outstanding_balance"`
	OpeningBalance     decimal.Decimal `db:"opening_balance"     json:"opening_balance"`
	OpeningUnlinked    decimal.Decimal `db:"opening_unlinked"    json:"opening_unlinked"`
	LoyaltyPoints      int             `db:"loyalty_points"      json:"loyalty_points"`
	IsActive           bool            `db:"is_active"           json:"is_active"`
	CreatedAt          time.Time       `db:"created_at"          json:"created_at"`
}

// AvailableCredit is how much more this customer may borrow.
func (c Customer) AvailableCredit() decimal.Decimal {
	return c.CreditLimit.Sub(c.OutstandingBalance)
}

// LinkedBalance is the transactional (document-backed) part of what this customer
// owes: the outstanding balance minus the still-unpaid old (opening) debt. Read-only
// in the UI; only the opening part is editable.
func (c Customer) LinkedBalance() decimal.Decimal {
	return c.OutstandingBalance.Sub(c.OpeningUnlinked)
}

type CreateInput struct {
	Name        string  `json:"name"           form:"name"           validate:"required,min=2,max=100"`
	Phone       *string `json:"phone"          form:"phone"          validate:"required,min=4,max=15"`
	Address     *string `json:"address"        form:"address"`
	CreditLimit string  `json:"credit_limit"   form:"credit_limit"`
	// OpeningBalance is the amount this customer already owed at onboarding. It is
	// applied once, at creation, and ignored thereafter (Update never touches it).
	OpeningBalance string `json:"opening_balance" form:"opening_balance"`
}

type UpdateInput = CreateInput

type PaymentInput struct {
	Amount    string  `json:"amount"    form:"amount"    validate:"required"`
	Method    string  `json:"method"    form:"method"`
	Reference *string `json:"reference" form:"reference"`
	Note      *string `json:"note"      form:"note"`
	// ApplyToOpening routes the payment against the old (opening) debt instead of
	// the transactional balance. Admin-only; the cashier flow leaves it false.
	ApplyToOpening bool `json:"apply_to_opening" form:"apply_to_opening"`
	// AccountID is the money account a card/online repayment lands in (a card- or
	// bank-kind locker). Required for card/online (RecordPaymentTx refuses 0); the
	// api layer books the move into it.
	AccountID int64 `json:"account_id" form:"account_id"`
	// Plugin marks Method as a plugin tender the api layer checked
	// (plugin.TenderMethod.OnPayments); its SettleTx books where the money lands.
	Plugin bool `json:"-" form:"-"`
}

// CustomerPayment is one recorded credit repayment (the statement ledger).
type CustomerPayment struct {
	ID            int64            `db:"id"            json:"id"`
	CustomerID    int64            `db:"customer_id"   json:"customer_id"`
	Amount        decimal.Decimal  `db:"amount"        json:"amount"`
	Method        string           `db:"method"        json:"method"`
	Reference     *string          `db:"reference"     json:"reference,omitempty"`
	Note          *string          `db:"note"          json:"note,omitempty"`
	CreatedBy     *int64           `db:"created_by"    json:"created_by,omitempty"`
	CreatedAt     time.Time        `db:"created_at"    json:"created_at"`
	ReceiptNo     *string          `db:"receipt_no"    json:"receipt_no"`
	BalanceBefore *decimal.Decimal `db:"balance_before" json:"balance_before"`
	BalanceAfter  *decimal.Decimal `db:"balance_after"  json:"balance_after"`
}

type Service struct {
	db   *sqlx.DB
	repo *Repository
}

func NewService(db *sqlx.DB) *Service { return &Service{db: db, repo: NewRepository(db)} }

func (s *Service) List(ctx context.Context, search string) ([]Customer, error) {
	rows, err := s.repo.List(ctx, search)
	if err != nil {
		return nil, apperr.Internal("failed to list customers", err)
	}
	return rows, nil
}

// AllActive returns every active customer for the CSV export.
func (s *Service) AllActive(ctx context.Context) ([]Customer, error) {
	rows, err := s.repo.AllActive(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list customers", err)
	}
	return rows, nil
}

// Owing lists customers with an outstanding balance (for the dues report).
func (s *Service) Owing(ctx context.Context) ([]OwingRow, error) {
	rows, err := s.repo.Owing(ctx)
	if err != nil {
		return nil, apperr.Internal("failed to list customer dues", err)
	}
	return rows, nil
}

func (s *Service) Get(ctx context.Context, id int64) (*Customer, error) {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("customer")
		}
		return nil, apperr.Internal("failed to load customer", err)
	}
	return c, nil
}

// normalizePhone trims and strips spaces/dashes so "077-123 4567" and
// "0771234567" are treated as the same customer.
func normalizePhone(p string) string {
	r := strings.NewReplacer(" ", "", "-", "")
	return r.Replace(strings.TrimSpace(p))
}

// Create makes a customer, or — when the phone already belongs to an active
// customer — reuses that one instead of creating a duplicate. The returned bool
// is true when an existing customer was reused. Phone is required: it is the key
// that stops the counter from spawning duplicate credit accounts.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Customer, bool, error) {
	limit, err := money.Parse(in.CreditLimit)
	if err != nil || limit.IsNegative() {
		return nil, false, apperr.Validation("credit limit must be a non-negative amount")
	}
	opening, err := parseOpening(in.OpeningBalance)
	if err != nil {
		return nil, false, err
	}
	if in.Phone == nil || strings.TrimSpace(*in.Phone) == "" {
		return nil, false, apperr.Validation("a phone number is required")
	}
	phone := normalizePhone(*in.Phone)
	if existing, ferr := s.repo.FindByPhone(ctx, phone); ferr == nil {
		return existing, true, nil // reuse — do not create a duplicate
	} else if !errors.Is(ferr, sql.ErrNoRows) {
		return nil, false, apperr.Internal("failed to check for an existing customer", ferr)
	}
	c, err := s.repo.Create(ctx, strings.TrimSpace(in.Name), &phone, in.Address, limit, opening)
	if err != nil {
		return nil, false, apperr.Internal("failed to create customer", err)
	}
	return c, false, nil
}

// parseOpening parses an optional opening-balance string (blank → 0), rejecting
// negatives. A customer can't start with a negative receivable.
func parseOpening(s string) (decimal.Decimal, error) {
	if strings.TrimSpace(s) == "" {
		return decimal.Zero, nil
	}
	v, err := money.Parse(s)
	if err != nil || v.IsNegative() {
		return decimal.Zero, apperr.Validation("opening balance must be a non-negative amount")
	}
	return v, nil
}

// SetCreditLimit validates and applies a new credit limit (till inline edit).
func (s *Service) SetCreditLimit(ctx context.Context, id int64, limit string) error {
	v, err := money.Parse(limit)
	if err != nil || v.IsNegative() {
		return apperr.Validation("credit limit must be a non-negative amount")
	}
	err = s.repo.SetCreditLimit(ctx, id, v)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("customer")
	}
	if err != nil {
		return apperr.Internal("failed to update credit limit", err)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) error {
	limit, err := money.Parse(in.CreditLimit)
	if err != nil || limit.IsNegative() {
		return apperr.Validation("credit limit must be a non-negative amount")
	}
	err = s.repo.Update(ctx, id, strings.TrimSpace(in.Name), in.Phone, in.Address, limit)
	if errors.Is(err, sql.ErrNoRows) {
		return apperr.NotFound("customer")
	}
	if err != nil {
		return apperr.Internal("failed to update customer", err)
	}
	return nil
}

// AdjustOpening corrects the editable opening (old pre-system debt) for a
// customer. It returns the customer as it was before the change (for the audit
// trail) and the refreshed customer. The transactional (linked) part is never
// touched — only the opening and, by the same delta, the outstanding balance.
func (s *Service) AdjustOpening(ctx context.Context, id int64, newOpeningStr string) (before, after *Customer, err error) {
	// The opening (old pre-system balance) may be negative here: a customer who was
	// in credit before go-live (a pre-system advance/deposit we still owe them).
	newOpening, perr := money.Parse(newOpeningStr)
	if perr != nil {
		return nil, nil, apperr.Validation("opening balance must be a valid amount")
	}
	before, err = s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if aerr := s.repo.AdjustOpening(ctx, id, newOpening); aerr != nil {
		if errors.Is(aerr, sql.ErrNoRows) {
			return nil, nil, apperr.NotFound("customer")
		}
		return nil, nil, apperr.Internal("failed to adjust opening balance", aerr)
	}
	after, err = s.Get(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	return before, after, nil
}

// ImportRow is one resolved row of a bulk customer import (fields already parsed
// by the web handler). OpeningBalance is applied only when the row creates a new
// customer.
type ImportRow struct {
	Name           string
	Phone          string
	Address        string
	CreditLimit    decimal.Decimal
	OpeningBalance decimal.Decimal
	// Given names the columns the sheet fills for this row ("phone",
	// "address", "credit_limit"); an existing customer keeps the rest. nil = all.
	Given map[string]bool
}

// ImportResult reports what one import row did, for the summary.
type ImportResult struct {
	Action string // "created" | "updated"
	Note   string
}

// ImportOne upserts one customer in a transaction. It matches an existing active
// customer by phone (when given) and updates master fields; otherwise it creates
// one. The opening balance is applied on create only, so re-running an import
// never re-adds a balance.
func (s *Service) ImportOne(ctx context.Context, in ImportRow) (ImportResult, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ImportResult{}, apperr.Validation("name is required")
	}
	// Same phone key as Create ("077-123 4567" = "0771234567"), so a re-import
	// or a differently typed list never duplicates a customer.
	phone := nilIfBlank(normalizePhone(in.Phone))
	address := nilIfBlank(in.Address)
	var res ImportResult
	err := db.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		r := NewRepository(tx)
		var existing *Customer
		if phone != nil {
			if c, ferr := r.FindByPhone(ctx, *phone); ferr == nil {
				existing = c
			} else if !errors.Is(ferr, sql.ErrNoRows) {
				return ferr
			}
		} else if c, ferr := r.FindPhonelessByName(ctx, name); ferr == nil {
			// No phone to match on: the same name without a phone is the same
			// customer, so re-importing a phoneless list updates instead of doubling.
			existing = c
		} else if !errors.Is(ferr, sql.ErrNoRows) {
			return ferr
		}
		if existing != nil {
			limit := in.CreditLimit
			if in.Given != nil {
				if !in.Given["phone"] {
					phone = existing.Phone
				}
				if !in.Given["address"] {
					address = existing.Address
				}
				if !in.Given["credit_limit"] {
					limit = existing.CreditLimit
				}
			}
			if uerr := r.Update(ctx, existing.ID, name, phone, address, limit); uerr != nil {
				return uerr
			}
			res.Action = "updated"
			if in.OpeningBalance.IsPositive() {
				res.Note = "opening balance skipped (existing customer)"
			}
			return nil
		}
		if _, cerr := r.Create(ctx, name, phone, address, in.CreditLimit, in.OpeningBalance); cerr != nil {
			return cerr
		}
		res.Action = "created"
		return nil
	})
	if err != nil {
		return ImportResult{}, apperr.Internal("failed to import customer", err)
	}
	return res, nil
}

func nilIfBlank(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// ListAll returns active + inactive customers for the admin list.
func (s *Service) ListAll(ctx context.Context, search string) ([]Customer, error) {
	rows, err := s.repo.ListAll(ctx, search)
	if err != nil {
		return nil, apperr.Internal("failed to list customers", err)
	}
	return rows, nil
}

// Delete soft-deletes a customer (sets is_active=false). Sales/credit history is
// preserved; the customer just drops out of the active list and POS picker.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Deactivate(ctx, id); err != nil {
		return apperr.Internal("failed to remove customer", err)
	}
	return nil
}

// Reactivate restores a soft-deleted customer.
func (s *Service) Reactivate(ctx context.Context, id int64) error {
	if err := s.repo.Reactivate(ctx, id); err != nil {
		return apperr.Internal("failed to reactivate customer", err)
	}
	return nil
}
