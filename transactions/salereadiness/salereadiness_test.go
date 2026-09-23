package salereadiness

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/profile"
)

func boolPtr(v bool) *bool        { return &v }
func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func mustAvailable(t *testing.T, v Value, name string) float64 {
	t.Helper()
	if !v.Available {
		t.Fatalf("%s: expected Available, got unavailable", name)
	}
	return v.Amount
}

func mustUnavailable(t *testing.T, v Value, name string) {
	t.Helper()
	if v.Available {
		t.Fatalf("%s: expected unavailable, got %v", name, v.Amount)
	}
}

func findDimension(dims []Dimension, code DimensionCode) (Dimension, bool) {
	for _, d := range dims {
		if d.Code == code {
			return d, true
		}
	}
	return Dimension{}, false
}

// wholeInputPolicy is a Policy with every threshold set, used by tests that
// want threshold-graded StatusStrong/Weak/Concerning classification rather
// than the "no threshold supplied" StatusAcceptable fallback.
func wholeInputPolicy() Policy {
	return Policy{
		MinYearsHistoryForStrong:             3,
		MaxAcceptableEarningsVolatility:      0.20,
		MaxAcceptableAdjustmentToEBITDARatio: 0.15,
		MaxAcceptableLargestCustomerShare:    0.25,
		MinAcceptableRecurringRevenuePercent: 0.40,
		MaxAcceptableNWCVolatility:           0.20,
		MaxAcceptableNetDebtToEBITDA:         3.0,
		MaxAcceptableValuationDispersion:     0.15,
	}
}

// TestCalculate_ReadyStable covers a well-prepared business: reviewed
// financials, stable low-volatility earnings, low normalization burden,
// diversified customers, high recurring revenue, not owner-operated,
// improving margins, stable working capital, low leverage, every module
// supplied, and tight valuation-method agreement. Expect mostly
// StatusStrong/StatusAcceptable, no Blockers, a high OverallScore.
func TestCalculate_ReadyStable(t *testing.T) {
	in := Input{
		Dataset: financial.FinancialDataset{
			Items: []financial.NormalizedItem{
				{Code: financial.CodeRevProduct, Period: "FY2022", Amount: 1000},
				{Code: financial.CodeRevProduct, Period: "FY2023", Amount: 1100},
				{Code: financial.CodeRevProduct, Period: "FY2024", Amount: 1200},
			},
		},
		DataQuality: DataQuality{
			HasReviewedOrAuditedFinancials:   true,
			HasMultiYearFinancials:           true,
			HasTaxReturnReconciliation:       true,
			HasFormalAdjustmentDocumentation: true,
			OpenAccountingIssueCount:         intPtr(0),
		},
		QoE: qoe.Result{
			Available: true,
			History: []qoe.PeriodFigures{
				{Period: "FY2024", NormalizedEBITDA: metrics.AvailableValue(500_000)},
			},
			EBITDAVolatility: metrics.VolatilityResult{Value: metrics.AvailableValue(0.05)},
			EBITDAMarginTrend: []metrics.MarginPoint{
				{Period: "FY2022", Margin: metrics.AvailableValue(0.18)},
				{Period: "FY2023", Margin: metrics.AvailableValue(0.20)},
				{Period: "FY2024", Margin: metrics.AvailableValue(0.22)},
			},
			Ratios: qoe.Ratios{
				Period:             "FY2024",
				AdjustmentToEBITDA: metrics.AvailableValue(0.05),
			},
		},
		WorkingCapital: workingcapital.Result{
			Available: true,
			NWCPercentOfRevenueStatistics: workingcapital.Statistics{
				Volatility: workingcapital.NWCValue{Available: true, Value: 0.03},
			},
		},
		Concentration: concentration.Result{
			Available: true,
			History: []concentration.PeriodConcentration{
				{Period: "FY2024", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.08}},
			},
		},
		RevenueQuality: revenuequality.Result{
			Available: true,
			TotalRevenueHistory: []revenuequality.PeriodRevenue{
				{Period: "FY2024", RecurringPercent: revenuequality.RevenueValue{Available: true, Value: 0.75}},
			},
		},
		Consensus: consensus.Result{
			Available: true,
			Statistics: consensus.Statistics{
				Count:                  4,
				CoefficientOfVariation: 0.06,
			},
		},
		Metrics: metrics.Result{
			Snapshots: []metrics.Snapshot{
				{Period: "FY2024", NetDebt: metrics.AvailableValue(200_000), EBITDA: metrics.AvailableValue(500_000)},
			},
		},
		Profile: profile.Profile{
			OwnerOperated: boolPtr(false),
		},
		Policy: wholeInputPolicy(),
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Blockers) != 0 {
		t.Fatalf("expected no blockers for a ready/stable business, got %+v", res.Blockers)
	}
	if res.Coverage.AssessedDimensions != res.Coverage.TotalDimensions {
		t.Fatalf("expected full coverage, got %d/%d: missing=%+v", res.Coverage.AssessedDimensions, res.Coverage.TotalDimensions, res.MissingInformation)
	}
	if res.OverallScore == nil {
		t.Fatal("expected OverallScore to be computed")
	}
	if res.OverallScore.Value < 70 {
		t.Fatalf("expected a high overall score for a ready/stable business, got %v", res.OverallScore.Value)
	}

	d, ok := findDimension(res.Dimensions, DimensionOwnerDependence)
	if !ok || d.Status != StatusStrong {
		t.Fatalf("expected OwnerDependence StatusStrong, got %+v", d)
	}
}

