package anomalies

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial"
)

// detectNewMaterialExpenseCategories implements RuleNewMaterialExpenseCategory:
// for every chronologically adjacent period pair and every COGS/OPEX account
// where the account has no NormalizedItem at all in the prior period but
// does in the current period, this checks the current amount against the
// two-leg materiality test (absolute floor OR percent-of-revenue). Iterates
// period pairs in the outer loop (mirroring detectExpenseOutpacingRevenue/
// detectMarginDeterioration in growth.go) so toPeriod's Total Revenue is
// computed once per period pair rather than once per (account, period pair)
// combination.
func detectNewMaterialExpenseCategories(idx codeIndex, ordered []orderedPeriod, thresholds Thresholds) []Anomaly {
	var out []Anomaly
	for i := 1; i < len(ordered); i++ {
		fromPeriod, toPeriod := ordered[i-1].period, ordered[i].period
		revenue, revOK := totalRevenue(idx, toPeriod)

		for _, code := range expenseCodes {
			_, fromOK := idx.lookup(code, fromPeriod)
			to, toOK := idx.lookup(code, toPeriod)
			if fromOK || !toOK {
				continue
			}

			material, floorUsed, floorLabel := isMaterialNewAmount(to.Amount, revenue, revOK, thresholds)
			if !material {
				continue
			}

			out = append(out, Anomaly{
				Code:           RuleNewMaterialExpenseCategory,
				Severity:       AnomalySeverityWarning,
				Account:        code,
				Period:         toPeriod,
				BaselinePeriod: fromPeriod,
				Baseline:       Unavailable(),
				Observed:       AvailableValue(to.Amount),
				Delta:          to.Amount,
				Threshold:      floorUsed,
				Explanation: fmt.Sprintf(
					"%s had no reported amount in %s but reports %.2f in %s, exceeding the %s materiality threshold; unusual pattern, review recommended",
					code, fromPeriod, to.Amount, toPeriod, floorLabel,
				),
				Provenance: provenanceFor(to, to, false),
			})
		}
	}
	return out
}

// isMaterialNewAmount implements the two-leg materiality test documented on
// Thresholds.NewCategoryMaterialAmount/NewCategoryMaterialPercentOfRevenue:
// either leg crossing is sufficient, mirroring review.IsMaterial's identical
// OR logic. revenue/revOK is the current period's already-computed Total
// Revenue (computed once per period pair by the caller — see
// detectNewMaterialExpenseCategories — rather than once per account).
// floorUsed/floorLabel describe whichever leg triggered (the smaller of the
// two floors, when both are evaluable, since that is the floor amount
// actually crossed).
func isMaterialNewAmount(amount float64, revenue float64, revOK bool, thresholds Thresholds) (material bool, floorUsed float64, floorLabel string) {
	absFloor := thresholds.NewCategoryMaterialAmount
	if amount >= absFloor {
		floorUsed, floorLabel = absFloor, fmt.Sprintf("%.2f absolute", absFloor)
		material = true
	}

	if revOK && revenue != 0 && thresholds.NewCategoryMaterialPercentOfRevenue > 0 {
		revenueFloor := thresholds.NewCategoryMaterialPercentOfRevenue * abs(revenue)
		if amount >= revenueFloor {
			if !material || revenueFloor < floorUsed {
				floorUsed, floorLabel = revenueFloor, fmt.Sprintf("%.1f%% of revenue", thresholds.NewCategoryMaterialPercentOfRevenue*100)
			}
			material = true
		}
	}

	return material, floorUsed, floorLabel
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// detectAccountDisappearedReappeared implements RuleAccountDisappearedReappeared:
// for every account present anywhere in idx, this walks ordered's
// chronological sequence and flags every transition from "had a nonzero
// amount" to "no NormalizedItem at all" (disappeared) and every transition
// back from "no NormalizedItem" to "has a nonzero amount" after at least one
// gap period (reappeared). Runs for every account, not only expenses — see
// RuleAccountDisappearedReappeared's doc comment.
func detectAccountDisappearedReappeared(idx codeIndex, ordered []orderedPeriod) []Anomaly {
	var out []Anomaly
	for _, code := range idx.codes() {
		out = append(out, disappearedReappearedForCode(idx, code, ordered)...)
	}
	return out
}

func disappearedReappearedForCode(idx codeIndex, code financial.Code, ordered []orderedPeriod) []Anomaly {
	var out []Anomaly

	// lastPresentIdx tracks the index in ordered of the most recent period
	// (at or before the current index) that had a NormalizedItem reported at
	// all, or -1 if none yet. Used both to detect "disappeared" (present
	// now, absent next) and to detect "reappeared after a gap" (absent for
	// one or more periods, then present again).
	lastPresentIdx := -1
	gapActive := false

	for i, op := range ordered {
		item, ok := idx.lookup(code, op.period)
		// present means "has a NormalizedItem at all," matching
		// RuleAccountDisappearedReappeared's own doc comment ("no reported
		// amount at all (no NormalizedItem)") — an explicitly reported $0 is
		// a real, present value (e.g. "no ad spend this quarter"), not an
		// absence, so it must not be treated as present == false here.
		present := ok

		switch {
		case present && gapActive:
			// Reappeared after at least one gap period.
			gapPeriod := ordered[lastPresentIdx+1].period
			priorPeriod := ordered[lastPresentIdx].period
			priorItem, _ := idx.lookup(code, priorPeriod)
			out = append(out, Anomaly{
				Code:           RuleAccountDisappearedReappeared,
				Severity:       AnomalySeverityInfo,
				Account:        code,
				Period:         op.period,
				BaselinePeriod: gapPeriod,
				Baseline:       Unavailable(),
				Observed:       AvailableValue(item.Amount),
				Delta:          item.Amount - priorItem.Amount,
				Explanation: fmt.Sprintf(
					"%s had no reported amount from %s through the period before %s, then reports %.2f in %s; unusual pattern, review recommended",
					code, gapPeriod, op.period, item.Amount, op.period,
				),
				Provenance: provenanceFor(item, financial.NormalizedItem{}, false),
			})
			lastPresentIdx = i
			gapActive = false
		case present:
			lastPresentIdx = i
			gapActive = false
		case !present && lastPresentIdx == i-1 && lastPresentIdx >= 0:
			// Just disappeared: present in the immediately preceding period,
			// absent now. lastPresentIdx >= 0 excludes the case where the
			// account has simply never appeared yet (i == 0 with
			// lastPresentIdx still -1 would otherwise also satisfy
			// lastPresentIdx == i-1) — that is not a disappearance, since
			// there is nothing prior for it to have disappeared FROM; a
			// brand-new account appearing later is RuleNewMaterialExpenseCategory's
			// concern, not this rule's.
			priorPeriod := ordered[lastPresentIdx].period
			priorItem, _ := idx.lookup(code, priorPeriod)
			out = append(out, Anomaly{
				Code:           RuleAccountDisappearedReappeared,
				Severity:       AnomalySeverityInfo,
				Account:        code,
				Period:         op.period,
				BaselinePeriod: priorPeriod,
				Baseline:       AvailableValue(priorItem.Amount),
				Observed:       Unavailable(),
				Delta:          -priorItem.Amount,
				Explanation: fmt.Sprintf(
					"%s reported %.2f in %s but has no reported amount in %s; unusual pattern, review recommended",
					code, priorItem.Amount, priorPeriod, op.period,
				),
				Provenance: provenanceFor(financial.NormalizedItem{}, priorItem, true),
			})
			gapActive = true
		case !present && lastPresentIdx >= 0:
			gapActive = true
		}
	}
	return out
}
