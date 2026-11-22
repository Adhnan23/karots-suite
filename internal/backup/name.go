package backup

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var nameJunk = regexp.MustCompile(`[^a-z0-9]+`)

// FileName builds a descriptive backup filename: <shop-slug>-<yyyymmdd-hhmmss>,
// with a "-partial" marker for selective backups, and the .json.gz suffix. The
// slug is the shop name lower-cased with each run of non-alphanumerics collapsed
// to a single "-"; an empty/blank shop name falls back to "pos".
func FileName(shop string, now time.Time, partial bool) string {
	slug := strings.Trim(nameJunk.ReplaceAllString(strings.ToLower(shop), "-"), "-")
	if slug == "" {
		slug = "pos"
	}
	name := slug + "-" + now.Format("20060102-150405")
	if partial {
		name += "-partial"
	}
	return name + fileSuffix
}

// shopName reads the shop's display name for filename slugs; tolerant of a
// missing settings row (returns "").
func shopName(ctx context.Context, db sqlx.QueryerContext) string {
	var name sql.NullString
	var has bool // asked first: inside a snapshot a failed read would end it
	if err := sqlx.GetContext(ctx, db, &has, `SELECT to_regclass('settings') IS NOT NULL`); err != nil || !has {
		return ""
	}
	if err := sqlx.GetContext(ctx, db, &name, `SELECT shop_name FROM settings ORDER BY id LIMIT 1`); err != nil {
		return ""
	}
	return name.String
}
