package cashforecast

import "testing"

func TestCoverage_RequiredVsOptionalSource(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		// No AR/AP/Payroll supplied at all.
	}
	result := Calculate(in, Options{RequiredInputs: RequiredInputs{AR: true, Payroll: true}})

	if !result.Available {
		t.Fatalf("expected Available=true (missing required input is a finding, not a hard failure), got %+v", result.Issues)
	}
	if !hasIssueCode(result.Issues, IssueMissingRequiredInput) {
		t.Errorf("expected IssueMissingRequiredInput, got %+v", result.Issues)
	}
	wantMissing := map[string]bool{"AR": true, "PAYROLL": true}
	if len(result.Coverage.RequiredInputsMissing) != 2 {
		t.Fatalf("RequiredInputsMissing = %v, want 2 entries", result.Coverage.RequiredInputsMissing)
	}
	for _, m := range result.Coverage.RequiredInputsMissing {
		if !wantMissing[m] {
			t.Errorf("unexpected missing-input entry: %s", m)
		}
	}
	// AP was not marked required, so it must not appear even though it's
	// also unsupplied.
	for _, m := range result.Coverage.RequiredInputsMissing {
		if m == "AP" {
			t.Errorf("AP should not appear in RequiredInputsMissing since it was not marked required")
		}
	}
}

func TestCoverage_NoRequiredInputsMeansNoFindings(t *testing.T) {
	in := Input{ForecastStartDate: testDate(t, "2025-01-06"), OpeningCash: OpeningCash{Amount: 1000, Currency: "USD"}}
	result := Calculate(in, Options{}) // RequiredInputs left zero-value.

	if hasIssueCode(result.Issues, IssueMissingRequiredInput) {
		t.Errorf("did not expect IssueMissingRequiredInput when no category was marked required")
	}
	if len(result.Coverage.RequiredInputsMissing) != 0 {
		t.Errorf("expected empty RequiredInputsMissing, got %v", result.Coverage.RequiredInputsMissing)
	}
}

func TestCoverage_StaleSourceFlagged(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-02-01"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		ARSources:         []ARReceivableSource{{ReceivableID: "r1", OpenAmount: 5000}},
		ARSnapshotDate:    testDate(t, "2025-01-01"), // 31 days before ForecastStartDate.
	}
	result := Calculate(in, Options{Staleness: StalenessPolicy{MaxARAgeDays: 14}})

	if !result.Coverage.ARStaleness.Available {
		t.Fatalf("expected ARStaleness.Available=true")
	}
	if result.Coverage.ARStaleness.AgeDays != 31 {
		t.Errorf("ARStaleness.AgeDays = %d, want 31", result.Coverage.ARStaleness.AgeDays)
	}
	if !result.Coverage.ARStaleness.Stale {
		t.Errorf("expected ARStaleness.Stale=true (31 days > 14-day threshold)")
	}
	if !hasFlagCode(result.Flags, FlagStaleARSource) {
		t.Errorf("expected FlagStaleARSource, got %+v", result.Flags)
	}
}

func TestCoverage_FreshSourceNotFlagged(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-02-01"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		ARSources:         []ARReceivableSource{{ReceivableID: "r1", OpenAmount: 5000}},
		ARSnapshotDate:    testDate(t, "2025-01-30"), // 2 days before.
	}
	result := Calculate(in, Options{Staleness: StalenessPolicy{MaxARAgeDays: 14}})

	if result.Coverage.ARStaleness.Stale {
		t.Errorf("did not expect ARStaleness.Stale=true for a 2-day-old source under a 14-day threshold")
	}
	if hasFlagCode(result.Flags, FlagStaleARSource) {
		t.Errorf("did not expect FlagStaleARSource")
	}
}

func TestCoverage_NoThresholdMeansNeverStale(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-02-01"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		ARSources:         []ARReceivableSource{{ReceivableID: "r1", OpenAmount: 5000}},
		ARSnapshotDate:    testDate(t, "2024-01-01"), // very old, but no threshold supplied.
	}
	result := Calculate(in, Options{}) // Staleness left zero-value.

	if !result.Coverage.ARStaleness.Available {
		t.Fatalf("expected ARStaleness.Available=true (age is always reported when AsOfDate is known)")
	}
	if result.Coverage.ARStaleness.Stale {
		t.Errorf("did not expect Stale=true without an explicit staleness policy")
	}
}

func hasFlagCode(flags []Flag, code FlagCode) bool {
	for _, f := range flags {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestCoverage_SuppliedFlagsPerCategory(t *testing.T) {
	in := Input{
		ForecastStartDate: testDate(t, "2025-01-06"),
		OpeningCash:       OpeningCash{Amount: 1000, Currency: "USD"},
		Payroll:           []PayrollEvent{{ID: "p1", Date: testDate(t, "2025-01-10"), EmployeeNetCash: 1000}},
		Capex:             []CapexEvent{{ID: "c1", Date: testDate(t, "2025-01-10"), Amount: 500}},
	}
	result := Calculate(in, Options{})

	if !result.Coverage.Payroll.Supplied {
		t.Errorf("expected Coverage.Payroll.Supplied=true")
	}
	if !result.Coverage.Capex.Supplied {
		t.Errorf("expected Coverage.Capex.Supplied=true")
	}
	if result.Coverage.DebtService.Supplied {
		t.Errorf("expected Coverage.DebtService.Supplied=false")
	}
	if result.Coverage.Tax.Supplied {
		t.Errorf("expected Coverage.Tax.Supplied=false")
	}
}
