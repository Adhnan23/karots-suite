package ledgercheck

// Area 2: the drawer, lockers, expenses (docs/rules/money.md).
func init() {
	checks = append(checks,
		// DRAW-1: a closed session's stored expected = opening + cash taken − change + its
		// drawer moves, worked out again from the rows; difference = counted − expected.
		check{"DRAW-1", `
			SELECT 'session ' || r.id, format('stored expected %s, rows say %s (opening %s + cash %s − change %s + moves %s); counted %s, difference %s',
			       r.expected_cash, r.opening_cash + x.cash - x.change + x.moves, r.opening_cash, x.cash, x.change, x.moves, r.closing_cash, r.difference)
			FROM cash_register r
			CROSS JOIN LATERAL (SELECT
			    COALESCE((SELECT SUM(p.amount) FROM payments p JOIN sales s ON s.id = p.sale_id
			              WHERE p.method = 'cash' AND s.session_id = r.id), 0) AS cash,
			    COALESCE((SELECT SUM(s.change_given) FROM sales s WHERE s.session_id = r.id), 0) AS change,
			    COALESCE((SELECT SUM(amount) FROM cash_movements
			              WHERE session_id = r.id AND type IN ('withdrawal','pay_in','credit_payment','refund','expense')), 0) AS moves) x
			WHERE r.closed_at IS NOT NULL
			  AND (r.expected_cash <> r.opening_cash + x.cash - x.change + x.moves OR r.difference <> r.closing_cash - r.expected_cash)`},

		// DRAW-6: a sale's till is its cashier's; one with no till yet is waiting for
		// the next open — never one rung before a till its cashier has since opened.
		check{"DRAW-6", `
			SELECT 'sale ' || s.id, format('cashier %s, till %s of %s', s.cashier_id, s.session_id, r.user_id)
			FROM sales s JOIN cash_register r ON r.id = s.session_id
			WHERE r.user_id <> s.cashier_id`},
		check{"DRAW-6", `
			SELECT 'sale ' || s.id, format('no till, rung %s; till %s opened %s after it, no close between', s.created_at, n.id, n.opened_at)
			FROM sales s
			JOIN LATERAL (SELECT id, opened_at FROM cash_register r
			              WHERE r.user_id = s.cashier_id AND r.opened_at > s.created_at ORDER BY r.opened_at LIMIT 1) n ON true
			WHERE s.session_id IS NULL
			  AND NOT EXISTS (SELECT 1 FROM cash_register c WHERE c.user_id = s.cashier_id
			                  AND c.closed_at > s.created_at AND c.closed_at <= n.opened_at)`},

		// DRAW-9: one opening record = the opening cash; a closed session has one closing record = what was counted.
		check{"DRAW-9", `
			SELECT 'session ' || r.id, format('opening %s, records %s; closing %s, records %s', r.opening_cash, o.v, r.closing_cash, c.v)
			FROM cash_register r
			LEFT JOIN LATERAL (SELECT array_agg(amount) AS v FROM cash_movements WHERE session_id = r.id AND type = 'opening') o ON true
			LEFT JOIN LATERAL (SELECT array_agg(amount) AS v FROM cash_movements WHERE session_id = r.id AND type = 'closing') c ON true
			WHERE o.v IS DISTINCT FROM ARRAY[r.opening_cash]
			   OR (r.closed_at IS NULL AND c.v IS NOT NULL)
			   OR (r.closed_at IS NOT NULL AND c.v IS DISTINCT FROM ARRAY[r.closing_cash])`},

		// LOCK-3 / DRAW-10: a locker row facing a till has its drawer twin in that session at the same
		// moment — opposite in sign, except the close (cash counted into the drawer record, banked into the locker).
		check{"LOCK-3", `
			SELECT 'locker row ' || ll.id, format('locker %s %s, session %s has no matching drawer record', ll.locker_id, ll.balance_delta, ll.counter_till_session)
			FROM locker_ledger ll
			WHERE ll.counter_till_session IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM cash_movements cm
			                  WHERE cm.session_id = ll.counter_till_session AND cm.created_at = ll.created_at
			                    AND ((cm.type = 'closing' AND cm.amount = ll.balance_delta)
			                      OR (cm.type <> 'closing' AND cm.amount = -ll.balance_delta)))`},

		// LOCK-2: a transfer between lockers is two rows, equal and opposite, each naming the other.
		check{"LOCK-2", `
			SELECT 'locker row ' || a.id, format('locker %s %s to / from locker %s has no other half', a.locker_id, a.balance_delta, a.counter_locker_id)
			FROM locker_ledger a
			WHERE a.counter_locker_id IS NOT NULL
			  AND NOT EXISTS (SELECT 1 FROM locker_ledger b
			                  WHERE b.locker_id = a.counter_locker_id AND b.counter_locker_id = a.locker_id
			                    AND b.created_at = a.created_at AND b.balance_delta = -a.balance_delta)`},

		// LOCK-6 / LOCK-7: a card-machine account fills only from takings (or its opening balance) and
		// empties only through Settle card — the fee as a bank charge, the rest to a bank.
		check{"LOCK-6", `
			SELECT 'locker row ' || ll.id, format('card account %s: %s %s (%s)', l.name, ll.kind, ll.balance_delta, COALESCE(ll.counterparty, ''))
			FROM locker_ledger ll JOIN lockers l ON l.id = ll.locker_id
			LEFT JOIN lockers o ON o.id = ll.counter_locker_id
			WHERE l.kind = 'card'
			  AND NOT (ll.balance_delta > 0 AND ll.kind IN ('payment','intake','open_balance') AND COALESCE(ll.counterparty, 'external') = 'external')
			  AND NOT (ll.balance_delta < 0 AND ll.kind = 'bank_charge')
			  AND NOT (ll.balance_delta < 0 AND ll.kind = 'transfer' AND o.kind = 'bank')`},

		// LOCK-8 / LOCK-7: a bank charge is an expense of the same amount (unless that expense was voided).
		check{"LOCK-8", `
			SELECT 'locker row ' || ll.id, format('bank charge %s, expense %s %s', ll.balance_delta, ll.ref_id, COALESCE(e.amount::text, 'missing'))
			FROM locker_ledger ll LEFT JOIN expenses e ON ll.ref_kind = 'expense' AND e.id = ll.ref_id
			WHERE ll.kind = 'bank_charge'
			  AND (ll.ref_kind IS DISTINCT FROM 'expense'
			       OR (e.id IS NOT NULL AND e.amount <> -ll.balance_delta)
			       OR (e.id IS NULL AND NOT EXISTS (SELECT 1 FROM money_receipts m
			                                        WHERE m.kind = 'expense_void' AND m.ref_kind = 'expense' AND m.ref_id = ll.ref_id)))`},

		// LOCK-12: a sale's card / online money is in the accounts picked, linked to the sale.
		check{"LOCK-12", `
			SELECT 'sale ' || p.sale_id, format('card / online %s, in accounts %s', p.v, COALESCE(a.v, 0))
			FROM (SELECT sale_id, SUM(amount) AS v FROM payments WHERE method IN ('card','online') GROUP BY sale_id) p
			LEFT JOIN (SELECT ref_id, SUM(balance_delta) AS v FROM locker_ledger
			           WHERE ref_kind = 'sale' AND kind = 'payment' GROUP BY ref_id) a ON a.ref_id = p.sale_id
			WHERE p.v <> COALESCE(a.v, 0)`},

		// EXP-1: an expense paid from a place (no cheques) moved amount − unpaid, no more, no less.
		check{"EXP-1", `
			SELECT 'expense ' || e.id, format('amount %s − unpaid %s, moved %s', e.amount, e.unpaid, m.v)
			FROM expenses e
			JOIN (SELECT ref_id, SUM(amount) AS v FROM money_receipts
			      WHERE ref_kind = 'expense' AND kind IN ('expense','bank_charge') GROUP BY ref_id) m ON m.ref_id = e.id
			WHERE NOT EXISTS (SELECT 1 FROM cheques c WHERE c.expense_id = e.id)
			  AND m.v <> e.amount - e.unpaid`},

		// EXP-2: a voided expense put back exactly what left for it.
		check{"EXP-2", `
			SELECT 'voided expense ' || v.ref_id, format('paid %s, put back %s', COALESCE(p.v, 0), v.v)
			FROM (SELECT ref_id, SUM(amount) AS v FROM money_receipts
			      WHERE ref_kind = 'expense' AND kind = 'expense_void' GROUP BY ref_id) v
			LEFT JOIN (SELECT ref_id, SUM(amount) AS v FROM money_receipts
			           WHERE ref_kind = 'expense' AND kind IN ('expense','bank_charge') GROUP BY ref_id) p ON p.ref_id = v.ref_id
			WHERE v.v <> COALESCE(p.v, 0)`},
	)
}
