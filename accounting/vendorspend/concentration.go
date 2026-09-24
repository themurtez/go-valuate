package vendorspend

import (
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/financial"
)

// SpendConcentration is this package's supplier-spend concentration
// summary for the chronologically most recent period with spend — task
// section 9. Computed by reusing analytics/concentration.Calculate
// directly (each supplier's NetSpend per period adapted into
// concentration.Observation rows) rather than reimplementing HHI/top-N
// share math — mirrors accounting/ap.ConcentrationSummary/
// accounting/inventory's identical reuse pattern.
//
// Explicitly labeled "supplier spend concentration," never "vendor
// dependency" or "supplier failure risk" — task section 9's "do not
// equate concentration alone with supplier failure risk" rule. A high
// concentration figure may simply reflect a large recent purchase volume
// or negotiated terms, not operational dependency; see
// DependencySummary for the separate, caller-declared dependency
// classification.
type SpendConcentration struct {
	Available bool `json:"available"`
	// Label reiterates the "not a risk score" caveat for any consumer
	// reading Result JSON without this package's doc comments.
	Label string `json:"label,omitempty"`
	// Period is the period this snapshot applies to.
	Period string `json:"period,omitempty"`
	// Top1/Top3/Top5/Top10 are each top-N supplier share of total net
	// spend for Period — task section 9's requested cutoffs.
	Top1  Value `json:"top1"`
	Top3  Value `json:"top3"`
	Top5  Value `json:"top5"`
	Top10 Value `json:"top10"`
	HHI   Value `json:"hhi"`
}

// concentrationValueToValue converts a concentration.ConcentrationValue
// into this package's Value — pure field mapping, no calculation, so any
// concentration math stays byte-for-byte identical to
// analytics/concentration's own output (verified in
// concentration_invariant_test.go).
func concentrationValueToValue(v concentration.ConcentrationValue) Value {
	if !v.Available {
		return Unavailable()
	}
	return AvailableValue(v.Value)
}

// SupplierSpendObservations converts supplierPeriods into
// analytics/concentration Observations of NetSpend — the typed adapter a
// caller (or this package's own computeConcentration) uses to run
// concentration.Calculate itself. Exported so a caller wanting its own
// concentration.Policy/Options can build the same Observations directly.
func SupplierSpendObservations(supplierPeriods []SupplierPeriodSummary) []concentration.Observation {
	out := make([]concentration.Observation, 0, len(supplierPeriods))
	for _, sp := range supplierPeriods {
		out = append(out, concentration.Observation{
			EntityKey: sp.SupplierID,
			Period:    financial.Period(sp.Period),
			Amount:    sp.Bridge.NetSpend,
		})
	}
	return out
}

// spendConcentrationFromPeriod maps one concentration.PeriodConcentration
// into this package's SpendConcentration — pure field mapping, no
// calculation.
func spendConcentrationFromPeriod(p concentration.PeriodConcentration) SpendConcentration {
	sc := SpendConcentration{
		Available: true,
		Label:     "supplier spend concentration — not a measure of vendor dependency or supplier failure risk by itself",
		Period:    string(p.Period),
		Top1:      concentrationValueToValue(p.LargestEntityShare),
	}
	for _, ts := range p.TopNShares {
		switch ts.N {
		case 1:
			sc.Top1 = concentrationValueToValue(ts.Share)
		case 3:
			sc.Top3 = concentrationValueToValue(ts.Share)
		case 5:
			sc.Top5 = concentrationValueToValue(ts.Share)
		case 10:
			sc.Top10 = concentrationValueToValue(ts.Share)
		}
	}
	sc.HHI = concentrationValueToValue(p.HHI)
	return sc
}

// runConcentration runs analytics/concentration.Calculate over
// supplierPeriods' NetSpend observations under periodMeta and topN
// (Policy.TopN, resolved — see resolvePolicy), returning the full
// concentration.Result unavailable-safe (its Available flag is false if
// there were no observations). Called exactly once per Calculate
// invocation (see Calculate itself): computeConcentration and
// computeConcentrationHistory both derive their answer from this single
// concentration.Result rather than each re-running the full
// analytics/concentration pipeline against identical input.
func runConcentration(supplierPeriods []SupplierPeriodSummary, periodMeta map[financial.Period]concentration.PeriodInfo, topN []int) concentration.Result {
	obs := SupplierSpendObservations(supplierPeriods)
	if len(obs) == 0 {
		return concentration.Result{}
	}
	return concentration.Calculate(concentration.Input{
		Basis:        concentration.BasisSupplierSpend,
		Observations: obs,
		PeriodMeta:   periodMeta,
		Policy:       concentration.Policy{TopN: topN},
	}, concentration.Options{})
}

// spendConcentrationFromResult maps a concentration.Result's most recent
// period back into SpendConcentration.
func spendConcentrationFromResult(result concentration.Result) SpendConcentration {
	sc := SpendConcentration{Label: "supplier spend concentration — not a measure of vendor dependency or supplier failure risk by itself"}
	if !result.Available || len(result.History) == 0 {
		return sc
	}
	return spendConcentrationFromPeriod(result.History[len(result.History)-1])
}

// concentrationHistoryFromResult maps every period in a
// concentration.Result's History (chronological order) into
// SpendConcentration, used only for the increasing-concentration flag
// trend check (task section 12 — first-vs-last comparison of largest-
// supplier share across the analysis).
func concentrationHistoryFromResult(result concentration.Result) []SpendConcentration {
	if !result.Available {
		return nil
	}
	out := make([]SpendConcentration, 0, len(result.History))
	for _, h := range result.History {
		out = append(out, spendConcentrationFromPeriod(h))
	}
	return out
}

// buildConcentrationPeriodMeta converts this package's Period list into
// the concentration.PeriodInfo map analytics/concentration needs for
// chronological ordering.
func buildConcentrationPeriodMeta(periods map[string]Period) map[financial.Period]concentration.PeriodInfo {
	out := make(map[financial.Period]concentration.PeriodInfo, len(periods))
	for label, p := range periods {
		out[financial.Period(label)] = concentration.PeriodInfo{
			Type:           concentration.PeriodTypeMonth,
			FiscalYear:     p.StartDate.Year(),
			SequenceInYear: p.SequenceInYear,
		}
	}
	return out
}
