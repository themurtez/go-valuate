package synthetic

import (
	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/analytics/variance"
	"github.com/themurtez/go-valuate/analytics/workingcapital"
	"github.com/themurtez/go-valuate/financial"
)

// ForecastPeriodMeta returns BuildPeriodMeta's identical fiscal-year data,
// reshaped into forecast.PeriodInfo — a distinct Go type from
// metrics.PeriodInfo despite an identical field-for-field shape (this
// package's own local PeriodInfo/PeriodType duplicate, matching several
// analytics siblings' identical choice — see docs/ANALYTICS_ARCHITECTURE.md's
// common-primitives section). Used by BuildForecastInput.
func ForecastPeriodMeta() map[financial.Period]forecast.PeriodInfo {
	meta := make(map[financial.Period]forecast.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = forecast.PeriodInfo{
			Type:           forecast.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}

// WorkingCapitalPeriodMeta reshapes BuildPeriodMeta into
// workingcapital.PeriodInfo — this package's own local duplicate type,
// same rationale as ForecastPeriodMeta.
func WorkingCapitalPeriodMeta() map[financial.Period]workingcapital.PeriodInfo {
	meta := make(map[financial.Period]workingcapital.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = workingcapital.PeriodInfo{
			Type:           workingcapital.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}

// RatiosPeriodMeta reshapes BuildPeriodMeta into ratios.PeriodInfo — this
// package's own local duplicate type, same rationale as
// ForecastPeriodMeta.
func RatiosPeriodMeta() map[financial.Period]ratios.PeriodInfo {
	meta := make(map[financial.Period]ratios.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = ratios.PeriodInfo{
			Type:           ratios.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}

// RevenueQualityPeriodMeta reshapes BuildPeriodMeta into
// revenuequality.PeriodInfo — this package's own local duplicate type,
// same rationale as ForecastPeriodMeta.
func RevenueQualityPeriodMeta() map[financial.Period]revenuequality.PeriodInfo {
	meta := make(map[financial.Period]revenuequality.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = revenuequality.PeriodInfo{
			Type:           revenuequality.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}

// AnomaliesPeriodMeta reshapes BuildPeriodMeta into anomalies.PeriodInfo —
// this package's own local duplicate type, same rationale as
// ForecastPeriodMeta.
func AnomaliesPeriodMeta() map[financial.Period]anomalies.PeriodInfo {
	meta := make(map[financial.Period]anomalies.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = anomalies.PeriodInfo{
			Type:           anomalies.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}

// ConcentrationPeriodMeta reshapes BuildPeriodMeta into
// concentration.PeriodInfo — this package's own local duplicate type,
// same rationale as ForecastPeriodMeta.
func ConcentrationPeriodMeta() map[financial.Period]concentration.PeriodInfo {
	meta := make(map[financial.Period]concentration.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = concentration.PeriodInfo{
			Type:           concentration.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}

// VariancePeriodMeta reshapes BuildPeriodMeta into variance.PeriodInfo —
// this package's own local duplicate type, same rationale as
// ForecastPeriodMeta.
func VariancePeriodMeta() map[financial.Period]variance.PeriodInfo {
	meta := make(map[financial.Period]variance.PeriodInfo, len(Periods))
	for _, p := range Periods {
		src := BuildPeriodMeta()[p]
		meta[p] = variance.PeriodInfo{
			Type:           variance.PeriodType(src.Type),
			FiscalYear:     src.FiscalYear,
			SequenceInYear: src.SequenceInYear,
		}
	}
	return meta
}
