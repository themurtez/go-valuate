package forecast

import (
	"encoding/json"
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestCalculate_Deterministic proves repeated execution against identical
// input returns byte-for-byte identical output, mirroring
// analytics/variance/determinism_test.go's TestCalculate_Deterministic.
func TestCalculate_Deterministic(t *testing.T) {
	in := fullFeaturedInput()

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: Calculate output differs from the first run", i)
		}
	}
}

// TestCalculate_DeterministicAcrossMapOrdering proves Calculate's output
// does not depend on Go's randomized map iteration order, using many
// revenue/COGS/opex codes (each exercised through both the codeIndex built
// from Input.Dataset and the priorByCode/overrides maps projectRevenue/
// projectCOGS/projectOpex build internally, all of which must be sorted
// before appearing in LineItem output) across multiple forecast periods —
// mirroring analytics/variance's identical map-order stress test.
func TestCalculate_DeterministicAcrossMapOrdering(t *testing.T) {
	revCodes := []financial.Code{
		financial.CodeRevProduct, financial.CodeRevService, financial.CodeRevRecurring, financial.CodeRevOther,
	}
	cogsCodesUsed := []financial.Code{
		financial.CodeCogsMaterial, financial.CodeCogsDirectLabor, financial.CodeCogsFreight, financial.CodeCogsOther,
	}
	opexCodesUsed := []financial.Code{
		financial.CodeOpexPayroll, financial.CodeOpexOwnerComp, financial.CodeOpexRent, financial.CodeOpexMarketing,
		financial.CodeOpexInsurance, financial.CodeOpexUtilities, financial.CodeOpexSoftware,
		financial.CodeOpexProfessionalFees, financial.CodeOpexRepairs, financial.CodeOpexVehicle,
		financial.CodeOpexTravel, financial.CodeOpexOffice, financial.CodeOpexOther,
	}

	b := newDataset()
	for i, c := range revCodes {
		b.add(c, "2025", float64(50_000+i*1_237))
	}
	for i, c := range cogsCodesUsed {
		b.add(c, "2025", float64(10_000+i*911))
	}
	for i, c := range opexCodesUsed {
		b.add(c, "2025", float64(5_000+i*613))
	}
	ds := b.build()

	revOverrides := make([]RevenueCodeAssumption, len(revCodes))
	for i, c := range revCodes {
		revOverrides[i] = RevenueCodeAssumption{Code: c, Method: RevenueMethodGrowthRate, GrowthRate: 0.01 * float64(i+1)}
	}
	opexOverrides := make([]OpexCodeAssumption, len(opexCodesUsed))
	for i, c := range opexCodesUsed {
		opexOverrides[i] = OpexCodeAssumption{Code: c, Method: OpexMethodGrowthRate, GrowthRate: 0.005 * float64(i+1)}
	}

	horizon := 3
	assumptions := Assumptions{
		Revenue: []RevenuePeriodAssumption{
			{Method: RevenueMethodGrowthRate, GrowthRate: 0.05, CodeOverrides: revOverrides},
			{Method: RevenueMethodGrowthRate, GrowthRate: 0.06},
			{Method: RevenueMethodGrowthRate, GrowthRate: 0.07},
		},
		COGS: flatGrossMargin(0.55, horizon),
		Opex: []OpexPeriodAssumption{
			{Method: OpexMethodGrowthRate, GrowthRate: 0.03, CodeOverrides: opexOverrides},
			{Method: OpexMethodGrowthRate, GrowthRate: 0.03},
			{Method: OpexMethodGrowthRate, GrowthRate: 0.03},
		},
	}

	in := Input{
		Dataset:    ds,
		PeriodMeta: singlePeriodMeta("2025", 2025),
		Horizon:    horizon,
		Scenarios: []Scenario{
			{Name: "Base", Assumptions: assumptions},
			{Name: "Downside", Assumptions: ApplyRevenueShock(assumptions, -0.1, 1, horizon)},
		},
	}

	first, err := json.Marshal(Calculate(in))
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	for i := 0; i < 20; i++ {
		got, err := json.Marshal(Calculate(in))
		if err != nil {
			t.Fatalf("run %d: json.Marshal failed: %v", i, err)
		}
		if string(got) != string(first) {
			t.Fatalf("run %d: output differs; suspect map-order nondeterminism", i)
		}
	}
}
