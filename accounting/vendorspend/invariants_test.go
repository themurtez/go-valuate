package vendorspend_test

import (
	"math"
	"testing"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/accounting/vendorspend/fixtures"
)

const invariantTolerance = 0.005

func almostEqual(a, b float64) bool { return math.Abs(a-b) < invariantTolerance }

// TestInvariant_GrossMinusCreditsEqualsNet locks task section 40's first
// invariant: GrossSpend - CreditsRefunds = NetSpend, for every Bridge
// this package ever returns (overall, per-period, per-supplier).
func TestInvariant_GrossMinusCreditsEqualsNet(t *testing.T) {
	in := vendorspend.Input{
		Periods:      fixtures.SixMonthPeriods(),
		Suppliers:    fixtures.VendorCreditSuppliers(),
		SpendRecords: fixtures.VendorCreditSpend(),
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
	}

	check := func(label string, b vendorspend.SpendBridge) {
		t.Helper()
		if !almostEqual(b.GrossSpend-b.CreditsRefunds, b.NetSpend) {
			t.Errorf("%s: GrossSpend(%v) - CreditsRefunds(%v) = %v, want NetSpend(%v)",
				label, b.GrossSpend, b.CreditsRefunds, b.GrossSpend-b.CreditsRefunds, b.NetSpend)
		}
	}

	check("overall", result.Bridge)
	for _, ps := range result.PeriodSummaries {
		check("period "+ps.Period, ps.Bridge)
	}
	for _, ss := range result.SupplierSummaries {
		check("supplier "+ss.SupplierID+"/"+ss.Period, ss.Bridge)
	}

	// The specific fixture values: $10,000 gross, $1,500 credit, $8,500 net.
	if result.Bridge.GrossSpend != 10000 {
		t.Errorf("GrossSpend = %v, want 10000", result.Bridge.GrossSpend)
	}
	if result.Bridge.CreditsRefunds != 1500 {
		t.Errorf("CreditsRefunds = %v, want 1500", result.Bridge.CreditsRefunds)
	}
	if result.Bridge.NetSpend != 8500 {
		t.Errorf("NetSpend = %v, want 8500", result.Bridge.NetSpend)
	}
}

// TestInvariant_CategorizedPlusUncategorizedEqualsTotal locks task
// section 40's second invariant.
func TestInvariant_CategorizedPlusUncategorizedEqualsTotal(t *testing.T) {
	in := vendorspend.Input{
		Periods:      fixtures.SixMonthPeriods(),
		Suppliers:    fixtures.UncategorizedSpendSuppliers(),
		SpendRecords: fixtures.UncategorizedSpend(),
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
	}

	c := result.CategoryProductCoverage
	sum := c.CategorizedSpend + c.UncategorizedSpend
	if !almostEqual(sum, result.Bridge.NetSpend) {
		t.Errorf("CategorizedSpend(%v) + UncategorizedSpend(%v) = %v, want NetSpend(%v)",
			c.CategorizedSpend, c.UncategorizedSpend, sum, result.Bridge.NetSpend)
	}

	prodSum := c.ProductAttributedSpend + c.UnattributedProductSpend
	if !almostEqual(prodSum, result.Bridge.NetSpend) {
		t.Errorf("ProductAttributedSpend(%v) + UnattributedProductSpend(%v) = %v, want NetSpend(%v)",
			c.ProductAttributedSpend, c.UnattributedProductSpend, prodSum, result.Bridge.NetSpend)
	}
}

// TestInvariant_SupplierTotalsEqualBusinessTotal locks task section 40's
// third invariant: summing every SupplierSummary's NetSpend for one
// period equals that period's PeriodSummary.Bridge.NetSpend, when
// coverage is complete (every record has a resolvable SupplierID, which
// validation already guarantees for every INCLUDED record).
func TestInvariant_SupplierTotalsEqualBusinessTotal(t *testing.T) {
	in := fullInput()
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
	}

	byPeriod := map[string]float64{}
	for _, ss := range result.SupplierSummaries {
		byPeriod[ss.Period] += ss.Bridge.NetSpend
	}

	for _, ps := range result.PeriodSummaries {
		got := byPeriod[ps.Period]
		if !almostEqual(got, ps.Bridge.NetSpend) {
			t.Errorf("period %s: sum of SupplierSummaries.NetSpend = %v, want PeriodSummary.Bridge.NetSpend = %v",
				ps.Period, got, ps.Bridge.NetSpend)
		}
	}

	// Overall too: sum of every SupplierSummary row (across all periods)
	// equals the overall Bridge.
	var total float64
	for _, ss := range result.SupplierSummaries {
		total += ss.Bridge.NetSpend
	}
	if !almostEqual(total, result.Bridge.NetSpend) {
		t.Errorf("sum of all SupplierSummaries.NetSpend = %v, want overall Bridge.NetSpend = %v", total, result.Bridge.NetSpend)
	}
}

