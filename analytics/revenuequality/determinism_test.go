package revenuequality

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// workingcapital/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_agency_multi_year.json")
	meta := threeYearMeta()
	customers := []CustomerPeriodRevenue{
		{CustomerKey: "alpha", Period: "2023", Amount: 40000, Segment: "enterprise", RecurringFlag: boolPtr(true)},
		{CustomerKey: "beta", Period: "2023", Amount: 20000, Segment: "smb"},
		{CustomerKey: "alpha", Period: "2024", Amount: 45000, Segment: "enterprise", RecurringFlag: boolPtr(true)},
		{CustomerKey: "gamma", Period: "2024", Amount: 15000, Segment: "smb"},
		{CustomerKey: "alpha", Period: "2025", Amount: 50000, Segment: "enterprise", RecurringFlag: boolPtr(true)},
		{CustomerKey: "delta", Period: "2025", Amount: 30000, Segment: "smb"},
	}

	in := Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}
	opts := Options{}

	first, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DeterministicAcrossMapOrdering proves Calculate's output
// does not depend on Go's randomized map iteration order, using a dataset
// with many customers/segments/codes that surfaces map-order dependencies
// in buildIndex, customerTotalsByKey, groupCustomerRevenueByPeriod, and
// calculateSegmentShares (all of which internally use maps before sorting
// into deterministic slices).
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	ds := newDataset().
		add(financial.CodeRevRecurring, "2024", 200000).
		add(financial.CodeRevProduct, "2024", 50000).
		add(financial.CodeRevRecurring, "2025", 240000).
		add(financial.CodeRevProduct, "2025", 60000).
		build()
	meta := map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}

	var customers []CustomerPeriodRevenue
	segments := []string{"east", "west", "north", "south", "central"}
	for i := 0; i < 30; i++ {
		key := "customer-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		seg := segments[i%len(segments)]
		customers = append(customers,
			CustomerPeriodRevenue{CustomerKey: key, Period: "2024", Amount: float64(1000 + i*137), Segment: seg, RecurringFlag: boolPtr(i%2 == 0)},
			CustomerPeriodRevenue{CustomerKey: key, Period: "2025", Amount: float64(1100 + i*151), Segment: seg, RecurringFlag: boolPtr(i%2 == 0)},
		)
	}

	in := Input{Dataset: ds, PeriodMeta: meta, CustomerRevenue: customers}
	opts := Options{}

	first, err := json.Marshal(Calculate(in, opts))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in, opts))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs; suspect map-order nondeterminism", i)
		}
	}
}