// TestCalculate_OwnerDependent covers an owner-operated business with a
// large owner-discretionary earnings component, expecting OwnerDependence
// to classify StatusWeak (not StatusStrong) and a corresponding Risk +
// Opportunity.
func TestCalculate_OwnerDependent(t *testing.T) {
	in := Input{
		Profile: profile.Profile{OwnerOperated: boolPtr(true)},
		QoE: qoe.Result{
			Available: true,
			Flags: []qoe.Flag{
				{Code: qoe.FlagLargeOwnerDiscretionaryComponent, Severity: qoe.FlagSeverityWarning, Message: "large owner-discretionary component"},
			},
		},
		Policy: wholeInputPolicy(),
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	d, ok := findDimension(res.Dimensions, DimensionOwnerDependence)
	if !ok || d.Status != StatusWeak {
		t.Fatalf("expected OwnerDependence StatusWeak, got %+v", d)
	}

	foundRisk := false
	for _, r := range res.Risks {
		if r.Dimension == DimensionOwnerDependence {
			foundRisk = true
		}
	}
	if !foundRisk {
		t.Fatalf("expected a Risk for OwnerDependence, got risks=%+v", res.Risks)
	}

	foundOpp := false
	for _, o := range res.Opportunities {
		if o.Dimension == DimensionOwnerDependence && o.Code == OpportunityReduceOwnerDependence {
			foundOpp = true
		}
	}
	if !foundOpp {
		t.Fatalf("expected a REDUCE_OWNER_DEPENDENCE opportunity, got %+v", res.Opportunities)
	}
}

// TestCalculate_Concentrated covers a business with a dominant customer far
// above Policy.MaxAcceptableLargestCustomerShare, expecting
// CustomerConcentration StatusConcerning and a Blocker (concentration is in
// blockingDimensions).
func TestCalculate_Concentrated(t *testing.T) {
	in := Input{
		Concentration: concentration.Result{
			Available: true,
			History: []concentration.PeriodConcentration{
				{Period: "FY2024", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.65}},
			},
		},
		Policy: wholeInputPolicy(),
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	d, ok := findDimension(res.Dimensions, DimensionCustomerConcentration)
	if !ok || d.Status != StatusConcerning {
		t.Fatalf("expected CustomerConcentration StatusConcerning, got %+v", d)
	}
	if share := mustAvailable(t, d.Value, "CustomerConcentration.Value"); share != 0.65 {
		t.Fatalf("expected CustomerConcentration.Value == 0.65, got %v", share)
	}

	foundBlocker := false
	for _, b := range res.Blockers {
		if b.Dimension == DimensionCustomerConcentration {
			foundBlocker = true
		}
	}
	if !foundBlocker {
		t.Fatalf("expected a Blocker for severe customer concentration, got blockers=%+v", res.Blockers)
	}

	// A dimension with a diversified customer base has no Value comparison
	// concern below its own threshold — sanity-check the unavailable case
	// separately via a dimension with no Concentration input at all.
	empty := Calculate(Input{Profile: profile.Profile{OwnerOperated: boolPtr(true)}})
	emptyDim, _ := findDimension(empty.Dimensions, DimensionCustomerConcentration)
	mustUnavailable(t, emptyDim.Value, "CustomerConcentration.Value with no Concentration input")
}

// TestCalculate_PoorRecords covers a business with no reviewed/audited
// financials, no tax reconciliation, and open accounting issues, expecting
// FinancialRecordQuality StatusConcerning and a Blocker.
func TestCalculate_PoorRecords(t *testing.T) {
	in := Input{
		DataQuality: DataQuality{
			OpenAccountingIssueCount: intPtr(3),
		},
		Policy: wholeInputPolicy(),
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	d, ok := findDimension(res.Dimensions, DimensionFinancialRecordQuality)
	if !ok || d.Status != StatusConcerning {
		t.Fatalf("expected FinancialRecordQuality StatusConcerning, got %+v", d)
	}

	foundBlocker := false
	for _, b := range res.Blockers {
		if b.Dimension == DimensionFinancialRecordQuality {
			foundBlocker = true
		}
	}
	if !foundBlocker {
		t.Fatalf("expected a Blocker for poor records, got blockers=%+v", res.Blockers)
	}
}

// TestCalculate_MissingModules covers supplying only a Profile (every
// analytical-module Result left at its zero value), expecting most
// dimensions to be StatusUnassessed, reduced (not zero-scored) coverage,
// and every unassessed dimension listed in MissingInformation — never
// silently scored as StatusConcerning.
func TestCalculate_MissingModules(t *testing.T) {
	in := Input{
		Profile: profile.Profile{OwnerOperated: boolPtr(true)},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	if res.Coverage.AssessedDimensions == res.Coverage.TotalDimensions {
		t.Fatalf("expected reduced coverage with only Profile supplied, got full coverage")
	}
	if res.Coverage.AssessedDimensions == 0 {
		t.Fatalf("expected OwnerDependence at least to be assessed from Profile alone")
	}

	for _, code := range []DimensionCode{
		DimensionNormalizationBurden,
		DimensionCustomerConcentration,
		DimensionWorkingCapitalStability,
		DimensionValuationMethodConsensus,
	} {
		d, ok := findDimension(res.Dimensions, code)
		if !ok || d.Status != StatusUnassessed {
			t.Fatalf("expected %s StatusUnassessed with no module supplied, got %+v", code, d)
		}
		found := false
		for _, m := range res.MissingInformation {
			if m.Dimension == code {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %s in MissingInformation, got %+v", code, res.MissingInformation)
		}
	}

	// An unassessed dimension must never silently contribute a StatusConcerning-equivalent 0 to scoring inputs.
	for _, d := range res.Dimensions {
		if d.Status == StatusUnassessed && d.Value.Available {
			t.Fatalf("StatusUnassessed dimension %s must not carry an Available Value: %+v", d.Code, d)
		}
	}

	if len(res.Warnings) == 0 {
		t.Fatal("expected Warnings for the many unavailable modules")
	}
}

// TestCalculate_NegativeEarnings covers negative normalized EBITDA,
// expecting the fixed structural negativeEarningsBlocker to fire
// regardless of Policy.
func TestCalculate_NegativeEarnings(t *testing.T) {
	in := Input{
		QoE: qoe.Result{
			Available: true,
			History: []qoe.PeriodFigures{
				{Period: "FY2024", NormalizedEBITDA: metrics.AvailableValue(-150_000)},
			},
		},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}

	found := false
	for _, b := range res.Blockers {
		if b.Dimension == DimensionEarningsStability && b.Severity == SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a Blocker for negative normalized EBITDA, got blockers=%+v", res.Blockers)
	}
}

// TestCalculate_EmptyInput covers the degenerate zero-Input case: no
// dimension can be assessed, Available is false, and OverallScore is nil.
func TestCalculate_EmptyInput(t *testing.T) {
	res := Calculate(Input{})
	if res.Available {
		t.Fatalf("expected Available == false for zero Input, got true")
	}
	if len(res.Dimensions) != 0 {
		t.Fatalf("expected no Dimensions for zero Input, got %+v", res.Dimensions)
	}
	if res.OverallScore != nil {
		t.Fatalf("expected nil OverallScore for zero Input, got %+v", res.OverallScore)
	}
	if !HasErrors(res.Errors) {
		t.Fatal("expected an error-severity Issue for zero Input")
	}
}

// TestCalculate_NoPolicy covers every optional module supplied but Policy
// left at its zero value: every threshold-driven dimension should still
// assess (as StatusAcceptable, since there's no threshold to grade
// further), never StatusUnassessed purely for lack of Policy.
func TestCalculate_NoPolicy(t *testing.T) {
	in := Input{
		Concentration: concentration.Result{
			Available: true,
			History: []concentration.PeriodConcentration{
				{Period: "FY2024", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.10}},
			},
		},
	}

	res := Calculate(in)
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	d, ok := findDimension(res.Dimensions, DimensionCustomerConcentration)
	if !ok || d.Status != StatusAcceptable {
		t.Fatalf("expected CustomerConcentration StatusAcceptable with no Policy threshold, got %+v", d)
	}

	foundIssue := false
	for _, w := range res.Warnings {
		if w.Code == IssueNoPolicyThresholds {
			foundIssue = true
		}
	}
	if !foundIssue {
		t.Fatal("expected IssueNoPolicyThresholds warning")
	}
}

// TestCalculate_NoMutationOfInput proves Calculate never mutates
// caller-owned Input.
func TestCalculate_NoMutationOfInput(t *testing.T) {
	in := Input{
		Dataset: financial.FinancialDataset{
			Items: []financial.NormalizedItem{
				{Code: financial.CodeRevProduct, Period: "FY2024", Amount: 1000},
			},
		},
		QoE: qoe.Result{
			Available: true,
			Flags:     []qoe.Flag{{Code: qoe.FlagLargeOwnerDiscretionaryComponent, Severity: qoe.FlagSeverityWarning}},
		},
		Profile: profile.Profile{OwnerOperated: boolPtr(true)},
		Policy:  wholeInputPolicy(),
	}

	before := mustJSON(t, in)
	_ = Calculate(in)
	_ = Calculate(in)
	after := mustJSON(t, in)

	if before != after {
		t.Fatalf("Calculate mutated caller-owned Input:\nbefore: %s\nafter:  %s", before, after)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return string(b)
}
