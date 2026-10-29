package api

import (
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/sheet"

	"github.com/labstack/echo/v4"
)

// maxImportRows caps one upload.
const maxImportRows = 5000

// writeSheet streams header + rows as a download in ?format=csv|xlsx|ods (CSV default).
func writeSheet(c echo.Context, filename string, header []string, rows [][]string) error {
	f := sheet.FormatFrom(c.QueryParam("format"))
	c.Response().Header().Set(echo.HeaderContentType, f.ContentType())
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+filename+f.Ext()+`"`)
	c.Response().WriteHeader(http.StatusOK)
	return sheet.Write(c.Response(), f, header, rows)
}

// readImportSheet reads the multipart "file" (CSV, XLSX or ODS) into a
// canonical-column → index map (header synonyms applied) and the data rows.
// required names a column the file must have.
func readImportSheet(c echo.Context, synonyms map[string]string, required string) (map[string]int, [][]string, error) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, nil, apperr.BadRequest("please choose a file to import")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, nil, apperr.BadRequest("could not read the uploaded file")
	}
	defer f.Close()
	rows, err := sheet.Read(fh.Filename, f)
	if err != nil {
		return nil, nil, apperr.BadRequest("could not read the file — is it a valid CSV, Excel or ODF spreadsheet?")
	}
	if len(rows) == 0 {
		return nil, nil, apperr.BadRequest("the file is empty")
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		key := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))
		if canon, ok := synonyms[key]; ok {
			key = canon
		}
		if _, dup := col[key]; !dup {
			col[key] = i
		}
	}
	if _, ok := col[required]; !ok {
		return nil, nil, apperr.BadRequest("the file must have a '" + required + "' column")
	}
	return col, rows[1:], nil
}

// cellGetter reads a canonical column from one row ("" when absent).
func cellGetter(col map[string]int, rec []string) func(string) string {
	return func(key string) string {
		if i, ok := col[key]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
}

// importErrMsg unwraps an apperr to its user-facing message.
func importErrMsg(err error) string {
	if ae, ok := apperr.As(err); ok {
		return ae.Message
	}
	return "import failed"
}
