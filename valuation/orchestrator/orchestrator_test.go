package orchestrator

import (
	"testing"

	"github.com/themurtez/go-valuate/settings"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/sde"
)

func fullRequest() Request {
	return Request{
		SDE:            &sde.Input{MaintainableSDE: 300_000, Multiple: 2.5},
		EBITDA:         &ebitda.Input{MaintainableEBITDA: 400_000, Multiple: 3.5},
		Capitalization: &capitalization.Input{MaintainableEarnings: 300_000, CapitalizationRate: 0.25},
		DCF: &dcf.Input{
			ForecastPeriods: []dcf.ForecastPeriod{
				{Period: "2026", FreeCashFlow: 100_000},
				{Period: "2027", FreeCashFlow: 110_000},
			},
			DiscountRate:       0.18,
			TerminalGrowthRate: 0.03,
		},
		NetAssets: &netassets.Input{
			Assets:      []netassets.AssetItem{{Label: "Cash", Amount: 100_000}},
			Liabilities: []netassets.LiabilityItem{{Label: "AP", Amount: 20_000}},
		},
	}
}

func outcomeFor(t *testing.T, run Run, code valuation.Code) MethodOutcome {
	t.Helper()
	for _, m := range run.Methods {
		if m.Method == code {
			return m
		}
	}
	t.Fatalf("no outcome for method %q", code)
	return MethodOutcome{}
}

func TestExecute_AllMethodsSuccessful(t *testing.T) {
	run := Execute(fullRequest())

	if len(run.Methods) != 5 {
		t.Fatalf("expected 5 method outcomes, got %d", len(run.Methods))
	}
	for _, m := range run.Methods {
		if m.Outcome != OutcomeSuccess {
			t.Errorf("%s: outcome = %v, want success", m.Method, m.Outcome)
		}
		if !m.Available {
			t.Errorf("%s: Available = false, want true", m.Method)
		}
	}
	if len(run.Excluded()) != 0 {
		t.Errorf("expected no excluded methods, got %d", len(run.Excluded()))
	}
	if len(run.Unavailable()) != 0 {
		t.Errorf("expected no unavailable methods, got %d", len(run.Unavailable()))
	}
	if len(run.Successful()) != 5 {
		t.Errorf("expected 5 successful methods, got %d", len(run.Successful()))
	}
}

func TestExecute_OneMethodFails(t *testing.T) {
	req := fullRequest()
	// Invalid multiple makes EBITDA's own Calculate blocked/unavailable.
	req.EBITDA = &ebitda.Input{MaintainableEBITDA: 400_000, Multiple: -1}

	run := Execute(req)

	ebitdaOutcome := outcomeFor(t, run, valuation.CodeEBITDAMultiple)
	if ebitdaOutcome.Outcome != OutcomeUnavailable {
		t.Errorf("EBITDA outcome = %v, want unavailable", ebitdaOutcome.Outcome)
	}
	if ebitdaOutcome.EBITDA == nil {
		t.Fatal("expected EBITDA Result to be populated even when unavailable")
	}
	if len(ebitdaOutcome.EBITDA.Errors) == 0 {
		t.Error("expected EBITDA Result to carry its own Errors")
	}

	// Every other method must remain unaffected.
	for _, code := range []valuation.Code{valuation.CodeSDEMultiple, valuation.CodeCapitalizationOfEarnings, valuation.CodeDCF, valuation.CodeAdjustedNetAssetValue} {
		m := outcomeFor(t, run, code)
		if m.Outcome != OutcomeSuccess {
			t.Errorf("%s: outcome = %v, want success (must be unaffected by EBITDA's failure)", code, m.Outcome)
		}
	}
}

func TestExecute_OneMethodExcluded_NoInput(t *testing.T) {
	req := fullRequest()
	req.NetAssets = nil

	run := Execute(req)

	m := outcomeFor(t, run, valuation.CodeAdjustedNetAssetValue)
	if m.Outcome != OutcomeExcluded {
		t.Errorf("NetAssets outcome = %v, want excluded", m.Outcome)
	}
	if m.ExclusionReason != ExclusionNoInput {
		t.Errorf("NetAssets exclusion reason = %v, want %v", m.ExclusionReason, ExclusionNoInput)
	}
	if m.NetAssets != nil {
		t.Error("expected NetAssets Result to be nil when excluded")
	}
}

func TestExecute_MissingDCFForecast_ExcludedNotFailed(t *testing.T) {
	req := fullRequest()
	req.DCF = nil

	run := Execute(req)
	m := outcomeFor(t, run, valuation.CodeDCF)
	if m.Outcome != OutcomeExcluded || m.ExclusionReason != ExclusionNoInput {
		t.Errorf("DCF with nil input: outcome = %v reason = %v, want excluded/no_input", m.Outcome, m.ExclusionReason)
	}
}

func TestExecute_DCFInputSuppliedButEmptyForecast_Unavailable(t *testing.T) {
	req := fullRequest()
	req.DCF = &dcf.Input{DiscountRate: 0.18, TerminalGrowthRate: 0.03}

	run := Execute(req)
	m := outcomeFor(t, run, valuation.CodeDCF)
	if m.Outcome != OutcomeUnavailable {
		t.Errorf("DCF with empty ForecastPeriods: outcome = %v, want unavailable (attempted, not excluded)", m.Outcome)
	}
	if m.DCF == nil || m.DCF.Available {
		t.Error("expected DCF Result to be populated and unavailable")
	}
}

