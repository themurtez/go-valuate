package review

import (
	"fmt"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

// benchmarkClassificationInput builds n synthetic RawLineItems and their
// already-classified Results, cycling through a mix of confident,
// low-confidence, and UNKNOWN outcomes — representative of a large
// multi-year statement's full classification set, which is exactly what
// Build's KindClassification item construction iterates over.
func benchmarkClassificationInput(n int) ([]financial.RawLineItem, []classification.Result) {
	raws := make([]financial.RawLineItem, n)
	results := make([]classification.Result, n)
	codes := []financial.Code{financial.CodeRevProduct, financial.CodeCogsMaterial, financial.CodeOpexPayroll}
	for i := 0; i < n; i++ {
		raws[i] = financial.RawLineItem{ID: fmt.Sprintf("row-%d", i), Label: fmt.Sprintf("Line %d", i), StatementType: financial.StatementIncomeStatement}
		switch i % 3 {
		case 0:
			results[i] = classification.Result{Code: codes[i%len(codes)], Confidence: 0.98, Source: classification.SourceAlias}
		case 1:
			results[i] = classification.Result{Code: codes[i%len(codes)], Confidence: 0.70, Source: classification.SourcePhraseRule, ReviewRequired: true}
		default:
			results[i] = classification.Result{Source: classification.SourceUnknown, ReviewRequired: true}
		}
	}
	return raws, results
}

// BenchmarkBuild_1000Rows exercises Build over a 1,000-row classification
// set (a mix of confident/low-confidence/UNKNOWN results, so every
// severity/materiality branch in buildClassificationItems does real work)
// — the candidate hot path named in docs/V1_CONTRACTS.md's performance
// sanity checks.
func BenchmarkBuild_1000Rows(b *testing.B) {
	raws, results := benchmarkClassificationInput(1000)
	policy := DefaultPolicy()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Build(BuildInput{Classifications: results, Raws: raws}, policy)
	}
}
