package aicatalog

import (
	"context"
	"database/sql"
	"errors"
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

// CategoryPaths returns every category as a "Parent > Child" path, so identify
// can have the AI reuse one instead of inventing a near-duplicate.
func (s *Store) CategoryPaths(ctx context.Context) ([]string, error) {
	rows, err := s.Categories(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]CatRow, len(rows))
	for _, r := range rows {
		byID[r.ID] = r
	}
	paths := make([]string, 0, len(rows))
	for _, r := range rows {
		parts := []string{r.Name}
		cur := r
		for i := 0; i < 20 && cur.ParentID != nil; i++ {
			p, ok := byID[*cur.ParentID]
			if !ok {
				break
			}
			parts = append([]string{p.Name}, parts...)
			cur = p
		}
		paths = append(paths, strings.Join(parts, " > "))
		if len(paths) >= 200 { // ponytail: dozens typical; cap guards a huge tree
			break
		}
	}
	return paths, nil
}
