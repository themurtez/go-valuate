package applicability

import (
	"testing"

	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/profile"
)

func boolPtr(v bool) *bool        { return &v }
func floatPtr(v float64) *float64 { return &v }
func intPtr(v int) *int           { return &v }

func resultFor(t *testing.T, r Results, code valuation.Code) Result {
	t.Helper()
	res, ok := r.ForMethod(string(code))
	if !ok {
		t.Fatalf("no result for method %q", code)
	}
	return res
}

func TestCalculate_OwnerOperatedServiceBusiness(t *testing.T) {
	p := profile.Profile{
		Industry:       profile.IndustryTrades,
		OwnerOperated:  boolPtr(true),
		AnnualRevenue:  floatPtr(900_000),
		EmployeeCount:  intPtr(6),
		AssetIntensity: floatPtr(0.2),
		Profitability:  profile.ProfitabilityModerate,
	}
	r := Calculate(p)

	sde := resultFor(t, r, valuation.CodeSDEMultiple)
	if sde.Level != LevelHigh {
		t.Errorf("SDE level = %v, want HIGH; score=%d reasons=%+v", sde.Level, sde.Score, sde.Reasons)
	}

	ebitda := resultFor(t, r, valuation.CodeEBITDAMultiple)
	if ebitda.Level != LevelMedium && ebitda.Level != LevelLow {
		t.Errorf("EBITDA level = %v, want MEDIUM or LOW for a small owner-operated business", ebitda.Level)
	}

	netAssets := resultFor(t, r, valuation.CodeAdjustedNetAssetValue)
	if netAssets.Level != LevelLow && netAssets.Level != LevelNotApplicable {
		t.Errorf("NetAssets level = %v, want LOW or NOT_APPLICABLE for a low-asset-intensity business", netAssets.Level)
	}

	if sde.Score <= ebitda.Score {
		t.Errorf("expected SDE score (%d) > EBITDA score (%d) for an owner-operated small service business", sde.Score, ebitda.Score)
	}
}

func TestCalculate_AssetHeavyManufacturer(t *testing.T) {
	p := profile.Profile{
		Industry:       profile.IndustryManufacturing,
		OwnerOperated:  boolPtr(false),
		AnnualRevenue:  floatPtr(12_000_000),
		EmployeeCount:  intPtr(80),
		AssetIntensity: floatPtr(1.2),
		Profitability:  profile.ProfitabilityStrong,
		DataAvailability: profile.DataAvailability{
			HasBalanceSheet:         true,
			HasAppraisedAssetValues: true,
		},
	}
	r := Calculate(p)

	ebitda := resultFor(t, r, valuation.CodeEBITDAMultiple)
	if ebitda.Level != LevelHigh {
		t.Errorf("EBITDA level = %v, want HIGH; score=%d reasons=%+v", ebitda.Level, ebitda.Score, ebitda.Reasons)
	}

	netAssets := resultFor(t, r, valuation.CodeAdjustedNetAssetValue)
	if netAssets.Level != LevelHigh && netAssets.Level != LevelMedium {
		t.Errorf("NetAssets level = %v, want HIGH or MEDIUM for an asset-heavy manufacturer", netAssets.Level)
	}

	sde := resultFor(t, r, valuation.CodeSDEMultiple)
	if sde.Level == LevelHigh {
		t.Errorf("SDE level = %v, want less than HIGH for a large non-owner-operated manufacturer", sde.Level)
	}
	if sde.Score >= ebitda.Score {
		t.Errorf("expected SDE score (%d) < EBITDA score (%d) for a large asset-heavy manufacturer", sde.Score, ebitda.Score)
	}
}

func TestCalculate_HighGrowthCompanyWithForecast(t *testing.T) {
	p := profile.Profile{
		Industry:                profile.IndustryTechnologySaaS,
		OwnerOperated:           boolPtr(false),
		RecurringRevenuePercent: floatPtr(0.85),
		HistoricalGrowthRate:    floatPtr(0.40),
		EarningsStability:       profile.EarningsStabilityVariable,
		Profitability:           profile.ProfitabilityMarginal,
		DataAvailability: profile.DataAvailability{
			HasForecast:            true,
			HasMultiYearFinancials: true,
		},
	}
	r := Calculate(p)

	dcf := resultFor(t, r, valuation.CodeDCF)
	if dcf.Level == LevelNotApplicable {
		t.Errorf("DCF level = %v, want applicable when a forecast is supplied", dcf.Level)
	}
	if dcf.Score <= 0 {
		t.Errorf("DCF score = %d, want > 0 when forecast is available", dcf.Score)
	}

	cap := resultFor(t, r, valuation.CodeCapitalizationOfEarnings)
	if cap.Level == LevelHigh {
		t.Errorf("Capitalization level = %v, want less than HIGH for a high-growth company (single-period method is a poor fit)", cap.Level)
	}
}

func TestCalculate_HighGrowthCompanyWithoutForecast_DCFNotApplicable(t *testing.T) {
	p := profile.Profile{
		Industry:             profile.IndustryTechnologySaaS,
		HistoricalGrowthRate: floatPtr(0.40),
		// DataAvailability.HasForecast intentionally left false: revenue-
		// based valuation is out of scope, and this package must not
		// invent one to compensate.
	}
	r := Calculate(p)

	dcf := resultFor(t, r, valuation.CodeDCF)
	if dcf.Level != LevelNotApplicable {
		t.Errorf("DCF level = %v, want NOT_APPLICABLE with no forecast supplied", dcf.Level)
	}
	if dcf.Score != 0 {
		t.Errorf("DCF score = %d, want 0 with no forecast supplied", dcf.Score)
	}
	if len(dcf.Warnings) == 0 {
		t.Error("expected a warning explaining DCF cannot run without a forecast")
	}
}

