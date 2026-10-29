package repairs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/features/products"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// Store writes only repair_*; it reads core products, customers, customer
// payments and expenses. Methods take a db.Queryer where they must run in a
// caller's transaction (a sale's tender settle, a payout).
type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

// Job is a repair_jobs row (karots-pos 0001 + 0002).
type Job struct {
	ID            int64           `db:"id"`
	TicketNo      string          `db:"ticket_no"`
	TicketCode    string          `db:"ticket_code"`
	CustomerID    *int64          `db:"customer_id"`
	CustomerName  string          `db:"customer_name"`
	CustomerPhone string          `db:"customer_phone"`
	RepairType    string          `db:"repair_type"`
	DeviceModel   string          `db:"device_model"`
	RepairedBy    string          `db:"repaired_by"`
	RepairerPaid  decimal.Decimal `db:"repairer_paid"` // karots-pos payouts (no expense link)
	Fault         string          `db:"fault"`
	Status        string          `db:"status"`
	PromisedDate  *time.Time      `db:"promised_date"`
	Urgent        bool            `db:"urgent"`
	WarrantyDays  int             `db:"warranty_days"`
	WarrantyUntil *time.Time      `db:"warranty_until"`
	SaleID        *int64          `db:"sale_id"`
	ReworkOf      *int64          `db:"rework_of"`
	Notes         string          `db:"notes"`
	CreatedBy     *int64          `db:"created_by"`
	CreatedAt     time.Time       `db:"created_at"`
	ReadyAt       *time.Time      `db:"ready_at"`
	CollectedAt   *time.Time      `db:"collected_at"`
	CancelledAt   *time.Time      `db:"cancelled_at"`
}

// Part is a product used on a job. UnitCharge / Discount* are karots-pos's
// frozen prices: the suite prices a part in the cart at pickup and only keeps
// an old row's discount to apply it there.
type Part struct {
	ID            int64           `db:"id"`
	JobID         int64           `db:"job_id"`
	ProductID     int64           `db:"product_id"`
	Qty           decimal.Decimal `db:"qty"`
	UnitCharge    decimal.Decimal `db:"unit_charge"`
	Discount      decimal.Decimal `db:"discount"`
	DiscountType  string          `db:"discount_type"`
	DiscountValue decimal.Decimal `db:"discount_value"`
	WarrantyDays  int             `db:"warranty_days"`
	ProductName   string          `db:"product_name"` // joined
	Price         decimal.Decimal `db:"price"`        // joined: today's selling price
	UnitCost      decimal.Decimal `db:"unit_cost"`    // joined: today's cost
}

type Charge struct {
	ID     int64           `db:"id"`
	JobID  int64           `db:"job_id"`
	Label  string          `db:"label"`
	Amount decimal.Decimal `db:"amount"`
}

// Payment is a karots-pos deposit ledger row: deposit | refund | applied.
type Payment struct {
	ID        int64           `db:"id"`
	JobID     int64           `db:"job_id"`
	Amount    decimal.Decimal `db:"amount"`
	Kind      string          `db:"kind"`
	UserID    *int64          `db:"user_id"`
	CreatedAt time.Time       `db:"created_at"`
}

// Deposit is a core customer payment taken for this job.
type Deposit struct {
	PaymentID int64           `db:"payment_id"`
	Amount    decimal.Decimal `db:"amount"`
	ReceiptNo string          `db:"receipt_no"`
	CreatedAt time.Time       `db:"created_at"`
}

// Payout is a core expense paying the repairer for this job (live: a voided
// expense is gone, so it drops out by itself).
type Payout struct {
	ExpenseID int64           `db:"expense_id"`
	Amount    decimal.Decimal `db:"amount"`
	Note      string          `db:"note"`
	Date      time.Time       `db:"expense_date"`
}

type Detail struct {
	Job      Job
	Parts    []Part
	Charges  []Charge
	Payments []Payment
	Deposits []Deposit
	Payouts  []Payout
}

type JobInput struct {
	CustomerID    *int64
	CustomerName  string
	CustomerPhone string
	RepairType    string
	DeviceModel   string
	RepairedBy    string
	Fault         string
	Notes         string
	PromisedDate  *time.Time
	WarrantyDays  int
	Urgent        bool
	ReworkOf      *int64
	CreatedBy     int64
}

// Config is the single repair_config row.
type Config struct {
	LabourProductID     int64  `db:"labour_product_id"`
	DefaultWarrantyDays int    `db:"default_warranty_days"`
	WarrantyMode        string `db:"warranty_mode"`
}

