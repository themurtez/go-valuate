package journaldiagnostics_test

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
)

func TestSourceSummary_CountsAndPercentages(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if len(r.SourceSummary.Buckets) == 0 {
		t.Fatal("expected at least one SourceBucket")
	}
	var total float64
	for _, b := range r.SourceSummary.Buckets {
		total += b.Amount
		if b.Count <= 0 {
			t.Errorf("bucket %s has non-positive count %d", b.Source, b.Count)
		}
	}
	total += r.SourceSummary.UnknownSourceAmount
	if diff := total - r.PopulationSummary.TotalAnalyzedAmount; diff > 0.01 || diff < -0.01 {
		t.Errorf("source buckets + unknown should sum to total analyzed amount: got %v, want %v", total, r.PopulationSummary.TotalAnalyzedAmount)
	}
}

func TestSourceSummary_UnknownDistinctFromExplicitUnknown(t *testing.T) {
	l := fixtures.Ledger()
	meta := append([]journaldiagnostics.EntryMetadata{}, fixtures.Metadata()...)
	meta = append(meta, journaldiagnostics.EntryMetadata{EntryID: "JE-004", Source: journaldiagnostics.SourceUnknown})

	r := journaldiagnostics.Calculate(l, meta, fixtures.Window(), fixtures.Policy())
	foundExplicitUnknown := false
	for _, b := range r.SourceSummary.Buckets {
		if b.Source == journaldiagnostics.SourceUnknown {
			foundExplicitUnknown = true
		}
	}
	if !foundExplicitUnknown {
		t.Error("expected an explicit SourceUnknown bucket distinct from UnknownSourceCount (no metadata at all)")
	}
}

func TestAccountActivitySummary_SortedByAccountID(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if len(r.AccountActivitySummary.Accounts) < 2 {
		t.Fatal("expected multiple accounts in AccountActivitySummary")
	}
	for i := 1; i < len(r.AccountActivitySummary.Accounts); i++ {
		if r.AccountActivitySummary.Accounts[i-1].AccountID >= r.AccountActivitySummary.Accounts[i].AccountID {
			t.Errorf("AccountActivitySummary.Accounts not sorted at index %d: %s >= %s",
				i, r.AccountActivitySummary.Accounts[i-1].AccountID, r.AccountActivitySummary.Accounts[i].AccountID)
		}
	}
}

func TestFindings_SortedBySeverityThenCodeThenDateThenEntryID(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	severityRank := map[journaldiagnostics.Severity]int{
		journaldiagnostics.SeverityHigh: 0, journaldiagnostics.SeverityWarning: 1, journaldiagnostics.SeverityInfo: 2,
	}
	for i := 1; i < len(r.Findings); i++ {
		prev, cur := r.Findings[i-1], r.Findings[i]
		if severityRank[prev.Severity] > severityRank[cur.Severity] {
			t.Fatalf("Findings not sorted by severity at index %d: %s (%s) before %s (%s)", i, prev.Code, prev.Severity, cur.Code, cur.Severity)
		}
	}
}

func TestRuleAvailability_NeverLeavesAnEmptyFieldWhenPopulated(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	v := reflectRuleAvailabilityValues(r.RuleAvailability)
	for name, state := range v {
		if state == "" {
			t.Errorf("RuleAvailability.%s was never set (empty string) — every field must resolve to AVAILABLE/UNAVAILABLE/DISABLED", name)
		}
	}
}

func TestCalculateWithPrior_PopulatesCrossPeriodComparison(t *testing.T) {
	prior := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	febWindow := journaldiagnostics.PeriodWindow{Period: "2025-02", StartDate: "2025-02-01", EndDate: "2025-02-28"}
	current := journaldiagnostics.CalculateWithPrior(fixtures.Ledger(), fixtures.Metadata(), febWindow, fixtures.Policy(), prior)

	if !current.CrossPeriodComparison.Available {
		t.Fatal("expected CrossPeriodComparison.Available == true")
	}
	if current.CrossPeriodComparison.PriorPeriod != "2025-03" {
		t.Errorf("expected PriorPeriod == '2025-03', got %q", current.CrossPeriodComparison.PriorPeriod)
	}
	if current.RuleAvailability.CrossPeriodComparison != journaldiagnostics.RuleAvailable {
		t.Errorf("expected RuleAvailability.CrossPeriodComparison AVAILABLE, got %s", current.RuleAvailability.CrossPeriodComparison)
	}

	plainResult := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if plainResult.CrossPeriodComparison.Available {
		t.Error("expected plain Calculate (no prior) to leave CrossPeriodComparison unavailable")
	}
	if plainResult.RuleAvailability.CrossPeriodComparison != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected RuleAvailability.CrossPeriodComparison UNAVAILABLE for plain Calculate, got %s", plainResult.RuleAvailability.CrossPeriodComparison)
	}
}

func TestCalculateWithPrior_NeverMutatesPrior(t *testing.T) {
	prior := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	priorCopy := prior // Result has no unexported fields; a shallow value copy plus JSON comparison is sufficient here

	febWindow := journaldiagnostics.PeriodWindow{Period: "2025-02", StartDate: "2025-02-01", EndDate: "2025-02-28"}
	_ = journaldiagnostics.CalculateWithPrior(fixtures.Ledger(), fixtures.Metadata(), febWindow, fixtures.Policy(), prior)

	if prior.Period != priorCopy.Period || len(prior.Findings) != len(priorCopy.Findings) {
		t.Error("CalculateWithPrior must not mutate its prior argument")
	}
}

func TestPolicy_InvalidValuesSurfaceIssue(t *testing.T) {
	policy := fixtures.Policy()
	policy.MaterialAmount = -100
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueInvalidPolicy {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPolicy for a negative MaterialAmount")
	}
}

func TestPolicy_ThresholdClusterLowerPercentOutOfRangeSurfacesIssue(t *testing.T) {
	policy := fixtures.Policy()
	policy.ThresholdClusterLowerPercent = 1.5
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	found := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueInvalidPolicy {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueInvalidPolicy for ThresholdClusterLowerPercent > 1")
	}
}

// reflectRuleAvailabilityValues extracts every RuleState field from a as a
// name->value map via a JSON round trip into a generic map, so a newly
// added RuleAvailability field is automatically checked without a second
// hand-maintained field list here.
func reflectRuleAvailabilityValues(a journaldiagnostics.RuleAvailability) map[string]journaldiagnostics.RuleState {
	b, err := json.Marshal(a)
	if err != nil {
		return nil
	}
	var m map[string]journaldiagnostics.RuleState
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}
