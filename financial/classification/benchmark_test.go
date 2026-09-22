package classification

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// benchmarkRows builds n synthetic RawLineItems cycling through a mix of
// alias hits, rule hits, and genuinely unresolvable labels — representative
// of a real multi-hundred-row financial statement rather than an
// artificially easy or artificially hard uniform input.
func benchmarkRows(n int) []financial.RawLineItem {
	labels := []string{"Product Sales", "Materials", "Field Labor", "Owner Compensation", "Rent Expense", "Something Ambiguous"}
	rows := make([]financial.RawLineItem, n)
	for i := 0; i < n; i++ {
		rows[i] = financial.RawLineItem{
			ID:            fmt.Sprintf("row-%d", i),
			Label:         labels[i%len(labels)],
			StatementType: financial.StatementIncomeStatement,
			Values:        map[financial.Period]float64{"2025": float64(i * 100)},
		}
	}
	return rows
}

func benchmarkConfig() Config {
	return Config{
		AliasLayers: []AliasLayer{{Name: "global", Aliases: []Alias{
			{Label: "Product Sales", Code: financial.CodeRevProduct},
			{Label: "Materials", Code: financial.CodeCogsMaterial},
			{Label: "Owner Compensation", Code: financial.CodeOpexOwnerComp},
			{Label: "Rent Expense", Code: financial.CodeOpexRent},
		}}},
		Rules: DefaultRules(),
	}
}

// BenchmarkClassifyBatch_1000Rows exercises ClassifyBatch over a
// realistically-mixed 1,000-row input — the candidate hot path named in
// docs/V1_CONTRACTS.md's performance sanity checks (a large multi-year or
// multi-entity statement can easily reach this row count).
func BenchmarkClassifyBatch_1000Rows(b *testing.B) {
	rows := benchmarkRows(1000)
	cfg := benchmarkConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ClassifyBatch(rows, cfg)
	}
}
