package valuedrivers

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/capitalization"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/ebitda"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/profile"
	"github.com/themurtez/go-valuate/valuation/sde"
)

// baselineRequest builds a realistic orchestrator.Request covering four of
// the five methods (SDE, EBITDA, Capitalization, DCF), deliberately
// omitting NetAssets so every test can exercise LinkageMethodExcluded
// without a separate fixture. EBITDA and DCF both request an equity
// bridge so debt-driver tests have something to adjust; SDE deliberately
// does not, so debt-driver tests can also exercise "bridge not requested"
// LinkageNotApplicable.
func baselineRequest() orchestrator.Request {
	return orchestrator.Request{
		SDE: &sde.Input{MaintainableSDE: 1_000_000, Multiple: 2.5},
		EBITDA: &ebitda.Input{
			MaintainableEBITDA: 2_000_000, Multiple: 4.0,
			EquityBridge: ebitda.EquityBridgeInput{Requested: true, ExcessCash: 200_000, LongTermDebt: 500_000},
		},
		Capitalization: &capitalization.Input{MaintainableEarnings: 1_200_000, CapitalizationRate: 0.20},
		DCF: &dcf.Input{
			ForecastPeriods: []dcf.ForecastPeriod{
				{Period: "Year 1", FreeCashFlow: 900_000},
				{Period: "Year 2", FreeCashFlow: 950_000},
				{Period: "Year 3", FreeCashFlow: 1_000_000},
			},
			DiscountRate: 0.15, TerminalGrowthRate: 0.03,
			EquityBridge: dcf.EquityBridgeInput{Requested: true, ExcessCash: 200_000, LongTermDebt: 500_000},
		},
	}
}

func baselineWeights() map[valuation.Code]float64 {
	return map[valuation.Code]float64{
		valuation.CodeSDEMultiple: 1, valuation.CodeEBITDAMultiple: 1,
		valuation.CodeCapitalizationOfEarnings: 1, valuation.CodeDCF: 1,
	}
}

func baseInput() Input {
	return Input{
		BaselineRequest:  baselineRequest(),
		ConsensusOptions: consensus.Options{TargetBasis: valuation.ValueTypeEquity},
		Weights:          baselineWeights(),
	}
}

func findScenario(results []ScenarioResult, id string) (ScenarioResult, bool) {
	for _, r := range results {
		if r.ScenarioID == id {
			return r, true
		}
	}
	return ScenarioResult{}, false
}

func findMethodDelta(deltas []MethodDelta, code valuation.Code) (MethodDelta, bool) {
	for _, d := range deltas {
		if d.Method == code {
			return d, true
		}
	}
	return MethodDelta{}, false
}

