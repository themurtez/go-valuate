package csv

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/ingestion"
)

// benchmarkCSVDocument builds a synthetic multi-year, multi-section income
// statement CSV — representative of a real QuickBooks/accountant-export
// statement (nested Income/COGS/Expense sections, subtotals, multiple
// period columns), rather than an artificially large or artificially
// trivial single-column input.
func benchmarkCSVDocument() string {
	var b strings.Builder
	b.WriteString("Income Statement,FY2022,FY2023,FY2024,FY2025\n")
	b.WriteString("Income\n")
	for _, label := range []string{"Product Sales", "Service Revenue", "Recurring Revenue", "Other Income"} {
		b.WriteString(label + ",100000,110000,121000,133100\n")
	}
	b.WriteString("Cost of Goods Sold\n")
	for _, label := range []string{"Materials", "Direct Labor", "Freight"} {
		b.WriteString(label + ",40000,44000,48400,53240\n")
	}
	b.WriteString("Total COGS,120000,132000,145200,159720\n")
	b.WriteString("Gross Profit,280000,308000,338800,372680\n")
	b.WriteString("Operating Expenses\n")
	for _, label := range []string{"Payroll", "Rent", "Marketing", "Professional Fees", "Insurance", "Utilities", "Owner Compensation", "Repairs and Maintenance", "Office Supplies", "Travel"} {
		b.WriteString(label + ",20000,22000,24200,26620\n")
	}
	b.WriteString("Total Operating Expenses,200000,220000,242000,266200\n")
	b.WriteString("Net Income,80000,88000,96800,106480\n")
	return b.String()
}

// BenchmarkParse_RepresentativeStatement exercises Parse over a realistic
// multi-section, multi-period income statement — the candidate hot path
// named in docs/V1_CONTRACTS.md's performance sanity checks. This is
// intentionally a "typical statement" size, not an artificially large
// input: ingestion's own row/column/statement-type detection is O(rows),
// so this benchmark is representative of real per-document cost rather
// than a synthetic stress test.
func BenchmarkParse_RepresentativeStatement(b *testing.B) {
	doc := benchmarkCSVDocument()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Parse(strings.NewReader(doc), ingestion.Options{})
		if err != nil {
			b.Fatalf("unexpected Parse error: %+v", err)
		}
	}
}
