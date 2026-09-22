package workingcapital

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func fourYearDataset() financial.FinancialDataset {
	return newDataset().
		bsPeriod("2022", 30000, 15000, 0, 20000, 4000, 400000). // NWC = 21000
		bsPeriod("2023", 35000, 17000, 0, 20000, 4000, 450000). // NWC = 28000
		bsPeriod("2024", 40000, 19000, 0, 20000, 4000, 500000). // NWC = 35000
		bsPeriod("2025", 45000, 21000, 0, 20000, 4000, 550000). // NWC = 42000
		build()
}

func fourYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2022": {Type: PeriodTypeFiscalYear, FiscalYear: 2022},
		"2023": {Type: PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

func TestSuggestedPeg_NoMethodRequested(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if res.SuggestedPeg.Method != "" {
		t.Errorf("expected empty Method when PegMethod not requested, got %q", res.SuggestedPeg.Method)
	}
	if res.SuggestedPeg.Value.Available {
		t.Errorf("expected SuggestedPeg.Value unavailable, got %+v", res.SuggestedPeg.Value)
	}
}

func TestSuggestedPeg_Latest(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{PegMethod: PegMethodLatest})
	if !res.SuggestedPeg.Value.Available || res.SuggestedPeg.Value.Value != 42000 {
		t.Fatalf("Latest peg = %+v, want 42000", res.SuggestedPeg.Value)
	}
	if len(res.SuggestedPeg.PeriodsUsed) != 1 || res.SuggestedPeg.PeriodsUsed[0] != "2025" {
		t.Errorf("PeriodsUsed = %v, want [2025]", res.SuggestedPeg.PeriodsUsed)
	}
}

func TestSuggestedPeg_SimpleAverage(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{PegMethod: PegMethodSimpleAverage})
	want := (21000.0 + 28000.0 + 35000.0 + 42000.0) / 4
	if !res.SuggestedPeg.Value.Available || res.SuggestedPeg.Value.Value != want {
		t.Fatalf("SimpleAverage peg = %+v, want %v", res.SuggestedPeg.Value, want)
	}
	if len(res.SuggestedPeg.PeriodsUsed) != 4 {
		t.Errorf("expected all 4 periods used, got %v", res.SuggestedPeg.PeriodsUsed)
	}
}

func TestSuggestedPeg_Median(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{PegMethod: PegMethodMedian})
	want := (28000.0 + 35000.0) / 2
	if !res.SuggestedPeg.Value.Available || res.SuggestedPeg.Value.Value != want {
		t.Fatalf("Median peg = %+v, want %v", res.SuggestedPeg.Value, want)
	}
}

func TestSuggestedPeg_TrailingAverage(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{
		PegMethod:       PegMethodTrailingAverage,
		TrailingPeriods: 2,
	})
	want := (35000.0 + 42000.0) / 2
	if !res.SuggestedPeg.Value.Available || res.SuggestedPeg.Value.Value != want {
		t.Fatalf("TrailingAverage(2) peg = %+v, want %v", res.SuggestedPeg.Value, want)
	}
	if len(res.SuggestedPeg.PeriodsUsed) != 2 || res.SuggestedPeg.PeriodsUsed[0] != "2024" || res.SuggestedPeg.PeriodsUsed[1] != "2025" {
		t.Errorf("PeriodsUsed = %v, want [2024 2025]", res.SuggestedPeg.PeriodsUsed)
	}
}

func TestSuggestedPeg_TrailingAverageExceedsAvailable(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{
		PegMethod:       PegMethodTrailingAverage,
		TrailingPeriods: 10,
	})
	if res.SuggestedPeg.Value.Available {
		t.Errorf("expected unavailable peg when TrailingPeriods exceeds history, got %+v", res.SuggestedPeg.Value)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Code == IssuePegMethodUnavailable {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssuePegMethodUnavailable warning, got %+v", res.Warnings)
	}
}

func TestSuggestedPeg_TrailingAverageMissingWindowSize(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{
		PegMethod: PegMethodTrailingAverage,
	})
	if res.SuggestedPeg.Value.Available {
		t.Error("expected unavailable peg when TrailingPeriods is 0")
	}
}

func TestSuggestedPeg_Fixed(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{
		PegMethod: PegMethodFixed,
		FixedPeg:  AvailableValue(30000),
	})
	if !res.SuggestedPeg.Value.Available || res.SuggestedPeg.Value.Value != 30000 {
		t.Fatalf("Fixed peg = %+v, want 30000", res.SuggestedPeg.Value)
	}
	if len(res.SuggestedPeg.PeriodsUsed) != 0 {
		t.Errorf("expected no PeriodsUsed for a fixed peg, got %v", res.SuggestedPeg.PeriodsUsed)
	}
}

func TestSuggestedPeg_FixedWithoutValue(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{
		PegMethod: PegMethodFixed,
	})
	if res.SuggestedPeg.Value.Available {
		t.Error("expected unavailable peg when FixedPeg not supplied")
	}
}

func TestSuggestedPeg_UnrecognizedMethod(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{
		PegMethod: PegMethod("not_a_real_method"),
	})
	if res.SuggestedPeg.Value.Available {
		t.Error("expected unavailable peg for unrecognized method")
	}
}

func TestPegComparison_Excess(t *testing.T) {
	res := Calculate(Input{
		Dataset:    fourYearDataset(),
		PeriodMeta: fourYearMeta(),
		CurrentNWC: AvailableValue(50000),
	}, Options{PegMethod: PegMethodLatest})

	if !res.PegComparison.ExcessDeficit.Available {
		t.Fatal("expected ExcessDeficit available")
	}
	want := 50000.0 - 42000.0
	if res.PegComparison.ExcessDeficit.Value != want {
		t.Errorf("ExcessDeficit = %v, want %v", res.PegComparison.ExcessDeficit.Value, want)
	}
	if res.PegComparison.ExcessDeficit.Value <= 0 {
		t.Error("expected a positive excess")
	}
}

func TestPegComparison_Deficit(t *testing.T) {
	res := Calculate(Input{
		Dataset:    fourYearDataset(),
		PeriodMeta: fourYearMeta(),
		CurrentNWC: AvailableValue(20000),
	}, Options{PegMethod: PegMethodLatest})

	want := 20000.0 - 42000.0
	if !res.PegComparison.ExcessDeficit.Available || res.PegComparison.ExcessDeficit.Value != want {
		t.Fatalf("ExcessDeficit = %+v, want %v", res.PegComparison.ExcessDeficit, want)
	}
	if res.PegComparison.ExcessDeficit.Value >= 0 {
		t.Error("expected a negative deficit")
	}
}

func TestPegComparison_UnavailableWhenPegUnavailable(t *testing.T) {
	res := Calculate(Input{
		Dataset:    fourYearDataset(),
		PeriodMeta: fourYearMeta(),
		CurrentNWC: AvailableValue(20000),
	}, Options{}) // no PegMethod requested

	if res.PegComparison.ExcessDeficit.Available {
		t.Errorf("expected ExcessDeficit unavailable with no peg, got %+v", res.PegComparison.ExcessDeficit)
	}
}