func findLinkage(linkages []Linkage, code valuation.Code) (Linkage, bool) {
	for _, l := range linkages {
		if l.Method == code {
			return l, true
		}
	}
	return Linkage{}, false
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

// TestCalculate_Baseline proves the baseline Run/Consensus reflects an
// unmutated re-run of Input.BaselineRequest.
func TestCalculate_Baseline(t *testing.T) {
	res := Calculate(baseInput())
	if !res.Available {
		t.Fatal("expected Result.Available")
	}
	if !res.Baseline.Consensus.Available {
		t.Fatalf("expected baseline consensus available, errors=%+v", res.Baseline.Consensus.Errors)
	}
	// SDE: 1,000,000 * 2.5 = 2,500,000
	wantSDE := 2_500_000.0
	sdeOutcome, ok := findOutcomeInRun(res.Baseline.Run, valuation.CodeSDEMultiple)
	if !ok || sdeOutcome.SDE == nil || !approxEqual(sdeOutcome.SDE.EquityValue, wantSDE) {
		t.Fatalf("expected baseline SDE equity value %.2f, got %+v", wantSDE, sdeOutcome)
	}
	if res.Baseline.Run.Methods[4].Method != valuation.CodeAdjustedNetAssetValue || res.Baseline.Run.Methods[4].Outcome != orchestrator.OutcomeExcluded {
		t.Fatalf("expected NetAssets excluded in baseline (no input supplied), got %+v", res.Baseline.Run.Methods[4])
	}
}

func findOutcomeInRun(run orchestrator.Run, code valuation.Code) (orchestrator.MethodOutcome, bool) {
	for _, m := range run.Methods {
		if m.Method == code {
			return m, true
		}
	}
	return orchestrator.MethodOutcome{}, false
}

// TestDriverRevenueGrowth covers the "revenue growth" driver example from
// the package brief: every earnings-linked method scales up, and NetAssets
// (excluded at baseline) stays excluded.
func TestDriverRevenueGrowth(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "REV_GROWTH_10", Label: "10% revenue growth", Type: DriverRevenueGrowth,
		RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.10},
	}}
	res := Calculate(in)

	sr, ok := findScenario(res.OneFactorAtATime, "REV_GROWTH_10")
	if !ok || !sr.Available {
		t.Fatalf("expected scenario result, got ok=%v sr=%+v", ok, sr)
	}

	sdeDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeSDEMultiple)
	wantSDEValue := 1_000_000 * 1.10 * 2.5
	if !sdeDelta.DeltaAvailable || !approxEqual(sdeDelta.Scenario.Value, wantSDEValue) {
		t.Fatalf("expected SDE scenario value %.2f, got %+v", wantSDEValue, sdeDelta)
	}
	if !approxEqual(sdeDelta.ValueDelta, wantSDEValue-2_500_000) {
		t.Fatalf("unexpected SDE value delta: %+v", sdeDelta)
	}

	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	wantEBITDAValue := 2_000_000 * 1.10 * 4.0
	if !approxEqual(ebitdaDelta.Scenario.Value, wantEBITDAValue) {
		t.Fatalf("expected EBITDA scenario value %.2f, got %+v", wantEBITDAValue, ebitdaDelta)
	}

	netAssetsDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeAdjustedNetAssetValue)
	if netAssetsDelta.DeltaAvailable {
		t.Fatalf("expected NetAssets delta unavailable (excluded in both baseline and scenario), got %+v", netAssetsDelta)
	}
	naLinkage, _ := findLinkage(netAssetsDelta.Linkages, valuation.CodeAdjustedNetAssetValue)
	if naLinkage.Status != LinkageNotApplicable {
		t.Fatalf("expected NetAssets LinkageNotApplicable for revenue growth, got %+v", naLinkage)
	}

	if !sr.ConsensusDeltaAvailable || sr.ConsensusValueDelta <= 0 {
		t.Fatalf("expected a positive consensus delta from revenue growth, got %+v", sr)
	}
}

// TestDriverMarginChange proves a margin-point change derives its dollar
// impact from the caller-supplied RevenueBase and never touches DCF or
// NetAssets (no documented linkage).
func TestDriverMarginChange(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "MARGIN_UP_2PT", Type: DriverMarginChange,
		MarginChange: &MarginChangeParams{MarginPointsDelta: 0.02, RevenueBase: 10_000_000},
	}}
	res := Calculate(in)

	sr, ok := findScenario(res.OneFactorAtATime, "MARGIN_UP_2PT")
	if !ok || !sr.Available {
		t.Fatalf("expected scenario result: ok=%v sr=%+v", ok, sr)
	}

	wantDollarDelta := 0.02 * 10_000_000.0 // 200,000
	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	wantEBITDAValue := (2_000_000 + wantDollarDelta) * 4.0
	if !approxEqual(ebitdaDelta.Scenario.Value, wantEBITDAValue) {
		t.Fatalf("expected EBITDA scenario value %.2f, got %+v", wantEBITDAValue, ebitdaDelta)
	}

	dcfDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeDCF)
	dcfLinkage, _ := findLinkage(dcfDelta.Linkages, valuation.CodeDCF)
	if dcfLinkage.Status != LinkageNotApplicable {
		t.Fatalf("expected DCF LinkageNotApplicable for a margin change (no revenue base to scale forecast cash flow), got %+v", dcfLinkage)
	}
	// DCF result must be byte-identical to baseline since nothing touched it.
	if !approxEqual(dcfDelta.ValueDelta, 0) {
		t.Fatalf("expected zero DCF delta for a driver with no DCF linkage, got %+v", dcfDelta)
	}
}

