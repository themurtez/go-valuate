package dcf

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/valuation"
)

// approxEqual compares floats with a small absolute tolerance, since DCF
// arithmetic (division, exponentiation) rarely lands on an exactly
// representable float64.
func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestCalculate_ThreeYearForecast_GordonGrowth_Discounting(t *testing.T) {
	// Independently computed (see completion report / scratch verification):
	// PV(y1)=86956.52, PV(y2)=83175.80, PV(y3)=79559.46, sum=249691.79
	// terminal CF = 121000*1.03 = 124630
	// terminal value = 124630 / (0.15-0.03) = 1038583.33
	// PV(terminal) = 1038583.33 / 1.15^3 = 682885.40
	// EV = 249691.79 + 682885.40 = 932577.19
	res := Calculate(Input{
		ForecastPeriods: []ForecastPeriod{
			{Period: "2026", FreeCashFlow: 100000},
			{Period: "2027", FreeCashFlow: 110000},
			{Period: "2028", FreeCashFlow: 121000},
		},
		DiscountRate:       0.15,
		TerminalGrowthRate: 0.03,
	})

	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if res.ValueType != valuation.ValueTypeEnterprise {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeEnterprise)
	}
	if len(res.ProjectedPeriods) != 3 {
		t.Fatalf("expected 3 projected periods, got %d", len(res.ProjectedPeriods))
	}

	wantPVs := []float64{86956.52173913045, 83175.80340264652, 79559.46412427058}
	for i, p := range res.ProjectedPeriods {
		if !approxEqual(p.PresentValue, wantPVs[i], 0.01) {
			t.Errorf("period %d PresentValue = %v, want ~%v", i+1, p.PresentValue, wantPVs[i])
		}
		if p.PeriodNumber != i+1 {
			t.Errorf("period %d PeriodNumber = %d, want %d", i, p.PeriodNumber, i+1)
		}
		wantDF := 1.0 / math.Pow(1.15, float64(i+1))
		if !approxEqual(p.DiscountFactor, wantDF, 1e-9) {
			t.Errorf("period %d DiscountFactor = %v, want %v", i+1, p.DiscountFactor, wantDF)
		}
	}

	if !approxEqual(res.SumOfPresentValues, 249691.78926604753, 0.01) {
		t.Errorf("SumOfPresentValues = %v, want ~249691.79", res.SumOfPresentValues)
	}
	if res.TerminalYearCashFlow != 121000 {
		t.Errorf("TerminalYearCashFlow = %v, want 121000", res.TerminalYearCashFlow)
	}
	wantTerminalCF := 121000 * 1.03
	if !approxEqual(res.TerminalCashFlow, wantTerminalCF, 0.001) {
		t.Errorf("TerminalCashFlow = %v, want %v", res.TerminalCashFlow, wantTerminalCF)
	}
	wantTV := wantTerminalCF / (0.15 - 0.03)
	if !approxEqual(res.TerminalValue, wantTV, 0.01) {
		t.Errorf("TerminalValue = %v, want %v", res.TerminalValue, wantTV)
	}
	if !approxEqual(res.TerminalValuePresentValue, 682885.4003999892, 0.01) {
		t.Errorf("TerminalValuePresentValue = %v, want ~682885.40", res.TerminalValuePresentValue)
	}
	if !approxEqual(res.EnterpriseValue, 932577.1896660367, 0.01) {
		t.Errorf("EnterpriseValue = %v, want ~932577.19", res.EnterpriseValue)
	}
	// Enterprise Value must equal sum of PVs + PV of terminal value exactly
	// (by construction, not just approximately).
	if res.EnterpriseValue != res.SumOfPresentValues+res.TerminalValuePresentValue {
		t.Errorf("EnterpriseValue (%v) != SumOfPresentValues (%v) + TerminalValuePresentValue (%v)",
			res.EnterpriseValue, res.SumOfPresentValues, res.TerminalValuePresentValue)
	}
}

func TestCalculate_FiveYearForecast(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods: []ForecastPeriod{
			{Period: "Y1", FreeCashFlow: 200000},
			{Period: "Y2", FreeCashFlow: 220000},
			{Period: "Y3", FreeCashFlow: 240000},
			{Period: "Y4", FreeCashFlow: 260000},
			{Period: "Y5", FreeCashFlow: 280000},
		},
		DiscountRate:       0.12,
		TerminalGrowthRate: 0.025,
	})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if len(res.ProjectedPeriods) != 5 {
		t.Fatalf("expected 5 projected periods, got %d", len(res.ProjectedPeriods))
	}
	if res.ProjectedPeriods[4].PeriodNumber != 5 {
		t.Errorf("last period number = %d, want 5", res.ProjectedPeriods[4].PeriodNumber)
	}
	if res.TerminalYearCashFlow != 280000 {
		t.Errorf("TerminalYearCashFlow = %v, want 280000 (must use the LAST forecast period)", res.TerminalYearCashFlow)
	}
	// Terminal value must discount at the 5th period's factor, not the 1st.
	if res.TerminalValueDiscountFactor != res.ProjectedPeriods[4].DiscountFactor {
		t.Errorf("TerminalValueDiscountFactor = %v, want it to equal the final period's factor %v",
			res.TerminalValueDiscountFactor, res.ProjectedPeriods[4].DiscountFactor)
	}
}