func TestExecute_SettingsDisableMethod(t *testing.T) {
	req := fullRequest()
	req.Resolution = settings.Resolve(
		settings.Settings{},
		settings.Settings{},
		settings.Settings{},
		settings.Settings{MethodEnabled: map[settings.Method]*bool{settings.MethodSDE: settings.Bool(false)}},
	)

	run := Execute(req)
	m := outcomeFor(t, run, valuation.CodeSDEMultiple)
	if m.Outcome != OutcomeExcluded {
		t.Errorf("SDE outcome = %v, want excluded", m.Outcome)
	}
	if m.ExclusionReason != ExclusionDisabledBySettings {
		t.Errorf("SDE exclusion reason = %v, want %v", m.ExclusionReason, ExclusionDisabledBySettings)
	}
	// Other methods remain enabled by default (no explicit entry).
	for _, code := range []valuation.Code{valuation.CodeEBITDAMultiple, valuation.CodeCapitalizationOfEarnings, valuation.CodeDCF, valuation.CodeAdjustedNetAssetValue} {
		m := outcomeFor(t, run, code)
		if m.Outcome != OutcomeSuccess {
			t.Errorf("%s: outcome = %v, want success", code, m.Outcome)
		}
	}
}

func TestExecute_SettingsDisabledTakesPrecedenceOverInput(t *testing.T) {
	// A method explicitly disabled by settings should be reported as
	// disabled even if an Input was also supplied for it.
	req := fullRequest()
	req.Resolution = settings.Resolve(
		settings.Settings{}, settings.Settings{}, settings.Settings{},
		settings.Settings{MethodEnabled: map[settings.Method]*bool{settings.MethodDCF: settings.Bool(false)}},
	)
	run := Execute(req)
	m := outcomeFor(t, run, valuation.CodeDCF)
	if m.ExclusionReason != ExclusionDisabledBySettings {
		t.Errorf("exclusion reason = %v, want %v even though Input was supplied", m.ExclusionReason, ExclusionDisabledBySettings)
	}
}

func TestExecute_EmptyResolutionDefaultsEnabled(t *testing.T) {
	req := fullRequest()
	req.Resolution = settings.Resolution{} // zero value, no Values map entries
	run := Execute(req)
	for _, m := range run.Methods {
		if m.Outcome != OutcomeSuccess {
			t.Errorf("%s: outcome = %v, want success under a zero-value Resolution", m.Method, m.Outcome)
		}
	}
}

func TestExecute_MinApplicabilityLevelExcludesLowScoring(t *testing.T) {
	req := fullRequest()
	falseVal := false
	results := applicability.Calculate(profile.Profile{
		OwnerOperated:  &falseVal,
		AssetIntensity: floatPtr(0.05),
	})
	req.Applicability = &results
	req.MinApplicabilityLevel = applicability.LevelMedium

	run := Execute(req)

	for _, m := range run.Methods {
		if m.Applicability == nil {
			t.Errorf("%s: expected Applicability to be echoed", m.Method)
			continue
		}
		rank := levelRank(m.Applicability.Level)
		wantExcluded := rank < levelRank(applicability.LevelMedium)
		if wantExcluded && m.Outcome != OutcomeExcluded {
			t.Errorf("%s: level %v below MinApplicabilityLevel but outcome = %v", m.Method, m.Applicability.Level, m.Outcome)
		}
		if wantExcluded && m.ExclusionReason != ExclusionLowApplicability {
			t.Errorf("%s: expected ExclusionLowApplicability, got %v", m.Method, m.ExclusionReason)
		}
	}
}

func TestExecute_ApplicabilityEchoedWithoutFiltering(t *testing.T) {
	req := fullRequest()
	results := applicability.Calculate(profile.Profile{})
	req.Applicability = &results
	// MinApplicabilityLevel intentionally left empty: no filtering.

	run := Execute(req)
	for _, m := range run.Methods {
		if m.Applicability == nil {
			t.Errorf("%s: expected Applicability echoed even without filtering", m.Method)
		}
	}
	// Every method still ran since filtering is opt-in.
	if len(run.Successful()) != 5 {
		t.Errorf("expected all 5 methods to succeed with no MinApplicabilityLevel set, got %d", len(run.Successful()))
	}
}

func TestExecute_WarningsCollectedAcrossMethods(t *testing.T) {
	req := fullRequest()
	req.SDE = &sde.Input{MaintainableSDE: -50_000, Multiple: 2.5} // triggers a warning, not an error

	run := Execute(req)
	m := outcomeFor(t, run, valuation.CodeSDEMultiple)
	if m.Outcome != OutcomeSuccess {
		t.Fatalf("negative SDE should still be Available (a warning, not an error): outcome = %v", m.Outcome)
	}
	found := false
	for _, w := range run.Warnings {
		if w.Method == valuation.CodeSDEMultiple {
			found = true
		}
	}
	if !found {
		t.Error("expected run.Warnings to include SDE's negative-SDE warning")
	}
}

func TestExecute_MethodOrderIsFixed(t *testing.T) {
	run := Execute(fullRequest())
	want := []valuation.Code{
		valuation.CodeSDEMultiple,
		valuation.CodeEBITDAMultiple,
		valuation.CodeCapitalizationOfEarnings,
		valuation.CodeDCF,
		valuation.CodeAdjustedNetAssetValue,
	}
	for i, code := range want {
		if run.Methods[i].Method != code {
			t.Errorf("Methods[%d] = %v, want %v", i, run.Methods[i].Method, code)
		}
	}
}

func floatPtr(v float64) *float64 { return &v }
