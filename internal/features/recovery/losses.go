package recovery

import (
	"context"
	"time"

	"karots-suite/internal/apperr"

	"github.com/shopspring/decimal"
)

// Loss is one stock loss in a period — a damage write-off or a warranty
// replacement — with any supplier recovery recorded against it. SourceID is
// what Record takes (the movement for damage, the faulty unit for warranty —
// a by-receipt claim has one too, without a serial); nil only when the
// movement's claim can't be found.
type Loss struct {
	Kind       string          `db:"kind"` // damage | warranty
	SourceID   *int64          `db:"source_id"`
	MovementID int64           `db:"movement_id"`
	ProductID  int64           `db:"product_id"`
	Product    string          `db:"product"`
	Quantity   decimal.Decimal `db:"quantity"`
	LossValue  decimal.Decimal `db:"loss_value"`
	Note       *string         `db:"note"`
	By         string          `db:"by"`
	At         time.Time       `db:"created_at"`
	Outcome    *string         `db:"outcome"`
	Recovered  decimal.Decimal `db:"recovered"`
}

// LossesIn pages the damage and warranty losses booked in [from,to), newest
// first.
func (s *Service) LossesIn(ctx context.Context, from, to time.Time, limit, offset int) ([]Loss, error) {
	var rows []Loss
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT * FROM (
			SELECT 'damage' AS kind, m.id AS source_id, m.id AS movement_id, p.id AS product_id, p.name AS product, ABS(m.quantity) AS quantity,
			       m.cost AS loss_value, m.note, COALESCE(u.name, '') AS by, m.created_at,
			       lr.outcome, COALESCE(lr.recovered_value, 0) AS recovered
			FROM stock_movements m
			JOIN products p ON p.id = m.product_id
			LEFT JOIN users u ON u.id = m.user_id
			LEFT JOIN loss_recoveries lr ON lr.source_type = 'damage' AND lr.source_id = m.id
			WHERE m.type = 'damage' AND m.created_at >= $1 AND m.created_at < $2
		  UNION ALL
			SELECT 'warranty', wcl.unit_id, m.id, p.id, p.name, ABS(m.quantity),
			       m.cost, m.note, COALESCE(u.name, ''), m.created_at,
			       lr.outcome, COALESCE(lr.recovered_value, 0)
			FROM stock_movements m
			JOIN products p ON p.id = m.product_id
			LEFT JOIN users u ON u.id = m.user_id
			LEFT JOIN warranty_claims wcl ON m.reference_type = 'warranty' AND wcl.id = m.reference_id
			LEFT JOIN loss_recoveries lr ON lr.source_type = 'warranty' AND lr.source_id = wcl.unit_id
			WHERE m.type = 'warranty_replacement' AND m.created_at >= $1 AND m.created_at < $2
		) x
		ORDER BY created_at DESC, movement_id DESC
		LIMIT NULLIF($3, 0) OFFSET $4`, from, to, limit, offset); err != nil {
		return nil, apperr.Internal("failed to load losses", err)
	}
	return rows, nil
}
