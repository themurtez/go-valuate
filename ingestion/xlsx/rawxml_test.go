package xlsx_test

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/xuri/excelize/v2"
)

// buildRawFormulaCellXLSX builds a minimal XLSX workbook where cell B2
// holds a formula AND a cached value, by patching the sheet XML directly
// after excelize writes it — excelize's public write API does not support
// setting both on one cell (see TestXLSXFormulaWithCachedValue), but every
// real authoring application (Excel, Google Sheets, QuickBooks) saves
// exactly this shape (<f>formula</f><v>cachedValue</v> in the same <c>
// element), so this reproduces that real-world case faithfully for
// testing this package's "prefer cached value" behavior.
func buildRawFormulaCellXLSX(t *testing.T) []byte {
	t.Helper()

	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "Account")
	f.SetCellValue("Sheet1", "B1", "2024")
	f.SetCellValue("Sheet1", "A2", "Revenue")
	f.SetCellValue("Sheet1", "B2", 100.0)
	f.SetCellValue("Sheet1", "A3", "junk")
	f.SetCellValue("Sheet1", "B3", 200.0)

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	f.Close()

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, file := range zr.File {
		w, err := zw.Create(file.Name)
		if err != nil {
			t.Fatalf("zw.Create(%q): %v", file.Name, err)
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("file.Open(%q): %v", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %q: %v", file.Name, err)
		}

		if file.Name == "xl/worksheets/sheet1.xml" {
			// Replace B2's plain value cell with a formula-plus-cached-value
			// cell: <c r="B2"><v>100</v></c> -> <c r="B2"><f>A1</f><v>300</v></c>.
			// The formula text itself is irrelevant to this package (it is
			// never evaluated); what matters is that both <f> and <v> are
			// present so GetCellFormula and GetCellValue/GetRows both
			// return non-empty results.
			data = bytes.Replace(data,
				[]byte(`<c r="B2"><v>100</v></c>`),
				[]byte(`<c r="B2"><f>SUM(B1:B1)</f><v>300</v></c>`),
				1)
		}

		if _, err := w.Write(data); err != nil {
			t.Fatalf("write %q: %v", file.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zw.Close: %v", err)
	}

	return out.Bytes()
}
