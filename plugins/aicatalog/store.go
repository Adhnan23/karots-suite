package aicatalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

type Store struct{ db *sqlx.DB }

func NewStore(db *sqlx.DB) *Store { return &Store{db: db} }

type Settings struct {
	Provider     string `db:"provider"`
	BaseURL      string `db:"base_url"`
	Model        string `db:"model"`
	APIKey       string `db:"api_key"`
	ManualMode   bool   `db:"manual_mode"`
	CustomPrompt string `db:"custom_prompt"`
	WebSearch    bool   `db:"web_search"`
	NamingStyle  string `db:"naming_style"`
}

type LearnedItem struct {
	ProductID         int64  `db:"product_id"`
	ResolvedName      string `db:"resolved_name"`
	SuggestedCategory string `db:"suggested_category"`
	Specs             string `db:"specs"`
	UserExplanation   string `db:"user_explanation"`
	Source            string `db:"source"`
	RawQuery          string `db:"raw_query"`
}

func (s *Store) GetSettings(ctx context.Context) (Settings, error) {
	var out Settings
	err := s.db.GetContext(ctx, &out,
		`SELECT provider, base_url, model, api_key, manual_mode, custom_prompt, web_search, naming_style FROM aicatalog_settings WHERE id = 1`)
	return out, err
}

// SaveSettings writes the settings; a blank APIKey keeps the stored one (the key
// is typed once and never sent back to the browser).
func (s *Store) SaveSettings(ctx context.Context, in Settings) error {
	_, err := s.db.ExecContext(ctx, `UPDATE aicatalog_settings SET provider=$1, base_url=$2, model=$3,
		api_key = CASE WHEN $4 = '' THEN api_key ELSE $4 END,
		manual_mode=$5, custom_prompt=$6, web_search=$7, naming_style=$8 WHERE id = 1`,
		in.Provider, in.BaseURL, in.Model, strings.TrimSpace(in.APIKey), in.ManualMode, in.CustomPrompt, in.WebSearch, in.NamingStyle)
	return err
}

func (s *Store) LearnItem(ctx context.Context, it LearnedItem) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO aicatalog_items
		  (product_id, resolved_name, suggested_category, specs, user_explanation, source, raw_query, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7, now())
		ON CONFLICT (product_id) DO UPDATE SET
		  resolved_name=$2, suggested_category=$3, specs=$4,
		  user_explanation=$5, source=$6, raw_query=$7, updated_at=now()`,
		it.ProductID, it.ResolvedName, it.SuggestedCategory, it.Specs,
		it.UserExplanation, it.Source, it.RawQuery)
	return err
}

func (s *Store) GetItem(ctx context.Context, productID int64) (*LearnedItem, error) {
	var out LearnedItem
	err := s.db.GetContext(ctx, &out, `
		SELECT product_id, resolved_name, suggested_category, specs,
		       user_explanation, source, raw_query
		FROM aicatalog_items WHERE product_id = $1`, productID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// CatRow is one category with its parent and how many active products sit in it.
type CatRow struct {
	ID       int64  `db:"id"`
	Name     string `db:"name"`
	ParentID *int64 `db:"parent_id"`
	Count    int    `db:"cnt"`
}

func (s *Store) Categories(ctx context.Context) ([]CatRow, error) {
	var rows []CatRow
	err := s.db.SelectContext(ctx, &rows, `
		SELECT c.id, c.name, c.parent_id,
		       (SELECT count(*) FROM products p WHERE p.category_id = c.id AND p.is_active) AS cnt
		FROM categories c
		ORDER BY c.parent_id NULLS FIRST, c.name`)
	return rows, err
}

// catInfo is one category for the identify prompt: what sits in it directly.
type catInfo struct {
	ID       int64  `db:"id"`
	Name     string `db:"name"`
	ParentID *int64 `db:"parent_id"`
	Items    int    `db:"items"`    // active products filed directly here
	Children int    `db:"children"` // sub-categories
	Examples string `db:"examples"` // up to 3 of those products' names, newest first
}

// CategoryLines describes every category for identify, so the AI reuses one
// instead of inventing a near-duplicate, and can tell a category that holds items
// from a group whose items all sit in its sub-categories:
//
//	Bearings — 12 items, e.g. 6201-2Z Ball Bearing, 6305 ZZ; has sub-categories
//	Spare parts — group only: its items sit in its sub-categories
func (s *Store) CategoryLines(ctx context.Context) ([]string, error) {
	var rows []catInfo
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT c.id, c.name, c.parent_id,
		       (SELECT count(*) FROM products p WHERE p.category_id = c.id AND p.is_active) AS items,
		       (SELECT count(*) FROM categories k WHERE k.parent_id = c.id) AS children,
		       COALESCE((SELECT string_agg(left(x.name, 40), ', ') FROM (
		           SELECT p.name FROM products p WHERE p.category_id = c.id AND p.is_active
		           ORDER BY p.id DESC LIMIT 3) x), '') AS examples
		FROM categories c
		ORDER BY c.parent_id NULLS FIRST, c.name`); err != nil {
		return nil, err
	}
	paths := pathsByID(catRows(rows))
	// By path, so a parent sits right above its sub-categories.
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(paths[rows[i].ID]) < strings.ToLower(paths[rows[j].ID]) })
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, categoryLine(paths[r.ID], r))
		if len(lines) >= 200 { // ponytail: dozens typical; cap guards a huge tree
			break
		}
	}
	return lines, nil
}

func catRows(in []catInfo) []CatRow {
	out := make([]CatRow, len(in))
	for i, r := range in {
		out[i] = CatRow{ID: r.ID, Name: r.Name, ParentID: r.ParentID, Count: r.Items}
	}
	return out
}

func categoryLine(path string, r catInfo) string {
	sub := ""
	if r.Children > 0 {
		sub = "; has sub-categories"
	}
	switch {
	case r.Items > 0:
		return fmt.Sprintf("%s — %d item%s, e.g. %s%s", path, r.Items, plural(r.Items), r.Examples, sub)
	case r.Children > 0:
		return path + " — group only: its items sit in its sub-categories"
	default:
		return path + " — empty"
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
