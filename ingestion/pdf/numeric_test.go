package pdf

import (
	"testing"

	"github.com/themurtez/go-valuate/ingestion/internal/tabular"
)

func TestNormalizePDFNumericText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"spaced currency prefix", "$ 1,234.00", "$1,234.00"},
		{"double-spaced currency prefix", "$  1,234.00", "$1,234.00"},
		{"spaced open paren", "( 1,234.56", "(1,234.56"},
		{"spaced close paren", "1,234.56 )", "1,234.56)"},
		{"both spaced parens", "( 1,234.56 )", "(1,234.56)"},
		{"trailing minus with gap", "45,000 -", "-45,000"},
		{"trailing em-dash with gap", "45,000 —", "-45,000"},
		{"space thousands", "1 234", "1234"},
		{"space thousands with decimal", "12 345.67", "12345.67"},
		{"multiple space thousands groups", "1 234 567", "1234567"},
		{"already clean value untouched", "1,234.56", "1,234.56"},
		{"bare dash untouched (not a trailing-minus case)", "-", "-"},
		{"blank untouched", "", ""},
		{"non-numeric text untouched", "N/A", "N/A"},
		{"percentage untouched", "12%", "12%"},
		{"unrelated trailing dash after text untouched", "See note -", "See note -"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := normalizePDFNumericText(c.in)
			if got != c.want {
				t.Errorf("normalizePDFNumericText(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNormalizePDFNumericText_NeverMasksMalformedValues(t *testing.T) {
	// A value that is genuinely malformed (not one of the documented PDF
	// spacing quirks) must pass through completely unchanged, so it still
	// fails tabular.ParseNumeric exactly as it would for CSV/XLSX — this
	// package must never silently coerce a bad value into something that
	// happens to parse.
	malformed := []string{"1,234.56 USD", "TBD", "--pending--", "1,234.56.78"}
	for _, m := range malformed {
		if got := normalizePDFNumericText(m); got != m {
			t.Errorf("normalizePDFNumericText(%q) = %q, want unchanged", m, got)
		}
	}
}

func TestParsePDFNumeric_IntegratesWithTabularParser(t *testing.T) {
	cases := []struct {
		raw  string
		want float64
	}{
		{"$ 1,234.00", 1234.00},
		{"( 1,234 )", -1234},
		{"45,000 -", -45000},
		{"1 234", 1234},
	}
	for _, c := range cases {
		res := parsePDFNumeric(c.raw, tabular.DashAsBlank)
		if !res.Parsed {
			t.Errorf("parsePDFNumeric(%q): Parsed = false, want true", c.raw)
			continue
		}
		if res.Value != c.want {
			t.Errorf("parsePDFNumeric(%q) = %v, want %v", c.raw, res.Value, c.want)
		}
	}
}

func TestMidNumberSpaceThousands_NeverMatchesRunningText(t *testing.T) {
	// "1 234 Main Street" must NOT be treated as a space-thousands number
	// — the anchor requires the WHOLE trimmed cell to be nothing but
	// digit groups.
	if reMidNumberSpaceThousands.MatchString("1 234 Main Street") {
		t.Error("reMidNumberSpaceThousands incorrectly matched running text")
	}
	if !reMidNumberSpaceThousands.MatchString("1 234") {
		t.Error("reMidNumberSpaceThousands should match a bare space-thousands value")
	}
}
