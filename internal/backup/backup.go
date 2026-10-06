// Package backup implements a dependency-free logical backup/restore that runs
// entirely over the application's existing SQL connection — no pg_dump, no psql,
// no docker CLI. This matches the deployment model "ship a binary + a connection
// string": the Postgres it points at may live in a Docker container or on a
// remote VPS, and either way the binary can already talk to it.
//
// Only DATA is captured; the schema is owned by the embedded migrations (which
// run on startup), so a restore TRUNCATEs the existing tables and reloads rows.
// Every value is read and written as text (columns are cast with ::text on dump
// and bound as text parameters on restore), so Postgres' own input/output
// functions handle every type — numeric, timestamptz, jsonb, enums — exactly,
// driver-independently.
package backup

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jmoiron/sqlx"
)

// FormatVersion is bumped if the on-disk shape ever changes incompatibly.
const FormatVersion = 1

// skipTable reports tables never dumped or touched on restore. All goose
// migration bookkeeping — core (goose_db_version) AND per-plugin
// (goose_db_version_<name>) — is owned by the embedded migrations that run at
// startup. Restoring an older snapshot must NOT rewind that record, or the next
// boot would try to re-run already-applied migrations. Matching the whole
// goose_db_version* family keeps plugin bookkeeping safe too.
func skipTable(name string) bool {
	return strings.HasPrefix(name, "goose_db_version")
}

// RestoreMode controls which tables a restore clears.
type RestoreMode int

const (
	// RestoreReplaceAll clears every current non-goose table so the DB ends up
	// exactly matching the backup. This is what a FULL backup restore uses — a
	// table present in the DB but absent from the archive (e.g. a plugin the
	// source binary didn't have) is emptied rather than left with stale rows.
	RestoreReplaceAll RestoreMode = iota
	// RestoreMerge clears only the tables the archive actually carries, for
	// selective/partial backups, so untouched plugins/tables are left alone.
	RestoreMerge
)

// tablesToClear decides which current tables a restore truncates. `existing` is
// the current DB's public tables, `archive` the table names the backup carries.
// goose bookkeeping is always preserved; order follows `existing`.
func tablesToClear(existing, archive []string, mode RestoreMode) []string {
	inArchive := make(map[string]bool, len(archive))
	for _, t := range archive {
		inArchive[t] = true
	}
	out := make([]string, 0, len(existing))
	for _, t := range existing {
		if skipTable(t) {
			continue
		}
		if mode == RestoreMerge && !inArchive[t] {
			continue
		}
		out = append(out, t)
	}
	return out
}

type Table struct {
	Name    string      `json:"name"`
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
}

type Archive struct {
	Version     int      `json:"version"`
	GeneratedAt string   `json:"generated_at"`
	Partial     bool     `json:"partial,omitempty"` // true when some groups/tables were excluded
	Groups      []string `json:"groups,omitempty"`  // included plugin-group keys (partial dumps)
	Tables      []Table  `json:"tables"`
}

// DumpOpts selects what a dump includes. The zero value = a full backup.
type DumpOpts struct {
	Exclude []string            // table names to skip (from unticked plugin groups + setup-only deny-list)
	Partial bool                // record that this is a partial snapshot
	Groups  []string            // plugin-group keys included, recorded in the archive
	Zero    map[string][]string // table -> columns rewritten to "0" (e.g. balances in a setup-only export)
}

// Dump writes a gzipped JSON snapshot of public table data to w. opts narrows
// which tables are included (zero value dumps everything).
func Dump(ctx context.Context, db *sqlx.DB, generatedAt string, w io.Writer, opts DumpOpts) error {
	names, err := publicTables(ctx, db)
	if err != nil {
		return err
	}
	exclude := make(map[string]bool, len(opts.Exclude))
	for _, t := range opts.Exclude {
		exclude[t] = true
	}
	arc := Archive{Version: FormatVersion, GeneratedAt: generatedAt, Partial: opts.Partial, Groups: opts.Groups}
	for _, t := range names {
		if exclude[t] {
			continue
		}
		cols, err := tableColumns(ctx, db, t)
		if err != nil {
			return err
		}
		rows, err := dumpRows(ctx, db, t, cols)
		if err != nil {
			return fmt.Errorf("dump %s: %w", t, err)
		}
		if zc := opts.Zero[t]; len(zc) > 0 {
			zeroColumns(cols, rows, zc)
		}
		arc.Tables = append(arc.Tables, Table{Name: t, Columns: cols, Rows: rows})
	}
	gz := gzip.NewWriter(w)
	defer gz.Close()
	enc := json.NewEncoder(gz)
	return enc.Encode(arc)
}

