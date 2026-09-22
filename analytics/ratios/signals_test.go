package ratios

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func twoYearMeta() map[financial.Period]PeriodInfo {
	return map[financial.Period]PeriodInfo{
		"2024": {Type: PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// hasSignal reports whether signals contains at least one Signal with code.
func hasSignal(signals []Signal, code SignalCode) bool {
	for _, s := range signals {
		if s.Code == code {
			return true
		}
	}
	return false
}

// TestSignal_WeakeningLiquidity builds a two-year dataset where the current
// ratio drops from 3.0 to 1.0 (a 2.0 decline, well past the default 0.20
// threshold).
func TestSignal_WeakeningLiquidity(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 400, 200, 0, 0, 0).
		balanceSheet("2024", 100, 200, 0, 100, 0, 0, 300). // CA=300, CL=100 -> 3.0
		incomeStatement("2025", 1000, 400, 200, 0, 0, 0).
		balanceSheet("2025", 100, 0, 0, 200, 0, 0, 300). // CA=100, CL=200 -> 0.5
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !hasSignal(res.Signals, SignalWeakeningLiquidity) {
		t.Fatalf("expected SignalWeakeningLiquidity, got %+v", res.Signals)
	}
}

// TestSignal_RisingLeverage builds a two-year dataset where DebtToEBITDA
// rises from 1.0x to 2.0x (a 1.0x increase, past the default 0.50x
// threshold).
func TestSignal_RisingLeverage(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 400, 500, 0, 0, 0). // EBITDA = 1000-400-500 = 100
		balanceSheet("2024", 0, 0, 0, 0, 100, 0, 200).    // TotalDebt=100 -> 100/100=1.0x
		incomeStatement("2025", 1000, 400, 500, 0, 0, 0).
		balanceSheet("2025", 0, 0, 0, 0, 200, 0, 200). // TotalDebt=200 -> 200/100=2.0x
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !hasSignal(res.Signals, SignalRisingLeverage) {
		t.Fatalf("expected SignalRisingLeverage, got %+v", res.Signals)
	}
}

// TestSignal_MarginCompression builds a two-year dataset where EBITDA
// margin falls from 30% to 20% (10 points, past the default 3-point
// threshold), which should also trigger SignalDeterioratingProfitability
// (default 2-point threshold) off the same underlying comparison.
func TestSignal_MarginCompressionAndDeterioratingProfitability(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 400, 300, 0, 0, 0). // EBITDA=300, margin=0.30
		incomeStatement("2025", 1000, 400, 400, 0, 0, 0). // EBITDA=200, margin=0.20
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !hasSignal(res.Signals, SignalMarginCompression) {
		t.Fatalf("expected SignalMarginCompression, got %+v", res.Signals)
	}
	if !hasSignal(res.Signals, SignalDeterioratingProfitability) {
		t.Fatalf("expected SignalDeterioratingProfitability, got %+v", res.Signals)
	}
	if hasSignal(res.Signals, SignalImprovingProfitability) {
		t.Fatal("did not expect SignalImprovingProfitability alongside a decline")
	}
}

// TestSignal_ImprovingProfitability is the mirror image: EBITDA margin
// rises from 20% to 30%.
func TestSignal_ImprovingProfitability(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 400, 400, 0, 0, 0). // EBITDA=200, margin=0.20
		incomeStatement("2025", 1000, 400, 300, 0, 0, 0). // EBITDA=300, margin=0.30
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !hasSignal(res.Signals, SignalImprovingProfitability) {
		t.Fatalf("expected SignalImprovingProfitability, got %+v", res.Signals)
	}
	if hasSignal(res.Signals, SignalMarginCompression) || hasSignal(res.Signals, SignalDeterioratingProfitability) {
		t.Fatal("did not expect decline signals alongside an improvement")
	}
}

// TestSignal_SlowingCollections builds a two-year dataset where DSO rises
// from ~36.5 days to ~73 days (past the default 10-day threshold).
func TestSignal_SlowingCollections(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 0, 0, 0, 0, 0).
		balanceSheet("2024", 0, 100, 0, 0, 0, 0, 0). // DSO=(100/1000)*365=36.5
		incomeStatement("2025", 1000, 0, 0, 0, 0, 0).
		balanceSheet("2025", 0, 200, 0, 0, 0, 0, 0). // DSO=(200/1000)*365=73
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !hasSignal(res.Signals, SignalSlowingCollections) {
		t.Fatalf("expected SignalSlowingCollections, got %+v", res.Signals)
	}
}

