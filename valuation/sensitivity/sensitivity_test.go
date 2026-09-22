package sensitivity

import (
	"testing"

	"github.com/themurtez/go-valuate/valuation/dcf"
)

func TestMultipleSensitivity_ValidMultiples(t *testing.T) {
	res := MultipleSensitivity(1_000_000, []float64{2.0, 2.5, 3.0})
	if len(res.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(res.Points))
	}
	wantValues := []float64{2_000_000, 2_500_000, 3_000_000}
	for i, p := range res.Points {
		if !p.Valid {
			t.Errorf("point %d: expected valid, reason=%q", i, p.Reason)
		}
		if p.Value != wantValues[i] {
			t.Errorf("point %d: value = %v, want %v", i, p.Value, wantValues[i])
		}
	}
}

func TestMultipleSensitivity_InvalidMultiplesMarkedNotDropped(t *testing.T) {
	res := MultipleSensitivity(1_000_000, []float64{2.0, 0, -1, 3.0})
	if len(res.Points) != 4 {
		t.Fatalf("expected all 4 points preserved (invalid marked, not dropped), got %d", len(res.Points))
	}
	if res.Points[0].Valid != true || res.Points[3].Valid != true {
		t.Error("expected the two valid multiples to remain valid")
	}
	if res.Points[1].Valid || res.Points[2].Valid {
		t.Error("expected zero and negative multiples to be invalid")
	}
	for _, i := range []int{1, 2} {
		if res.Points[i].Reason == "" {
			t.Errorf("point %d: expected a Reason for invalidity", i)
		}
		if res.Points[i].Value != 0 {
			t.Errorf("point %d: expected Value = 0 when invalid", i)
		}
	}
}

func TestMultipleSensitivity_EmptyMultiples(t *testing.T) {
	res := MultipleSensitivity(1_000_000, nil)
	if len(res.Points) != 0 {
		t.Errorf("expected 0 points for nil multiples, got %d", len(res.Points))
	}
}

func TestEarningsMultipleMatrix_Shape(t *testing.T) {
	scenarios := []EarningsScenario{
		{Label: "Downside", Earnings: 800_000},
		{Label: "Base Case", Earnings: 1_000_000},
		{Label: "Upside", Earnings: 1_200_000},
	}
	multiples := []float64{2.0, 3.0}
	m := EarningsMultipleMatrix(scenarios, multiples)

	if len(m.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(m.Rows))
	}
	for i, row := range m.Rows {
		if len(row) != 2 {
			t.Fatalf("row %d: expected 2 columns, got %d", i, len(row))
		}
	}
}

func TestEarningsMultipleMatrix_CellValues(t *testing.T) {
	scenarios := []EarningsScenario{{Label: "Base Case", Earnings: 500_000}}
	multiples := []float64{2.0, 4.0}
	m := EarningsMultipleMatrix(scenarios, multiples)

	cell00 := m.Rows[0][0]
	if !cell00.Valid || cell00.Value != 1_000_000 {
		t.Errorf("Rows[0][0] = %+v, want Value=1000000 Valid=true", cell00)
	}
	cell01 := m.Rows[0][1]
	if !cell01.Valid || cell01.Value != 2_000_000 {
		t.Errorf("Rows[0][1] = %+v, want Value=2000000 Valid=true", cell01)
	}
	if cell00.EarningsLabel != "Base Case" {
		t.Errorf("EarningsLabel = %q, want %q", cell00.EarningsLabel, "Base Case")
	}
}

func TestEarningsMultipleMatrix_InvalidColumnDoesNotAffectOthers(t *testing.T) {
	scenarios := []EarningsScenario{{Label: "Base Case", Earnings: 500_000}}
	multiples := []float64{2.0, -1.0, 3.0}
	m := EarningsMultipleMatrix(scenarios, multiples)

	row := m.Rows[0]
	if !row[0].Valid || !row[2].Valid {
		t.Error("expected columns 0 and 2 to remain valid")
	}
	if row[1].Valid {
		t.Error("expected column 1 (negative multiple) to be invalid")
	}
}

func sampleForecast() []dcf.ForecastPeriod {
	return []dcf.ForecastPeriod{
		{Period: "2026", FreeCashFlow: 100_000},
		{Period: "2027", FreeCashFlow: 110_000},
		{Period: "2028", FreeCashFlow: 120_000},
	}
}

