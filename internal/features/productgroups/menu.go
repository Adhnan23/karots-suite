package productgroups

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"unicode/utf8"

	"karots-suite/internal/db"

	"github.com/jmoiron/sqlx"
)

// The cashier menu sheet (docs/rules/menu.md): the whole menu as lines, and a
// rebuild that makes the menu exactly what the lines say.

// MenuLine is one row of the sheet. Line is its number as the sheet shows it
// (the header is line 1); a line without Product is a group row.
type MenuLine struct {
	Line         int
	Group        string // the path, "Parent > Child"
	Emoji        string // the group's (group rows)
	Active       string // "no" = hidden; "" / anything else = shown
	Product      string
	Barcode      string
	ProductEmoji string
}

type Skipped struct {
	Line   int
	Reason string
}

// MenuResult is what a rebuild made, and how much of the old menu it removed
// (groups by path, items by path + product).
type MenuResult struct {
	Groups, Items               int
	RemovedGroups, RemovedItems int
	Skipped                     []Skipped
}

// FindProduct finds a line's product; a non-empty reason skips the line.
type FindProduct func(ctx context.Context, barcode, name string) (id int64, reason string, err error)

const (
	maxGroupName = 80 // product_groups.name
	maxEmoji     = 16 // product_groups.emoji, product_group_items.emoji
)

type menuItem struct {
	line MenuLine
	pid  int64
}

// MenuLines is the menu as the sheet writes it: depth-first in till order, each
// group's row then its items; hidden groups and disabled products included.
func (r *Repository) MenuLines(ctx context.Context) ([]MenuLine, error) {
	rows, err := menu(ctx, r.db)
	if err != nil {
		return nil, err
	}
	out := make([]MenuLine, len(rows))
	for i, m := range rows {
		out[i] = m.line
		out[i].Line = i + 2
	}
	return out, nil
}

func menu(ctx context.Context, q db.Queryer) ([]menuItem, error) {
	var groups []Group
	if err := q.SelectContext(ctx, &groups, groupSelect+` ORDER BY g.sort_order, g.id`); err != nil {
		return nil, err
	}
	var items []struct {
		GroupID int64          `db:"group_id"`
		PID     int64          `db:"product_id"`
		Name    string         `db:"name"`
		Barcode sql.NullString `db:"barcode"`
		Emoji   sql.NullString `db:"emoji"`
	}
	if err := q.SelectContext(ctx, &items, `
		SELECT i.group_id, p.id AS product_id, p.name, p.barcode, i.emoji
		FROM product_group_items i JOIN products p ON p.id = i.product_id
		ORDER BY i.sort_order, p.name, p.id`); err != nil {
		return nil, err
	}
	kids := map[int64][]Group{} // parent (0 = top) → children in order
	for _, g := range groups {
		var p int64
		if g.ParentID != nil {
			p = *g.ParentID
		}
		kids[p] = append(kids[p], g)
	}
	var out []menuItem
	var walk func(parent int64, path string)
	walk = func(parent int64, path string) {
		for _, g := range kids[parent] {
			p := g.Name
			if path != "" {
				p = path + " > " + g.Name
			}
			row := MenuLine{Group: p}
			if g.Emoji != nil {
				row.Emoji = *g.Emoji
			}
			if !g.IsActive {
				row.Active = "no"
			}
			out = append(out, menuItem{line: row})
			for _, it := range items {
				if it.GroupID == g.ID {
					out = append(out, menuItem{line: MenuLine{Group: p, Product: it.Name, Barcode: it.Barcode.String, ProductEmoji: it.Emoji.String}, pid: it.PID})
				}
			}
			walk(g.ID, p)
		}
	}
	walk(0, "")
	return out, nil
}