// TestSignal_InventoryBuildup builds a two-year dataset where DIO rises
// substantially (past the default 10-day threshold).
func TestSignal_InventoryBuildup(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 0, 1000, 0, 0, 0, 0).
		balanceSheet("2024", 0, 0, 100, 0, 0, 0, 0). // DIO=(100/1000)*365=36.5
		incomeStatement("2025", 0, 1000, 0, 0, 0, 0).
		balanceSheet("2025", 0, 0, 200, 0, 0, 0, 0). // DIO=(200/1000)*365=73
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !hasSignal(res.Signals, SignalInventoryBuildup) {
		t.Fatalf("expected SignalInventoryBuildup, got %+v", res.Signals)
	}
}

// TestSignal_WeakInterestCoverage builds a single period where EBIT barely
// covers interest expense (1.2x, at or below the default 1.5x threshold).
func TestSignal_WeakInterestCoverage(t *testing.T) {
	ds := newDataset().
		incomeStatement("2025", 1000, 400, 480, 0, 100, 0). // EBIT=1000-400-480=120, coverage=120/100=1.2
		build()

	res := Calculate(Input{Dataset: ds}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if !hasSignal(res.Signals, SignalWeakInterestCoverage) {
		t.Fatalf("expected SignalWeakInterestCoverage, got %+v", res.Signals)
	}
}

// TestSignal_NoneTriggeredForStableBusiness proves a flat, healthy
// two-year dataset triggers no signals at all — the negative-case
// counterpart to every positive-case test above.
func TestSignal_NoneTriggeredForStableBusiness(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 400, 300, 20, 10, 15).
		balanceSheet("2024", 100, 200, 50, 80, 40, 60, 300).
		incomeStatement("2025", 1000, 400, 300, 20, 10, 15).
		balanceSheet("2025", 100, 200, 50, 80, 40, 60, 300).
		build()

	res := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if len(res.Signals) != 0 {
		t.Fatalf("expected no signals for an unchanged, healthy business, got %+v", res.Signals)
	}
}

// TestSignal_CustomThresholds proves a caller-supplied Thresholds value
// changes trigger behavior (a threshold too high to cross suppresses a
// signal that would otherwise fire under DefaultThresholds).
func TestSignal_CustomThresholds(t *testing.T) {
	ds := newDataset().
		incomeStatement("2024", 1000, 400, 400, 0, 0, 0). // EBITDA margin 0.20
		incomeStatement("2025", 1000, 400, 500, 0, 0, 0). // EBITDA margin 0.10 (10-point decline)
		build()

	// Default thresholds (3 points) should trigger compression.
	def := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{})
	if !hasSignal(def.Signals, SignalMarginCompression) {
		t.Fatal("expected SignalMarginCompression under DefaultThresholds")
	}

	// A threshold higher than the actual 10-point decline should suppress it.
	custom := Calculate(Input{Dataset: ds, PeriodMeta: twoYearMeta()}, Options{
		Thresholds: Thresholds{
			LiquidityDeclineThreshold:    DefaultThresholds().LiquidityDeclineThreshold,
			LeverageIncreaseThreshold:    DefaultThresholds().LeverageIncreaseThreshold,
			MarginCompressionThreshold:   0.50, // 50 points -- far above the actual 10-point decline
			CollectionsSlowdownDays:      DefaultThresholds().CollectionsSlowdownDays,
			InventoryBuildupDays:         DefaultThresholds().InventoryBuildupDays,
			WeakInterestCoverageRatio:    DefaultThresholds().WeakInterestCoverageRatio,
			ProfitabilityChangeThreshold: 0.50,
		},
	})
	if hasSignal(custom.Signals, SignalMarginCompression) {
		t.Fatal("expected SignalMarginCompression suppressed under a high custom threshold")
	}
}

// TestDefaultThresholds_ZeroValueResolves proves the zero Options.Thresholds
// resolves to DefaultThresholds, mirroring qoe/workingcapital's identical
// zero-value-means-defaults convention.
func TestDefaultThresholds_ZeroValueResolves(t *testing.T) {
	res := Calculate(Input{Dataset: newDataset().incomeStatement("2025", 1000, 400, 300, 20, 10, 15).build()}, Options{})
	if res.Thresholds != DefaultThresholds() {
		t.Fatalf("expected zero Options.Thresholds to resolve to DefaultThresholds, got %+v", res.Thresholds)
	}
}
