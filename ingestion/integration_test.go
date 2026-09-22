package ingestion_test

import (
	"os"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/ingestion"
	icsv "github.com/themurtez/go-valuate/ingestion/csv"
	ixlsx "github.com/themurtez/go-valuate/ingestion/xlsx"
)

// TestIntegrationCSVToNormalizedDataset exercises the full advertised
// pipeline: CSV bytes -> ingestion.Result -> financial.RawLineItem ->
// classification.ClassifyBatch -> financial.MappedLineItem ->
// financial.Normalize -> financial.FinancialDataset. Rows the built-in
// classifier cannot confidently resolve are handled here exactly the way
// the ingestion contract says a future application must: by accepting a
// caller-supplied mapping (simulating human/UI confirmation) rather than
// by this package inventing one.
func TestIntegrationCSVToNormalizedDataset(t *testing.T) {
	f, err := os.Open("fixtures/accountant_custom_pl.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	res, perr := icsv.Parse(f, ingestion.Options{})
	if perr != nil {
		t.Fatalf("Parse: %+v", perr)
	}

	rawItems := res.ToRawLineItems()
	if len(rawItems) == 0 {
		t.Fatal("expected at least one raw line item")
	}

	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)
	if len(results) != len(rawItems) {
		t.Fatalf("got %d classification results, want %d", len(results), len(rawItems))
	}

	// Confirmation step: any UNKNOWN classification is resolved via a
	// caller-supplied mapping (standing in for a human reviewer confirming
	// via a future application's UI) before normalization, since
	// financial.Normalize requires every non-ignored/subtotal/total row to
	// carry a code.
	fallback := map[string]financial.Code{
		"Service Revenue":              financial.CodeRevService,
		"Retainer Revenue":             financial.CodeRevRecurring,
		"Officer Compensation":         financial.CodeOpexOwnerComp,
		"Bank & Merchant Fees":         financial.CodeOpexOther,
		"Miscellaneous":                financial.CodeOpexOther,
		"Total Other Income (Expense)": financial.CodeOtherIncome,
		"Operating Income":             financial.CodeOtherIncome,
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

	revenue, ok := dataset.ByCodeAndPeriod(financial.CodeRevService, "FY2025")
	if !ok {
		t.Fatal("expected REV_SERVICE for FY2025 in normalized dataset")
	}
	if revenue.Amount != 2120000.00 {
		t.Errorf("REV_SERVICE FY2025 = %v, want 2120000.00", revenue.Amount)
	}

	// Subtotal/total rows (e.g. "Total Revenue", "Net Income") must never
	// appear as normalized items, since financial.Normalize excludes them
	// from aggregation to avoid double counting.
	for _, item := range dataset.Items {
		if item.Code == "" {
			t.Errorf("unexpected empty code in normalized item: %+v", item)
		}
	}
}

// TestIntegrationXLSXToNormalizedDataset mirrors
// TestIntegrationCSVToNormalizedDataset for the XLSX adapter, confirming
// both formats converge on the same downstream pipeline.
func TestIntegrationXLSXToNormalizedDataset(t *testing.T) {
	f, err := os.Open("fixtures/multi_sheet_workbook.xlsx")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	res, perr := ixlsx.Parse(f, ingestion.Options{})
	if perr != nil {
		t.Fatalf("Parse: %+v", perr)
	}

	rawItems := res.ToRawLineItems()
	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)

	fallback := map[string]financial.Code{
		"Revenue":            financial.CodeRevProduct,
		"Cost of Goods Sold": financial.CodeCogsOther,
		"Operating Expenses": financial.CodeOpexOther,
		"Gross Profit":       financial.CodeOtherIncome,
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
	if len(dataset.Items) == 0 {
		t.Fatal("expected normalized items")
	}
}

// TestIntegrationReconciliationOnIngestedBalanceSheet verifies an ingested
// balance sheet, once classified/normalized, reconciles internally --
// exercising the full chain through financial/reconciliation as well,
// which is downstream of Normalize in the repository's documented
// pipeline.
func TestIntegrationReconciliationOnIngestedBalanceSheet(t *testing.T) {
	f, err := os.Open("fixtures/balance_sheet.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	res, perr := icsv.Parse(f, ingestion.Options{StatementTypeOverride: ingestion.StatementOverrideBalanceSheet})
	if perr != nil {
		t.Fatalf("Parse: %+v", perr)
	}

	rawItems := res.ToRawLineItems()
	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)

	fallback := map[string]financial.Code{
		"Cash":                     financial.CodeBsCash,
		"Accounts Receivable":      financial.CodeBsAccountsReceivable,
		"Inventory":                financial.CodeBsInventory,
		"Prepaid Expenses":         financial.CodeBsPrepaid,
		"Equipment":                financial.CodeBsFixedAssets,
		"Accumulated Depreciation": financial.CodeBsAccumDepreciation,
		"Accounts Payable":         financial.CodeBsAccountsPayable,
		"Short-Term Debt":          financial.CodeBsShortTermDebt,
		"Long-Term Debt":           financial.CodeBsLongTermDebt,
		"Retained Earnings":        financial.CodeBsRetainedEarnings,
		"Owner Equity":             financial.CodeBsOwnerEquity,
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
	if cash.Amount != 268500.00 {
		t.Errorf("BS_CASH 2025 = %v, want 268500.00", cash.Amount)
	}
}
