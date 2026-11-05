package aicatalog

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"karots-suite/internal/apperr"
	appdb "karots-suite/internal/db"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
)

// A hint is what the chatbot said about a product: specs + explanation.
const (
	maxHint     = 2000
	hintLineLen = 80
	hintCaution = "AI suggestion, confirm before selling"
	hintHay     = `(specs || ' ' || user_explanation || ' ' || old_names || ' ' || raw_query)`
)

func capHint(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= maxHint {
		return s
	}
	return string([]rune(s)[:maxHint])
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// MatchHints finds products whose hint has every word of the query, in any order.
func (s *Store) MatchHints(ctx context.Context, q string) ([]int64, error) {
	words := strings.Fields(q)
	if len(words) == 0 {
		return nil, nil
	}
	pats := make([]string, len(words))
	for i, w := range words {
		pats[i] = "%" + likeEscaper.Replace(w) + "%"
	}
	var ids []int64
	err := s.db.SelectContext(ctx, &ids, `SELECT product_id FROM aicatalog_items WHERE `+hintHay+` ILIKE ALL($1) ORDER BY product_id LIMIT 500`, pq.Array(pats))
	return ids, err
}

// HintLines is the short line under a product in the till's results.
func (s *Store) HintLines(ctx context.Context, ids []int64) (map[int64]string, error) {
	var rows []struct {
		ID   int64  `db:"product_id"`
		Line string `db:"line"`
	}
	err := s.db.SelectContext(ctx, &rows, `SELECT product_id, COALESCE(NULLIF(specs, ''), user_explanation) AS line
		FROM aicatalog_items WHERE product_id = ANY($1) AND (specs <> '' OR user_explanation <> '')`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	out := make(map[int64]string, len(rows))
	for _, r := range rows {
		line := []rune(r.Line)
		out[r.ID] = string(line[:min(len(line), hintLineLen)])
	}
	return out, nil
}

// SaveHint is the owner's correction: it changes the hint and nothing else.
func (s *Store) SaveHint(ctx context.Context, productID int64, specs, explanation string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO aicatalog_items (product_id, resolved_name, specs, user_explanation, source)
		VALUES ($1, '', $2, $3, 'edited')
		ON CONFLICT (product_id) DO UPDATE SET specs = $2, user_explanation = $3, updated_at = now()`,
		productID, capHint(specs), capHint(explanation))
	return err
}

// AddOldName keeps a replaced name searchable, so a code staff know still finds it.
func (s *Store) AddOldName(ctx context.Context, q appdb.Queryer, productID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO aicatalog_items (product_id, resolved_name, source, old_names) VALUES ($1, '', 'edited', $2)
		ON CONFLICT (product_id) DO UPDATE SET
		  old_names = CASE WHEN position(lower($2) IN lower(aicatalog_items.old_names)) > 0 THEN aicatalog_items.old_names
		                   ELSE trim(aicatalog_items.old_names || ' ' || $2) END`, productID, name)
	return err
}

// HintBody is the product page's AI hint section.
type HintBody struct {
	Specs       string `json:"specs"`
	Explanation string `json:"explanation"`
}

func (h handlers) hintRoutes(a plugin.Group) {
	a.GET("/aicatalog/items/:id", h.getHint)
	a.PUT("/aicatalog/items/:id", h.putHint)
}

func itemID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid product id")
	}
	return id, nil
}

func (h handlers) getHint(c echo.Context) error {
	id, err := itemID(c)
	if err != nil {
		return err
	}
	it, err := h.p.store.GetItem(c.Request().Context(), id)
	if err != nil {
		return apperr.Internal("read hint", err)
	}
	if it == nil {
		return c.JSON(http.StatusOK, HintBody{})
	}
	return c.JSON(http.StatusOK, HintBody{Specs: it.Specs, Explanation: it.UserExplanation})
}

func (h handlers) putHint(c echo.Context) error {
	id, err := itemID(c)
	if err != nil {
		return err
	}
	var in HintBody
	if err := c.Bind(&in); err != nil {
		return apperr.BadRequest("invalid request")
	}
	if err := h.p.store.SaveHint(c.Request().Context(), id, in.Specs, in.Explanation); err != nil {
		var pe *pq.Error
		if errors.As(err, &pe) && pe.Code == "23503" { // no such product
			return apperr.NotFound("product")
		}
		return apperr.Internal("save hint", err)
	}
	return c.JSON(http.StatusOK, HintBody{Specs: capHint(in.Specs), Explanation: capHint(in.Explanation)})
}