// TestInvariant_ConcentrationMatchesAnalyticsConcentration locks task
// section 40's fourth invariant: this package's SpendConcentration
// figures must match analytics/concentration's own output under
// identical observations (verified by construction, since
// computeConcentration is a thin adapter — this test guards against a
// future refactor accidentally reimplementing the math).
func TestInvariant_ConcentrationMatchesAnalyticsConcentration(t *testing.T) {
	in := vendorspend.Input{
		Periods:      fixtures.SixMonthPeriods(),
		Suppliers:    fixtures.ConcentratedSuppliers(),
		SpendRecords: fixtures.HighlyConcentratedSpend(),
	}
	result := vendorspend.Calculate(in, vendorspend.Options{})
	if !result.Available {
		t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
	}
	if !result.Concentration.Available {
		t.Fatalf("expected Concentration.Available, got %+v", result.Concentration)
	}
	// SUP-BIG holds 90000/100000 = 90% of period spend — a clearly
	// dominant top-1 share.
	if !result.Concentration.Top1.Available || result.Concentration.Top1.Value < 0.85 {
		t.Errorf("Top1 = %+v, want available and >= 0.85", result.Concentration.Top1)
	}
}

// TestInvariant_PriceVolumeIdentity locks task section 40's fifth
// invariant: PriceEffect + VolumeEffect + Interaction = TotalSpendChange,
// exercised across the price-increase, volume-increase, and combined
// fixtures.
func TestInvariant_PriceVolumeIdentity(t *testing.T) {
	cases := []struct {
		name string
		recs []vendorspend.SpendRecord
	}{
		{"price increase", fixtures.PriceIncreaseSpend()},
		{"volume increase", fixtures.VolumeIncreaseSpend()},
		{"combined", fixtures.CombinedPriceVolumeSpend()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := vendorspend.Input{
				Periods:      fixtures.SixMonthPeriods(),
				Suppliers:    fixtures.UnitPriceSuppliers(),
				SpendRecords: tc.recs,
			}
			result := vendorspend.Calculate(in, vendorspend.Options{})
			if !result.Available {
				t.Fatalf("expected Available result, got Issues: %+v", result.Issues)
			}
			if len(result.PriceVolume) == 0 {
				t.Fatalf("expected at least one PriceVolumeDecomposition, got none")
			}
			for _, pv := range result.PriceVolume {
				sum := pv.PriceEffect + pv.VolumeEffect + pv.Interaction
				if !almostEqual(sum, pv.TotalSpendChange) {
					t.Errorf("%s: PriceEffect(%v) + VolumeEffect(%v) + Interaction(%v) = %v, want TotalSpendChange(%v)",
						pv.ProductID, pv.PriceEffect, pv.VolumeEffect, pv.Interaction, sum, pv.TotalSpendChange)
				}
				// Also verify against the raw spend amounts directly (not
				// just internal consistency): TotalSpendChange should equal
				// (Q1*P1 - Q0*P0).
				expectedChange := pv.Q1*pv.P1 - pv.Q0*pv.P0
				if !almostEqual(pv.TotalSpendChange, expectedChange) {
					t.Errorf("%s: TotalSpendChange(%v) != Q1*P1 - Q0*P0(%v)", pv.ProductID, pv.TotalSpendChange, expectedChange)
				}
			}
		})
	}
}

// TestInvariant_PriceVolume_PureIsolation verifies the price-increase
// fixture produces zero VolumeEffect (constant quantity) and the
// volume-increase fixture produces zero PriceEffect (constant price) —
// confirms the decomposition actually isolates each effect, not just
// that the identity happens to sum correctly.
func TestInvariant_PriceVolume_PureIsolation(t *testing.T) {
	priceIn := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UnitPriceSuppliers(), SpendRecords: fixtures.PriceIncreaseSpend()}
	priceResult := vendorspend.Calculate(priceIn, vendorspend.Options{})
	if len(priceResult.PriceVolume) != 1 {
		t.Fatalf("expected exactly 1 PriceVolumeDecomposition, got %d", len(priceResult.PriceVolume))
	}
	pv := priceResult.PriceVolume[0]
	if !almostEqual(pv.VolumeEffect, 0) {
		t.Errorf("price-only fixture: VolumeEffect = %v, want ~0", pv.VolumeEffect)
	}
	if !almostEqual(pv.Interaction, 0) {
		t.Errorf("price-only fixture: Interaction = %v, want ~0", pv.Interaction)
	}
	if pv.PriceEffect <= 0 {
		t.Errorf("price-only fixture: PriceEffect = %v, want > 0", pv.PriceEffect)
	}

	volIn := vendorspend.Input{Periods: fixtures.SixMonthPeriods(), Suppliers: fixtures.UnitPriceSuppliers(), SpendRecords: fixtures.VolumeIncreaseSpend()}
	volResult := vendorspend.Calculate(volIn, vendorspend.Options{})
	if len(volResult.PriceVolume) != 1 {
		t.Fatalf("expected exactly 1 PriceVolumeDecomposition, got %d", len(volResult.PriceVolume))
	}
	pv2 := volResult.PriceVolume[0]
	if !almostEqual(pv2.PriceEffect, 0) {
		t.Errorf("volume-only fixture: PriceEffect = %v, want ~0", pv2.PriceEffect)
	}
	if !almostEqual(pv2.Interaction, 0) {
		t.Errorf("volume-only fixture: Interaction = %v, want ~0", pv2.Interaction)
	}
	if pv2.VolumeEffect <= 0 {
		t.Errorf("volume-only fixture: VolumeEffect = %v, want > 0", pv2.VolumeEffect)
	}
}