// TestDriverMarginChange_MissingRevenueBase proves a margin driver with no
// RevenueBase is rejected as invalid input, not silently ignored.
func TestDriverMarginChange_MissingRevenueBase(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{ID: "BAD_MARGIN", Type: DriverMarginChange, MarginChange: &MarginChangeParams{MarginPointsDelta: 0.02}}}
	res := Calculate(in)

	sr, ok := findScenario(res.OneFactorAtATime, "BAD_MARGIN")
	if !ok {
		t.Fatal("expected a scenario result even for an invalid driver")
	}
	if !HasDriverErrors(sr.Issues) {
		t.Fatalf("expected a blocking issue for missing RevenueBase, got %+v", sr.Issues)
	}
	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	if !approxEqual(ebitdaDelta.ValueDelta, 0) {
		t.Fatalf("expected no mutation from an invalid driver, got %+v", ebitdaDelta)
	}
}

// TestDriverMultipleChange covers the "method multiple change" example,
// targeting only EBITDA and proving SDE is left at LinkageNotApplicable
// because it was not named in Methods (not because the driver type cannot
// reach it in principle).
func TestDriverMultipleChange(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "EBITDA_MULTIPLE_UP", Type: DriverMultipleChange,
		MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeEBITDA}, ChangeDelta: 0.5},
	}}
	res := Calculate(in)

	sr, _ := findScenario(res.OneFactorAtATime, "EBITDA_MULTIPLE_UP")
	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	wantValue := 2_000_000 * 4.5
	if !approxEqual(ebitdaDelta.Scenario.Value, wantValue) {
		t.Fatalf("expected EBITDA scenario value %.2f, got %+v", wantValue, ebitdaDelta)
	}

	sdeDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeSDEMultiple)
	if !approxEqual(sdeDelta.ValueDelta, 0) {
		t.Fatalf("expected zero SDE delta since SDE was not named in Methods, got %+v", sdeDelta)
	}
	sdeLinkage, _ := findLinkage(sdeDelta.Linkages, valuation.CodeSDEMultiple)
	if sdeLinkage.Status != LinkageNotApplicable {
		t.Fatalf("expected SDE LinkageNotApplicable, got %+v", sdeLinkage)
	}
}

// TestDriverMultipleChange_NewValueReplacesAbsolute proves NewValue
// replaces Multiple outright rather than being added as a delta.
func TestDriverMultipleChange_NewValueReplacesAbsolute(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "SDE_MULTIPLE_SET", Type: DriverMultipleChange,
		MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeSDE}, ChangeDelta: 99, NewValue: 3.0},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "SDE_MULTIPLE_SET")
	sdeDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeSDEMultiple)
	wantValue := 1_000_000 * 3.0
	if !approxEqual(sdeDelta.Scenario.Value, wantValue) {
		t.Fatalf("expected NewValue to replace Multiple outright (%.2f), got %+v", wantValue, sdeDelta)
	}
}

// TestDriverMethodMultipleRule proves the caller must supply the resulting
// multiple explicitly — this package never derives it from
// TriggerLabel/TriggerValue (e.g. a recurring-revenue percentage), per the
// package brief's "do not claim causation" requirement.
func TestDriverMethodMultipleRule(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "RECURRING_REVENUE_RULE", Type: DriverMethodMultipleRule,
		MethodMultipleRule: &MethodMultipleRuleParams{
			Method: MultipleChangeEBITDA, NewMultiple: 5.5,
			TriggerLabel: "Recurring revenue percentage", TriggerValue: "62%",
		},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "RECURRING_REVENUE_RULE")
	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	wantValue := 2_000_000 * 5.5
	if !approxEqual(ebitdaDelta.Scenario.Value, wantValue) {
		t.Fatalf("expected EBITDA value at caller-supplied multiple 5.5x (%.2f), got %+v", wantValue, ebitdaDelta)
	}
	link, _ := findLinkage(ebitdaDelta.Linkages, valuation.CodeEBITDAMultiple)
	if link.Status != LinkageApplied {
		t.Fatalf("expected LinkageApplied, got %+v", link)
	}
}

