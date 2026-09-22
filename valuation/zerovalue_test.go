package valuation_test

import (
	"testing"

	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/sde"
)

// TestZeroValue_SDEInput_NeverPanics proves sde.Calculate(sde.Input{}) —
// MaintainableSDE and Multiple both 0 — never panics: a zero Multiple is
// caught by validation (blocking, IssueNonPositiveMultiple) BEFORE any
// arithmetic runs, per the package's "never panics on bad financial
// input" guarantee.
func TestZeroValue_SDEInput_NeverPanics(t *testing.T) {
	res := sde.Calculate(sde.Input{})
	if res.Available {
		t.Error("expected Available=false for a zero (non-positive) Multiple")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one blocking Issue for a zero Multiple")
	}
	if res.EquityValue != 0 {
		t.Errorf("expected the headline value to stay 0 when unavailable, got %v", res.EquityValue)
	}
}

// TestZeroValue_CapitalizationInput_NeverPanics proves
// capitalization.Calculate(Input{}) — CapitalizationRate 0, the direct
// divide-by-zero candidate — is caught by validation before the division
// runs, never producing NaN/Inf or a panic.
func TestZeroValue_CapitalizationInput_NeverPanics(t *testing.T) {
	res := capitalization.Calculate(capitalization.Input{})
	if res.Available {
		t.Error("expected Available=false for a zero CapitalizationRate")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one blocking Issue for a zero CapitalizationRate")
	}
}

// TestZeroValue_DCFInput_NeverPanics proves dcf.Calculate(Input{}) — an
// empty ForecastPeriods, the "nothing to discount" case, plus
// DiscountRate/TerminalGrowthRate both 0 (which would otherwise divide by
// zero in the Gordon Growth terminal-value formula) — never panics.
func TestZeroValue_DCFInput_NeverPanics(t *testing.T) {
	res := dcf.Calculate(dcf.Input{})
	if res.Available {
		t.Error("expected Available=false for an empty ForecastPeriods")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one blocking Issue for an empty ForecastPeriods")
	}
}

// TestZeroValue_Profile_ApplicabilityNeverPanics proves
// applicability.Calculate(profile.Profile{}) — every field unset — is
// explicitly documented as valid input, producing every method's neutral
// baseline score, with DCF as the one method that starts hard-blocked
// (NOT_APPLICABLE) absent an explicit forecast signal.
func TestZeroValue_Profile_ApplicabilityNeverPanics(t *testing.T) {
	results := applicability.Calculate(profile.Profile{})
	if len(results.Methods) == 0 {
		t.Fatal("expected every method to still produce a Result for an empty Profile")
	}
	dcfResult, ok := results.ForMethod("DCF")
	if !ok {
		t.Fatal("expected a DCF result to be present")
	}
	if dcfResult.HardBlockReason == "" {
		t.Error("expected DCF to be hard-blocked (HardBlockReason populated) for a Profile with no forecast signal")
	}
}

// TestZeroValue_ConsensusOptions_EmptyInputsNeverPanics proves
// consensus.Calculate(nil, consensus.Options{}) is a safe zero value: an
// empty inputs slice is checked first and returns a structured error
// Result, never a panic on an empty slice or an unset TargetBasis.
func TestZeroValue_ConsensusOptions_EmptyInputsNeverPanics(t *testing.T) {
	result := consensus.Calculate(nil, consensus.Options{})
	if result.Available {
		t.Error("expected Available=false for an empty inputs slice")
	}
	if len(result.Errors) == 0 {
		t.Error("expected at least one Issue explaining why the result is unavailable")
	}
}
