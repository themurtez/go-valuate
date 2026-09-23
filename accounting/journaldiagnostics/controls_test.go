package journaldiagnostics_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
)

func TestThresholdCluster_DetectsBandActivity(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingThresholdCluster)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-THRESH-01") && containsEntryID(f, "JE-THRESH-02") {
			matched = true
			if !f.Evidence.CombinedAmount.Available {
				t.Error("expected CombinedAmount evidence to be available")
			}
		}
	}
	if !matched {
		t.Error("expected JE-THRESH-01/02 ($9,500/$9,600, just under $10,000) to be flagged as THRESHOLD_CLUSTER")
	}
}

func TestThresholdCluster_UnavailableWithoutApprovalThreshold(t *testing.T) {
	policy := fixtures.Policy()
	policy.ApprovalThreshold = 0
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.ThresholdCluster != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected ThresholdCluster UNAVAILABLE without ApprovalThreshold, got %s", r.RuleAvailability.ThresholdCluster)
	}
}

func TestThresholdCluster_NeverInventsApprovalThreshold(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), journaldiagnostics.DefaultPolicy())
	if len(findingsWithCode(r.Findings, journaldiagnostics.FindingThresholdCluster)) != 0 {
		t.Error("DefaultPolicy has no ApprovalThreshold; this package must never invent one")
	}
}

func TestSplitEntryCluster_DetectsCombinedThresholdMeet(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingSplitEntryCluster)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-SPLIT-01") && containsEntryID(f, "JE-SPLIT-02") {
			matched = true
			if f.Evidence.CombinedAmount.Amount < 10000 {
				t.Errorf("expected combined amount >= approval threshold, got %v", f.Evidence.CombinedAmount.Amount)
			}
		}
	}
	if !matched {
		t.Error("expected JE-SPLIT-01/02 ($6,000 each, combining to $12,000) to be flagged as SPLIT_ENTRY_CLUSTER")
	}
}

func TestBlankDescription_AlwaysAvailable(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), nil, fixtures.Window(), fixtures.Policy())
	if r.RuleAvailability.BlankDescription != journaldiagnostics.RuleAvailable {
		t.Errorf("expected BlankDescription AVAILABLE even with no metadata, got %s", r.RuleAvailability.BlankDescription)
	}
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingBlankDescription)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-BLANK-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-BLANK-01 (empty Description) to be flagged as BLANK_DESCRIPTION")
	}
}

func TestGenericDescription_ExactNormalizedMatch(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingGenericDescription)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-GENERIC-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-GENERIC-01 (description 'Adjustment', in policy.GenericDescriptions) to be flagged as GENERIC_DESCRIPTION")
	}
}

func TestGenericDescription_DisabledWithoutPolicyList(t *testing.T) {
	policy := fixtures.Policy()
	policy.GenericDescriptions = nil
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.GenericDescription != journaldiagnostics.RuleDisabled {
		t.Errorf("expected GenericDescription DISABLED without a policy list, got %s", r.RuleAvailability.GenericDescription)
	}
}

func TestGenericDescription_NoHardCodedBroadHeuristic(t *testing.T) {
	// "Consulting invoice #501" is a normal, specific description; even
	// though it might loosely resemble some generic term to a fuzzy
	// matcher, this package uses exact normalized matching only.
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingGenericDescription) {
		if containsEntryID(f, "JE-002") {
			t.Error("a specific, non-matching description must never be flagged as generic")
		}
	}
}

func TestMissingReference_DetectsMaterialEntryMissingReference(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingMissingReference)
	if len(found) == 0 {
		t.Fatal("expected at least one MISSING_REFERENCE finding")
	}
}

func TestMissingReference_DisabledWithoutOptIn(t *testing.T) {
	policy := fixtures.Policy()
	policy.RequiredReferenceFields = nil
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)
	if r.RuleAvailability.MissingReference != journaldiagnostics.RuleDisabled {
		t.Errorf("expected MissingReference DISABLED without opt-in, got %s", r.RuleAvailability.MissingReference)
	}
}

func TestMissingReference_PresentReferenceNotFlagged(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingMissingReference) {
		if containsEntryID(f, "JE-LARGE-01") {
			t.Error("JE-LARGE-01 has an ExternalRef in its metadata and must not be flagged as missing a reference")
		}
	}
}

func TestSamePreparerApprover_Detects(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingSamePreparerApprover)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-SAMEPA-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-SAMEPA-01 (preparer==approver=='prep-D') to be flagged as SAME_PREPARER_APPROVER")
	}
}

func TestMissingApprover_Detects(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingMissingApprover)
	matched := false
	for _, f := range found {
		if containsEntryID(f, "JE-SPLIT-01") {
			matched = true
		}
	}
	if !matched {
		t.Error("expected JE-SPLIT-01 (has PreparerID 'prep-E' but no ApproverID) to be flagged as MISSING_APPROVER")
	}
}

func TestPreparerApproverDiagnostics_UnavailableWithoutAnyPreparerMetadata(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), nil, fixtures.Window(), fixtures.Policy())
	if r.RuleAvailability.PreparerApproverDiagnostics != journaldiagnostics.RuleUnavailable {
		t.Errorf("expected PreparerApproverDiagnostics UNAVAILABLE with no metadata, got %s", r.RuleAvailability.PreparerApproverDiagnostics)
	}
}

func TestHighVolumeByPreparer_Detects(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	found := findingsWithCode(r.Findings, journaldiagnostics.FindingHighVolumeByPreparer)
	matched := false
	for _, f := range found {
		if f.Evidence.PreparerID == "prep-A" {
			matched = true
		}
	}
	if !matched {
		t.Error("expected prep-A (JE-002/JE-LARGE-01/JE-WKND-01/JE-AFTERHRS-01 == 4 entries, meeting HighVolumePreparerCount=3) to be flagged")
	}
}

func TestPreparerApprover_OpaqueIDsPreservedExactly(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range findingsWithCode(r.Findings, journaldiagnostics.FindingSamePreparerApprover) {
		if containsEntryID(f, "JE-SAMEPA-01") {
			if f.Evidence.PreparerID != "prep-D" || f.Evidence.ApproverID != "prep-D" {
				t.Errorf("expected opaque IDs preserved exactly as supplied, got preparer=%q approver=%q", f.Evidence.PreparerID, f.Evidence.ApproverID)
			}
		}
	}
}
