// Package palettes stores the custom palettes the system user makes on Appearance.
// The SPA builds and parses the CSS;
// the server only keeps it and serves the active one.
package palettes

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
)

// Theme is the only theme there is (Dock). Every custom palette belongs to it.
const Theme = "dock"

// MaxCSS caps one palette file (64 KB).
const MaxCSS = 64 << 10

type Palette struct {
	ID        int64     `db:"id"`
	Theme     string    `db:"theme"`
	Name      string    `db:"name"`
	Base      string    `db:"base"`
	CSS       string    `db:"css"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Input struct{ Name, Base, CSS string }

type Service struct{ db db.Queryer }

func New(q db.Queryer) *Service { return &Service{db: q} }

// SkinOf is the settings.skin value that turns palette id on.
func SkinOf(id int64) string { return "custom-" + strconv.FormatInt(id, 10) }

// IDOf reads a custom skin back; ok is false for a built-in or junk.
func IDOf(skin string) (int64, bool) {
	n, err := strconv.ParseInt(strings.TrimPrefix(skin, "custom-"), 10, 64)
	return n, strings.HasPrefix(skin, "custom-") && err == nil && n > 0
}

const cols = `id, theme, name, base, css, updated_at`

// List leaves css out (the picker doesn't need it).
func (s *Service) List(ctx context.Context) ([]Palette, error) {
	rows := []Palette{}
	err := s.db.SelectContext(ctx, &rows, `SELECT id, theme, name, base, '' AS css, updated_at FROM palettes WHERE theme = $1 ORDER BY lower(name)`, Theme)
	return rows, err
}

func (s *Service) Get(ctx context.Context, id int64) (Palette, error) {
	var p Palette
	err := s.db.GetContext(ctx, &p, `SELECT `+cols+` FROM palettes WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return p, apperr.NotFound("palette")
	}
	return p, err
}

func (s *Service) Create(ctx context.Context, in Input) (Palette, error) {
	if err := s.check(ctx, &in, 0); err != nil {
		return Palette{}, err
	}
	var p Palette
	err := s.db.GetContext(ctx, &p, `INSERT INTO palettes (theme, name, base, css) VALUES ($1, $2, $3, $4) RETURNING `+cols, Theme, in.Name, in.Base, in.CSS)
	return p, dupe(err)
}

func (s *Service) Update(ctx context.Context, id int64, in Input) (Palette, error) {
	if err := s.check(ctx, &in, id); err != nil {
		return Palette{}, err
	}
	var p Palette
	err := s.db.GetContext(ctx, &p, `UPDATE palettes SET name = $2, base = $3, css = $4, updated_at = now() WHERE id = $1 RETURNING `+cols, id, in.Name, in.Base, in.CSS)
	if errors.Is(err, sql.ErrNoRows) {
		return p, apperr.NotFound("palette")
	}
	return p, dupe(err)
}

// Delete refuses the palette the shop is using (activeSkin = settings.skin).
func (s *Service) Delete(ctx context.Context, id int64, activeSkin string) error {
	if activeSkin == SkinOf(id) {
		return apperr.Conflict("Choose another palette first")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM palettes WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("palette")
	}
	return nil
}

// ActiveCSS is the CSS to serve for skin: empty for a built-in or a palette that's gone.
func (s *Service) ActiveCSS(ctx context.Context, skin string) (string, error) {
	id, ok := IDOf(skin)
	if !ok {
		return "", nil
	}
	var css string
	err := s.db.GetContext(ctx, &css, `SELECT css FROM palettes WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return css, err
}

// check validates in and looks for a same-named palette first: a unique violation
// would abort the caller's transaction.
func (s *Service) check(ctx context.Context, in *Input, self int64) error {
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "" || len(in.Name) > 60:
		return apperr.Validation("Give the palette a name (60 characters at most)")
	case strings.TrimSpace(in.Base) == "":
		return apperr.Validation("Palette has no base")
	case len(in.CSS) > MaxCSS:
		return apperr.Validation("Palette is too big (64 KB max)")
	}
	var taken bool
	if err := s.db.GetContext(ctx, &taken, `SELECT EXISTS (SELECT 1 FROM palettes WHERE theme = $1 AND lower(name) = lower($2) AND id <> $3)`, Theme, in.Name, self); err != nil {
		return err
	}
	if taken {
		return apperr.Conflict("A palette with that name exists")
	}
	return nil
}

func dupe(err error) error {
	if db.IsUniqueViolation(err) {
		return apperr.Conflict("A palette with that name exists")
	}
	return err
}
