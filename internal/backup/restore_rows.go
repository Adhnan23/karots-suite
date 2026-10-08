package backup

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// projectColumns keeps only the archive columns present in cols (this schema's
// columns for the table) and returns the names it dropped as "table.column".
func projectColumns(t Table, cols []string) (Table, []string) {
	have := make(map[string]bool, len(cols))
	for _, c := range cols {
		have[c] = true
	}
	var keep []int
	var dropped []string
	for i, c := range t.Columns {
		if have[c] {
			keep = append(keep, i)
		} else {
			dropped = append(dropped, t.Name+"."+c)
		}
	}
	if len(dropped) == 0 {
		return t, nil
	}
	out := Table{Name: t.Name, Columns: make([]string, len(keep)), Rows: make([][]*string, len(t.Rows))}
	for j, i := range keep {
		out.Columns[j] = t.Columns[i]
	}
	for r, row := range t.Rows {
		rec := make([]*string, len(keep))
		for j, i := range keep {
			if i < len(row) {
				rec[j] = row[i]
			}
		}
		out.Rows[r] = rec
	}
	return out, dropped
}

func insertRows(ctx context.Context, tx *sqlx.Tx, t Table) error {
	if len(t.Rows) == 0 {
		return nil
	}
	colList := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		colList[i] = quoteIdent(c)
	}
	prefix := `INSERT INTO ` + quoteIdent(t.Name) + ` (` + strings.Join(colList, ", ") + `) VALUES `

	// Chunk rows to keep statements (and the parameter count) reasonable.
	const chunk = 200
	for start := 0; start < len(t.Rows); start += chunk {
		end := min(start+chunk, len(t.Rows))
		batch := t.Rows[start:end]

		var b strings.Builder
		b.WriteString(prefix)
		args := make([]any, 0, len(batch)*len(t.Columns))
		p := 1
		for ri, row := range batch {
			if ri > 0 {
				b.WriteByte(',')
			}
			b.WriteByte('(')
			for ci := range t.Columns {
				if ci > 0 {
					b.WriteByte(',')
				}
				fmt.Fprintf(&b, "$%d", p)
				p++
				if ci < len(row) && row[ci] != nil {
					args = append(args, *row[ci])
				} else {
					args = append(args, nil)
				}
			}
			b.WriteByte(')')
		}
		if _, err := tx.ExecContext(ctx, b.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

// resetSequences advances each serial column's sequence past the largest value
// just loaded, so future inserts don't collide with restored ids.
func resetSequences(ctx context.Context, tx *sqlx.Tx, table string, cols []string) error {
	for _, c := range cols {
		var seq sql.NullString
		if err := tx.GetContext(ctx, &seq, `SELECT pg_get_serial_sequence($1, $2)`, table, c); err != nil {
			return err
		}
		if !seq.Valid || seq.String == "" {
			continue
		}
		_, err := tx.ExecContext(ctx, fmt.Sprintf(
			`SELECT setval('%s', GREATEST((SELECT COALESCE(MAX(%s), 0) FROM %s), 1), (SELECT COUNT(*) FROM %s) > 0)`,
			seq.String, quoteIdent(c), quoteIdent(table), quoteIdent(table)))
		if err != nil {
			return err
		}
	}
	return nil
}

// receiptSequences lists the standalone (non-column-owned) sequences that mint
// human-friendly receipt numbers, paired with the table/column those numbers
// live in. The numeric value is recovered by stripping the prefix (e.g. "CR-").
var receiptSequences = []struct{ Seq, Table, Col string }{
	{"sales_receipt_seq", "sales", "receipt_no"},            // S-00001
	{"money_receipt_seq", "money_receipts", "receipt_no"},   // CR-000001
	{"debt_receipt_seq", "customer_payments", "receipt_no"}, // DP-000001
}

// resetReceiptSequences advances each receipt-number sequence past the largest
// number already present, so future receipts don't collide with restored ones.
// It is tolerant of missing tables/sequences (older or plugin-trimmed schemas).
func resetReceiptSequences(ctx context.Context, tx *sqlx.Tx, exists map[string]bool) error {
	for _, rs := range receiptSequences {
		if !exists[rs.Table] {
			continue
		}
		var hasSeq bool
		if err := tx.GetContext(ctx, &hasSeq,
			`SELECT EXISTS (SELECT 1 FROM pg_class WHERE relkind = 'S' AND relname = $1)`, rs.Seq); err != nil {
			return err
		}
		if !hasSeq {
			continue
		}
		// max numeric suffix across the column (non-digits stripped); 0 when empty.
		_, err := tx.ExecContext(ctx, fmt.Sprintf(
			`SELECT setval('%s',
				GREATEST((SELECT COALESCE(MAX(NULLIF(regexp_replace(%s, '\D', '', 'g'), '')::bigint), 0) FROM %s), 1),
				(SELECT COUNT(*) FROM %s) > 0)`,
			rs.Seq, quoteIdent(rs.Col), quoteIdent(rs.Table), quoteIdent(rs.Table)))
		if err != nil {
			return err
		}
	}
	return nil
}
