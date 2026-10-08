package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"

	"github.com/labstack/echo/v4"
)

// settingsSkip are this install's own settings, never taken from a sheet:
// install_id is the shop's identity (the support PIN derives from it), the
// printers and the default locker point at this machine's hardware and data.
var settingsSkip = map[string]bool{
	"install_id": true, "receipt_printer": true, "label_printer": true,
	"default_locker_id": true, "id": true, "updated_at": true,
}

func (h *handlers) settingsColumns(ctx context.Context) ([]string, error) {
	var cols []string
	err := h.d.DB.SelectContext(ctx, &cols, `
		SELECT column_name FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'settings'
		ORDER BY ordinal_position`)
	return cols, err
}

// AdminSettingsExport writes the settings row as key, value, importable.
func (h *handlers) AdminSettingsExport(c echo.Context) error {
	ctx := c.Request().Context()
	cols, err := h.settingsColumns(ctx)
	if err != nil {
		return err
	}
	sel := make([]string, len(cols))
	vals := make([]sql.NullString, len(cols))
	ptrs := make([]any, len(cols))
	for i, col := range cols {
		sel[i] = `"` + strings.ReplaceAll(col, `"`, `""`) + `"::text`
		ptrs[i] = &vals[i]
	}
	if err := h.d.DB.QueryRowContext(ctx, `SELECT `+strings.Join(sel, ", ")+` FROM settings ORDER BY id LIMIT 1`).Scan(ptrs...); err != nil {
		return err
	}
	rows := make([][]string, 0, len(cols))
	for i, col := range cols {
		flag := "yes"
		if settingsSkip[col] {
			flag = "no (this shop's own)"
		}
		rows = append(rows, []string{col, vals[i].String, flag})
	}
	return writeSheet(c, "settings", []string{"key", "value", "importable"}, rows)
}

// AdminSettingsImport applies a key / value sheet to the settings row in one
// UPDATE (a value that doesn't fit its column fails the whole import). Blank
// values mean "leave as is"; skipped and unknown keys are reported.
func (h *handlers) AdminSettingsImport(c echo.Context) error {
	ctx := c.Request().Context()
	col, recs, err := readImportSheet(c, nil, "key")
	if err != nil {
		return err
	}
	if _, ok := col["value"]; !ok {
		return apperr.BadRequest("the file must have a 'value' column")
	}
	cols, err := h.settingsColumns(ctx)
	if err != nil {
		return err
	}
	isCol := map[string]bool{}
	for _, c := range cols {
		isCol[c] = true
	}
	res := SettingsImportResult{Skipped: []string{}, Unknown: []string{}}
	var sets []string
	var args []any
	seen := map[string]bool{}
	for _, rec := range recs {
		get := cellGetter(col, rec)
		k, v := get("key"), get("value")
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		switch {
		case settingsSkip[k]:
			res.Skipped = append(res.Skipped, k)
		case !isCol[k]:
			res.Unknown = append(res.Unknown, k)
		case v != "":
			args = append(args, v)
			sets = append(sets, fmt.Sprintf(`"%s" = $%d`, strings.ReplaceAll(k, `"`, `""`), len(args)))
		}
	}
	sort.Strings(res.Skipped)
	sort.Strings(res.Unknown)
	if len(sets) == 0 {
		return apperr.Validation("nothing to import — no settings this shop can take in the file")
	}
	sets = append(sets, `"updated_at" = now()`)
	if _, err := h.d.DB.ExecContext(ctx, `UPDATE settings SET `+strings.Join(sets, ", "), args...); err != nil {
		return apperr.Validation("nothing was imported — a value doesn't fit its setting: " + err.Error())
	}
	res.Applied = len(args)
	h.audit(c, audit.ActionSettings, "settings", "", fmt.Sprintf("imported %d settings from a sheet", res.Applied))
	return c.JSON(http.StatusOK, res)
}