// ensureRow puts the config singleton back if a partial restore dropped it
// (karots-pos EnsureConfig self-heal).
func ensureRow(ctx context.Context, q appdb.Queryer) error {
	_, err := q.ExecContext(ctx, `INSERT INTO repair_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	return err
}

func (s *Store) Config(ctx context.Context) (Config, error) {
	var c Config
	if err := ensureRow(ctx, s.db); err != nil {
		return c, err
	}
	err := s.db.GetContext(ctx, &c, `SELECT labour_product_id, default_warranty_days, warranty_mode FROM repair_config WHERE id = 1`)
	return c, err
}

func (s *Store) SetConfig(ctx context.Context, mode string, days int) error {
	if mode != "days" && mode != "parts" {
		return apperr.Validation("warranty is by days or by part")
	}
	if days < 0 || days > 3650 {
		return apperr.Validation("warranty days must be 0 to 3650")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE repair_config SET warranty_mode = $1, default_warranty_days = $2 WHERE id = 1`, mode, days)
	return err
}

// LabourID is the service product labour lines ring on: the one config names,
// else a new "Repair labour" in a top-level "Repairs" category (made once, under
// the config row's lock). Never adopted by name — karots-pos picked up any
// "Repair Service" product, whatever its category.
func (s *Store) LabourID(ctx context.Context) (int64, error) {
	var id int64
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		if err := ensureRow(ctx, tx); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &id, `
			SELECT COALESCE(p.id, 0) FROM repair_config c
			LEFT JOIN products p ON p.id = c.labour_product_id AND p.is_service
			WHERE c.id = 1 FOR UPDATE OF c`); err != nil {
			return err
		}
		if id != 0 {
			return nil
		}
		var cat, unit int64
		err := tx.GetContext(ctx, &cat, `SELECT id FROM categories WHERE name = 'Repairs' AND parent_id IS NULL ORDER BY id LIMIT 1`)
		if errors.Is(err, sql.ErrNoRows) {
			err = tx.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ('Repairs') RETURNING id`)
		}
		if err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &unit, `SELECT id FROM units ORDER BY (abbreviation = 'pcs') DESC, id LIMIT 1`); err != nil {
			return apperr.Validation("add a unit first")
		}
		if id, err = products.CreateTx(ctx, tx, products.CreateInput{Name: "Repair labour", CategoryID: cat, UnitID: unit,
			CostPrice: "0", SellingPrice: "0", WholesalePrice: "0", TaxRate: "0", IsService: true}); err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE repair_config SET labour_product_id = $1 WHERE id = 1`, id)
		return err
	})
	return id, err
}

// nextTicket advances the shop's counter: R-0001 to read, RPR000001 to scan.
func nextTicket(ctx context.Context, q appdb.Queryer) (string, string, error) {
	if err := ensureRow(ctx, q); err != nil {
		return "", "", err
	}
	var seq int64
	if err := q.GetContext(ctx, &seq, `UPDATE repair_config SET ticket_seq = ticket_seq + 1 WHERE id = 1 RETURNING ticket_seq`); err != nil {
		return "", "", err
	}
	return fmt.Sprintf("R-%04d", seq), fmt.Sprintf("RPR%06d", seq), nil
}

func (s *Store) CreateJob(ctx context.Context, in JobInput) (int64, error) {
	var id int64
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		no, code, err := nextTicket(ctx, tx)
		if err != nil {
			return err
		}
		return tx.GetContext(ctx, &id, `
			INSERT INTO repair_jobs
				(ticket_no, ticket_code, customer_id, customer_name, customer_phone,
				 repair_type, device_model, repaired_by, fault, promised_date, urgent, warranty_days,
				 rework_of, notes, created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING id`,
			no, code, in.CustomerID, in.CustomerName, in.CustomerPhone,
			in.RepairType, in.DeviceModel, in.RepairedBy, in.Fault, in.PromisedDate, in.Urgent, in.WarrantyDays,
			in.ReworkOf, in.Notes, in.CreatedBy)
	})
	return id, err
}

func (s *Store) GetJob(ctx context.Context, id int64) (*Detail, error) {
	var j Job
	if err := s.db.GetContext(ctx, &j, `SELECT * FROM repair_jobs WHERE id = $1`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperr.NotFound("repair")
		}
		return nil, err
	}
	d := &Detail{Job: j}
	if err := s.db.SelectContext(ctx, &d.Parts, `
		SELECT rp.*, COALESCE(p.name, '') AS product_name, COALESCE(p.selling_price, 0) AS price,
		       COALESCE(p.cost_price, 0) AS unit_cost
		FROM repair_parts rp LEFT JOIN products p ON p.id = rp.product_id
		WHERE rp.job_id = $1 ORDER BY rp.id`, id); err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &d.Charges, `SELECT * FROM repair_charges WHERE job_id = $1 ORDER BY id`, id); err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &d.Payments, `SELECT * FROM repair_payments WHERE job_id = $1 ORDER BY id`, id); err != nil {
		return nil, err
	}
	if err := s.db.SelectContext(ctx, &d.Deposits, `
		SELECT cp.id AS payment_id, cp.amount, COALESCE(cp.receipt_no, '') AS receipt_no, cp.created_at
		FROM repair_deposits rd JOIN customer_payments cp ON cp.id = rd.payment_id
		WHERE rd.job_id = $1 ORDER BY cp.id`, id); err != nil {
		return nil, err
	}
	err := s.db.SelectContext(ctx, &d.Payouts, `
		SELECT e.id AS expense_id, e.amount - e.unpaid AS amount, COALESCE(e.description, '') AS note, e.expense_date
		FROM repair_payouts rp JOIN expenses e ON e.id = rp.expense_id
		WHERE rp.job_id = $1 ORDER BY e.id`, id)
	return d, err
}