// TestDriverMethodMultipleRule_MissingMethodIsRejected proves an empty or
// unrecognized MethodMultipleRuleParams.Method is a blocking issue, not a
// silent no-op — the same validation strictness MultipleChangeParams.Methods
// and WorkingCapitalChangeParams.Field already apply to their own required
// target fields.
func TestDriverMethodMultipleRule_MissingMethodIsRejected(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "NO_TARGET_METHOD", Type: DriverMethodMultipleRule,
		MethodMultipleRule: &MethodMultipleRuleParams{NewMultiple: 5.0},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "NO_TARGET_METHOD")
	if !HasDriverErrors(sr.Issues) {
		t.Fatalf("expected a blocking issue for an empty target Method, got %+v", sr.Issues)
	}
	foundCode := false
	for _, iss := range sr.Issues {
		if iss.Code == IssueNoTargetMethod {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatalf("expected IssueNoTargetMethod specifically, got %+v", sr.Issues)
	}
	sdeDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeSDEMultiple)
	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	if !approxEqual(sdeDelta.ValueDelta, 0) || !approxEqual(ebitdaDelta.ValueDelta, 0) {
		t.Fatalf("expected no mutation from a driver with an invalid target Method, got sde=%+v ebitda=%+v", sdeDelta, ebitdaDelta)
	}
}

// TestDriverDebtChange covers the "debt" driver example: EBITDA and DCF
// both had EquityBridge.Requested=true at baseline, so their bridges move;
// SDE's bridge was never requested, so it is LinkageNotApplicable, not
// silently skipped.
func TestDriverDebtChange(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "PAY_DOWN_DEBT", Type: DriverDebtChange,
		DebtChange: &DebtChangeParams{LongTermDebtDelta: -200_000},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "PAY_DOWN_DEBT")

	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	// Enterprise value unchanged (2,000,000 * 4.0 = 8,000,000); only the
	// bridge's equity value should move, which is reflected in the Run's
	// EBITDA.Bridge, not in the headline EnterpriseValue delta.
	if !approxEqual(ebitdaDelta.ValueDelta, 0) {
		t.Fatalf("expected zero headline EnterpriseValue delta from a debt-only change, got %+v", ebitdaDelta)
	}
	ebitdaOutcome, _ := findOutcomeInRun(sr.Run, valuation.CodeEBITDAMultiple)
	wantBridgeEquity := 8_000_000.0 + 200_000 - (500_000 - 200_000)
	if !approxEqual(ebitdaOutcome.EBITDA.Bridge.EquityValue, wantBridgeEquity) {
		t.Fatalf("expected EBITDA bridge equity value %.2f after debt paydown, got %.2f", wantBridgeEquity, ebitdaOutcome.EBITDA.Bridge.EquityValue)
	}

	sdeDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeSDEMultiple)
	sdeLinkage, _ := findLinkage(sdeDelta.Linkages, valuation.CodeSDEMultiple)
	if sdeLinkage.Status != LinkageNotApplicable {
		t.Fatalf("expected SDE LinkageNotApplicable (no bridge requested at baseline), got %+v", sdeLinkage)
	}
}

// TestDriverWorkingCapitalChange proves the working-capital driver reuses
// the debt-bridge mechanism via an explicit BridgeField, never an invented
// separate NWC field.
func TestDriverWorkingCapitalChange(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "NWC_INVESTMENT", Type: DriverWorkingCapitalChange,
		WorkingCapitalChange: &WorkingCapitalChangeParams{Field: BridgeFieldOtherDebt, Amount: 100_000},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "NWC_INVESTMENT")
	ebitdaOutcome, _ := findOutcomeInRun(sr.Run, valuation.CodeEBITDAMultiple)
	if !approxEqual(ebitdaOutcome.EBITDA.Bridge.TotalDebt, 500_000+100_000) {
		t.Fatalf("expected OtherDebt to add 100,000 to TotalDebt, got %.2f", ebitdaOutcome.EBITDA.Bridge.TotalDebt)
	}
}

// TestDriverWorkingCapitalChange_InvalidField proves an unrecognized
// BridgeField is rejected rather than silently ignored.
func TestDriverWorkingCapitalChange_InvalidField(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{ID: "BAD_NWC", Type: DriverWorkingCapitalChange, WorkingCapitalChange: &WorkingCapitalChangeParams{Field: "BOGUS", Amount: 1}}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "BAD_NWC")
	if !HasDriverErrors(sr.Issues) {
		t.Fatalf("expected a blocking issue for an invalid BridgeField, got %+v", sr.Issues)
	}
}

