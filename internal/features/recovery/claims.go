package recovery

import (
	"context"
	"time"

	"karots-suite/internal/apperr"

	"github.com/shopspring/decimal"
)

// lotSupplierSQL is the supplier a batch (alias b) belongs to: the one whose
// delivery it came on, else its product's (alias p) preferred supplier (also
// when b is NULL — a write-off that named no batch).
const lotSupplierSQL = `COALESCE((SELECT pu.supplier_id FROM purchase_items pi JOIN purchases pu ON pu.id = pi.purchase_id
	WHERE pi.id = b.purchase_item_id), p.preferred_supplier_id)`

// claimWindow is how far back an unsettled write-off stays on a supplier's
// list; older ones are still in Losses & Recovery.
// ponytail: fixed 180 days, make it a setting if a shop claims older stock.
const claimWindow = 180 * 24 * time.Hour

// ExpiredLot is an expired batch still on the shelf.
type ExpiredLot struct {
	LotID     int64           `db:"lot_id"`
	ProductID int64           `db:"product_id"`
	Product   string          `db:"product"`
	Unit      string          `db:"unit"`
	Qty       decimal.Decimal `db:"qty"`
	Expiry    time.Time       `db:"expiry"`
	Cost      decimal.Decimal `db:"cost"`
}

// Claims is what to settle with a supplier when they call: their expired
// batches still on the shelf, and damage write-offs of their stock that no
// recovery has been recorded against yet.
func (s *Service) Claims(ctx context.Context, supplierID int64) ([]ExpiredLot, []Loss, error) {
	var expired []ExpiredLot
	if err := s.db.SelectContext(ctx, &expired, `
		SELECT b.id AS lot_id, p.id AS product_id, p.name AS product, u.abbreviation AS unit,
		       b.qty_remaining AS qty, b.expiry_date AS expiry, b.cost_price AS cost
		FROM stock_batches b JOIN products p ON p.id = b.product_id JOIN units u ON u.id = p.unit_id
		WHERE b.qty_remaining > 0 AND b.expiry_date < CURRENT_DATE AND `+lotSupplierSQL+` = $1
		ORDER BY b.expiry_date, b.id`, supplierID); err != nil {
		return nil, nil, apperr.Internal("failed to load expired batches", err)
	}
	var waiting []Loss
	if err := s.db.SelectContext(ctx, &waiting, `
		SELECT 'damage' AS kind, m.id AS source_id, m.id AS movement_id, p.id AS product_id, p.name AS product,
		       ABS(m.quantity) AS quantity, m.cost AS loss_value, m.note, COALESCE(u.name, '') AS by, m.created_at,
		       NULL::text AS outcome, 0::numeric AS recovered
		FROM stock_movements m
		JOIN products p ON p.id = m.product_id
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN stock_batches b ON m.reference_type = 'batch' AND b.id = m.reference_id
		WHERE m.type = 'damage' AND m.created_at >= $2
		  AND NOT EXISTS (SELECT 1 FROM loss_recoveries lr WHERE lr.source_type = 'damage' AND lr.source_id = m.id)
		  AND `+lotSupplierSQL+` = $1
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 200`, supplierID, time.Now().Add(-claimWindow)); err != nil {
		return nil, nil, apperr.Internal("failed to load write-offs", err)
	}
	return expired, waiting, nil
}
