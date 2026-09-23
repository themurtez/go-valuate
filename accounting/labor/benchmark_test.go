package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/labor/fixtures"
)

// BenchmarkCalculate_LargePopulation exercises 10,000 workers across 60
// historical monthly periods (600,000 payroll records) spread across 100
// departments/cost centers — well beyond the task's section 67 minimum
// (100,000 payroll records / 10,000 workers / 100 departments / 60
// periods) — to check for accidental O(N^2) behavior in aggregation.
//
// This benchmark caught two real quadratic-in-practice bugs during
// development: (1) sortedStringKeys re-normalized both comparator
// operands on every one of its O(N log N) sort comparisons instead of
// normalizing each key once up front, dominated by
// runtime.slicerunetostring/encoderune at large N (fixed in sort.go);
// (2) Calculate re-scanned the full validated payroll/contractor slices
// once per period via recordsForPeriod/contractorsForPeriod (O(N*P))
// instead of partitioning once (O(N)) (fixed in calculate.go via
// partitionPayrollByPeriod/partitionContractorsByPeriod). Together these
// took the 600,000-record/60-period case from ~22.5s to ~4.8s on the
// same hardware.
func BenchmarkCalculate_LargePopulation(b *testing.B) {
	periods, workers, records := fixtures.LargePopulation(10000, 60, 100)
	in := labor.Input{Periods: periods, Workers: workers, PayrollRecords: records}
	policy := labor.DefaultPolicy()
	policy.StandardFullTimeHoursPerPeriod = 173.33

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = labor.Calculate(in, policy)
	}
}

// BenchmarkCalculate_ScalingLinearity runs Calculate at two population
// sizes (10x apart) and reports ns/op for each, so `go test -bench`
// output makes it easy to spot-check that per-record cost does not grow
// superlinearly (an O(N^2) aggregation would show a >10x-per-record
// slowdown between the two).
func BenchmarkCalculate_ScalingLinearity(b *testing.B) {
	sizes := []int{1000, 10000}
	for _, n := range sizes {
		periods, workers, records := fixtures.LargePopulation(n, 12, 20)
		in := labor.Input{Periods: periods, Workers: workers, PayrollRecords: records}
		policy := labor.DefaultPolicy()

		b.Run(itoaBench(n)+"_workers", func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = labor.Calculate(in, policy)
			}
		})
	}
}

// BenchmarkFindPossibleDuplicates specifically exercises the duplicate-
// detection scan (section 66 "duplicate detection confirmed linear via
// scaling benchmark" precedent from journaldiagnostics) at 100,000
// payroll records.
func BenchmarkCalculate_DuplicateScan(b *testing.B) {
	_, _, records := fixtures.LargePopulation(2000, 50, 10) // 100,000 records
	in := labor.Input{Periods: nil, PayrollRecords: records}
	// Supply matching periods so records are not excluded.
	periods, _, _ := fixtures.LargePopulation(2000, 50, 10)
	in.Periods = periods
	policy := labor.DefaultPolicy()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = labor.Calculate(in, policy)
	}
}

func itoaBench(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