// Rebuild makes the menu exactly what lines say (MENU-2), inside tx: the caller
// commits, or rolls back for a preview (MENU-3) or on an error (MENU-4).
func Rebuild(ctx context.Context, tx *sqlx.Tx, lines []MenuLine, find FindProduct) (MenuResult, error) {
	var res MenuResult
	// Two imports at once would each delete only the rows they saw and leave both
	// menus behind: the second waits here for the first. Tills still read.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE product_groups IN EXCLUSIVE MODE`); err != nil {
		return res, err
	}
	old, err := menu(ctx, tx)
	if err != nil {
		return res, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM product_groups`); err != nil {
		return res, err
	}
	ids := map[string]int64{}    // path → group id
	order := map[int64]int{}     // parent (0 = top) → next group sort_order
	itemOrder := map[int64]int{} // group → next item sort_order
	have := map[[2]int64]bool{}  // group, product
	newItems := map[string]bool{}
	skip := func(l MenuLine, why string) { res.Skipped = append(res.Skipped, Skipped{l.Line, why}) }
	group := func(parts []string) (int64, error) {
		var parent int64
		path := ""
		for _, name := range parts {
			if path != "" {
				path += " > "
			}
			path += name
			if id, ok := ids[path]; ok {
				parent = id
				continue
			}
			var pid *int64
			if parent != 0 {
				pid = &parent
			}
			var id int64
			if err := tx.GetContext(ctx, &id, `INSERT INTO product_groups (name, parent_id, sort_order) VALUES ($1, $2, $3) RETURNING id`,
				name, pid, order[parent]); err != nil {
				return 0, err
			}
			order[parent]++
			ids[path], parent = id, id
			res.Groups++
		}
		return parent, nil
	}
	for _, l := range lines {
		if strings.TrimSpace(l.Group+l.Emoji+l.Active+l.Product+l.Barcode+l.ProductEmoji) == "" {
			continue // a blank line is no line
		}
		parts := splitPath(l.Group)
		product := l.Product != "" || l.Barcode != ""
		switch {
		case len(parts) == 0:
			skip(l, "no group")
			continue
		case longest(parts) > maxGroupName:
			skip(l, "a group name is longer than 80 characters")
			continue
		case utf8.RuneCountInString(l.ProductEmoji) > maxEmoji || !product && utf8.RuneCountInString(l.Emoji) > maxEmoji:
			skip(l, "the emoji is longer than 16 characters")
			continue
		}
		var pid int64
		if product {
			id, reason, err := find(ctx, l.Barcode, l.Product)
			if err != nil {
				return res, err
			}
			if reason != "" {
				skip(l, reason)
				continue
			}
			pid = id
		}
		gid, err := group(parts)
		if err != nil {
			return res, err
		}
		if !product {
			if _, err := tx.ExecContext(ctx, `UPDATE product_groups SET emoji = NULLIF($2, ''), is_active = $3 WHERE id = $1`,
				gid, l.Emoji, !hidden(l.Active)); err != nil {
				return res, err
			}
			continue
		}
		if have[[2]int64{gid, pid}] {
			continue // MENU-9: once per group, the first line's emoji
		}
		have[[2]int64{gid, pid}] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO product_group_items (group_id, product_id, emoji, sort_order) VALUES ($1, $2, NULLIF($3, ''), $4)`,
			gid, pid, l.ProductEmoji, itemOrder[gid]); err != nil {
			return res, err
		}
		itemOrder[gid]++
		res.Items++
		newItems[itemKey(strings.Join(parts, " > "), pid)] = true
	}
	for _, m := range old {
		path := strings.Join(splitPath(m.line.Group), " > ") // today's names read the way the sheet's are
		switch {
		case m.pid == 0 && ids[path] == 0:
			res.RemovedGroups++
		case m.pid != 0 && !newItems[itemKey(path, m.pid)]:
			res.RemovedItems++
		}
	}
	return res, nil
}

func itemKey(path string, pid int64) string { return path + "\x00" + strconv.FormatInt(pid, 10) }

// hidden reads Active the ways a person or an AI writes "off" (MENU-8).
func hidden(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "no", "n", "false", "0", "hidden", "off":
		return true
	}
	return false
}

// splitPath reads "A > B" (spaces around ">" don't count) into its names.
func splitPath(p string) []string {
	var out []string
	for _, s := range strings.Split(p, ">") {
		if s = strings.Join(strings.Fields(s), " "); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func longest(parts []string) int {
	n := 0
	for _, p := range parts {
		n = max(n, utf8.RuneCountInString(p))
	}
	return n
}
