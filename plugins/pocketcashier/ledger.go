package pocketcashier

import "karots-suite/internal/ledgercheck"

// Pocket Cashier's own rules the data can show (docs/rules/plugins/pocketcashier.md).
func init() {
	// PKT-4: an applied sale action is the one sale carrying its ref.
	ledgercheck.Add("PKT-4", `
		SELECT 'phone action ' || a.ref, format('applied as sale %s, whose one-time ref is %s', a.sale_id, s.client_ref)
		FROM pocket_actions a LEFT JOIN sales s ON s.id = a.sale_id
		WHERE a.kind = 'sale' AND a.status = 'applied' AND s.client_ref IS DISTINCT FROM a.ref`)
	// PKT-7: never more refused-on-phone cash moves for a person than their handed, refused actions
	// (one still running may have booked its cash a moment before its record).
	ledgercheck.Add("PKT-7", `
		SELECT 'user ' || r.created_by, format('%s refused-on-phone cash moves for %s refused actions', r.n, COALESCE(a.n, 0))
		FROM (SELECT created_by, count(*) AS n FROM money_receipts WHERE kind = 'refused_on_phone' GROUP BY created_by) r
		LEFT JOIN (SELECT user_id, count(*) AS n FROM pocket_actions WHERE (status = 'failed' AND cash_settled) OR status = 'running' GROUP BY user_id) a
		  ON a.user_id = r.created_by
		WHERE r.n > COALESCE(a.n, 0)`)
	// PKT-12: a send's counts agree with its actions (a claimed one counts once it ends).
	ledgercheck.Add("PKT-12", `
		SELECT 'phone send ' || s.id, format('says %s actions, %s refused; has %s, %s', s.actions, s.failed, count(a.ref), count(a.ref) FILTER (WHERE a.status = 'failed'))
		FROM pocket_sends s LEFT JOIN pocket_actions a ON a.send_id = s.id AND a.status <> 'running'
		GROUP BY s.id HAVING s.actions <> count(a.ref) OR s.failed <> count(a.ref) FILTER (WHERE a.status = 'failed')`)
}