// TestDriverOwnerCompensationAdjustment_SDEOnlyByDefault proves the
// addback applies to SDE only unless AppliesBeyondSDE is set.
func TestDriverOwnerCompensationAdjustment_SDEOnlyByDefault(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "OWNER_COMP_ADDBACK", Type: DriverOwnerCompensationAdjustment,
		OwnerCompensationAdjustment: &OwnerCompensationAdjustmentParams{Amount: 50_000},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "OWNER_COMP_ADDBACK")

	sdeDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeSDEMultiple)
	wantSDEValue := (1_000_000 + 50_000) * 2.5
	if !approxEqual(sdeDelta.Scenario.Value, wantSDEValue) {
		t.Fatalf("expected SDE value %.2f, got %+v", wantSDEValue, sdeDelta)
	}

	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	if !approxEqual(ebitdaDelta.ValueDelta, 0) {
		t.Fatalf("expected zero EBITDA delta when AppliesBeyondSDE is false, got %+v", ebitdaDelta)
	}
}

// TestDriverCustomerLossImpact proves the "customer-loss impact" example:
// a revenue-at-risk figure times a margin reduces earnings-linked methods
// and shifts every DCF forecast period down by the same dollar amount.
func TestDriverCustomerLossImpact(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "LOSE_TOP_CUSTOMER", Type: DriverCustomerLossImpact,
		CustomerLossImpact: &CustomerLossImpactParams{RevenueAtRiskAmount: 1_000_000, EarningsMarginOnLostRevenue: 0.30},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "LOSE_TOP_CUSTOMER")

	wantEarningsDelta := -300_000.0
	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	wantEBITDAValue := (2_000_000 + wantEarningsDelta) * 4.0
	if !approxEqual(ebitdaDelta.Scenario.Value, wantEBITDAValue) {
		t.Fatalf("expected EBITDA value %.2f after customer loss, got %+v", wantEBITDAValue, ebitdaDelta)
	}

	dcfOutcome, _ := findOutcomeInRun(sr.Run, valuation.CodeDCF)
	wantYear1CF := 900_000 + wantEarningsDelta
	if !approxEqual(dcfOutcome.DCF.Input.ForecastPeriods[0].FreeCashFlow, wantYear1CF) {
		t.Fatalf("expected Year 1 forecast cash flow %.2f, got %.2f", wantYear1CF, dcfOutcome.DCF.Input.ForecastPeriods[0].FreeCashFlow)
	}
	if !ebitdaDelta.DeltaAvailable || ebitdaDelta.ValueDelta >= 0 {
		t.Fatalf("expected a negative EBITDA delta from a customer loss, got %+v", ebitdaDelta)
	}
}

// TestDriverCapRateChange_And_DiscountRateChange covers the remaining two
// rate-shaped drivers together with a single scenario each.
func TestDriverCapRateChange(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{ID: "CAP_RATE_UP", Type: DriverCapRateChange, CapRateChange: &CapRateChangeParams{NewValue: 0.25}}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "CAP_RATE_UP")
	capDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeCapitalizationOfEarnings)
	wantValue := 1_200_000 / 0.25
	if !approxEqual(capDelta.Scenario.Value, wantValue) {
		t.Fatalf("expected capitalized value %.2f at a 25%% cap rate, got %+v", wantValue, capDelta)
	}
}

func TestDriverDiscountRateChange(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{
		ID: "DISCOUNT_RATE_UP", Type: DriverDiscountRateChange,
		DiscountRateChange: &DiscountRateChangeParams{DiscountRateNewValue: 0.20},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "DISCOUNT_RATE_UP")
	dcfDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeDCF)
	if !dcfDelta.DeltaAvailable || dcfDelta.ValueDelta >= 0 {
		t.Fatalf("expected a lower DCF value at a higher discount rate, got %+v", dcfDelta)
	}
	dcfOutcome, _ := findOutcomeInRun(sr.Run, valuation.CodeDCF)
	if dcfOutcome.DCF.Input.DiscountRate != 0.20 {
		t.Fatalf("expected discount rate replaced with 0.20, got %v", dcfOutcome.DCF.Input.DiscountRate)
	}
}

