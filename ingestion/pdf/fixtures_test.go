package pdf_test

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

// TestFixture_SimplePL exercises simple_pl.pdf: a baseline one-page
// income statement.
func TestFixture_SimplePL(t *testing.T) {
	res := mustParse(t, "simple_pl.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	if len(res.Statements[0].Rows) == 0 {
		t.Fatal("expected rows")
	}
}

// TestFixture_MultiYearPL exercises multi_year_pl.pdf: a one-page income
// statement with three period columns (2023/2024/2025).
func TestFixture_MultiYearPL(t *testing.T) {
	res := mustParse(t, "multi_year_pl.pdf", ipdf.Options{})
	if len(res.Statements[0].Metadata.Periods) != 3 {
		t.Fatalf("got %d periods, want 3", len(res.Statements[0].Metadata.Periods))
	}
}

// TestFixture_MultiPagePL exercises multi_page_pl.pdf: a two-page income
// statement with the title/column-header block reprinted on page 2.
func TestFixture_MultiPagePL(t *testing.T) {
	res := mustParse(t, "multi_page_pl.pdf", ipdf.Options{})
	if res.PageCount != 2 {
		t.Fatalf("PageCount = %d, want 2", res.PageCount)
	}
}

// TestFixture_BalanceSheet exercises balance_sheet.pdf: a one-page balance
// sheet with nested sections and a grand total.
func TestFixture_BalanceSheet(t *testing.T) {
	res := mustParse(t, "balance_sheet.pdf", ipdf.Options{})
	if res.Statements[0].Metadata.StatementType != financial.StatementBalanceSheet {
		t.Errorf("StatementType = %q, want balance_sheet", res.Statements[0].Metadata.StatementType)
	}
}

// TestFixture_PLAndBalanceSheet exercises pl_and_balance_sheet.pdf: an
// income statement (page 1) and a balance sheet (page 2) in one PDF.
func TestFixture_PLAndBalanceSheet(t *testing.T) {
	res := mustParse(t, "pl_and_balance_sheet.pdf", ipdf.Options{})
	if len(res.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(res.Statements))
	}
}

// TestFixture_NegativeParentheses exercises negative_parentheses.pdf:
// parenthetical-negative values, including the "( 1,234 )" internal-
// whitespace PDF extraction quirk.
func TestFixture_NegativeParentheses(t *testing.T) {
	res := mustParse(t, "negative_parentheses.pdf", ipdf.Options{})
	interest, ok := rowByLabel(res.Statements[0].Rows, "Interest Expense")
	if !ok || interest.Values[financial.Period("FY2025")] >= 0 {
		t.Errorf("Interest Expense = %+v, want a negative value", interest)
	}
}

// TestFixture_IndentedSections exercises indented_sections.pdf: X-offset
// indentation under a section heading.
func TestFixture_IndentedSections(t *testing.T) {
	res := mustParse(t, "indented_sections.pdf", ipdf.Options{})
	payroll, ok := rowByLabel(res.Statements[0].Rows, "Payroll")
	if !ok || payroll.IndentLevel == 0 {
		t.Errorf("Payroll = %+v, want IndentLevel > 0", payroll)
	}
}

// TestFixture_UnusualSpacing exercises unusual_spacing.pdf: a spaced
// currency prefix, a space-thousands value, and a trailing minus sign.
func TestFixture_UnusualSpacing(t *testing.T) {
	res := mustParse(t, "unusual_spacing.pdf", ipdf.Options{})
	if len(res.Statements[0].Rows) < 3 {
		t.Fatalf("got %d rows, want at least 3", len(res.Statements[0].Rows))
	}
}

// TestFixture_ImageOnly exercises image_only.pdf: zero text objects (only
// a drawn rectangle), for OCR-required detection.
func TestFixture_ImageOnly(t *testing.T) {
	_, err := ipdf.Parse(openFixture(t, "image_only.pdf"), ipdf.Options{})
	if err == nil {
		t.Fatal("expected an error")
	}
}

// TestFixture_AmbiguousLayout exercises ambiguous_layout.pdf: scattered
// text with no discernible row/column alignment.
func TestFixture_AmbiguousLayout(t *testing.T) {
	res := mustParse(t, "ambiguous_layout.pdf", ipdf.Options{})
	if len(res.Statements[0].Warnings) == 0 {
		t.Error("expected at least one ambiguity warning")
	}
}
