package alternatives

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/products"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"
	"karots-suite/internal/validator"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
)

// upload posts a CSV to the import route, past the role guard (the battle cases
// check the guard on the real server).
func upload(t *testing.T, db *sqlx.DB, query, csv string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = validator.New()
	e.HTTPErrorHandler = middleware.ErrorHandler(false)
	p := &Plugin{}
	p.Setup(plugin.NewRegistry(plugin.Core{DB: db, Products: products.NewService(db), Audit: audit.NewService(db)}, plugin.NewMux(), e))
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	f, err := w.CreateFormFile("file", "alternatives.csv")
	noErr(t, err)
	_, _ = f.Write([]byte(csv))
	noErr(t, w.Close())
	req := httptest.NewRequest("POST", "/import"+query, &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := (handlers{p}).sheetImport(c); err != nil {
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

// ALT-11: the preview reports and writes nothing; Apply does what it reported.
func TestSheetPreviewWritesNothing_ALT11(t *testing.T) {
	s, db, tag := testStore(t)
	a := product(t, db, tag+" Anchor 1kg", "5", false, true)
	b := product(t, db, tag+" Bakers 1kg", "5", false, true)
	product(t, db, tag+" Cable 1m", "5", false, true)
	groupTier(t, s, tag+" Rice 1kg", a, b)
	csv := "Group Name,Quality,Item,Code\n" +
		tag + " Rice 1kg,," + tag + " Anchor 1kg,\n" +
		tag + " Rice 1kg,," + tag + " Cable 1m,\n" +
		tag + " Rice 1kg,," + tag + " Nowhere,\n"

	rec := upload(t, db, "?dry=1", csv)
	pre := decode[SheetImportDTO](t, rec)
	if rec.Code != 200 || pre.Applied || pre.Members != 2 || pre.Placed != 1 || pre.TakenOut != 1 || len(pre.Skipped) != 1 ||
		!strings.HasPrefix(pre.Skipped[0], "line 4: ") {
		t.Fatalf("%d %+v", rec.Code, pre)
	}
	if got := mine(ok(s.SheetLines(context.Background())), tag); len(got) != 2 || got[1].Product != tag+" Bakers 1kg" {
		t.Fatalf("the preview wrote: %+v", got)
	}
	done := decode[SheetImportDTO](t, upload(t, db, "", csv))
	pre.Applied = true
	if done.Members != pre.Members || done.Placed != pre.Placed || done.TakenOut != pre.TakenOut || !done.Applied {
		t.Fatalf("apply %+v, preview %+v", done, pre)
	}
	if got := mine(ok(s.SheetLines(context.Background())), tag); len(got) != 2 || got[1].Product != tag+" Cable 1m" {
		t.Fatalf("after apply: %+v", got)
	}
}

// ALT-14: a header with no lines is refused.
func TestSheetNoLines_ALT14(t *testing.T) {
	_, db, _ := testStore(t)
	if rec := upload(t, db, "?dry=1", "Group,Tier,Product,Barcode\n,,,\n"); rec.Code != 400 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
