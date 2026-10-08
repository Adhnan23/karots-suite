package sheet

import (
	"archive/zip"
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRoundTrip(t *testing.T) {
	header := []string{"barcode", "name", "qty"}
	rows := [][]string{
		{"007", "Onion, red", "12"},   // leading zero must survive; comma must not split
		{"123", "Sugar \"1kg\"", "0"}, // quotes need escaping
		{"", "No barcode", "3"},       // empty leading cell
	}

	for _, f := range []Format{CSV, XLSX, ODS} {
		t.Run(string(f), func(t *testing.T) {
			var buf bytes.Buffer
			if err := Write(&buf, f, header, rows); err != nil {
				t.Fatalf("write %s: %v", f, err)
			}
			got, err := Read("file"+f.Ext(), bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			want := append([][]string{header}, rows...)
			if len(got) != len(want) {
				t.Fatalf("%s: got %d rows, want %d: %v", f, len(got), len(want), got)
			}
			for i := range want {
				// Trailing empties may differ; compare up to the wanted width.
				for j := range want[i] {
					var cell string
					if j < len(got[i]) {
						cell = got[i][j]
					}
					if cell != want[i][j] {
						t.Errorf("%s row %d col %d = %q, want %q", f, i, j, cell, want[i][j])
					}
				}
			}
		})
	}
}

func TestFormatFrom(t *testing.T) {
	cases := map[string]Format{
		"": CSV, "csv": CSV, "xlsx": XLSX, "excel": XLSX,
		"ods": ODS, "odf": ODS, "weird": CSV,
	}
	for in, want := range cases {
		if got := FormatFrom(in); got != want {
			t.Errorf("FormatFrom(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadODSHandlesRepeatedEmptyTail(t *testing.T) {
	// A row padded with a huge empty repeated cell (as LibreOffice writes) must
	// not blow up and must drop the trailing empties.
	ods := buildODS(`<table:table-row>` +
		`<table:table-cell office:value-type="string"><text:p>a</text:p></table:table-cell>` +
		`<table:table-cell table:number-columns-repeated="16384"/>` +
		`</table:table-row>`)
	got, err := readODS(ods)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, [][]string{{"a"}}) {
		t.Errorf("got %v, want [[a]]", got)
	}
}

// buildODS wraps a table-row fragment into a minimal ODS zip for read tests.
func buildODS(rowsXML string) []byte {
	content := `<?xml version="1.0"?><office:document-content ` +
		`xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" ` +
		`xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0" ` +
		`xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">` +
		`<office:body><office:spreadsheet><table:table table:name="S">` +
		rowsXML +
		`</table:table></office:spreadsheet></office:body></office:document-content>`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("content.xml")
	_, _ = io.WriteString(w, content)
	_ = zw.Close()
	return buf.Bytes()
}

// A sheet typed in Excel holds numbers, not text: a 13-digit barcode and a
// price formatted "#,##0.00" must read back as the plain digits, never as
// "4.79E+12" or "1,250.00".
func TestReadXLSXNumbersAsTyped(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	f.SetCellStr("Sheet1", "A1", "barcode")
	f.SetCellStr("Sheet1", "B1", "price")
	f.SetCellInt("Sheet1", "A2", 4790000000011)
	f.SetCellFloat("Sheet1", "B2", 1250.5, -1, 64)
	style, _ := f.NewStyle(&excelize.Style{NumFmt: 4}) // #,##0.00
	f.SetCellStyle("Sheet1", "B2", "B2", style)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	rows, err := Read("x.xlsx", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][0] != "4790000000011" || rows[1][1] != "1250.5" {
		t.Fatalf("row = %q, want barcode digits and 1250.5", rows[1])
	}
}

func TestReadCSVKeepsBlankLines(t *testing.T) {
	rows, err := Read("x.csv", strings.NewReader("name\na\n\nb\n"))
	if err != nil || len(rows) != 4 || rows[3][0] != "b" || len(rows[2]) != 0 {
		t.Fatalf("rows = %q, %v", rows, err)
	}
}
