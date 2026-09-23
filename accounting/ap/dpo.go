package ap

import "github.com/themurtez/go-valuate/financial"

// DPOResult is a single DPO figure with its basis/quality labeled —
// distinguishing textbook-correct purchases-basis DPO from the more
// commonly available COGS-proxy approximation, per the task's "never
// imply purchases were known when they were not" instruction.
type DPOResult struct {
	Available bool             `json:"available"`
	Value     float64          `json:"value"`
	Basis     DPOBasis         `json:"basis,omitempty"`
	Period    financial.Period `json:"period,omitempty"`
}

// DPOHistoryPoint is one period's DPO in a historical series.
type DPOHistoryPoint struct {
	Period financial.Period `json:"period"`
	DPO    DPOResult        `json:"dpo"`
}

// DPOHistory is the historical DPO series, computed only from
// caller-supplied historical AP + matching denominators — this package
// never reconstructs historical AP from today's bills.
type DPOHistory struct {
	Available bool              `json:"available"`
	Points    []DPOHistoryPoint `json:"points,omitempty"`
	// Trend characterizes the first-vs-last direction using the same
	// on-time-payment-behavior framing accounting/ar.DSOHistory.Trend
	// uses for collections: "improving" means DPO decreased (paying
	// faster, closer to terms); "deteriorating" means DPO increased
	// (paying slower). This label describes direction only — whether a
	// rising DPO is strategically good (extending float) or bad
	// (liquidity strain) depends on context this package does not judge;
	// see AGING_DETERIORATION-style flags and PaymentMetrics for the
	// supporting detail a caller needs to interpret it. "flat" means no
	// material change; "" (empty) if unavailable (fewer than two points).
	Trend string `json:"trend,omitempty"`
	// FirstVsLastChange is Points[last].DPO - Points[0].DPO.
	FirstVsLastChange AmountValue `json:"first_vs_last_change"`
	// AdjacentChanges is one entry per adjacent pair of Points,
	// Points[i+1].DPO - Points[i].DPO.
	AdjacentChanges []AmountValue `json:"adjacent_changes,omitempty"`
}

// calculateCurrentDPO computes simple DPO for the most recent
// PayablesPeriod supplied (by chronological input order — this package
// does not infer period chronology from the financial.Period string; a
// caller supplying PurchasesHistory should supply it in chronological
// order), using PortfolioSummary.TotalOpenPayables as the ending-AP
// numerator, per:
//
//	Ending AP / Period Denominator x Days
func calculateCurrentDPO(totalOpenAP float64, history []PayablesPeriod) DPOResult {
	if len(history) == 0 {
		return DPOResult{}
	}
	latest := history[len(history)-1]
	if latest.DenominatorAmount == 0 || latest.Days <= 0 {
		return DPOResult{}
	}
	basis := latest.Basis
	if basis == "" {
		basis = DPOBasisPurchases
	}
	dpo := (totalOpenAP / latest.DenominatorAmount) * float64(latest.Days)
	return DPOResult{Available: true, Value: dpo, Basis: basis, Period: latest.Period}
}

// calculateDPOHistory computes one DPO point per PayablesPeriod that
// supplies EndingAP, in caller-supplied (input) order — this package
// never derives historical AP from today's open-item list.
func calculateDPOHistory(history []PayablesPeriod) DPOHistory {
	var points []DPOHistoryPoint
	for _, pp := range history {
		if pp.EndingAP == nil || pp.DenominatorAmount == 0 || pp.Days <= 0 {
			continue
		}
		basis := pp.Basis
		if basis == "" {
			basis = DPOBasisPurchases
		}
		dpo := (*pp.EndingAP / pp.DenominatorAmount) * float64(pp.Days)
		points = append(points, DPOHistoryPoint{Period: pp.Period, DPO: DPOResult{Available: true, Value: dpo, Basis: basis, Period: pp.Period}})
	}
	if len(points) == 0 {
		return DPOHistory{}
	}

	h := DPOHistory{Available: true, Points: points}
	if len(points) >= 2 {
		first, last := points[0].DPO.Value, points[len(points)-1].DPO.Value
		change := last - first
		h.FirstVsLastChange = AvailableAmount(change)
		switch {
		case change > dpoFlatTolerance:
			h.Trend = "deteriorating"
		case change < -dpoFlatTolerance:
			h.Trend = "improving"
		default:
			h.Trend = "flat"
		}
		for i := 1; i < len(points); i++ {
			h.AdjacentChanges = append(h.AdjacentChanges, AvailableAmount(points[i].DPO.Value-points[i-1].DPO.Value))
		}
	}
	return h
}

// dpoFlatTolerance is the minimum |change in days| for DPOHistory.Trend to
// report "improving"/"deteriorating" rather than "flat".
const dpoFlatTolerance = 0.5
