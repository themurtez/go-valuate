package reconciliation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func loadFixtureDataset(t *testing.T, name string) financial.FinancialDataset {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var ds financial.FinancialDataset
	if err := json.Unmarshal(b, &ds); err != nil {
		t.Fatalf("unmarshaling fixture %s: %v", name, err)
	}
	return ds
}

func TestFixtures_CleanArchetypesBalanceForEveryPeriod(t *testing.T) {
	for _, name := range []string{
		"normalized_hvac_multi_year.json",
		"normalized_agency_multi_year.json",
		"normalized_manufacturer_multi_year.json",
		"normalized_saas_multi_year.json",
	} {
		t.Run(name, func(t *testing.T) {
			ds := loadFixtureDataset(t, name)
			result := Run(ds, Options{})

			periods := ds.Periods()
			if len(periods) != 3 {
				t.Fatalf("expected 3 fiscal years in %s, got %d", name, len(periods))
			}
			for _, period := range periods {
				check := mustFindCheck(t, result, CheckBalanceSheetBalances, string(period))
				if check.Status != StatusPass {
					t.Errorf("%s: balance sheet check for %s = %s, want PASS: %s", name, period, check.Status, check.Explanation)
				}
			}
			if result.HasFailures() {
				for _, c := range result.Checks {
					if c.Status == StatusFail {
						t.Errorf("%s: unexpected FAIL: %s/%s: %s", name, c.Code, c.Period, c.Explanation)
					}
				}
			}
		})
	}
}

func TestFixtures_InconsistentDatasetFailsBalanceSheetCheck(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_inconsistent_balance_sheet.json")
	result := Run(ds, Options{})
	check := mustFindCheck(t, result, CheckBalanceSheetBalances, "2025")
	if check.Status != StatusFail {
		t.Errorf("status = %v, want FAIL: %s", check.Status, check.Explanation)
	}
	if check.Difference == nil {
		t.Fatal("expected a non-nil Difference")
	}
	if *check.Difference != 50000 {
		t.Errorf("difference = %v, want 50000 (the fixture's intentional imbalance)", *check.Difference)
	}
}

func TestFixtures_ManufacturerHasNoOwnerCompensation(t *testing.T) {
	// The manufacturer archetype has no owner-operator: SDE should equal
	// EBITDA exactly (owner comp contributes 0), verified indirectly via
	// the metrics-backed checks not erroring out.
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	result := Run(ds, Options{})
	if result.HasFailures() {
		t.Fatal("did not expect any FAIL checks for the manufacturer fixture")
	}
}
