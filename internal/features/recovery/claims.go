package recovery

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/stock"
)

// claimWindow is how far back an unsettled write-off stays on a supplier's
// list; older ones are still in Losses & Recovery.
// ponytail: fixed 180 days, make it a setting if a shop claims older stock.
const claimWindow = 180 * 24 * time.Hour

// Claims is what to settle with a supplier when they call: their lots past or
// inside their warning days and still on the shelf, and damage write-offs of
// their stock that no recovery has been recorded against yet. A write-off given
// back to them waits until settled; any other drops off after claimWindow.
func (s *Service) Claims(ctx context.Context, supplierID int64) (expired, expiring []stock.Batch, waiting []Loss, err error) {
	lots, err := stock.NewService(s.db).SupplierLots(ctx, supplierID)
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	for _, b := range lots {
		if b.Expired(now) {
			expired = append(expired, b)
		} else if b.ExpiringSoon(now) {
			expiring = append(expiring, b)
		}
	}
	if err := s.db.SelectContext(ctx, &waiting, `
		SELECT 'damage' AS kind, m.id AS source_id, m.id AS movement_id, p.id AS product_id, p.name AS product,
		       ABS(m.quantity) AS quantity, m.cost AS loss_value, m.note, COALESCE(u.name, '') AS by, m.created_at,
		       NULL::text AS outcome, 0::numeric AS recovered, m.given_back_to IS NOT NULL AS given_back
		FROM stock_movements m
		JOIN products p ON p.id = m.product_id
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN stock_batches b ON m.reference_type = 'batch' AND b.id = m.reference_id
		WHERE m.type = 'damage'
		  AND NOT EXISTS (SELECT 1 FROM loss_recoveries lr WHERE lr.source_type = 'damage' AND lr.source_id = m.id)
		  AND (m.given_back_to = $1 OR (m.given_back_to IS NULL AND m.created_at >= $2 AND `+stock.LotSupplierSQL+` = $1))
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 200`, supplierID, time.Now().Add(-claimWindow)); err != nil {
		return nil, nil, nil, apperr.Internal("failed to load write-offs", err)
	}
	return expired, expiring, waiting, nil
}