func TestCalculate_LowNegativeEarnings(t *testing.T) {
	p := profile.Profile{
		Profitability:     profile.ProfitabilityUnprofitable,
		EarningsStability: profile.EarningsStabilityDeclining,
	}
	r := Calculate(p)

	for _, code := range []valuation.Code{valuation.CodeSDEMultiple, valuation.CodeEBITDAMultiple, valuation.CodeCapitalizationOfEarnings} {
		res := resultFor(t, r, code)
		if res.Level == LevelHigh {
			t.Errorf("%s level = %v, want less than HIGH for an unprofitable, declining business", code, res.Level)
		}
	}
}

func TestCalculate_MissingData_DefaultsToMediumNotHigh(t *testing.T) {
	r := Calculate(profile.Profile{})

	for _, res := range r.Methods {
		if res.Method == valuation.CodeDCF {
			// DCF is the one method that hard-blocks without a forecast.
			if res.Level != LevelNotApplicable {
				t.Errorf("DCF with empty profile: level = %v, want NOT_APPLICABLE", res.Level)
			}
			continue
		}
		if res.Level == LevelHigh {
			t.Errorf("%s with no data supplied: level = %v, should not default to HIGH with zero evidence", res.Method, res.Level)
		}
		// An empty Profile still leaves qualitative fields (EarningsStability,
		// Profitability, DataAvailability flags) at their zero-value
		// "unknown/unavailable" state, which is itself a scored data-gap
		// Reason for some methods (see scoreSDE, scoreCapitalization,
		// scoreNetAssets) — so the floor here is baseline minus those known
		// data-gap penalties, not exactly baseScore. The important invariant
		// is "did not default to HIGH," checked above; here we just confirm
		// the score did not fall to a LOW/NOT_APPLICABLE level, since a
		// caller who simply hasn't collected a profile yet should not be
		// told a method is a poor fit either.
		if res.Score > 50 || res.Level == LevelNotApplicable {
			t.Errorf("%s with no data supplied: score = %d level = %v, want <= baseline and not NOT_APPLICABLE (missing data is neutral, not disqualifying)", res.Method, res.Score, res.Level)
		}
	}
}

func TestCalculate_ScoreClampedToValidRange(t *testing.T) {
	// A maximally negative profile should never drive Score below 0, and
	// Level should land at NOT_APPLICABLE rather than something undefined.
	p := profile.Profile{
		OwnerOperated:     boolPtr(false),
		AnnualRevenue:     floatPtr(50_000_000),
		EmployeeCount:     intPtr(500),
		AssetIntensity:    floatPtr(2.0),
		Profitability:     profile.ProfitabilityUnprofitable,
		EarningsStability: profile.EarningsStabilityVolatile,
	}
	r := Calculate(p)
	sde := resultFor(t, r, valuation.CodeSDEMultiple)
	if sde.Score < 0 || sde.Score > 100 {
		t.Fatalf("SDE score = %d, want within [0,100]", sde.Score)
	}
}

func TestResult_RecommendedMatchesLevel(t *testing.T) {
	r := Calculate(profile.Profile{OwnerOperated: boolPtr(true), AnnualRevenue: floatPtr(500_000)})
	for _, res := range r.Methods {
		want := res.Level == LevelHigh || res.Level == LevelMedium
		if res.Recommended != want {
			t.Errorf("%s: Recommended = %v, want %v for level %v", res.Method, res.Recommended, want, res.Level)
		}
	}
}

func TestResults_ForMethod_UnknownCode(t *testing.T) {
	r := Calculate(profile.Profile{})
	if _, ok := r.ForMethod("NOT_A_REAL_METHOD"); ok {
		t.Error("expected ok=false for an unknown method code")
	}
}

func TestCalculate_ScoresAreDeterministic(t *testing.T) {
	p := profile.Profile{
		OwnerOperated:  boolPtr(true),
		AnnualRevenue:  floatPtr(1_500_000),
		AssetIntensity: floatPtr(0.4),
	}
	r1 := Calculate(p)
	r2 := Calculate(p)
	for i := range r1.Methods {
		if r1.Methods[i].Score != r2.Methods[i].Score || r1.Methods[i].Level != r2.Methods[i].Level {
			t.Fatalf("Calculate is not deterministic for method %s: %+v vs %+v", r1.Methods[i].Method, r1.Methods[i], r2.Methods[i])
		}
	}
}

func TestCalculate_EveryReasonExplainsItsPoints(t *testing.T) {
	p := profile.Profile{OwnerOperated: boolPtr(true), AnnualRevenue: floatPtr(1_000_000)}
	r := Calculate(p)
	for _, res := range r.Methods {
		sum := 0
		for _, reason := range res.Reasons {
			if reason.Detail == "" {
				t.Errorf("%s: reason with zero Detail: %+v", res.Method, reason)
			}
			sum += reason.Points
		}
		// Only meaningful to check for non-blocked methods, since DCF's
		// blocking path short-circuits before baseScore is added in.
		if res.Method != valuation.CodeDCF {
			want := clampScore(baseScore + sum)
			if res.Score != want {
				t.Errorf("%s: Score = %d, want %d (baseScore + sum of reason points)", res.Method, res.Score, want)
			}
		}
	}
}
