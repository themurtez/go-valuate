package workingcapital

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// seasonalRetailDataset builds two fiscal years of quarterly data for a
// retail-shaped business: Q4 inventory build (holiday season) inflates NWC
// as a percent of revenue every year, a recurring seasonal pattern
// SeasonalProfile should surface consistently across both years.
func seasonalRetailDataset() (financial.FinancialDataset, map[financial.Period]PeriodInfo) {
	type q struct {
		period               string
		year, seq            int
		ar, inv, ap, revenue float64
	}
	quarters := []q{
		{"2024-Q1", 2024, 1, 20000, 30000, 15000, 200000},
		{"2024-Q2", 2024, 2, 22000, 32000, 16000, 210000},
		{"2024-Q3", 2024, 3, 25000, 40000, 18000, 220000},
		{"2024-Q4", 2024, 4, 30000, 70000, 25000, 260000}, // holiday build
		{"2025-Q1", 2025, 1, 21000, 31000, 15500, 205000},
		{"2025-Q2", 2025, 2, 23000, 33000, 16500, 215000},
		{"2025-Q3", 2025, 3, 26000, 41000, 18500, 225000},
		{"2025-Q4", 2025, 4, 31000, 72000, 25500, 265000}, // holiday build
	}

	b := newDataset()
	meta := make(map[financial.Period]PeriodInfo)
	for _, item := range quarters {
		b.add(financial.CodeBsAccountsReceivable, item.period, item.ar).
			add(financial.CodeBsInventory, item.period, item.inv).
			add(financial.CodeBsAccountsPayable, item.period, item.ap).
			add(financial.CodeRevProduct, item.period, item.revenue)
		meta[financial.Period(item.period)] = PeriodInfo{
			Type:           PeriodTypeQuarter,
			FiscalYear:     item.year,
			SequenceInYear: item.seq,
		}
	}
	return b.build(), meta
}

func TestSeasonalProfile_RetailQ4Spike(t *testing.T) {
	ds, meta := seasonalRetailDataset()
	res := Calculate(Input{Dataset: ds, PeriodMeta: meta}, Options{})
	if !res.Available {
		t.Fatalf("expected Available, errors=%+v", res.Errors)
	}
	if res.SeasonalProfile.PeriodType != PeriodTypeQuarter {
		t.Fatalf("expected PeriodTypeQuarter profile, got %v", res.SeasonalProfile.PeriodType)
	}
	if len(res.SeasonalProfile.Periods) != 4 {
		t.Fatalf("expected 4 seasonal buckets (Q1-Q4), got %d", len(res.SeasonalProfile.Periods))
	}

	byseq := make(map[int]SeasonalPeriod)
	for _, p := range res.SeasonalProfile.Periods {
		byseq[p.SequenceInYear] = p
	}
	q4 := byseq[4]
	q1 := byseq[1]
	if !q4.AverageNWCPercentOfRevenue.Available || !q1.AverageNWCPercentOfRevenue.Available {
		t.Fatal("expected both Q1 and Q4 averages available")
	}
	if q4.SampleSize != 2 {
		t.Errorf("expected Q4 SampleSize 2 (both fiscal years), got %d", q4.SampleSize)
	}
	if q4.AverageNWCPercentOfRevenue.Value <= q1.AverageNWCPercentOfRevenue.Value {
		t.Errorf("expected Q4 NWC%%revenue (%v) > Q1 (%v) due to holiday inventory build",
			q4.AverageNWCPercentOfRevenue.Value, q1.AverageNWCPercentOfRevenue.Value)
	}
}

func TestSeasonalProfile_AnnualOnlyDatasetIsEmpty(t *testing.T) {
	res := Calculate(Input{Dataset: fourYearDataset(), PeriodMeta: fourYearMeta()}, Options{})
	if len(res.SeasonalProfile.Periods) != 0 {
		t.Errorf("expected no seasonal profile for an annual-only dataset, got %+v", res.SeasonalProfile)
	}
}

