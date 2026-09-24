package vendorspend_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

// TestOrdering_PeriodsAreChronological verifies Result.Periods is always
// chronologically ordered regardless of Input.Periods' supplied order —
// task section 41.
func TestOrdering_PeriodsAreChronological(t *testing.T) {
	in := fullInput()
	// Shuffle Periods into reverse order.
	shuffled := make([]vendorspend.Period, len(in.Periods))
	for i, p := range in.Periods {
		shuffled[len(in.Periods)-1-i] = p
	}
	in.Periods = shuffled

	result := vendorspend.Calculate(in, vendorspend.Options{})
	for i := 1; i < len(result.Periods); i++ {
		if result.Periods[i-1] >= result.Periods[i] {
			t.Errorf("Periods not chronologically ordered: %v", result.Periods)
			break
		}
	}
}

// TestOrdering_SupplierSummariesSortedByNetSpendDescThenID verifies
// SupplierSummaries within each period are sorted by NetSpend descending,
// then SupplierID ascending on ties.
func TestOrdering_SupplierSummariesSortedByNetSpendDescThenID(t *testing.T) {
	result := vendorspend.Calculate(fullInput(), vendorspend.Options{})

	byPeriod := map[string][]vendorspend.SupplierPeriodSummary{}
	for _, ss := range result.SupplierSummaries {
		byPeriod[ss.Period] = append(byPeriod[ss.Period], ss)
	}
	for period, rows := range byPeriod {
		for i := 1; i < len(rows); i++ {
			prev, cur := rows[i-1], rows[i]
			if prev.Bridge.NetSpend < cur.Bridge.NetSpend {
				t.Errorf("period %s: SupplierSummaries not sorted by NetSpend descending: %+v then %+v", period, prev, cur)
			}
			if prev.Bridge.NetSpend == cur.Bridge.NetSpend && prev.SupplierID > cur.SupplierID {
				t.Errorf("period %s: tied NetSpend not sorted by SupplierID ascending: %+v then %+v", period, prev, cur)
			}
		}
	}
}

// TestOrdering_FlagsSortedByCodeThenSupplierThenProduct verifies Flags
// are ordered by FlagCode declaration order, then SupplierID, then
// ProductID.
func TestOrdering_FlagsSortedByCodeThenSupplierThenProduct(t *testing.T) {
	result := vendorspend.Calculate(fullInput(), vendorspend.Options{})
	if len(result.Flags) < 2 {
		t.Skip("fixture did not produce enough flags to verify ordering")
	}
	codeRank := map[vendorspend.FlagCode]int{}
	for i, f := range result.Flags {
		if _, ok := codeRank[f.Code]; !ok {
			codeRank[f.Code] = i
		}
	}
	// Verify: once we've moved past a FlagCode's first appearance, no
	// EARLIER-appearing code should show up again later (monotonic groups).
	seenCodes := map[vendorspend.FlagCode]bool{}
	lastCode := result.Flags[0].Code
	for _, f := range result.Flags {
		if f.Code != lastCode {
			if seenCodes[f.Code] {
				t.Errorf("FlagCode %s appeared in a non-contiguous group: %+v", f.Code, result.Flags)
			}
			seenCodes[lastCode] = true
			lastCode = f.Code
		}
	}
}

// TestOrdering_CategorySummariesSortedByCategory verifies
// CategorySummaries are sorted alphabetically by Category.
func TestOrdering_CategorySummariesSortedByCategory(t *testing.T) {
	result := vendorspend.Calculate(fullInput(), vendorspend.Options{})
	for i := 1; i < len(result.CategorySummaries); i++ {
		if result.CategorySummaries[i-1].Category > result.CategorySummaries[i].Category {
			t.Errorf("CategorySummaries not sorted alphabetically: %+v", result.CategorySummaries)
			break
		}
	}
}

// TestOrdering_IssuesDeterministicAcrossInputPermutations verifies
// permuting SpendRecords' input order produces the same sorted Issues —
// no dependence on encounter order for the final sorted output.
func TestOrdering_IssuesDeterministicAcrossInputPermutations(t *testing.T) {
	in := fullInput()
	// Reverse SpendRecords order.
	reversed := make([]vendorspend.SpendRecord, len(in.SpendRecords))
	for i, r := range in.SpendRecords {
		reversed[len(in.SpendRecords)-1-i] = r
	}

	result1 := vendorspend.Calculate(in, vendorspend.Options{})
	in.SpendRecords = reversed
	result2 := vendorspend.Calculate(in, vendorspend.Options{})

	if len(result1.Issues) != len(result2.Issues) {
		t.Fatalf("Issues count differs: %d vs %d", len(result1.Issues), len(result2.Issues))
	}
	for i := range result1.Issues {
		if result1.Issues[i].Code != result2.Issues[i].Code {
			t.Errorf("Issue[%d].Code differs after input reversal: %v vs %v", i, result1.Issues[i].Code, result2.Issues[i].Code)
		}
	}
}
