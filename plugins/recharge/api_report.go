package recharge

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/reports"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// LedgerRow is one float row (a move, its commission, a count, a refill, …).
type LedgerRow struct {
	ID         int64           `db:"id"          json:"id"`
	At         time.Time       `db:"created_at"  json:"at"`
	Carrier    string          `db:"carrier"     json:"carrier"`
	Device     string          `db:"device"      json:"device"`
	Type       string          `db:"type"        json:"type"`
	Amount     decimal.Decimal `db:"amount"      json:"amount"`
	FloatDelta decimal.Decimal `db:"float_delta" json:"floatDelta"`
	Detail     string          `db:"detail"      json:"detail"`
	Receipt    string          `db:"receipt"     json:"receipt"`
	By         string          `db:"by_name"     json:"by"`
}

func rangeOf(c echo.Context) (time.Time, time.Time, error) {
	from, to, _, _, err := reports.ResolveRange(c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return from, to, apperr.Validation(err.Error())
	}
	return from, to, nil
}

// ledger: every float row in a range, filtered by carrier / device / type / text;
// ?format=csv downloads the whole match.
func (h handlers) ledger(c echo.Context) error {
	from, to, err := rangeOf(c)
	if err != nil {
		return err
	}
	num := func(k string) int64 { v, _ := strconv.ParseInt(c.QueryParam(k), 10, 64); return v }
	limit, offset := int64(100), num("offset")
	csvOut := c.QueryParam("format") == "csv"
	if csvOut {
		limit, offset = 100000, 0
	}
	rows := []LedgerRow{}
	err = h.p.store.db.SelectContext(c.Request().Context(), &rows, `
		SELECT t.id, t.created_at, c.name AS carrier, COALESCE(d.label, '') AS device, t.type, t.amount, t.float_delta,
		       concat_ws(' · ', NULLIF(t.customer_number, ''), NULLIF(t.bill_type, ''), NULLIF(t.reference, ''), pk.name, NULLIF(t.note, '')) AS detail,
		       COALESCE(s.receipt_no, '') AS receipt, COALESCE(u.name, '') AS by_name
		FROM recharge_transactions t JOIN recharge_carriers c ON c.id = t.carrier_id
		LEFT JOIN recharge_devices d ON d.id = t.device_id LEFT JOIN recharge_packages pk ON pk.id = t.package_id
		LEFT JOIN sales s ON s.id = t.sale_id LEFT JOIN users u ON u.id = t.created_by
		WHERE t.created_at >= $1 AND t.created_at < $2
		  AND ($3 = 0 OR t.carrier_id = $3) AND ($4 = 0 OR t.device_id = $4) AND ($5 = '' OR t.type = $5)
		  AND ($6 = '' OR concat_ws(' ', t.customer_number, t.reference, t.note, t.bill_type, s.receipt_no) ILIKE '%'||$6||'%')
		ORDER BY t.created_at DESC, t.id DESC LIMIT $7 OFFSET $8`,
		from, to, num("carrier"), num("device"), c.QueryParam("type"), strings.TrimSpace(c.QueryParam("q")), limit, offset)
	if err != nil {
		return err
	}
	if !csvOut {
		return c.JSON(http.StatusOK, rows)
	}
	c.Response().Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="reload-float-ledger.csv"`)
	w := csv.NewWriter(c.Response())
	w.Write([]string{"when", "carrier", "device", "type", "amount", "float change", "detail", "receipt", "by"}) //nolint:errcheck
	for _, r := range rows {
		w.Write([]string{r.At.Format("2006-01-02 15:04"), r.Carrier, r.Device, r.Type, r.Amount.StringFixed(2), //nolint:errcheck
			r.FloatDelta.StringFixed(2), r.Detail, r.Receipt, r.By})
	}
	w.Flush()
	return w.Error()
}

// ReportLine is one carrier's moves of one kind in the range.
type ReportLine struct {
	Carrier    string          `db:"carrier"    json:"carrier"`
	Kind       string          `db:"kind"       json:"kind"`
	Count      int             `db:"n"          json:"count"`
	Amount     decimal.Decimal `db:"amount"     json:"amount"`
	Commission decimal.Decimal `db:"commission" json:"commission"`
}

// CountRow is a float count.
type CountRow struct {
	At       time.Time       `db:"at"       json:"at"`
	Device   string          `db:"device"   json:"device"`
	Expected decimal.Decimal `db:"expected" json:"expected"`
	Counted  decimal.Decimal `db:"counted"  json:"counted"`
	By       string          `db:"by_name"  json:"by"`
}

// Report is earnings by carrier × kind for a range, bank bills, and the counts.
type Report struct {
	From     string          `json:"from"`
	To       string          `json:"to"`
	Earnings decimal.Decimal `json:"earnings"`
	Lines    []ReportLine    `json:"lines"`
	Banks    []ReportLine    `json:"banks"` // bills paid from / cash outs into bank lockers
	Counts   []CountRow      `json:"counts"`
}

func (h handlers) report(c echo.Context) error {
	from, to, fromS, toS, err := reports.ResolveRange(c.QueryParam("preset"), c.QueryParam("from"), c.QueryParam("to"))
	if err != nil {
		return apperr.Validation(err.Error())
	}
	ctx := c.Request().Context()
	db := h.p.store.db
	out := Report{From: fromS, To: toS, Lines: []ReportLine{}, Banks: []ReportLine{}, Counts: []CountRow{}}
	if out.Earnings, err = Earnings(ctx, db, from, to); err != nil {
		return err
	}
	// A package is a reload with a package; its commission is its own row.
	if err := db.SelectContext(ctx, &out.Lines, `
		SELECT c.name AS carrier, CASE WHEN t.package_id IS NOT NULL THEN 'package' ELSE t.type END AS kind, count(*) AS n,
		       SUM(t.amount) AS amount,
		       COALESCE(SUM((SELECT SUM(x.float_delta) FROM recharge_transactions x WHERE x.parent_id = t.id AND x.type = 'commission')), 0) AS commission
		FROM recharge_transactions t JOIN recharge_carriers c ON c.id = t.carrier_id
		WHERE t.parent_id IS NULL AND t.type IN ('reload','billpay','deposit','cashout','withdrawal','wallet_in','refill','topup','count')
		  AND t.created_at >= $1 AND t.created_at < $2
		GROUP BY 1, 2 ORDER BY 1, 2`, from, to); err != nil {
		return err
	}
	if err := db.SelectContext(ctx, &out.Banks, `
		SELECT bank_name AS carrier, type AS kind, count(*) AS n, SUM(amount - returned) AS amount, 0 AS commission
		FROM recharge_bill_tx WHERE created_at >= $1 AND created_at < $2 GROUP BY 1, 2 ORDER BY 1, 2`, from, to); err != nil {
		return err
	}
	if err := db.SelectContext(ctx, &out.Counts, `
		SELECT * FROM (
			SELECT k.created_at AS at, c.name || ' · ' || d.label AS device, k.expected, k.counted, COALESCE(u.name, '') AS by_name
			FROM recharge_counts k JOIN recharge_devices d ON d.id = k.device_id JOIN recharge_carriers c ON c.id = d.carrier_id
			LEFT JOIN users u ON u.id = k.created_by
			UNION ALL
			SELECT ds.closed_at, c.name || ' · ' || d.label, ds.opening + COALESCE((SELECT SUM(float_delta) FROM recharge_transactions t
			         WHERE t.session_id = ds.session_id AND t.device_id = ds.device_id), 0), ds.closing, ''
			FROM recharge_device_sessions ds JOIN recharge_devices d ON d.id = ds.device_id JOIN recharge_carriers c ON c.id = d.carrier_id
			WHERE ds.closing IS NOT NULL
		) x WHERE at >= $1 AND at < $2 ORDER BY at DESC LIMIT 500`, from, to); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, out)
}