func TestCalculate_MidYearConvention(t *testing.T) {
	plain := Calculate(Input{
		ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 100000}, {Period: "Y2", FreeCashFlow: 100000}},
		DiscountRate:       0.10,
		TerminalGrowthRate: 0.02,
	})
	midYear := Calculate(Input{
		ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 100000}, {Period: "Y2", FreeCashFlow: 100000}},
		DiscountRate:       0.10,
		TerminalGrowthRate: 0.02,
		MidYearConvention:  true,
	})

	if !plain.Available || !midYear.Available {
		t.Fatalf("expected both Available, got plain errors=%+v midYear errors=%+v", plain.Errors, midYear.Errors)
	}

	wantMidYearDF1 := 1.0 / math.Pow(1.10, 0.5)
	if !approxEqual(midYear.ProjectedPeriods[0].DiscountFactor, wantMidYearDF1, 1e-9) {
		t.Errorf("mid-year period 1 DiscountFactor = %v, want %v", midYear.ProjectedPeriods[0].DiscountFactor, wantMidYearDF1)
	}
	// Mid-year discounting discounts less (exponent is smaller), so present
	// values — and therefore enterprise value — should be strictly higher
	// than plain end-of-year discounting for identical cash flows.
	if midYear.EnterpriseValue <= plain.EnterpriseValue {
		t.Errorf("mid-year EnterpriseValue (%v) should exceed plain EnterpriseValue (%v)", midYear.EnterpriseValue, plain.EnterpriseValue)
	}
}

func TestCalculate_InvalidRateRelationship(t *testing.T) {
	tests := []struct {
		name         string
		discountRate float64
		terminalRate float64
	}{
		{"equal rates", 0.10, 0.10},
		{"discount below terminal", 0.05, 0.08},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Calculate(Input{
				ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 100000}},
				DiscountRate:       tt.discountRate,
				TerminalGrowthRate: tt.terminalRate,
			})
			if res.Available {
				t.Fatalf("expected Available=false for discount=%v terminal=%v", tt.discountRate, tt.terminalRate)
			}
			found := false
			for _, e := range res.Errors {
				if e.Code == IssueDiscountRateNotAboveTerminalGrowth {
					found = true
				}
			}
			if !found {
				t.Errorf("expected IssueDiscountRateNotAboveTerminalGrowth, got %+v", res.Errors)
			}
		})
	}
}

func TestCalculate_NonPositiveDiscountRate(t *testing.T) {
	for _, rate := range []float64{0, -0.05} {
		res := Calculate(Input{
			ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 100000}},
			DiscountRate:       rate,
			TerminalGrowthRate: -0.5, // deliberately below any tested rate
		})
		if res.Available {
			t.Fatalf("discountRate=%v: expected Available=false", rate)
		}
		found := false
		for _, e := range res.Errors {
			if e.Code == IssueNonPositiveDiscountRate {
				found = true
			}
		}
		if !found {
			t.Errorf("discountRate=%v: expected IssueNonPositiveDiscountRate, got %+v", rate, res.Errors)
		}
	}
}

func TestCalculate_ZeroCashFlow(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods: []ForecastPeriod{
			{Period: "Y1", FreeCashFlow: 0},
			{Period: "Y2", FreeCashFlow: 100000},
		},
		DiscountRate:       0.12,
		TerminalGrowthRate: 0.03,
	})
	if !res.Available {
		t.Fatalf("expected Available=true for a zero (not negative) forecast cash flow, got errors: %+v", res.Errors)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNegativeForecastCashFlow {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNegativeForecastCashFlow warning for the zero-CF period, got %+v", res.Warnings)
	}
}

func TestCalculate_NegativeCashFlows(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods: []ForecastPeriod{
			{Period: "Y1", FreeCashFlow: -50000},
			{Period: "Y2", FreeCashFlow: 30000},
			{Period: "Y3", FreeCashFlow: 80000},
		},
		DiscountRate:       0.12,
		TerminalGrowthRate: 0.03,
	})
	if !res.Available {
		t.Fatalf("expected Available=true (negative forecast CF is a warning, not a blocking error), got errors: %+v", res.Errors)
	}
	if res.ProjectedPeriods[0].PresentValue >= 0 {
		t.Errorf("period 1 PresentValue = %v, expected negative (from negative FCF)", res.ProjectedPeriods[0].PresentValue)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssueNegativeForecastCashFlow {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNegativeForecastCashFlow warning, got %+v", res.Warnings)
	}
}

