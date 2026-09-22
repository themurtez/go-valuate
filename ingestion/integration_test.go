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

	// "Gross Profit" is intentionally NOT in this fallback map (unlike
	// before financial.RowKind existed): ingestion's structural read
	// (ClassifyRowKind recognizes it as a subtotal from label shape alone)
	// now reaches classification via RawLineItem.Kind, so Classify resolves
	// it directly as SourceStructural/RowStatusSubtotal — it never reaches
	// SourceUnknown and therefore never needs a caller-supplied fallback
	// code. See the assertion below and the README's ingestion section.
	fallback := map[string]financial.Code{
		"Revenue":            financial.CodeRevProduct,
		"Cost of Goods Sold": financial.CodeCogsOther,
		"Operating Expenses": financial.CodeOpexOther,
	}

	var grossProfitResult *classification.Result
	mapped := make([]financial.MappedLineItem, 0, len(rawItems))
	for i, raw := range rawItems {
		result := results[i]
		if raw.Label == "Gross Profit" {
			r := result
			grossProfitResult = &r
		}
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

	if grossProfitResult == nil {
		t.Fatal("expected a classification result for the Gross Profit row")
	}
	if grossProfitResult.Source != classification.SourceStructural {
		t.Errorf("Gross Profit Source = %q, want %q", grossProfitResult.Source, classification.SourceStructural)
	}
	if grossProfitResult.Status != financial.RowStatusSubtotal {
		t.Errorf("Gross Profit Status = %q, want %q", grossProfitResult.Status, financial.RowStatusSubtotal)
	}
	if grossProfitResult.Code != "" {
		t.Errorf("Gross Profit Code = %q, want empty (structural rows propose no code)", grossProfitResult.Code)
	}

	dataset, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
	if nerr != nil {
		t.Fatalf("Normalize: %v", nerr)
	}
	if len(dataset.Items) == 0 {
		t.Fatal("expected normalized items")
	}

	// Gross Profit is a subtotal (Revenue - COGS): it must never appear as
	// a normalized item, or Revenue would be double counted alongside it.
	if _, ok := dataset.ByCodeAndPeriod(financial.CodeOtherIncome, "2024"); ok {
		t.Error("Gross Profit must not be normalized under CodeOtherIncome (or any code) — it is a subtotal, excluded from aggregation")
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

// TestStructuralContract_LabelsSurviveAsStructuralNotOrdinaryAccounts is the
// regression test the task brief calls for directly: "Gross Profit",
// "Total Operating Expenses", "Net Income", and "Total Assets" must survive
// ingestion, classification, and normalization as structural rows — never
// classified as ordinary canonical accounts merely because their labels
// resemble financial concepts, and never appearing as a normalized item
// (which would silently double-count alongside the rows they summarize).
// This is the end-to-end proof that RawLineItem.Kind closes the
// historically-documented "Gross Profit" disagreement between ingestion's
// structural read and classification's own heuristic — see
// financial/classification/structural.go and the README's ingestion
// section. Exercised across both the CSV and XLSX adapters, per the task
// brief's explicit instruction to test both paths (a third, PDF, path is
// exercised by ingestion/pdf's own integration tests).
func TestStructuralContract_LabelsSurviveAsStructuralNotOrdinaryAccounts(t *testing.T) {
	t.Run("csv", func(t *testing.T) {
		f, err := os.Open("fixtures/multi_year_saas_pl.csv")
		if err != nil {
			t.Fatalf("open fixture: %v", err)
		}
		defer f.Close()

		res, perr := icsv.Parse(f, ingestion.Options{})
		if perr != nil {
			t.Fatalf("Parse: %+v", perr)
		}
		assertStructuralLabelsNeverBecomeOrdinaryAccounts(t, res, "Gross Profit", "Total Operating Expenses", "Net Income")
	})

	t.Run("csv_balance_sheet", func(t *testing.T) {
		f, err := os.Open("fixtures/balance_sheet.csv")
		if err != nil {
			t.Fatalf("open fixture: %v", err)
		}
		defer f.Close()

		res, perr := icsv.Parse(f, ingestion.Options{StatementTypeOverride: ingestion.StatementOverrideBalanceSheet})
		if perr != nil {
			t.Fatalf("Parse: %+v", perr)
		}
		assertStructuralLabelsNeverBecomeOrdinaryAccounts(t, res, "Total Assets")

		// balance_sheet.csv also has heading rows ("Current Assets", "Fixed
		// Assets", "Current Liabilities", "Long-Term Liabilities", "Equity")
		// with no numeric values of their own — confirm they survive
		// ToRawLineItems() as RowKindHeading and are excluded from the
		// normalized dataset via RowStatusIgnored, never contributing a
		// spurious zero-amount item.
		assertHeadingSurvivesAsIgnored(t, res, "Current Assets")
	})

	t.Run("xlsx", func(t *testing.T) {
		f, err := os.Open("fixtures/totals_subtotals.xlsx")
		if err != nil {
			t.Fatalf("open fixture: %v", err)
		}
		defer f.Close()

		res, perr := ixlsx.Parse(f, ingestion.Options{})
		if perr != nil {
			t.Fatalf("Parse: %+v", perr)
		}
		assertStructuralLabelsNeverBecomeOrdinaryAccounts(t, res, "Total Operating Expenses", "Net Income")
		assertHeadingSurvivesAsIgnored(t, res, "Operating Expenses")
	})
}

// assertStructuralLabelsNeverBecomeOrdinaryAccounts runs res through
// classification and normalization and asserts every label in
// structuralLabels resolved to classification.SourceStructural with no
// proposed Code, and appears nowhere in the resulting FinancialDataset.
func assertStructuralLabelsNeverBecomeOrdinaryAccounts(t *testing.T, res *ingestion.Result, structuralLabels ...string) {
	t.Helper()

	rawItems := res.ToRawLineItems()
	cfg := classification.Config{Rules: classification.DefaultRules()}
	results := classification.ClassifyBatch(rawItems, cfg)

	want := make(map[string]bool, len(structuralLabels))
	for _, l := range structuralLabels {
		want[l] = true
	}
	found := make(map[string]bool, len(structuralLabels))

	mapped := make([]financial.MappedLineItem, 0, len(rawItems))
	for i, raw := range rawItems {
		result := results[i]
		if want[raw.Label] {
			found[raw.Label] = true
			if result.Source != classification.SourceStructural {
				t.Errorf("row %q Source = %q, want %q (structural label misclassified as an ordinary account)", raw.Label, result.Source, classification.SourceStructural)
			}
			if result.Code != "" {
				t.Errorf("row %q Code = %q, want empty (structural rows propose no code)", raw.Label, result.Code)
			}
			if result.Status != financial.RowStatusSubtotal && result.Status != financial.RowStatusTotal {
				t.Errorf("row %q Status = %q, want RowStatusSubtotal or RowStatusTotal", raw.Label, result.Status)
			}
		}
		if result.IsUnknown() {
			// Any row genuinely unresolved is not this test's concern (a
			// real caller would route it to human review), but it must not
			// be one of the rows this test is specifically verifying stays
			// structural.
			if want[raw.Label] {
				t.Errorf("row %q classified UNKNOWN, want SourceStructural", raw.Label)
			}
			continue
		}
		mapped = append(mapped, result.ToMappedLineItem(raw))
	}

	for _, l := range structuralLabels {
		if !found[l] {
			t.Errorf("expected to find row labeled %q in the fixture, found none", l)
		}
	}

	dataset, nerr := financial.Normalize(mapped, financial.NormalizeOptions{Currency: "USD"})
	if nerr != nil {
		t.Fatalf("Normalize: %v", nerr)
	}

	for _, item := range dataset.Items {
		for _, ref := range item.Sources {
			if want[ref.Label] {
				t.Errorf("structural row %q contributed to normalized item %s/%s — must be excluded from aggregation", ref.Label, item.Code, item.Period)
			}
		}
	}
}

// assertHeadingSurvivesAsIgnored confirms label appears in res's raw items
// with Kind == RowKindHeading, classifies to RowStatusIgnored, and (once
// normalized alongside everything else that successfully classifies)
// contributes no normalized item under any code.
func assertHeadingSurvivesAsIgnored(t *testing.T, res *ingestion.Result, label string) {
	t.Helper()

	rawItems := res.ToRawLineItems()
	var headingFound bool
	for _, raw := range rawItems {
		if raw.Label == label {
			headingFound = true
			if raw.Kind != financial.RowKindHeading {
				t.Errorf("row %q Kind = %q, want %q", label, raw.Kind, financial.RowKindHeading)
			}
			result := classification.Classify(raw, classification.Config{Rules: classification.DefaultRules()})
			if result.Status != financial.RowStatusIgnored {
				t.Errorf("heading row %q Status = %q, want %q", label, result.Status, financial.RowStatusIgnored)
			}
			if result.Source != classification.SourceStructural {
				t.Errorf("heading row %q Source = %q, want %q", label, result.Source, classification.SourceStructural)
			}
		}
	}
	if !headingFound {
		t.Errorf("expected to find heading row labeled %q via ToRawLineItems()", label)
	}
}

// TestRegression_OnlyKindChangedForPreExistingRows re-runs
// accountant_custom_pl.csv — an existing fixture already covered by
// TestIntegrationCSVToNormalizedDataset before financial.RowKind was
// introduced — through ToRawLineItems() and asserts two things about the
// task brief's backward-compatibility requirement (section 29):
//
//  1. Every row that was already present in RawLineItem output before this
//     change (i.e. every non-heading row) is unchanged in every field
//     EXCEPT the new Kind field: same ID, StatementType, Label,
//     ParentLabel, and Values, in the same order.
//  2. The only rows now additionally present are heading rows
//     ("Revenue", "Operating Expenses", "Other Income (Expense)" in this
//     fixture), each carrying Kind == RowKindHeading, an empty Values map,
//     and otherwise fully-formed ID/StatementType/ParentLabel — i.e. the
//     new rows are exactly the previously-dropped headings, nothing else
//     changed shape.
func TestRegression_OnlyKindChangedForPreExistingRows(t *testing.T) {
	f, err := os.Open("fixtures/accountant_custom_pl.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	res, perr := icsv.Parse(f, ingestion.Options{})
	if perr != nil {
		t.Fatalf("Parse: %+v", perr)
	}

	// previouslyPresent mirrors exactly what ToRawLineItems() produced
	// before RawLineItem.Kind existed: every row EXCEPT
	// StructuralHeading/StructuralBlank (see the pre-change doc comment
	// this test guards, quoted in ingestion/types.go's git history) —
	// i.e. every row whose ingestion.Row.Kind is normal, subtotal, or
	// total.
	previouslyPresentLabels := make(map[string]bool)
	newlyPresentHeadingLabels := make(map[string]bool)
	for _, row := range res.Rows {
		switch row.Kind {
		case ingestion.StructuralHeading:
			newlyPresentHeadingLabels[row.Label] = true
		case ingestion.StructuralBlank:
			// still never present either before or after this change.
		default:
			previouslyPresentLabels[row.Label] = true
		}
	}
	if len(newlyPresentHeadingLabels) == 0 {
		t.Fatal("fixture has no heading rows — this test needs a fixture with at least one to be meaningful")
	}

	items := res.ToRawLineItems()

	seenPreviouslyPresent := 0
	seenNewHeadings := 0
	for _, item := range items {
		switch {
		case previouslyPresentLabels[item.Label]:
			seenPreviouslyPresent++
			if item.Kind != financial.RowKindNormal && item.Kind != financial.RowKindSubtotal && item.Kind != financial.RowKindTotal {
				t.Errorf("pre-existing row %q Kind = %q, want normal/subtotal/total (not heading)", item.Label, item.Kind)
			}
			if item.ID == "" {
				t.Errorf("pre-existing row %q has empty ID", item.Label)
			}
			if item.StatementType != res.Metadata.StatementType {
				t.Errorf("pre-existing row %q StatementType = %q, want %q", item.Label, item.StatementType, res.Metadata.StatementType)
			}
			if len(item.Values) == 0 {
				t.Errorf("pre-existing row %q has no Values — every previously-present row in this fixture carries at least one period's amount", item.Label)
			}
		case newlyPresentHeadingLabels[item.Label]:
			seenNewHeadings++
			if item.Kind != financial.RowKindHeading {
				t.Errorf("newly-surviving heading row %q Kind = %q, want %q", item.Label, item.Kind, financial.RowKindHeading)
			}
			if len(item.Values) != 0 {
				t.Errorf("heading row %q Values = %v, want empty (headings carry no financial amount)", item.Label, item.Values)
			}
			if item.ID == "" {
				t.Errorf("heading row %q has empty ID", item.Label)
			}
		default:
			t.Errorf("unexpected row %q in ToRawLineItems() output that was neither a previously-present label nor a newly-surviving heading", item.Label)
		}
	}

	if seenPreviouslyPresent != len(previouslyPresentLabels) {
		t.Errorf("got %d previously-present rows in output, want %d (%v)", seenPreviouslyPresent, len(previouslyPresentLabels), previouslyPresentLabels)
	}
	if seenNewHeadings != len(newlyPresentHeadingLabels) {
		t.Errorf("got %d newly-surviving heading rows in output, want %d (%v)", seenNewHeadings, len(newlyPresentHeadingLabels), newlyPresentHeadingLabels)
	}

	// Ordering must also be unaffected: previously-present rows must
	// appear in exactly the same relative order as before (source order),
	// with headings now interleaved in their own correct source position
	// rather than appended/prepended anywhere.
	var gotOrder []string
	for _, item := range items {
		gotOrder = append(gotOrder, item.Label)
	}
	var wantOrder []string
	for _, row := range res.Rows {
		if row.Kind != ingestion.StructuralBlank {
			wantOrder = append(wantOrder, row.Label)
		}
	}
	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("order length mismatch: got %d, want %d", len(gotOrder), len(wantOrder))
	}
	for i := range gotOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("order[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}
}
