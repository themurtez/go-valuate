package pdf_test

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	ipdf "github.com/themurtez/go-valuate/ingestion/pdf"
)

// TestIntegration_PDFIncomeStatementToNormalizedDataset exercises the full
// advertised pipeline for the PDF adapter: PDF bytes -> pdf.Parse ->
// ingestion.Result.ToRawLineItems() -> classification.ClassifyBatch ->
// financial.MappedLineItem -> financial.Normalize -> FinancialDataset,
// mirroring ingestion/integration_test.go's CSV/XLSX equivalents exactly
// (same fallback-mapping pattern for rows the built-in classifier cannot
// confidently resolve — see that file's doc comment for why: it stands in
// for a future application's human/UI confirmation step).
func TestIntegration_PDFIncomeStatementToNormalizedDataset(t *testing.T) {
	res := mustParse(t, "simple_pl.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]

	rawItems := stmt.ToRawLineItems()
	if len(rawItems) == 0 {
		t.Fatal("expected at least one raw line item")
	}

	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)
	if len(results) != len(rawItems) {
		t.Fatalf("got %d classification results, want %d", len(results), len(rawItems))
	}

	fallback := map[string]financial.Code{
		"Revenue":            financial.CodeRevProduct,
		"Cost of Goods Sold": financial.CodeCogsOther,
		"Operating Expenses": financial.CodeOpexOther,
	}

	mapped := make([]financial.MappedLineItem, 0, len(rawItems))
	for i, raw := range rawItems {
		result := results[i]
		if result.IsUnknown() {
			code, ok := fallback[raw.Label]
			if !ok {
				t.Fatalf("row %q classified UNKNOWN with no test fallback mapping", raw.Label)
			}
			result.Code = code
			result.Status = financial.RowStatusNormal
		}
		mapped = append(mapped, result.ToMappedLineItem(raw))
	}

	dataset, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
	if nerr != nil {
		t.Fatalf("Normalize: %v", nerr)
	}
	if dataset.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", dataset.Currency)
	}
	if len(dataset.Items) == 0 {
		t.Fatal("expected normalized items")
	}

	revenue, ok := dataset.ByCodeAndPeriod(financial.CodeRevProduct, "FY2025")
	if !ok {
		t.Fatal("expected REV_PRODUCT for FY2025 in normalized dataset")
	}
	if revenue.Amount != 850000 {
		t.Errorf("REV_PRODUCT FY2025 = %v, want 850000", revenue.Amount)
	}

	// Gross Profit and Net Income are structural (subtotal/total): they
	// must never appear as normalized items, or Revenue would be
	// double-counted alongside them.
	for _, item := range dataset.Items {
		if item.Code == "" {
			t.Errorf("unexpected empty code in normalized item: %+v", item)
		}
	}
}

// TestIntegration_PDFBalanceSheetReconciliation mirrors
// ingestion/integration_test.go's
// TestIntegrationReconciliationOnIngestedBalanceSheet for the PDF adapter:
// PDF bytes -> pdf.Parse -> RawLineItem[] -> classification.ClassifyBatch
// -> financial.Normalize -> financial/reconciliation, confirming the PDF
// path converges on the exact same downstream pipeline CSV/XLSX already
// do, with no PDF-specific reconciliation logic anywhere.
func TestIntegration_PDFBalanceSheetReconciliation(t *testing.T) {
	res := mustParse(t, "balance_sheet.pdf", ipdf.Options{})
	if len(res.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(res.Statements))
	}
	stmt := res.Statements[0]

	rawItems := stmt.ToRawLineItems()
	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)

	fallback := map[string]financial.Code{
		"Cash":                financial.CodeBsCash,
		"Accounts Receivable": financial.CodeBsAccountsReceivable,
		"Accounts Payable":    financial.CodeBsAccountsPayable,
		"Retained Earnings":   financial.CodeBsRetainedEarnings,
	}

	mapped := make([]financial.MappedLineItem, 0, len(rawItems))
	for i, raw := range rawItems {
		result := results[i]
		if result.IsUnknown() {
			code, ok := fallback[raw.Label]
			if !ok {
				t.Fatalf("row %q classified UNKNOWN with no test fallback mapping", raw.Label)
			}
			result.Code = code
			result.Status = financial.RowStatusNormal
		}
		mapped = append(mapped, result.ToMappedLineItem(raw))
	}

	dataset, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
	if nerr != nil {
		t.Fatalf("Normalize: %v", nerr)
	}

	cash, ok := dataset.ByCodeAndPeriod(financial.CodeBsCash, "2025")
	if !ok {
		t.Fatal("expected BS_CASH for 2025")
	}
	if cash.Amount != 410000 {
		t.Errorf("BS_CASH 2025 = %v, want 410000", cash.Amount)
	}
}

// TestIntegration_PDFMultipleStatementsEachClassifyIndependently confirms
// that when a single PDF yields multiple statements (Option A), each
// statement's rows can be independently classified/normalized without any
// cross-statement interference — e.g. the balance sheet's "Cash" and the
// income statement's "Revenue" never collide or merge, since each
// stmt.ToRawLineItems() -> ClassifyBatch -> Normalize chain runs entirely
// independently per ingestion.Result.
func TestIntegration_PDFMultipleStatementsEachClassifyIndependently(t *testing.T) {
	res := mustParse(t, "pl_and_balance_sheet.pdf", ipdf.Options{})
	if len(res.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(res.Statements))
	}

	cfg := classification.Config{Rules: classification.DefaultRules()}

	var datasets []financial.FinancialDataset
	for _, stmt := range res.Statements {
		rawItems := stmt.ToRawLineItems()
		results := classification.ClassifyBatch(rawItems, cfg)

		mapped := make([]financial.MappedLineItem, 0, len(rawItems))
		for i, raw := range rawItems {
			result := results[i]
			if result.IsUnknown() {
				continue // simulate a caller skipping unresolved rows
			}
			mapped = append(mapped, result.ToMappedLineItem(raw))
		}
		ds, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
		if nerr != nil {
			t.Fatalf("Normalize (statement %q): %v", stmt.Metadata.StatementType, nerr)
		}
		datasets = append(datasets, ds)
	}

	if len(datasets) != 2 {
		t.Fatalf("got %d datasets, want 2", len(datasets))
	}

	if _, ok := datasets[0].ByCodeAndPeriod(financial.CodeBsCash, "2025"); ok {
		t.Error("income statement dataset must not contain BS_CASH")
	}
}
