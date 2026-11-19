package ledgercheck

// Area 3: customers' balances, payments and advances (docs/rules/customers.md).
func init() {
	checks = append(checks,
		// CUST-1: balance = opening + on account − kept change − returns cleared − payments
		// (advance refunds are negative payments) + bounced / cancelled cheques.
		check{"CUST-1", `
			SELECT 'customer ' || c.id, format('balance %s, rows say %s (opening %s + on account %s − kept %s − returns %s − paid %s + cheques back %s)',
			       c.outstanding_balance, c.opening_balance + x.acct - x.kept - x.cleared - x.paid + x.back,
			       c.opening_balance, x.acct, x.kept, x.cleared, x.paid, x.back)
			FROM customers c
			CROSS JOIN LATERAL (SELECT
			    COALESCE((SELECT SUM(GREATEST(total - paid_amount, 0)) FROM sales WHERE customer_id = c.id), 0) AS acct,
			    COALESCE((SELECT SUM(rounding_to_account) FROM sales WHERE customer_id = c.id), 0) AS kept,
			    COALESCE((SELECT SUM(r.credit_reduction) FROM sale_returns r JOIN sales s ON s.id = r.sale_id
			              WHERE s.customer_id = c.id), 0) AS cleared,
			    COALESCE((SELECT SUM(amount) FROM customer_payments WHERE customer_id = c.id), 0) AS paid,
			    COALESCE((SELECT SUM(amount) FROM cheques WHERE direction = 'in' AND customer_id = c.id
			              AND status IN ('bounced', 'cancelled')), 0) AS back) x
			WHERE c.outstanding_balance <> c.opening_balance + x.acct - x.kept - x.cleared - x.paid + x.back`},

		// CUST-3: one active customer per phone, spaces and dashes ignored.
		check{"CUST-3", `
			SELECT 'phone ' || p, format('active customers %s', ids)
			FROM (SELECT replace(replace(trim(phone), ' ', ''), '-', '') AS p, string_agg(id::text, ', ' ORDER BY id) AS ids
			      FROM customers WHERE is_active AND COALESCE(trim(phone), '') <> ''
			      GROUP BY 1 HAVING count(*) > 1) d`},

		// CUST-5: a payment's money landed — cash / card / online and an advance refund have their
		// money receipt of the same amount; a cheque payment's cheques add up to it.
		check{"CUST-5", `
			SELECT 'customer payment ' || cp.id, format('%s %s, money receipts %s', cp.method, cp.amount, COALESCE(m.v::text, 'none'))
			FROM customer_payments cp
			LEFT JOIN LATERAL (SELECT SUM(amount) AS v FROM money_receipts
			                   WHERE ref_kind = 'customer_payment' AND ref_id = cp.id) m ON true
			WHERE cp.method IN ('cash', 'card', 'online', 'refund') AND m.v IS DISTINCT FROM abs(cp.amount)`},
		check{"CUST-5", `
			SELECT 'customer payment ' || cp.id, format('cheque payment %s, its cheques %s', cp.amount, COALESCE(q.v::text, 'none'))
			FROM customer_payments cp
			LEFT JOIN LATERAL (SELECT SUM(amount) AS v FROM cheques WHERE customer_payment_id = cp.id) q ON true
			WHERE cp.method = 'cheque' AND q.v IS DISTINCT FROM cp.amount`},

		// CUST-6: a slip's balance after = before − amount.
		check{"CUST-6", `
			SELECT 'customer payment ' || id, format('before %s − %s ≠ after %s', balance_before, amount, balance_after)
			FROM customer_payments
			WHERE balance_before IS NOT NULL AND balance_after IS NOT NULL AND balance_after <> balance_before - amount`},

		// CUST-9: the old debt still owed never passes the opening.
		check{"CUST-9", `
			SELECT 'customer ' || id, format('old debt %s, opening %s', opening_unlinked, opening_balance)
			FROM customers WHERE opening_unlinked > opening_balance`},

		// CUST-15: the old debt is never more than what they owe.
		check{"CUST-15", `
			SELECT 'customer ' || id, format('old debt %s, balance %s', opening_unlinked, outstanding_balance)
			FROM customers WHERE opening_unlinked > 0 AND opening_unlinked > outstanding_balance`},
	)
}
