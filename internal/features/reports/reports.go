// Package reports computes the shop's financials: a profit & loss summary plus
// supporting breakdowns over a date range.
package reports

import (
	"context"
	"time"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// PL is a profit & loss summary for [From, To).
type PL struct {
	From         time.Time       `json:"from"`
	To           time.Time       `json:"to"`
	SalesCount   int             `json:"sales_count"`
	GrossRevenue decimal.Decimal `json:"gross_revenue"` // sale totals before returns (excl. void)
	Returns      decimal.Decimal `json:"returns"`       // value of returned lines, by sale date
	Revenue      decimal.Decimal `json:"revenue"`       // net revenue = gross - returns
	Tax          decimal.Decimal `json:"tax"`           // tax charged on the period's sales — owed to the tax office, not revenue
	Received     decimal.Decimal `json:"received"`      // tender taken AT point of sale less change given back (excludes later debt collections — see Cashflow view for true cash in)
	COGS         decimal.Decimal `json:"cogs"`          // cost of goods sold, net of returns
	GrossProfit  decimal.Decimal `json:"gross_profit"`  // revenue - COGS
	GrossMargin  decimal.Decimal `json:"gross_margin"`  // gross profit / revenue, percent
	Expenses     decimal.Decimal `json:"expenses"`      // operating expenses in range
	Losses       decimal.Decimal `json:"losses"`        // damage + warranty replacement cost
	OwnUse       decimal.Decimal `json:"own_use"`       // stock the shop consumed itself
	StaffWelfare decimal.Decimal `json:"staff_welfare"` // stock taken by staff
	Recoveries   decimal.Decimal `json:"recoveries"`    // value reclaimed from suppliers
	OtherIncome  decimal.Decimal `json:"other_income"`  // bank interest credited to lockers in range
	// StockCorrections is the net value of stock adjustments: positive when a
	// count wrote stock OFF, negative when it found more than the books showed.
	// Shops that correct stock this way (rather than by stock-take) book all of
	// their shrinkage here, so it has to reach the profit line.
	StockCorrections decimal.Decimal `json:"stock_corrections"`
	// ZeroCostRevenue is revenue from stocked lines that were sold at no recorded
	// cost — a product captured without a cost price, whose opening lot is
	// therefore worth nothing. Those lines read as pure profit, so the gross
	// margin above is overstated by however much they really cost. Reported so
	// the number carries its own caveat instead of quietly flattering the shop.
	ZeroCostRevenue decimal.Decimal `json:"zero_cost_revenue"`
	ZeroCostLines   int             `json:"zero_cost_lines"`
	NetProfit       decimal.Decimal `json:"net_profit"`  // gross - expenses - losses - own use - staff welfare - stock corrections + recoveries + other income
	Receivables     decimal.Decimal `json:"receivables"` // customer dues (current snapshot; advances excluded)
	// CustomerAdvances is what the shop holds for customers who paid ahead (a
	// negative balance) — money the shop owes back, reported apart from dues.
	CustomerAdvances decimal.Decimal `json:"customer_advances"`
	Payables         decimal.Decimal `json:"payables"` // supplier dues (current snapshot; credits excluded)
	// SupplierCredit is the mirror of Payables: what suppliers owe US, from
	// returned goods or advances paid ahead of a delivery.
	SupplierCredit decimal.Decimal  `json:"supplier_credit"`
	CashWithdrawn  decimal.Decimal  `json:"cash_withdrawn"` // drawer withdrawals in range
	RegisterDiff   decimal.Decimal  `json:"register_diff"`  // net over/short of sessions closed in range
	TopProducts    []ProductRevenue `json:"top_products"`
	ByPayment      []MethodTotal    `json:"by_payment"`
}

type ProductRevenue struct {
	ProductID   int64           `db:"product_id"   json:"product_id"`
	ProductName string          `db:"product_name" json:"product_name"`
	Qty         decimal.Decimal `db:"qty"          json:"qty"`
	Revenue     decimal.Decimal `db:"revenue"      json:"revenue"`
	Profit      decimal.Decimal `db:"profit"       json:"profit"`
}

type MethodTotal struct {
	Method string          `db:"method" json:"method"`
	Count  int             `db:"count"  json:"count"`
	Total  decimal.Decimal `db:"total"  json:"total"`
}

// db is a Queryer (satisfied by *sqlx.DB and *sqlx.Tx) so reports are read-only
// and can run inside a rolled-back test transaction.
type Service struct{ db appdb.Queryer }

func NewService(db *sqlx.DB) *Service { return &Service{db: db} }

func (s *Service) Compute(ctx context.Context, from, to time.Time) (*PL, error) {
	pl := &PL{From: from, To: to}

	// Gross sales (every real sale at its original total, before tax — tax
	// collected is the government's, shown as its own line) and cash collected. A
	// return is accounted by sale date: it reduces this sale's period below via the
	// Returns figure and the net COGS, rather than dropping the whole sale.
	var head struct {
		Count int             `db:"count"`
		Gross decimal.Decimal `db:"gross"`
		Paid  decimal.Decimal `db:"paid"`
		Tax   decimal.Decimal `db:"tax"`
	}
	if err := s.db.GetContext(ctx, &head, `
		SELECT COUNT(*) AS count,
		       COALESCE(SUM(total - tax),0) AS gross,
		       COALESCE(SUM(tax),0) AS tax,
		       COALESCE(SUM(paid_amount - change_given),0) AS paid
		FROM sales
		WHERE status <> 'void' AND created_at >= $1 AND created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute revenue", err)
	}
	pl.SalesCount = head.Count
	pl.GrossRevenue = head.Gross
	pl.Received = head.Paid
	pl.Tax = head.Tax

	// Pass-through lines (resold airtime, bill face value) are money passing
	// through, not the shop's margin — subtract their face value from revenue. A
	// plugin flags such products; core only honors the flag.
	var passThrough decimal.Decimal
	if err := s.db.GetContext(ctx, &passThrough, `
		SELECT COALESCE(SUM(si.subtotal),0)
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute pass-through", err)
	}
	pl.GrossRevenue = pl.GrossRevenue.Sub(passThrough)

	// Returns: value of returned lines at what was paid for them (line net / qty
	// × the sale's paid factor, so a bill discount is shared out — the refund
	// actually given), for sales in this period. Gross is the discounted total,
	// so the line sticker price here would take back more than came in.
	if err := s.db.GetContext(ctx, &pl.Returns, `
		SELECT COALESCE(SUM( (si.subtotal / NULLIF(si.quantity,0)) * si.returned_qty * `+preTaxFactorSQL+` ),0)
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute returns", err)
	}
	pl.Revenue = pl.GrossRevenue.Sub(pl.Returns)

	// COGS net of returns — returned units are restocked, so their cost is excluded.
	if err := s.db.GetContext(ctx, &pl.COGS, `
		SELECT COALESCE(SUM( (si.quantity - si.returned_qty) * si.cost_price ),0)
		FROM sale_items si JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute COGS", err)
	}
	pl.GrossProfit = pl.Revenue.Sub(pl.COGS)
	pl.GrossMargin = grossMargin(pl.GrossProfit, pl.Revenue)

	if err := s.db.GetContext(ctx, &pl.Expenses, `
		SELECT COALESCE(SUM(amount),0) FROM expenses
		WHERE expense_date >= $1 AND expense_date < $2`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute expenses", err)
	}
	// Stock losses (damaged + warranty replacements) and any value reclaimed from
	// suppliers against them. These never touch revenue, so without these lines the
	// P&L would silently overstate profit by the cost of the lost goods.
	if err := s.db.GetContext(ctx, &pl.Losses, `
		SELECT COALESCE(SUM(cost),0) FROM stock_movements
		WHERE type IN ('damage','warranty_replacement') AND created_at >= $1 AND created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute losses", err)
	}
	// Stock the shop consumed itself, and stock taken by staff. Kept off the
	// Losses line on purpose: both are deliberate and expected, and folding them
	// into losses would make breakage look worse than it is while hiding how
	// much the shop actually consumes.
	if err := s.db.GetContext(ctx, &pl.OwnUse, `
		SELECT COALESCE(SUM(cost),0) FROM stock_movements
		WHERE type = 'own_use' AND created_at >= $1 AND created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute own-use cost", err)
	}
	if err := s.db.GetContext(ctx, &pl.StaffWelfare, `
		SELECT COALESCE(SUM(cost),0) FROM stock_movements
		WHERE type = 'staff' AND created_at >= $1 AND created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute staff welfare cost", err)
	}
	if err := s.db.GetContext(ctx, &pl.Recoveries, `
		SELECT COALESCE(SUM(recovered_value),0) FROM loss_recoveries
		WHERE created_at >= $1 AND created_at < $2`,
		from, to); err != nil {
		return nil, apperr.Internal("failed to compute recoveries", err)
	}

	// Other income: bank interest credited to a locker in the period (recorded via
	// the locker "Adjust → Interest" path as a locker_ledger row). Bank charges
	// already hit the P&L through expenses; interest is the symmetric income side,
	// read straight from the ledger (no separate income subsystem).
	_ = s.db.GetContext(ctx, &pl.OtherIncome, `
		SELECT COALESCE(SUM(balance_delta),0) FROM locker_ledger
		WHERE kind = 'interest' AND created_at >= $1 AND created_at < $2`, from, to)

	// Stock corrections: what counting the shelves added or took away, valued at
	// the cost of the lots involved. Kept off Losses because a correction is not
	// breakage — it is the count disagreeing with the books, and it swings both
	// ways. Positive here means value written OFF (quantity carries the sign, the
	// cost column is always the positive worth moved).
	_ = s.db.GetContext(ctx, &pl.StockCorrections, `
		SELECT COALESCE(SUM(CASE WHEN quantity < 0 THEN cost ELSE -cost END),0)
		FROM stock_movements
		WHERE type = 'adjust' AND created_at >= $1 AND created_at < $2`, from, to)

	// Stocked lines that cost nothing on the books. Services are excluded: they
	// legitimately have no stock cost.
	_ = s.db.GetContext(ctx, &pl.ZeroCostRevenue, `
		SELECT COALESCE(SUM(si.subtotal),0)
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE NOT p.is_service AND COALESCE(si.cost_price,0) = 0
		  AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`, from, to)
	_ = s.db.GetContext(ctx, &pl.ZeroCostLines, `
		SELECT COUNT(*)
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE NOT p.is_service AND COALESCE(si.cost_price,0) = 0
		  AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2`, from, to)

	_ = s.db.GetContext(ctx, &pl.RegisterDiff, `
		SELECT COALESCE(SUM(difference),0) FROM cash_register
		WHERE closed_at >= $1 AND closed_at < $2`, from, to)

	// The till drawer coming up short is money gone, the same as any other loss —
	// standard retail books it as "cash over/short". RegisterDiff is negative when
	// short, so adding it subtracts the shortage and credits an overage.
	pl.NetProfit = pl.GrossProfit.Sub(pl.Expenses).Sub(pl.Losses).
		Sub(pl.OwnUse).Sub(pl.StaffWelfare).Sub(pl.StockCorrections).
		Add(pl.Recoveries).Add(pl.OtherIncome).Add(pl.RegisterDiff)

	// Only customers who OWE count as receivables. One holding an advance has a
	// negative balance; summing raw netted it against everyone else's dues and
	// understated what the shop is owed. The advance is owed back — reported apart.
	_ = s.db.GetContext(ctx, &pl.Receivables,
		`SELECT COALESCE(SUM(GREATEST(outstanding_balance,0)),0) FROM customers`)
	_ = s.db.GetContext(ctx, &pl.CustomerAdvances,
		`SELECT COALESCE(SUM(GREATEST(-outstanding_balance,0)),0) FROM customers`)
	// Only suppliers we actually OWE count as payables. A supplier in credit (goods
	// returned, or an advance paid) has a negative balance, and summing it raw
	// netted that credit against everyone else's invoices — understating the debt.
	// The credit is a receivable, reported separately.
	_ = s.db.GetContext(ctx, &pl.Payables,
		`SELECT COALESCE(SUM(GREATEST(outstanding_balance,0)),0) FROM suppliers`)
	_ = s.db.GetContext(ctx, &pl.SupplierCredit,
		`SELECT COALESCE(SUM(GREATEST(-outstanding_balance,0)),0) FROM suppliers`)

	// Cash drawer: total withdrawn and the net over/short of sessions closed in
	// the period (negative = short).
	_ = s.db.GetContext(ctx, &pl.CashWithdrawn, `
		SELECT COALESCE(SUM(ABS(amount)),0) FROM cash_movements
		WHERE type = 'withdrawal' AND created_at >= $1 AND created_at < $2`, from, to)

	// Top products on net (kept) quantities and value.
	if err := s.db.SelectContext(ctx, &pl.TopProducts, `
		SELECT p.name AS product_name,
		       SUM(si.quantity - si.returned_qty) AS qty,
		       SUM( `+KeptRevenueSQL+` ) AS revenue,
		       SUM( `+KeptRevenueSQL+`
		            - (si.quantity - si.returned_qty) * si.cost_price ) AS profit
		FROM sale_items si
		JOIN sales s ON s.id = si.sale_id
		JOIN products p ON p.id = si.product_id
		WHERE NOT p.pass_through AND s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		GROUP BY p.name
		HAVING SUM(si.quantity - si.returned_qty) > 0
		ORDER BY revenue DESC
		LIMIT 10`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute top products", err)
	}

	if err := s.db.SelectContext(ctx, &pl.ByPayment, `
		-- Change always leaves as cash, so the cash share is net of it.
		-- ponytail: a sale with two cash payment rows would subtract its change twice;
		-- the till records one cash row per sale.
		SELECT pmt.method, COUNT(*) AS count,
		       COALESCE(SUM(pmt.amount),0) - COALESCE(SUM(s.change_given) FILTER (WHERE pmt.method = 'cash'),0) AS total
		FROM payments pmt JOIN sales s ON s.id = pmt.sale_id
		WHERE s.status <> 'void' AND s.created_at >= $1 AND s.created_at < $2
		GROUP BY pmt.method ORDER BY total DESC`, from, to); err != nil {
		return nil, apperr.Internal("failed to compute payment breakdown", err)
	}

	return pl, nil
}
