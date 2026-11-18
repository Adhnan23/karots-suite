package ledgercheck

import "karots-suite/internal/features/sales"

// Area 1: sales, discounts, payment, returns (docs/rules/sales.md, returns.md).
func init() {
	checks = append(checks,
		// SALE-1: total = subtotal − discount + tax, and the lines add up to the subtotal
		// (each line's net + its item discount = its gross).
		check{"SALE-1", `
			SELECT 'sale ' || id, format('total %s, subtotal %s − discount %s + tax %s', total, subtotal, discount, tax)
			FROM sales WHERE total <> round(subtotal - discount + tax, 2)`},
		check{"SALE-1", `
			SELECT 'sale ' || s.id, format('lines add up to %s, subtotal %s', i.gross, s.subtotal)
			FROM sales s JOIN (SELECT sale_id, SUM(subtotal + discount) AS gross FROM sale_items GROUP BY sale_id) i
			  ON i.sale_id = s.id
			WHERE i.gross <> s.subtotal`},

		// TEND-9: the money rows add up to what the sale says was paid (on account is a debt, not money).
		check{"TEND-9", `
			SELECT 'sale ' || s.id, format('payments %s, paid %s', COALESCE(p.paid, 0), s.paid_amount)
			FROM sales s LEFT JOIN (SELECT sale_id, SUM(amount) AS paid FROM payments WHERE method <> 'credit' GROUP BY sale_id) p
			  ON p.sale_id = s.id
			WHERE COALESCE(p.paid, 0) <> s.paid_amount`},

		// TEND-1 / TEND-10: paid + on account − change covers the total; an account sale lands on it exactly.
		check{"TEND-1", `
			SELECT 'sale ' || s.id, format('paid %s + on account %s − change %s, total %s',
			       s.paid_amount, COALESCE(c.acct, 0), s.change_given, s.total)
			FROM sales s LEFT JOIN (SELECT sale_id, SUM(amount) AS acct FROM payments WHERE method = 'credit' GROUP BY sale_id) c
			  ON c.sale_id = s.id
			WHERE s.paid_amount + COALESCE(c.acct, 0) - s.change_given < s.total
			   OR (COALESCE(c.acct, 0) > 0 AND s.paid_amount + c.acct <> s.total)`},

		// RET-1: what came back ≤ what was sold, and = the return lines.
		check{"RET-1", `
			SELECT 'sale item ' || i.id, format('sold %s, returned %s, return lines %s', i.quantity, i.returned_qty, COALESCE(r.q, 0))
			FROM sale_items i LEFT JOIN (SELECT sale_item_id, SUM(quantity) AS q FROM sale_return_items GROUP BY sale_item_id) r
			  ON r.sale_item_id = i.id
			WHERE i.returned_qty > i.quantity OR i.returned_qty <> COALESCE(r.q, 0) OR i.returned_qty < 0`},

		// RET-2: the status follows what is still out.
		check{"RET-2", `
			SELECT 'sale ' || s.id, format('status %s, %s of %s returnable lines fully back, %s with some back',
			       s.status, x.full, x.lines, x.some)
			FROM sales s JOIN (
			  SELECT sale_id, COUNT(*) AS lines,
			         COUNT(*) FILTER (WHERE returned_qty >= quantity) AS full,
			         COUNT(*) FILTER (WHERE returned_qty > 0) AS some
			  FROM sale_items WHERE subtotal >= 0 GROUP BY sale_id) x ON x.sale_id = s.id
			WHERE (s.status = 'returned') <> (x.full = x.lines)
			   OR (s.status = 'partially_returned') <> (x.some > 0 AND x.full < x.lines)`},

		// RET-4: a line's refunds ≤ what was paid for it; fully back = exactly that.
		check{"RET-4", `
			SELECT 'sale item ' || i.id, format('refunded %s, paid %s, sold %s, returned %s', r.refunded, i.paid, i.quantity, i.returned_qty)
			FROM (SELECT i.*, round(i.subtotal * ` + sales.PaidFactorSQL + `, 2) AS paid
			      FROM sale_items i JOIN sales s ON s.id = i.sale_id) i
			JOIN (SELECT sale_item_id, SUM(refund_amount) AS refunded FROM sale_return_items GROUP BY sale_item_id) r
			  ON r.sale_item_id = i.id
			WHERE r.refunded > i.paid OR (i.returned_qty >= i.quantity AND r.refunded <> i.paid)`},

		// RET-5: a sale's refunds ≤ what its goods were paid; fully back = exactly that.
		check{"RET-5", `
			SELECT 'sale ' || s.id, format('refunded %s, goods paid %s', r.refunded, s.total - ` + sales.PayoutsSQL + `)
			FROM sales s JOIN (SELECT ri.sale_item_id, i.sale_id, ri.refund_amount FROM sale_return_items ri
			                   JOIN sale_items i ON i.id = ri.sale_item_id) x ON x.sale_id = s.id
			JOIN LATERAL (SELECT SUM(refund_amount) AS refunded FROM sale_return_items ri JOIN sale_items i ON i.id = ri.sale_item_id
			              WHERE i.sale_id = s.id) r ON true
			GROUP BY s.id, r.refunded
			HAVING r.refunded > s.total - ` + sales.PayoutsSQL + `
			    OR (s.status = 'returned' AND r.refunded <> s.total - ` + sales.PayoutsSQL + `)`},

		// RET-21: a return's refund + credit cleared = its lines.
		check{"RET-21", `
			SELECT 'return ' || r.id, format('refund %s + credit %s, lines %s', r.refund_amount, r.credit_reduction, COALESCE(l.v, 0))
			FROM sale_returns r LEFT JOIN (SELECT return_id, SUM(refund_amount) AS v FROM sale_return_items GROUP BY return_id) l
			  ON l.return_id = r.id
			WHERE r.refund_amount + r.credit_reduction <> COALESCE(l.v, 0)`},

		// SALE-20: on hand = the sum of the product's movements.
		check{"SALE-20", `
			SELECT 'product ' || st.product_id, format('on hand %s, movements %s', st.quantity, COALESCE(m.q, 0))
			FROM stock st LEFT JOIN (SELECT product_id, SUM(quantity) AS q FROM stock_movements GROUP BY product_id) m
			  ON m.product_id = st.product_id
			WHERE st.quantity <> COALESCE(m.q, 0)`},
	)
}
