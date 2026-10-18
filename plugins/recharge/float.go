package recharge

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// TxInput is one float row. FloatDelta is the signed effect on the device's
// float; the cash side is always core's (the sale, a money move), so cash_delta
// stays 0 on every suite row.
type TxInput struct {
	CarrierID      int64
	DeviceID       int64
	Type           string
	Amount         decimal.Decimal
	FloatDelta     decimal.Decimal
	SaleID         *int64
	SaleItemID     *int64
	ParentID       *int64
	PackageID      *int64
	CountID        *int64
	ExpenseID      *int64
	PurchaseID     *int64 // a refill's core supplier bill
	CustomerNumber string
	BillType       string
	Reference      string
	Note           string
	CreatedBy      int64
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func recordTx(ctx context.Context, q appdb.Queryer, in TxInput) (int64, error) {
	var id int64
	var by *int64
	if in.CreatedBy > 0 {
		by = &in.CreatedBy
	}
	err := q.GetContext(ctx, &id, `
		INSERT INTO recharge_transactions (carrier_id, device_id, type, amount, cash_delta, float_delta, sale_id, sale_item_id,
			parent_id, package_id, count_id, expense_id, customer_number, bill_type, reference, note, created_by, purchase_id)
		VALUES ($1,$2,$3,$4,0,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17) RETURNING id`,
		in.CarrierID, in.DeviceID, in.Type, in.Amount, in.FloatDelta, in.SaleID, in.SaleItemID, in.ParentID, in.PackageID,
		in.CountID, in.ExpenseID, nullStr(in.CustomerNumber), nullStr(in.BillType), nullStr(in.Reference), nullStr(in.Note), by, in.PurchaseID)
	return id, err
}

// recordWithCommission writes a move and, when the carrier pays on it, its
// commission row (+float) linked to it.
func recordWithCommission(ctx context.Context, q appdb.Queryer, in TxInput, commission decimal.Decimal) (int64, error) {
	id, err := recordTx(ctx, q, in)
	if err != nil || commission.IsZero() {
		return id, err
	}
	_, err = recordTx(ctx, q, TxInput{CarrierID: in.CarrierID, DeviceID: in.DeviceID, Type: "commission", Amount: commission.Abs(),
		FloatDelta: commission, SaleID: in.SaleID, SaleItemID: in.SaleItemID, ParentID: &id, CreatedBy: in.CreatedBy})
	return id, err
}

// balanceSQL is a device's float, correlated on d: its last karots-pos close
// (old recharge_device_sessions; the suite never writes them) plus every float
// row since — karots-pos's own "float now", so restored data reads as it did.
// Suite counts are float rows too ('count'), so they simply add on.
const balanceSQL = `(COALESCE(lc.closing, 0) + COALESCE((SELECT SUM(t.float_delta) FROM recharge_transactions t
	WHERE t.device_id = d.id AND (lc.closed_at IS NULL OR t.created_at > lc.closed_at)), 0))`

const lastCloseJoin = `LEFT JOIN LATERAL (SELECT closing, closed_at FROM recharge_device_sessions
	WHERE device_id = d.id AND closed_at IS NOT NULL ORDER BY closed_at DESC LIMIT 1) lc ON true`

// Float is a device with its balance now and its last count.
type Float struct {
	Device
	Balance   decimal.Decimal `db:"balance"    json:"balance"`
	LastCount *time.Time      `db:"last_count" json:"lastCount"`
	LastMove  *time.Time      `db:"last_move"  json:"lastMove"`
}

// Floats lists the float devices with their balances (active ones unless all).
func (s *Store) Floats(ctx context.Context, q appdb.Queryer, all bool) ([]Float, error) {
	out := []Float{}
	err := q.SelectContext(ctx, &out, `
		SELECT `+deviceCols+`, `+balanceSQL+` AS balance,
		       GREATEST(lc.closed_at, (SELECT MAX(created_at) FROM recharge_counts WHERE device_id = d.id)) AS last_count,
		       (SELECT MAX(created_at) FROM recharge_transactions WHERE device_id = d.id) AS last_move
		FROM recharge_devices d JOIN recharge_carriers c ON c.id = d.carrier_id `+lastCloseJoin+`
		WHERE d.tracks_float AND ((d.is_active AND c.is_active) OR $1)
		ORDER BY c.name, d.label`, all)
	return out, err
}

// balance is one device's float on q (a sale's tx sees its own earlier lines).
func balance(ctx context.Context, q appdb.Queryer, deviceID int64) (decimal.Decimal, error) {
	var b decimal.Decimal
	err := q.GetContext(ctx, &b, `SELECT `+balanceSQL+` FROM recharge_devices d `+lastCloseJoin+` WHERE d.id = $1`, deviceID)
	return b, err
}

// CountInput is one device's count: what the phone shows.
type CountInput struct {
	DeviceID int64           `json:"deviceId"`
	Counted  decimal.Decimal `json:"counted"`
}

// CountResult is what a count found: the difference booked (+ unexplained, − short).
type CountResult struct {
	DeviceID int64           `json:"deviceId"`
	Expected decimal.Decimal `json:"expected"`
	Counted  decimal.Decimal `json:"counted"`
	Diff     decimal.Decimal `json:"diff"`
}

// Count records counts in one tx: each writes a recharge_counts row and, when
// the phone and the suite disagree, a 'count' float row for the difference. The
// device row is locked so a sale can't slip between reading and booking.
func (s *Store) Count(ctx context.Context, counts []CountInput, sessionID *int64, by int64) ([]CountResult, error) {
	out := []CountResult{}
	err := appdb.WithTx(ctx, s.db, func(tx *sqlx.Tx) error {
		for _, c := range counts {
			if c.Counted.IsNegative() {
				return apperr.Validation("a count can't be below zero")
			}
			dv, err := s.Device(ctx, tx, c.DeviceID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT 1 FROM recharge_devices WHERE id = $1 FOR UPDATE`, dv.ID); err != nil {
				return err
			}
			exp, err := balance(ctx, tx, dv.ID)
			if err != nil {
				return err
			}
			var cid int64
			if err := tx.GetContext(ctx, &cid, `INSERT INTO recharge_counts (device_id, counted, expected, session_id, created_by)
				VALUES ($1, $2, $3, $4, $5) RETURNING id`, dv.ID, c.Counted, exp, sessionID, by); err != nil {
				return err
			}
			diff := c.Counted.Sub(exp)
			if !diff.IsZero() {
				if _, err := recordTx(ctx, tx, TxInput{CarrierID: dv.CarrierID, DeviceID: dv.ID, Type: "count", Amount: diff.Abs(),
					FloatDelta: diff, CountID: &cid, CreatedBy: by}); err != nil {
					return err
				}
			}
			out = append(out, CountResult{DeviceID: dv.ID, Expected: exp, Counted: c.Counted, Diff: diff})
		}
		return nil
	})
	return out, err
}