// TestNoEffectDriver proves a driver that structurally cannot affect any
// method in this baseline (a multiple change naming only SDE, applied
// against a Request that also lacks a NetAssets input) still runs to
// completion with zero deltas everywhere and a recorded IssueNoLinkedMethod
// only when literally nothing applied. Here, to guarantee true "no effect
// at all," we target DriverCapRateChange but omit Capitalization entirely.
func TestNoEffectDriver(t *testing.T) {
	in := baseInput()
	in.BaselineRequest.Capitalization = nil // remove the only method this driver can ever touch
	in.Drivers = []Driver{{ID: "NO_EFFECT", Type: DriverCapRateChange, CapRateChange: &CapRateChangeParams{NewValue: 0.30}}}
	res := Calculate(in)

	sr, ok := findScenario(res.OneFactorAtATime, "NO_EFFECT")
	if !ok || !sr.Available {
		t.Fatalf("expected an Available scenario result even with no effect, got ok=%v sr=%+v", ok, sr)
	}
	for _, d := range sr.MethodDeltas {
		if d.Method == valuation.CodeCapitalizationOfEarnings {
			continue
		}
		if !approxEqual(d.ValueDelta, 0) {
			t.Fatalf("expected zero delta for method %s under a driver with no linkage to it, got %+v", d.Method, d)
		}
	}
	capDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeCapitalizationOfEarnings)
	capLinkage, _ := findLinkage(capDelta.Linkages, valuation.CodeCapitalizationOfEarnings)
	if capLinkage.Status != LinkageMethodExcluded {
		t.Fatalf("expected LinkageMethodExcluded (no Capitalization input at all), got %+v", capLinkage)
	}

	foundNoLinked := false
	for _, iss := range sr.Issues {
		if iss.Code == IssueNoLinkedMethod {
			foundNoLinked = true
		}
	}
	if !foundNoLinked {
		t.Fatalf("expected IssueNoLinkedMethod to be recorded, got %+v", sr.Issues)
	}
	if sr.ConsensusValueDelta != 0 && sr.ConsensusDeltaAvailable {
		t.Fatalf("expected zero consensus delta for a no-effect driver, got %+v", sr)
	}
}

// TestCombinedScenario proves Result.Scenarios applies every Driver in a
// Scenario together against one cloned Request (compounding), separately
// from Result.OneFactorAtATime, and that the combined effect is not simply
// the sum of two independently-run one-factor-at-a-time deltas (since
// growth compounds against the already-larger base before the multiple
// change is applied — though for a pure multiplicative chain like this one
// the combined dollar delta happens to still differ from the naive sum
// because the multiple driver also touches the grown base, not the
// original one).
func TestCombinedScenario(t *testing.T) {
	in := baseInput()
	growth := Driver{ID: "GROWTH", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.10}}
	multiple := Driver{
		ID: "MULTIPLE_UP", Type: DriverMultipleChange,
		MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeEBITDA}, ChangeDelta: 0.5},
	}
	in.Drivers = []Driver{growth, multiple}
	in.Scenarios = []Scenario{{ID: "GROWTH_PLUS_MULTIPLE", Label: "Growth + higher multiple", Drivers: []Driver{growth, multiple}}}

	res := Calculate(in)

	combined, ok := findScenario(res.Scenarios, "GROWTH_PLUS_MULTIPLE")
	if !ok || !combined.Available {
		t.Fatalf("expected combined scenario result, got ok=%v combined=%+v", ok, combined)
	}
	ebitdaDelta, _ := findMethodDelta(combined.MethodDeltas, valuation.CodeEBITDAMultiple)
	wantValue := (2_000_000 * 1.10) * 4.5
	if !approxEqual(ebitdaDelta.Scenario.Value, wantValue) {
		t.Fatalf("expected compounded EBITDA value %.2f, got %+v", wantValue, ebitdaDelta)
	}
	if len(ebitdaDelta.Linkages) != 2 {
		t.Fatalf("expected 2 linkages (one per driver) on the combined scenario's EBITDA delta, got %d: %+v", len(ebitdaDelta.Linkages), ebitdaDelta.Linkages)
	}

	growthOnly, _ := findScenario(res.OneFactorAtATime, "GROWTH")
	multipleOnly, _ := findScenario(res.OneFactorAtATime, "MULTIPLE_UP")
	growthOnlyDelta, _ := findMethodDelta(growthOnly.MethodDeltas, valuation.CodeEBITDAMultiple)
	multipleOnlyDelta, _ := findMethodDelta(multipleOnly.MethodDeltas, valuation.CodeEBITDAMultiple)
	naiveSum := growthOnlyDelta.ValueDelta + multipleOnlyDelta.ValueDelta
	if approxEqual(ebitdaDelta.ValueDelta, naiveSum) {
		t.Fatalf("expected the combined scenario's compounded delta to differ from the naive one-factor-at-a-time sum (%.2f), got the same value", naiveSum)
	}

	if len(combined.Assumptions) != 2 {
		t.Fatalf("expected 2 assumptions echoed on the combined scenario, got %+v", combined.Assumptions)
	}
}

