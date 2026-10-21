package palettes

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"karots-suite/internal/apperr"
	"karots-suite/internal/db"
)

func code(err error) string {
	var ae *apperr.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}

func TestPalettes(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	tx, err := conn.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	s := New(tx)

	p, err := s.Create(ctx, Input{Name: "TEST Sea", Base: "lagoon", CSS: ":root{--brand:#123456}"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, p.ID)
	if err != nil || got.CSS != ":root{--brand:#123456}" || got.Theme != "dock" || got.Base != "lagoon" {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	if _, err := s.Create(ctx, Input{Name: "test sea", Base: "studio", CSS: "x"}); code(err) != "CONFLICT" {
		t.Fatalf("duplicate name (other case): %v", err)
	}
	if _, err := s.Create(ctx, Input{Name: "  ", Base: "studio", CSS: "x"}); code(err) != "VALIDATION_ERROR" {
		t.Fatalf("blank name: %v", err)
	}
	if _, err := s.Create(ctx, Input{Name: "TEST Big", Base: "studio", CSS: strings.Repeat("a", MaxCSS+1)}); code(err) != "VALIDATION_ERROR" {
		t.Fatalf("too big: %v", err)
	}
	if _, err := s.Update(ctx, p.ID, Input{Name: "TEST Sea 2", Base: "lagoon", CSS: "y"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, 999999999, Input{Name: "TEST Gone", Base: "studio", CSS: "y"}); code(err) != "NOT_FOUND" {
		t.Fatalf("update missing: %v", err)
	}

	skin := SkinOf(p.ID)
	if skin != "custom-"+strconv.FormatInt(p.ID, 10) {
		t.Fatalf("skin %q", skin)
	}
	if css, _ := s.ActiveCSS(ctx, skin); css != "y" {
		t.Fatalf("active css %q", css)
	}
	for _, sk := range []string{"studio", "custom-999999999", "custom-x"} {
		if css, err := s.ActiveCSS(ctx, sk); css != "" || err != nil {
			t.Fatalf("ActiveCSS(%q) = %q %v", sk, css, err)
		}
	}
	if err := s.Delete(ctx, p.ID, skin); code(err) != "CONFLICT" {
		t.Fatalf("deleted the active palette: %v", err)
	}
	if err := s.Delete(ctx, p.ID, "studio"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, p.ID); code(err) != "NOT_FOUND" {
		t.Fatalf("after delete: %v", err)
	}
}