func TestSeasonalProfile_NoPeriodMetaIsEmpty(t *testing.T) {
	ds, _ := seasonalRetailDataset()
	res := Calculate(Input{Dataset: ds}, Options{})
	if len(res.SeasonalProfile.Periods) != 0 {
		t.Errorf("expected no seasonal profile without PeriodMeta, got %+v", res.SeasonalProfile)
	}
}

func TestSeasonalProfile_MonthlyGranularity(t *testing.T) {
	b := newDataset()
	meta := make(map[financial.Period]PeriodInfo)
	for month := 1; month <= 12; month++ {
		period := financial.Period(monthPeriodLabel(2025, month))
		amount := 10000.0 + float64(month)*500
		b.add(financial.CodeBsAccountsReceivable, string(period), amount).
			add(financial.CodeBsAccountsPayable, string(period), 5000).
			add(financial.CodeRevProduct, string(period), 50000)
		meta[period] = PeriodInfo{Type: PeriodTypeMonth, FiscalYear: 2025, SequenceInYear: month}
	}
	res := Calculate(Input{Dataset: b.build(), PeriodMeta: meta}, Options{})
	if res.SeasonalProfile.PeriodType != PeriodTypeMonth {
		t.Fatalf("expected PeriodTypeMonth profile, got %v", res.SeasonalProfile.PeriodType)
	}
	if len(res.SeasonalProfile.Periods) != 12 {
		t.Fatalf("expected 12 monthly buckets, got %d", len(res.SeasonalProfile.Periods))
	}
}

func monthPeriodLabel(year, month int) string {
	if month < 10 {
		return "2025-0" + string(rune('0'+month))
	}
	return "2025-1" + string(rune('0'+month-10))
}

// TestCalculateSeasonalProfile_BucketSumOrderMatchesHistoryOrder is a
// regression test for a bug where calculateSeasonalProfile's internal
// bucket-sum accumulation ranged over a map built from history (map
// iteration order is randomized per Go's spec) rather than history itself,
// so AverageNWCPercentOfRevenue could vary across otherwise-identical calls
// whenever float64 addition is not associative for the specific values
// involved. These three NWCPercentOfRevenue values are deliberately chosen
// (verified directly in Go, not just reasoned about) so that summing them
// in a different order produces a different float64 bit pattern —
// (a+b)+c != (c+a)+b for these exact operands — making this test fail
// against the pre-fix map-range implementation regardless of which
// particular map iteration order Go's runtime happened to pick.
func TestCalculateSeasonalProfile_BucketSumOrderMatchesHistoryOrder(t *testing.T) {
	a := 0.15646189528649826
	b := 0.15808004052684904
	c := 0.1596957693719232
	wantSum := (a + b) + c // the correct, history-order sum

	meta := map[financial.Period]PeriodInfo{
		"2023-Q1": {Type: PeriodTypeQuarter, FiscalYear: 2023, SequenceInYear: 1},
		"2024-Q1": {Type: PeriodTypeQuarter, FiscalYear: 2024, SequenceInYear: 1},
		"2025-Q1": {Type: PeriodTypeQuarter, FiscalYear: 2025, SequenceInYear: 1},
	}
	history := []PeriodNWC{
		{Period: "2023-Q1", NWCPercentOfRevenue: AvailableValue(a)},
		{Period: "2024-Q1", NWCPercentOfRevenue: AvailableValue(b)},
		{Period: "2025-Q1", NWCPercentOfRevenue: AvailableValue(c)},
	}

	for i := 0; i < 20; i++ {
		profile := calculateSeasonalProfile(history, meta)
		if len(profile.Periods) != 1 {
			t.Fatalf("run %d: expected 1 bucket, got %d", i, len(profile.Periods))
		}
		got := profile.Periods[0].AverageNWCPercentOfRevenue
		if !got.Available {
			t.Fatalf("run %d: expected Available average", i)
		}
		wantAvg := wantSum / 3
		if got.Value != wantAvg {
			t.Fatalf("run %d: AverageNWCPercentOfRevenue = %v, want exactly %v (bucket sum must follow history order, not map order)", i, got.Value, wantAvg)
		}
	}
}