// TestScenario_EmptyDriversUnavailable proves a structurally empty
// Scenario produces an unavailable ScenarioResult rather than a
// zero-effect one.
func TestScenario_EmptyDriversUnavailable(t *testing.T) {
	in := baseInput()
	in.Scenarios = []Scenario{{ID: "EMPTY"}}
	res := Calculate(in)
	sr, ok := findScenario(res.Scenarios, "EMPTY")
	if !ok {
		t.Fatal("expected a ScenarioResult entry even for an invalid scenario")
	}
	if sr.Available {
		t.Fatalf("expected Available == false for a scenario with no drivers, got %+v", sr)
	}
	if !HasDriverErrors(sr.Issues) {
		t.Fatalf("expected a blocking issue, got %+v", sr.Issues)
	}
}

// TestDriver_UnrecognizedTypeIsRejected proves an unrecognized DriverType
// is a blocking issue, not a silent no-op indistinguishable from a
// legitimate zero-effect driver.
func TestDriver_UnrecognizedTypeIsRejected(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{ID: "BOGUS", Type: "NOT_A_REAL_TYPE"}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "BOGUS")
	if !HasDriverErrors(sr.Issues) {
		t.Fatalf("expected IssueUnrecognizedDriverType, got %+v", sr.Issues)
	}
	found := false
	for _, iss := range sr.Issues {
		if iss.Code == IssueUnrecognizedDriverType {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected IssueUnrecognizedDriverType specifically, got %+v", sr.Issues)
	}
}

// TestDriver_DuplicateIDWarns proves a scenario with two drivers sharing
// an ID is advisory, not blocking — both still apply.
func TestDriver_DuplicateIDWarns(t *testing.T) {
	in := baseInput()
	in.Scenarios = []Scenario{{
		ID: "DUP_TEST",
		Drivers: []Driver{
			{ID: "SAME_ID", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.05}},
			{ID: "SAME_ID", Type: DriverMultipleChange, MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeSDE}, ChangeDelta: 0.1}},
		},
	}}
	res := Calculate(in)
	sr, _ := findScenario(res.Scenarios, "DUP_TEST")
	if !sr.Available {
		t.Fatalf("expected the scenario to still run despite duplicate IDs, got %+v", sr)
	}
	foundDup := false
	for _, iss := range sr.Issues {
		if iss.Code == IssueDuplicateDriverID {
			foundDup = true
		}
	}
	if !foundDup {
		t.Fatalf("expected IssueDuplicateDriverID warning, got %+v", sr.Issues)
	}
}