func TestCalculate_NegativeTerminalCashFlow(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods: []ForecastPeriod{
			{Period: "Y1", FreeCashFlow: 100000},
			{Period: "Y2", FreeCashFlow: -20000},
		},
		DiscountRate:       0.12,
		TerminalGrowthRate: 0.03,
	})
	if !res.Available {
		t.Fatalf("expected Available=true, got errors: %+v", res.Errors)
	}
	if res.TerminalYearCashFlow != -20000 {
		t.Errorf("TerminalYearCashFlow = %v, want -20000", res.TerminalYearCashFlow)
	}
	// A negative terminal-year CF grown by (1+g) and divided by a positive
	// (r-g) must produce a negative terminal value — not clamped to zero.
	if res.TerminalValue >= 0 {
		t.Errorf("TerminalValue = %v, expected negative", res.TerminalValue)
	}
	foundTerminalWarning := false
	for _, w := range res.Warnings {
		if w.Code == IssueNegativeTerminalCashFlow {
			foundTerminalWarning = true
		}
	}
	if !foundTerminalWarning {
		t.Errorf("expected IssueNegativeTerminalCashFlow warning, got %+v", res.Warnings)
	}
}

func TestCalculate_NoForecastPeriods(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods:    nil,
		DiscountRate:       0.12,
		TerminalGrowthRate: 0.03,
	})
	if res.Available {
		t.Fatal("expected Available=false with no forecast periods")
	}
	if res.EnterpriseValue != 0 {
		t.Errorf("EnterpriseValue = %v, want 0", res.EnterpriseValue)
	}
	found := false
	for _, e := range res.Errors {
		if e.Code == IssueNoForecastPeriods {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNoForecastPeriods, got %+v", res.Errors)
	}
}

func TestCalculate_EquityBridge(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods: []ForecastPeriod{
			{Period: "Y1", FreeCashFlow: 100000},
			{Period: "Y2", FreeCashFlow: 110000},
			{Period: "Y3", FreeCashFlow: 121000},
		},
		DiscountRate:       0.15,
		TerminalGrowthRate: 0.03,
		EquityBridge: EquityBridgeInput{
			Requested: true, ExcessCash: 80000, ShortTermDebt: 20000, LongTermDebt: 300000,
		},
	})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if !res.Bridge.Available {
		t.Fatalf("expected Bridge.Available=true")
	}
	if res.Bridge.EnterpriseValue != res.EnterpriseValue {
		t.Errorf("Bridge.EnterpriseValue = %v, want it to equal Result.EnterpriseValue %v", res.Bridge.EnterpriseValue, res.EnterpriseValue)
	}
	wantEquity := res.EnterpriseValue + 80000 - (20000 + 300000)
	if !approxEqual(res.Bridge.EquityValue, wantEquity, 0.001) {
		t.Errorf("Bridge.EquityValue = %v, want %v", res.Bridge.EquityValue, wantEquity)
	}
}

func TestCalculate_DoesNotGenerateForecasts(t *testing.T) {
	// A single caller-supplied period must be used exactly as given; the
	// package must never synthesize additional periods.
	res := Calculate(Input{
		ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 50000}},
		DiscountRate:       0.10,
		TerminalGrowthRate: 0.02,
	})
	if !res.Available {
		t.Fatalf("expected Available, got errors: %+v", res.Errors)
	}
	if len(res.ProjectedPeriods) != 1 {
		t.Fatalf("expected exactly 1 projected period (no generated periods), got %d", len(res.ProjectedPeriods))
	}
}

func TestCalculate_NonFiniteInputsRejected(t *testing.T) {
	base := Input{
		ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 100000}},
		DiscountRate:       0.12,
		TerminalGrowthRate: 0.03,
	}

	nanCF := base
	nanCF.ForecastPeriods = []ForecastPeriod{{Period: "Y1", FreeCashFlow: math.NaN()}}

	nanDiscount := base
	nanDiscount.DiscountRate = math.NaN()

	infTerminal := base
	infTerminal.TerminalGrowthRate = math.Inf(1)

	for i, in := range []Input{nanCF, nanDiscount, infTerminal} {
		res := Calculate(in)
		if res.Available {
			t.Errorf("case %d: expected Available=false", i)
		}
	}
}

func TestCalculate_ResultEnvelopeIsValid(t *testing.T) {
	res := Calculate(Input{
		ForecastPeriods:    []ForecastPeriod{{Period: "Y1", FreeCashFlow: 100000}, {Period: "Y2", FreeCashFlow: 110000}},
		DiscountRate:       0.15,
		TerminalGrowthRate: 0.03,
	})
	if issues := valuation.ValidateResultEnvelope(res.Method, res.MethodVersion, res.ValueType); len(issues) != 0 {
		t.Errorf("ValidateResultEnvelope() = %+v, want no issues", issues)
	}
	if issues := valuation.ValidateFiniteSteps(res.Steps); len(issues) != 0 {
		t.Errorf("ValidateFiniteSteps() = %+v, want no issues", issues)
	}
	if res.Method != Code {
		t.Errorf("Method = %v, want %v", res.Method, Code)
	}
	if res.MethodVersion != Version {
		t.Errorf("MethodVersion = %v, want %v", res.MethodVersion, Version)
	}
	if res.ValueType != valuation.ValueTypeEnterprise {
		t.Errorf("ValueType = %v, want %v", res.ValueType, valuation.ValueTypeEnterprise)
	}
}
