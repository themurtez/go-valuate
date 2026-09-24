package profitability

import (
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/financial"
)

// ConcentrationObservationsFromEntityPeriods converts one dimension's
// EntityPeriodResults into analytics/concentration Observations of
// NetRevenue — task section 43. This is a typed adapter, not a
// compile-time dependency baked into Calculate: this package's core
// types never import analytics/concentration, and a caller who does not
// want revenue concentration never needs this function. The caller runs
// concentration.Calculate on the returned Observations itself (supplying
// its own Input.PeriodMeta/Policy/Options) and may then convert the
// concentration.Result back with RevenueConcentrationFromConcentrationResult.
func ConcentrationObservationsFromEntityPeriods(entityPeriods []EntityPeriodResult) []concentration.Observation {
	out := make([]concentration.Observation, 0, len(entityPeriods))
	for _, ep := range entityPeriods {
		out = append(out, concentration.Observation{
			EntityKey: ep.EntityID,
			Period:    financial.Period(ep.Period),
			Amount:    ep.RevenueBridge.NetRevenue,
		})
	}
	return out
}

// RevenueConcentrationFromConcentrationResult converts an
// analytics/concentration Result's most recent period into this
// package's RevenueConcentration — task section 43. Called "revenue
// concentration," never "dependency risk," per the task's explicit
// naming instruction; this function performs no calculation of its own,
// only a field mapping, so any concentration math (HHI, top-N shares)
// stays byte-for-byte identical to analytics/concentration's own output.
func RevenueConcentrationFromConcentrationResult(dim Dimension, result concentration.Result) RevenueConcentration {
	rc := RevenueConcentration{Dimension: dim}
	if !result.Available || len(result.History) == 0 {
		return rc
	}
	latest := result.History[len(result.History)-1]
	rc.Top1Share = concentrationValueToValue(latest.LargestEntityShare)
	for _, ts := range latest.TopNShares {
		switch ts.N {
		case 3:
			rc.Top3Share = concentrationValueToValue(ts.Share)
		case 5:
			rc.Top5Share = concentrationValueToValue(ts.Share)
		case 10:
			rc.Top10Share = concentrationValueToValue(ts.Share)
		}
	}
	rc.HHI = concentrationValueToValue(latest.HHI)
	return rc
}

func concentrationValueToValue(v concentration.ConcentrationValue) Value {
	if !v.Available {
		return Unavailable()
	}
	return AvailableValue(v.Value)
}