// TestCalculate_NoMutationOfCallerInput proves cloneRequest is a true deep
// copy: mutating a Driver's target field never reaches the caller's own
// BaselineRequest, and running the same Input twice produces identical
// baselines.
func TestCalculate_NoMutationOfCallerInput(t *testing.T) {
	in := baseInput()
	originalSDEMultiple := in.BaselineRequest.SDE.Multiple
	originalForecastLen := len(in.BaselineRequest.DCF.ForecastPeriods)
	originalFirstCF := in.BaselineRequest.DCF.ForecastPeriods[0].FreeCashFlow

	in.Drivers = []Driver{
		{ID: "MULT", Type: DriverMultipleChange, MultipleChange: &MultipleChangeParams{Methods: []MultipleChangeMethod{MultipleChangeSDE}, NewValue: 99}},
		{ID: "GROW", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.50}},
	}
	_ = Calculate(in)

	if in.BaselineRequest.SDE.Multiple != originalSDEMultiple {
		t.Fatalf("caller's SDE.Multiple was mutated: got %v, want %v", in.BaselineRequest.SDE.Multiple, originalSDEMultiple)
	}
	if len(in.BaselineRequest.DCF.ForecastPeriods) != originalForecastLen {
		t.Fatal("caller's DCF.ForecastPeriods slice length was mutated")
	}
	if in.BaselineRequest.DCF.ForecastPeriods[0].FreeCashFlow != originalFirstCF {
		t.Fatalf("caller's DCF.ForecastPeriods[0].FreeCashFlow was mutated: got %v, want %v", in.BaselineRequest.DCF.ForecastPeriods[0].FreeCashFlow, originalFirstCF)
	}

	// Run again — a mutated caller Input would produce a different baseline.
	res2 := Calculate(in)
	if !approxEqual(res2.Baseline.Consensus.Statistics.SimpleMean, Calculate(baseInput()).Baseline.Consensus.Statistics.SimpleMean) {
		t.Fatal("baseline consensus differs between independent Calculate calls with equivalent Input — caller input may have been mutated")
	}
}

// TestCalculate_NoMutationOfApplicability proves cloneRequest also deep-
// copies BaselineRequest.Applicability (not just the five method Inputs):
// a caller-supplied applicability.Results must never be aliased into the
// mutated Request, and Applicability.Methods must survive a re-run intact.
func TestCalculate_NoMutationOfApplicability(t *testing.T) {
	in := baseInput()
	results := applicability.Calculate(profile.Profile{})
	in.BaselineRequest.Applicability = &results
	originalMethodCount := len(results.Methods)

	in.Drivers = []Driver{{ID: "GROW", Type: DriverRevenueGrowth, RevenueGrowth: &RevenueGrowthParams{GrowthPercent: 0.10}}}
	res := Calculate(in)

	if len(results.Methods) != originalMethodCount {
		t.Fatalf("caller's Applicability.Methods slice length was mutated: got %d, want %d", len(results.Methods), originalMethodCount)
	}
	if in.BaselineRequest.Applicability != &results {
		t.Fatal("expected the caller's own Applicability field to be unchanged (still pointing at the caller's original value)")
	}

	sr, _ := findScenario(res.OneFactorAtATime, "GROW")
	if len(sr.Run.Methods) == 0 || sr.Run.Methods[0].Applicability == nil {
		t.Fatalf("expected the scenario's own Run to still carry Applicability on each MethodOutcome, got %+v", sr.Run.Methods)
	}
}

// TestDriverDebtChange_AllZeroDeltaIsNotApplicable proves a debt-change
// driver whose deltas are all zero reports LinkageNotApplicable (nothing
// to point to as evidence of a change) rather than a LinkageApplied with
// an empty ChangedInputs list, matching every other driver type's
// "LinkageApplied implies at least one ChangedInput" pattern.
func TestDriverDebtChange_AllZeroDeltaIsNotApplicable(t *testing.T) {
	in := baseInput()
	in.Drivers = []Driver{{ID: "NO_OP_DEBT", Type: DriverDebtChange, DebtChange: &DebtChangeParams{}}}
	res := Calculate(in)
	sr, _ := findScenario(res.OneFactorAtATime, "NO_OP_DEBT")

	ebitdaDelta, _ := findMethodDelta(sr.MethodDeltas, valuation.CodeEBITDAMultiple)
	link, _ := findLinkage(ebitdaDelta.Linkages, valuation.CodeEBITDAMultiple)
	if link.Status != LinkageNotApplicable {
		t.Fatalf("expected LinkageNotApplicable for an all-zero debt change, got %+v", link)
	}
	for _, ci := range sr.ChangedInputs {
		if ci.Method == valuation.CodeEBITDAMultiple {
			t.Fatalf("expected no ChangedInputs for an all-zero debt change, got %+v", ci)
		}
	}
}
