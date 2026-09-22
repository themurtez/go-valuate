package financial

import (
	"fmt"
	"testing"
)

// benchmarkMappedLineItems builds n synthetic MappedLineItems cycling
// through a handful of codes/periods, representative of a large multi-year
// statement's classified output.
func benchmarkMappedLineItems(n int) []MappedLineItem {
	codes := []Code{CodeRevProduct, CodeCogsMaterial, CodeOpexPayroll, CodeOpexRent, CodeOpexOther}
	periods := []Period{"2023", "2024", "2025"}
	items := make([]MappedLineItem, n)
	for i := 0; i < n; i++ {
		items[i] = MappedLineItem{
			SourceID: fmt.Sprintf("row-%d", i),
			Label:    fmt.Sprintf("Line %d", i),
			Code:     codes[i%len(codes)],
			Status:   RowStatusNormal,
			Values:   map[Period]float64{periods[i%len(periods)]: float64(i * 10)},
		}
	}
	return items
}

// BenchmarkNormalize_1000Rows exercises Normalize over 1,000 mapped line
// items — the candidate hot path named in docs/V1_CONTRACTS.md's
// performance sanity checks.
func BenchmarkNormalize_1000Rows(b *testing.B) {
	items := benchmarkMappedLineItems(1000)
	opts := NormalizeOptions{Currency: "USD"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Normalize(items, opts)
		if err != nil {
			b.Fatalf("unexpected Normalize error: %v", err)
		}
	}
}

// BenchmarkNormalize_1000Rows_WithProvenance is the same benchmark with
// IncludeProvenance enabled, since attaching SourceRef entries is
// meaningfully more allocation-heavy than the bare aggregation path.
func BenchmarkNormalize_1000Rows_WithProvenance(b *testing.B) {
	items := benchmarkMappedLineItems(1000)
	opts := NormalizeOptions{Currency: "USD", IncludeProvenance: true}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Normalize(items, opts)
		if err != nil {
			b.Fatalf("unexpected Normalize error: %v", err)
		}
	}
}
