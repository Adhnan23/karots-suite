package sheet

import (
	"bytes"
	"encoding/csv"
	"io"
	"strings"
)

func readCSV(b []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(b))
	r.FieldsPerRecord = -1 // ragged rows are fine; callers index by header
	r.TrimLeadingSpace = true
	// encoding/csv skips blank lines; keep them as empty rows so a row's index
	// stays its line in the file, and "line N" in an import error is the line
	// the owner sees in the sheet.
	var rows [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		line, _ := r.FieldPos(0)
		for len(rows) < line-1 {
			rows = append(rows, []string{})
		}
		rows = append(rows, rec)
	}
	// Strip a UTF-8 BOM that Excel/Windows often prepends to the first cell.
	if len(rows) > 0 && len(rows[0]) > 0 {
		rows[0][0] = strings.TrimPrefix(rows[0][0], "\ufeff")
	}
	return rows, nil
}

func writeCSV(w io.Writer, header []string, rows [][]string) error {
	cw := csv.NewWriter(w)
	if len(header) > 0 {
		if err := cw.Write(header); err != nil {
			return err
		}
	}
	for _, r := range rows {
		if err := cw.Write(r); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
