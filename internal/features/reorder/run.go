package reorder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

const (
	historyDays = 400 // sales and stock moves loaded: last year's window and then some
	batchSize   = 500
	runHour     = 3 // the nightly run, local time
	staleAfter  = 20 * time.Hour
)

// Loop runs the calculation a minute after boot when the last run is over 20 h
// old, then every night at 03:00 local, until ctx ends.
func Loop(ctx context.Context, db *sqlx.DB) {
	wait := time.Minute
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		var ran *time.Time
		_ = db.GetContext(ctx, &ran, `SELECT reorder_ran_at FROM settings WHERE id = 1`)
		if ran == nil || time.Since(*ran) > staleAfter {
			if n, err := Run(ctx, db, time.Now()); err != nil {
				log.Printf("reorder levels: %v", err)
			} else {
				log.Printf("reorder levels: %d products recalculated", n)
			}
		}
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), runHour, 0, 0, 0, now.Location())
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}
		wait = next.Sub(now)
	}
}

// Run recalculates every active stocked product, a batch at a time, and stamps
// settings.reorder_ran_at.
func Run(ctx context.Context, db *sqlx.DB, now time.Time) (int, error) {
	var ids []int64
	if err := db.SelectContext(ctx, &ids, `SELECT id FROM products WHERE is_active AND NOT is_service ORDER BY id`); err != nil {
		return 0, fmt.Errorf("list products: %w", err)
	}
	for from := 0; from < len(ids); from += batchSize {
		if err := RunIDs(ctx, db, now, ids[from:min(from+batchSize, len(ids))]); err != nil {
			return from, err
		}
	}
	_, err := db.ExecContext(ctx, `UPDATE settings SET reorder_ran_at = $1 WHERE id = 1`, now)
	return len(ids), err
}

// RunOne recalculates one product now (it was just switched to Auto).
func RunOne(ctx context.Context, db *sqlx.DB, id int64) error {
	return RunIDs(ctx, db, time.Now(), []int64{id})
}

type stamped struct {
	ProductID int64     `db:"product_id"`
	At        time.Time `db:"at"`
	Qty       float64   `db:"qty"`
}

// RunIDs writes reorder_calc for these products, and reorder_level for the Auto ones.
func RunIDs(ctx context.Context, db *sqlx.DB, now time.Time, ids []int64) error {
	var set struct {
		Default int `db:"reorder_default_days"`
		Safety  int `db:"reorder_safety_days"`
	}
	if err := db.GetContext(ctx, &set, `SELECT reorder_default_days, reorder_safety_days FROM settings WHERE id = 1`); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	var prods []struct {
		ID       int64     `db:"id"`
		Auto     bool      `db:"reorder_auto"`
		Start    int       `db:"reorder_start"`
		Created  time.Time `db:"created_at"`
		Supplier *int64    `db:"preferred_supplier_id"`
		OnHand   float64   `db:"on_hand"`
	}
	if err := db.SelectContext(ctx, &prods, `
		SELECT p.id, p.reorder_auto, p.reorder_start, p.created_at, p.preferred_supplier_id,
		       COALESCE(s.quantity, 0)::float8 AS on_hand
		FROM products p LEFT JOIN stock s ON s.product_id = p.id
		WHERE p.id = ANY($1)`, pq.Array(ids)); err != nil {
		return fmt.Errorf("load products: %w", err)
	}
	from := now.AddDate(0, 0, -historyDays)
	load := func(what, q string, args ...any) (map[int64][]stamped, error) {
		var rows []stamped
		if err := db.SelectContext(ctx, &rows, q, args...); err != nil {
			return nil, fmt.Errorf("load %s: %w", what, err)
		}
		m := map[int64][]stamped{}
		for _, r := range rows {
			m[r.ProductID] = append(m[r.ProductID], r)
		}
		return m, nil
	}
	sales, err := load("sales", `
		SELECT si.product_id, s.created_at AS at, SUM(si.quantity - si.returned_qty)::float8 AS qty
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		WHERE si.product_id = ANY($1) AND s.status <> 'void' AND s.created_at >= $2
		GROUP BY si.product_id, s.id, s.created_at`, pq.Array(ids), from)
	if err != nil {
		return err
	}
	// Sales of products made from these (loose rice from a bag), in the original's units.
	derived, err := load("derived sales", `
		SELECT pc.from_product_id AS product_id, s.created_at AS at,
		       (SUM(si.quantity - si.returned_qty) / pc.ratio)::float8 AS qty
		FROM product_conversions pc
		JOIN sale_items si ON si.product_id = pc.to_product_id
		JOIN sales s ON s.id = si.sale_id
		WHERE pc.is_active AND pc.from_product_id = ANY($1) AND s.status <> 'void' AND s.created_at >= $2
		GROUP BY pc.id, pc.from_product_id, pc.ratio, s.id, s.created_at`, pq.Array(ids), from)
	if err != nil {
		return err
	}
	moves, err := load("stock moves", `
		SELECT product_id, created_at AS at, quantity::float8 AS qty FROM stock_movements
		WHERE product_id = ANY($1) AND created_at >= $2`, pq.Array(ids), from)
	if err != nil {
		return err
	}
	since := now.AddDate(0, 0, -deliveryDays)
	own, err := load("deliveries", `
		SELECT product_id, created_at AS at, 0::float8 AS qty FROM stock_movements
		WHERE product_id = ANY($1) AND type = 'purchase' AND created_at >= $2`, pq.Array(ids), since)
	if err != nil {
		return err
	}
	var sups []int64
	for _, p := range prods {
		if p.Supplier != nil {
			sups = append(sups, *p.Supplier)
		}
	}
	// A supplier's deliveries of anything, keyed by supplier id.
	bySup, err := load("supplier deliveries", `
		SELECT pu.supplier_id AS product_id, sm.created_at AS at, 0::float8 AS qty
		FROM stock_movements sm JOIN purchases pu ON pu.id = sm.reference_id
		WHERE sm.type = 'purchase' AND sm.reference_type = 'purchase' AND pu.supplier_id = ANY($1) AND sm.created_at >= $2`,
		pq.Array(sups), since)
	if err != nil {
		return err
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, p := range prods {
		in := Input{OnHand: p.OnHand, DefaultDays: set.Default, SafetyDays: set.Safety, Since: p.Created}
		for _, s := range sales[p.ID] {
			in.Sales = append(in.Sales, Sale{At: s.At, Qty: s.Qty})
			in.Since = earliest(in.Since, s.At)
		}
		for _, s := range derived[p.ID] {
			in.Derived = append(in.Derived, Sale{At: s.At, Qty: s.Qty})
		}
		for _, m := range moves[p.ID] {
			in.Moves = append(in.Moves, Move{At: m.At, Qty: m.Qty})
			in.Since = earliest(in.Since, m.At)
		}
		for _, d := range own[p.ID] {
			in.OwnDeliveries = append(in.OwnDeliveries, d.At)
		}
		if p.Supplier != nil {
			for _, d := range bySup[*p.Supplier] {
				in.SupplierDeliveries = append(in.SupplierDeliveries, d.At)
			}
		}
		c := Compute(in, now)
		js, _ := json.Marshal(c)
		if _, err := tx.ExecContext(ctx, `
			UPDATE products SET reorder_calc = $2,
			       reorder_level = CASE WHEN reorder_auto THEN $3 ELSE reorder_level END
			WHERE id = $1`, p.ID, js, Blend(p.Start, c.CalcLevel, c.HistoryDays)); err != nil {
			return fmt.Errorf("save product %d: %w", p.ID, err)
		}
	}
	return tx.Commit()
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