func TestDCFSensitivity_ValidCombinations(t *testing.T) {
	grid := DCFSensitivity(sampleForecast(), []float64{0.15, 0.20}, []float64{0.02, 0.03}, dcf.EquityBridgeInput{})

	if len(grid.Rows) != 2 || len(grid.Rows[0]) != 2 {
		t.Fatalf("expected a 2x2 grid, got %dx%d", len(grid.Rows), len(grid.Rows[0]))
	}
	for i, dr := range grid.DiscountRates {
		for j, tg := range grid.TerminalGrowthRates {
			cell := grid.Rows[i][j]
			if !cell.Valid {
				t.Errorf("cell (%v,%v): expected valid, errors=%v", dr, tg, cell.Result.Errors)
			}
			if cell.EnterpriseValue <= 0 {
				t.Errorf("cell (%v,%v): expected positive EnterpriseValue, got %v", dr, tg, cell.EnterpriseValue)
			}
			// Recompute directly to confirm the cell matches dcf.Calculate exactly.
			want := dcf.Calculate(dcf.Input{ForecastPeriods: sampleForecast(), DiscountRate: dr, TerminalGrowthRate: tg})
			if cell.EnterpriseValue != want.EnterpriseValue {
				t.Errorf("cell (%v,%v): EnterpriseValue = %v, want %v", dr, tg, cell.EnterpriseValue, want.EnterpriseValue)
			}
		}
	}
}

func TestDCFSensitivity_InvalidCombinationsMarkedNotComputed(t *testing.T) {
	// discount rate <= terminal growth rate for several combinations.
	grid := DCFSensitivity(sampleForecast(), []float64{0.05, 0.20}, []float64{0.05, 0.10, 0.02}, dcf.EquityBridgeInput{})

	// Row 0 (discount rate 0.05): terminal growth 0.05 (equal, invalid) and
	// 0.10 (exceeds, invalid) should both be invalid; 0.02 should be valid.
	row0 := grid.Rows[0]
	if row0[0].Valid {
		t.Error("expected discount=0.05, terminal=0.05 to be invalid (equal rates)")
	}
	if row0[1].Valid {
		t.Error("expected discount=0.05, terminal=0.10 to be invalid (terminal exceeds discount)")
	}
	if !row0[2].Valid {
		t.Error("expected discount=0.05, terminal=0.02 to be valid")
	}

	// Row 1 (discount rate 0.20): every terminal growth here is below it.
	row1 := grid.Rows[1]
	for j, cell := range row1 {
		if !cell.Valid {
			t.Errorf("row1[%d]: expected valid (discount=0.20 exceeds every supplied terminal growth), errors=%v", j, cell.Result.Errors)
		}
	}

	// Invalid cells must still report zero EnterpriseValue and carry the
	// underlying dcf.Result's own Errors, not be silently dropped.
	if row0[0].EnterpriseValue != 0 {
		t.Errorf("invalid cell EnterpriseValue = %v, want 0", row0[0].EnterpriseValue)
	}
	if len(row0[0].Result.Errors) == 0 {
		t.Error("expected the invalid cell's Result to carry its own Errors")
	}
}

func TestDCFSensitivity_NonPositiveDiscountRateInvalid(t *testing.T) {
	grid := DCFSensitivity(sampleForecast(), []float64{0, -0.1}, []float64{0.02}, dcf.EquityBridgeInput{})
	for i, row := range grid.Rows {
		for j, cell := range row {
			if cell.Valid {
				t.Errorf("cell (%d,%d): expected invalid for non-positive discount rate", i, j)
			}
		}
	}
}

func TestDCFSensitivity_EquityBridgeAppliedToEveryCell(t *testing.T) {
	bridge := dcf.EquityBridgeInput{Requested: true, ExcessCash: 50_000, ShortTermDebt: 10_000}
	grid := DCFSensitivity(sampleForecast(), []float64{0.18}, []float64{0.03}, bridge)
	cell := grid.Rows[0][0]
	if !cell.Result.Bridge.Available {
		t.Fatal("expected the equity bridge to be computed for the cell")
	}
	wantEquity := cell.Result.EnterpriseValue + 50_000 - 10_000
	if cell.Result.Bridge.EquityValue != wantEquity {
		t.Errorf("Bridge.EquityValue = %v, want %v", cell.Result.Bridge.EquityValue, wantEquity)
	}
}

func TestDCFSensitivity_EmptyRatesProduceEmptyGrid(t *testing.T) {
	grid := DCFSensitivity(sampleForecast(), nil, []float64{0.03}, dcf.EquityBridgeInput{})
	if len(grid.Rows) != 0 {
		t.Errorf("expected 0 rows for empty discount rates, got %d", len(grid.Rows))
	}
}