func (s *Store) UpdateJob(ctx context.Context, id int64, in JobInput) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE repair_jobs SET
			customer_id = $2, customer_name = $3, customer_phone = $4,
			repair_type = $5, device_model = $6, repaired_by = $7, fault = $8,
			promised_date = $9, urgent = $10, warranty_days = $11, notes = $12
		WHERE id = $1`,
		id, in.CustomerID, in.CustomerName, in.CustomerPhone,
		in.RepairType, in.DeviceModel, in.RepairedBy, in.Fault,
		in.PromisedDate, in.Urgent, in.WarrantyDays, in.Notes)
	return err
}

func (s *Store) SetUrgent(ctx context.Context, id int64, urgent bool, promised *time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE repair_jobs SET urgent = $2, promised_date = COALESCE($3, promised_date) WHERE id = $1`, id, urgent, promised)
	return err
}

// SetStatus moves an open job and stamps ready_at / cancelled_at.
func (s *Store) SetStatus(ctx context.Context, id int64, status string) error {
	q := `UPDATE repair_jobs SET status = $2 WHERE id = $1`
	switch status {
	case "ready":
		q = `UPDATE repair_jobs SET status = $2, ready_at = now() WHERE id = $1`
	case "cancelled":
		q = `UPDATE repair_jobs SET status = $2, cancelled_at = now() WHERE id = $1`
	}
	_, err := s.db.ExecContext(ctx, q, id, status)
	return err
}

// MarkCollected links the pickup sale and stamps the warranty, once: a job
// already collected is left alone (false).
func (s *Store) MarkCollected(ctx context.Context, q appdb.Queryer, id, saleID int64, warrantyUntil *time.Time) (bool, error) {
	res, err := q.ExecContext(ctx, `
		UPDATE repair_jobs SET status = 'collected', sale_id = $2, warranty_until = $3, collected_at = now()
		WHERE id = $1 AND status NOT IN ('collected', 'cancelled')`, id, saleID, warrantyUntil)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

func (s *Store) AddPart(ctx context.Context, jobID, productID int64, qty decimal.Decimal, warrantyDays int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO repair_parts (job_id, product_id, qty, warranty_days) VALUES ($1, $2, $3, $4)`,
		jobID, productID, qty, warrantyDays)
	return err
}

func (s *Store) RemovePart(ctx context.Context, jobID, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM repair_parts WHERE id = $1 AND job_id = $2`, id, jobID)
	return err
}

func (s *Store) AddCharge(ctx context.Context, jobID int64, label string, amount decimal.Decimal) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO repair_charges (job_id, label, amount) VALUES ($1, $2, $3)`, jobID, label, amount)
	return err
}

func (s *Store) RemoveCharge(ctx context.Context, jobID, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM repair_charges WHERE id = $1 AND job_id = $2`, id, jobID)
	return err
}

func addPayment(ctx context.Context, q appdb.Queryer, jobID int64, amount decimal.Decimal, kind string, userID int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO repair_payments (job_id, amount, kind, user_id) VALUES ($1, $2, $3, $4)`,
		jobID, amount, kind, userID)
	return err
}

// legacyHeldTx is a karots-pos deposit still held for a job (deposits − refunds
// − applied), the job row locked so two settles can't both spend it.
func legacyHeldTx(ctx context.Context, tx *sqlx.Tx, jobID int64) (decimal.Decimal, error) {
	var held decimal.Decimal
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM repair_jobs WHERE id = $1 FOR UPDATE`, jobID); err != nil {
		return held, err
	}
	err := tx.GetContext(ctx, &held, `
		SELECT COALESCE(SUM(CASE WHEN kind = 'deposit' THEN amount ELSE -amount END), 0)
		FROM repair_payments WHERE job_id = $1`, jobID)
	return held, err
}

// LinkDeposit ties a core customer payment to a job — only one of the job's
// customer and not linked already.
func (s *Store) LinkDeposit(ctx context.Context, jobID, paymentID int64) error {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO repair_deposits (job_id, payment_id)
		SELECT j.id, cp.id FROM repair_jobs j JOIN customer_payments cp ON cp.customer_id = j.customer_id
		WHERE j.id = $1 AND cp.id = $2
		ON CONFLICT (payment_id) DO NOTHING`, jobID, paymentID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Validation("that payment isn't this customer's, or is already on a job")
	}
	return nil
}

func linkPayout(ctx context.Context, q appdb.Queryer, jobID, expenseID int64) error {
	_, err := q.ExecContext(ctx, `INSERT INTO repair_payouts (job_id, expense_id) VALUES ($1, $2)`, jobID, expenseID)
	return err
}
