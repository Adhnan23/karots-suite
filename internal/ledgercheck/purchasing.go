package ledgercheck

// Area 5: purchasing — bills, goods sent back, supplier balances and payments
// (docs/rules/purchasing.md).
func init() {
	checks = append(checks,
		// SUP-1: balance = opening + bills + replacement charges + refunds received +
		// bounced / cancelled given cheques − debit notes − payments − credits for losses.
		check{"SUP-1", `
			SELECT 'supplier ' || s.id, format('balance %s, rows say %s (opening %s + bills %s + charges %s + refunds %s + cheques back %s − sent back %s − paid %s − credits %s)',
			       s.outstanding_balance, s.opening_balance + x.bills + x.charges + x.refunds + x.back - x.returned - x.paid - x.credits,
			       s.opening_balance, x.bills, x.charges, x.refunds, x.back, x.returned, x.paid, x.credits)
			FROM suppliers s
			CROSS JOIN LATERAL (SELECT
			    COALESCE((SELECT SUM(total) FROM purchases WHERE supplier_id = s.id AND status <> 'draft'), 0) AS bills,
			    COALESCE((SELECT SUM(extra_charge) FROM loss_recoveries WHERE supplier_id = s.id), 0) AS charges,
			    COALESCE((SELECT SUM(amount) FROM supplier_refunds WHERE supplier_id = s.id), 0) AS refunds,
			    COALESCE((SELECT SUM(amount) FROM cheques WHERE direction = 'out' AND supplier_id = s.id
			              AND status IN ('bounced', 'cancelled')), 0) AS back,
			    COALESCE((SELECT SUM(total) FROM purchase_returns WHERE supplier_id = s.id), 0) AS returned,
			    COALESCE((SELECT SUM(amount) FROM supplier_payments WHERE supplier_id = s.id), 0) AS paid,
			    COALESCE((SELECT SUM(recovered_value) FROM loss_recoveries WHERE supplier_id = s.id AND outcome = 'paid'), 0) AS credits) x
			WHERE s.outstanding_balance <> s.opening_balance + x.bills + x.charges + x.refunds + x.back - x.returned - x.paid - x.credits`},

		// SUP-2: the opening is never below zero; the old debt never passes it nor what's owed.
		check{"SUP-2", `
			SELECT 'supplier ' || id, format('opening %s, old debt %s, balance %s', opening_balance, opening_unlinked, outstanding_balance)
			FROM suppliers
			WHERE opening_balance < 0 OR opening_unlinked < 0 OR opening_unlinked > opening_balance
			   OR (opening_unlinked > 0 AND opening_unlinked > outstanding_balance)`},

		// PUR-1: total = subtotal − bill discount (0 ≤ discount ≤ subtotal); subtotal = Σ lines.
		check{"PUR-1", `
			SELECT 'purchase ' || pu.id, format('subtotal %s − discount %s ≠ total %s; lines %s', pu.subtotal, pu.discount, pu.total, COALESCE(l.v::text, 'none'))
			FROM purchases pu
			LEFT JOIN LATERAL (SELECT SUM(subtotal) AS v FROM purchase_items WHERE purchase_id = pu.id) l ON true
			WHERE pu.total <> pu.subtotal - pu.discount OR pu.discount < 0 OR pu.discount > pu.subtotal
			   OR (l.v IS NOT NULL AND l.v <> pu.subtotal)`},

		// PUR-10: a debit note's total = Σ its lines (units × cost, to the cent); its
		// credits to bills never pass it.
		check{"PUR-10", `
			SELECT 'purchase return ' || pr.id, format('total %s, lines %s, credited to bills %s', pr.total, COALESCE(l.v, 0), COALESCE(a.v, 0))
			FROM purchase_returns pr
			LEFT JOIN LATERAL (SELECT SUM(subtotal) AS v, bool_or(subtotal <> round(quantity * cost_price, 2)) AS off
			                   FROM purchase_return_items WHERE purchase_return_id = pr.id) l ON true
			LEFT JOIN LATERAL (SELECT SUM(amount) AS v FROM purchase_return_allocations WHERE purchase_return_id = pr.id) a ON true
			WHERE pr.total <> COALESCE(l.v, 0) OR l.off OR COALESCE(a.v, 0) > pr.total`},

		// PUR-11: paid and credited never below zero nor past the total; status says what's left.
		check{"PUR-11", `
			SELECT 'purchase ' || id, format('%s: total %s, paid %s, credited %s', status, total, paid_amount, credited_amount)
			FROM purchases
			WHERE status <> 'draft' AND (paid_amount < 0 OR credited_amount < 0 OR paid_amount + credited_amount > total
			   OR status::text <> CASE WHEN paid_amount + credited_amount >= total THEN 'paid'
			                           WHEN paid_amount > 0 THEN 'partial' ELSE 'received' END)`},

		// PUR-12: a bill's paid = the money put on it less what bounced cheques took back.
		check{"PUR-12", `
			SELECT 'purchase ' || pu.id, format('paid %s, money put on it %s', pu.paid_amount, COALESCE(a.v, 0))
			FROM purchases pu
			LEFT JOIN LATERAL (SELECT SUM(al.amount - al.reopened) AS v
			                   FROM supplier_payment_allocations al JOIN supplier_payments sp ON sp.id = al.payment_id
			                   WHERE al.purchase_id = pu.id AND sp.method <> 'discount') a ON true
			WHERE pu.status <> 'draft' AND pu.paid_amount <> COALESCE(a.v, 0)`},

		// PUR-13: a bill's lines landed — one lot per line, purchase movements = paid + free.
		check{"PUR-13", `
			SELECT 'purchase item ' || pi.id, 'no lot'
			FROM purchase_items pi JOIN purchases pu ON pu.id = pi.purchase_id
			WHERE pu.status <> 'draft' AND NOT EXISTS (SELECT 1 FROM stock_batches b WHERE b.purchase_item_id = pi.id)`},
		check{"PUR-13", `
			SELECT 'purchase ' || pu.id, format('lines %s units, purchase movements %s', l.v, COALESCE(m.v, 0))
			FROM purchases pu
			JOIN LATERAL (SELECT SUM(quantity + free_qty) AS v FROM purchase_items WHERE purchase_id = pu.id) l ON l.v IS NOT NULL
			LEFT JOIN LATERAL (SELECT SUM(quantity) AS v FROM stock_movements
			                   WHERE type = 'purchase' AND reference_type = 'purchase' AND reference_id = pu.id) m ON true
			WHERE pu.status <> 'draft' AND l.v <> COALESCE(m.v, 0)`},

		// SUP-3: a payment's money landed — its money receipts (when it moved money) are
		// its amount; a cheque payment's cheques add up to it.
		check{"SUP-3", `
			SELECT 'supplier payment ' || sp.id, format('%s %s, money receipts %s', sp.method, sp.amount, m.v)
			FROM supplier_payments sp
			JOIN LATERAL (SELECT SUM(amount) AS v FROM money_receipts
			              WHERE ref_kind = 'supplier_payment' AND ref_id = sp.id) m ON m.v IS NOT NULL
			WHERE sp.method IN ('cash', 'card', 'online', 'discount') AND m.v <> sp.amount`},
		check{"SUP-3", `
			SELECT 'supplier payment ' || sp.id, format('cheque payment %s, its cheques %s', sp.amount, COALESCE(q.v::text, 'none'))
			FROM supplier_payments sp
			LEFT JOIN LATERAL (SELECT SUM(amount) AS v FROM cheques WHERE supplier_payment_id = sp.id) q ON true
			WHERE sp.method = 'cheque' AND q.v IS DISTINCT FROM sp.amount`},

		// SUP-4: a payment's allocations never add up to more than it.
		check{"SUP-4", `
			SELECT 'supplier payment ' || sp.id, format('amount %s, put on bills %s', sp.amount, a.v)
			FROM supplier_payments sp
			JOIN LATERAL (SELECT SUM(amount) AS v FROM supplier_payment_allocations WHERE payment_id = sp.id) a ON a.v IS NOT NULL
			WHERE a.v > sp.amount`},
	)
}