// Restore reads a gzipped JSON snapshot from r and replaces the current data
// with it, all in one transaction. Foreign keys are dropped for the load and
// re-added (validated) before commit, so insertion order never matters.
func Restore(ctx context.Context, db *sqlx.DB, r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("not a valid backup file (gzip): %w", err)
	}
	defer gz.Close()
	var arc Archive
	if err := json.NewDecoder(gz).Decode(&arc); err != nil {
		return fmt.Errorf("not a valid backup file (json): %w", err)
	}
	if arc.Version != FormatVersion {
		return fmt.Errorf("unsupported backup version %d (expected %d)", arc.Version, FormatVersion)
	}

	// Which dumped tables actually exist in the current schema.
	existing, err := publicTables(ctx, db)
	if err != nil {
		return err
	}
	exists := map[string]bool{}
	for _, t := range existing {
		exists[t] = true
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Clear the tables we're about to reload. For a full restore (ReplaceAll)
	// this includes current tables absent from the archive — otherwise stale
	// rows (e.g. a plugin the source binary lacked) would survive the restore.
	archiveNames := make([]string, 0, len(arc.Tables))
	for _, t := range arc.Tables {
		archiveNames = append(archiveNames, t.Name)
	}
	// A full backup (Partial=false) replaces everything so stale tables absent
	// from the archive are cleared; a selective backup only touches its own.
	mode := RestoreReplaceAll
	if arc.Partial {
		mode = RestoreMerge
	}
	var toClear []string
	for _, name := range tablesToClear(existing, archiveNames, mode) {
		toClear = append(toClear, quoteIdent(name))
	}
	if len(toClear) > 0 {
		if _, err := tx.ExecContext(ctx, `TRUNCATE `+strings.Join(toClear, ", ")+` RESTART IDENTITY CASCADE`); err != nil {
			return fmt.Errorf("truncate failed: %w", err)
		}
	}

	// Drop the foreign keys and switch off user triggers (e.g. products →
	// auto-create stock row) for the load, restoring both before commit, so row
	// order never matters and triggers don't add rows the archive already has.
	// Done after TRUNCATE so CASCADE still clears referencing rows. This only
	// needs table ownership — installed shops run as the database owner, not a
	// superuser, so session_replication_role is not available to them.
	fks, err := dropForeignKeys(ctx, tx)
	if err != nil {
		return fmt.Errorf("drop foreign keys: %w", err)
	}
	var trigTables []string
	if err := tx.SelectContext(ctx, &trigTables, `
		SELECT DISTINCT tgrelid::regclass::text FROM pg_trigger t
		JOIN pg_class c ON c.oid = t.tgrelid
		WHERE NOT t.tgisinternal AND t.tgenabled <> 'D' AND c.relnamespace = 'public'::regnamespace`); err != nil {
		return err
	}
	for _, tbl := range trigTables {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+tbl+` DISABLE TRIGGER USER`); err != nil {
			return fmt.Errorf("disable triggers on %s: %w", tbl, err)
		}
	}

	for _, t := range arc.Tables {
		if !exists[t.Name] || skipTable(t.Name) {
			continue
		}
		if err := insertRows(ctx, tx, t); err != nil {
			return fmt.Errorf("restore %s: %w", t.Name, err)
		}
		if err := resetSequences(ctx, tx, t.Name, t.Columns); err != nil {
			return fmt.Errorf("reset sequences for %s: %w", t.Name, err)
		}
	}

	// Standalone receipt-number sequences (S-/CR-/DP-) are not owned by a column,
	// so resetSequences (which relies on pg_get_serial_sequence) cannot see them.
	// Advance them explicitly or the first post-restore receipt collides on the
	// unique receipt_no constraint.
	if err := resetReceiptSequences(ctx, tx, exists); err != nil {
		return fmt.Errorf("reset receipt sequences: %w", err)
	}

	for _, tbl := range trigTables {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+tbl+` ENABLE TRIGGER USER`); err != nil {
			return fmt.Errorf("enable triggers on %s: %w", tbl, err)
		}
	}
	// Re-adding validates every key, so a backup with dangling references fails
	// here (and rolls back) instead of leaving broken data behind.
	for _, fk := range fks {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+fk.Table+` ADD CONSTRAINT `+quoteIdent(fk.Name)+` `+fk.Def); err != nil {
			return fmt.Errorf("backup data breaks %s on %s: %w", fk.Name, fk.Table, err)
		}
	}

	return tx.Commit()
}

type foreignKey struct{ Table, Name, Def string }

// dropForeignKeys drops every FK in the public schema and returns their
// definitions so they can be re-added.
func dropForeignKeys(ctx context.Context, tx *sqlx.Tx) ([]foreignKey, error) {
	var fks []foreignKey
	err := tx.SelectContext(ctx, &fks, `
		SELECT conrelid::regclass::text AS table, conname AS name, pg_get_constraintdef(oid) AS def
		FROM pg_constraint
		WHERE contype = 'f' AND connamespace = 'public'::regnamespace
		ORDER BY conrelid::regclass::text, conname`)
	if err != nil {
		return nil, err
	}
	for _, fk := range fks {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE `+fk.Table+` DROP CONSTRAINT `+quoteIdent(fk.Name)); err != nil {
			return nil, err
		}
	}
	return fks, nil
}

// --- helpers ---

func publicTables(ctx context.Context, db *sqlx.DB) ([]string, error) {
	var all []string
	err := db.SelectContext(ctx, &all,
		`SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, t := range all {
		if !skipTable(t) {
			out = append(out, t)
		}
	}
	return out, nil
}

func tableColumns(ctx context.Context, db *sqlx.DB, table string) ([]string, error) {
	var cols []string
	err := db.SelectContext(ctx, &cols, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
		ORDER BY ordinal_position`, table)
	return cols, err
}

func dumpRows(ctx context.Context, db *sqlx.DB, table string, cols []string) ([][]*string, error) {
	sel := make([]string, len(cols))
	for i, c := range cols {
		sel[i] = quoteIdent(c) + "::text"
	}
	q := `SELECT ` + strings.Join(sel, ", ") + ` FROM ` + quoteIdent(table)
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out [][]*string
	for rows.Next() {
		raw := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := make([]*string, len(cols))
		for i := range raw {
			if raw[i].Valid {
				v := raw[i].String
				rec[i] = &v
			}
		}
		out = append(out, rec)
	}
	return out, rows.Err()
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

// quoteIdent safely double-quotes a SQL identifier coming from the catalog.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
