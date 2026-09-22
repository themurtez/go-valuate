package dcf

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

type archetypeAssumptions struct {
	DCF struct {
		ForecastPeriods    []ForecastPeriod `json:"forecast_periods"`
		DiscountRate       float64          `json:"discount_rate"`
		TerminalGrowthRate float64          `json:"terminal_growth_rate"`
	} `json:"dcf"`
}

func loadAssumptions(t *testing.T, archetype string) archetypeAssumptions {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "valuation_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading valuation fixture: %v", err)
	}
	var byArchetype map[string]json.RawMessage
	if err := json.Unmarshal(b, &byArchetype); err != nil {
		t.Fatalf("unmarshaling valuation fixture: %v", err)
	}
	raw, ok := byArchetype[archetype]
	if !ok {
		t.Fatalf("no %q entry in valuation fixture", archetype)
	}
	var a archetypeAssumptions
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshaling %q assumptions: %v", archetype, err)
	}
	return a
}

// wantEnterpriseValue independently recomputes the expected discounted-cash-
// flow result from an archetype's assumptions, so the test does not simply
// call Calculate and assert on Calculate's own output (which would prove
// nothing about correctness, only self-consistency).
func wantEnterpriseValue(a archetypeAssumptions) (sumPV, terminalValue, terminalValuePV, ev float64) {
	r := a.DCF.DiscountRate
	g := a.DCF.TerminalGrowthRate
	for i, fp := range a.DCF.ForecastPeriods {
		sumPV += fp.FreeCashFlow / math.Pow(1+r, float64(i+1))
	}
	n := len(a.DCF.ForecastPeriods)
	terminalYearCF := a.DCF.ForecastPeriods[n-1].FreeCashFlow
	terminalCF := terminalYearCF * (1 + g)
	terminalValue = terminalCF / (r - g)
	terminalValuePV = terminalValue / math.Pow(1+r, float64(n))
	ev = sumPV + terminalValuePV
	return
}

func TestFixtures_HVAC_ThreeYearForecast(t *testing.T) {
	a := loadAssumptions(t, "hvac")
	if len(a.DCF.ForecastPeriods) != 3 {
		t.Fatalf("expected 3 forecast periods, got %d", len(a.DCF.ForecastPeriods))
	}

	res := Calculate(Input{
		ForecastPeriods:    a.DCF.ForecastPeriods,
		DiscountRate:       a.DCF.DiscountRate,
		TerminalGrowthRate: a.DCF.TerminalGrowthRate,
	})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}

	_, wantTV, wantTVPV, wantEV := wantEnterpriseValue(a)
	if !approxEqual(res.TerminalValue, wantTV, 0.01) {
		t.Errorf("TerminalValue = %v, want ~%v", res.TerminalValue, wantTV)
	}
	if !approxEqual(res.TerminalValuePresentValue, wantTVPV, 0.01) {
		t.Errorf("TerminalValuePresentValue = %v, want ~%v", res.TerminalValuePresentValue, wantTVPV)
	}
	if !approxEqual(res.EnterpriseValue, wantEV, 0.01) {
		t.Errorf("EnterpriseValue = %v, want ~%v", res.EnterpriseValue, wantEV)
	}
}

func TestFixtures_Manufacturer_FourYearForecast(t *testing.T) {
	a := loadAssumptions(t, "manufacturer")
	if len(a.DCF.ForecastPeriods) != 4 {
		t.Fatalf("expected 4 forecast periods, got %d", len(a.DCF.ForecastPeriods))
	}

	res := Calculate(Input{
		ForecastPeriods:    a.DCF.ForecastPeriods,
		DiscountRate:       a.DCF.DiscountRate,
		TerminalGrowthRate: a.DCF.TerminalGrowthRate,
	})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	_, _, _, wantEV := wantEnterpriseValue(a)
	if !approxEqual(res.EnterpriseValue, wantEV, 0.01) {
		t.Errorf("EnterpriseValue = %v, want ~%v", res.EnterpriseValue, wantEV)
	}
}

func TestFixtures_SaaS_FiveYearForecastWithEquityBridge(t *testing.T) {
	a := loadAssumptions(t, "saas")
	if len(a.DCF.ForecastPeriods) != 5 {
		t.Fatalf("expected 5 forecast periods, got %d", len(a.DCF.ForecastPeriods))
	}

	res := Calculate(Input{
		ForecastPeriods:    a.DCF.ForecastPeriods,
		DiscountRate:       a.DCF.DiscountRate,
		TerminalGrowthRate: a.DCF.TerminalGrowthRate,
		EquityBridge:       EquityBridgeInput{Requested: true, ExcessCash: 1410000},
	})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	_, _, _, wantEV := wantEnterpriseValue(a)
	if !approxEqual(res.EnterpriseValue, wantEV, 0.01) {
		t.Errorf("EnterpriseValue = %v, want ~%v", res.EnterpriseValue, wantEV)
	}
	if !res.Bridge.Available {
		t.Fatal("expected Bridge.Available=true")
	}
	wantEquity := wantEV + 1410000
	if !approxEqual(res.Bridge.EquityValue, wantEquity, 0.01) {
		t.Errorf("Bridge.EquityValue = %v, want ~%v", res.Bridge.EquityValue, wantEquity)
	}
	// SaaS's large cash balance means equity value should meaningfully
	// exceed enterprise value.
	if res.Bridge.EquityValue <= res.EnterpriseValue {
		t.Errorf("expected equity value (%v) to exceed enterprise value (%v) given SaaS's large cash balance", res.Bridge.EquityValue, res.EnterpriseValue)
	}
}

func TestFixtures_Agency_ThreeYearForecast(t *testing.T) {
	a := loadAssumptions(t, "agency")
	res := Calculate(Input{
		ForecastPeriods:    a.DCF.ForecastPeriods,
		DiscountRate:       a.DCF.DiscountRate,
		TerminalGrowthRate: a.DCF.TerminalGrowthRate,
	})
	if !res.Available {
		t.Fatalf("unexpected errors: %+v", res.Errors)
	}
	_, _, _, wantEV := wantEnterpriseValue(a)
	if !approxEqual(res.EnterpriseValue, wantEV, 0.01) {
		t.Errorf("EnterpriseValue = %v, want ~%v", res.EnterpriseValue, wantEV)
	}
}

func TestFixtures_EveryArchetypeSatisfiesRateOrdering(t *testing.T) {
	// Every archetype's fixture assumptions must satisfy the DCF's own hard
	// validation constraint; if a future edit to the fixture broke this,
	// this test fails with a clear message rather than a cryptic
	// downstream Available=false in one of the tests above.
	for _, archetype := range []string{"hvac", "agency", "manufacturer", "saas"} {
		t.Run(archetype, func(t *testing.T) {
			a := loadAssumptions(t, archetype)
			if a.DCF.DiscountRate <= a.DCF.TerminalGrowthRate {
				t.Errorf("discount rate (%v) must exceed terminal growth rate (%v)", a.DCF.DiscountRate, a.DCF.TerminalGrowthRate)
			}
			if len(a.DCF.ForecastPeriods) == 0 {
				t.Error("expected at least one forecast period")
			}
			for _, fp := range a.DCF.ForecastPeriods {
				if fp.FreeCashFlow <= 0 {
					t.Errorf("expected a positive forecast cash flow for %q, got %v", fp.Period, fp.FreeCashFlow)
				}
			}
		})
	}
}
