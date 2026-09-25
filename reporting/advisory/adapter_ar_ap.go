package advisory

import (
	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/analytics/ratios"
)

// latestPeriodRatios returns the analytics/ratios.PeriodRatios entry in r
// matching label, or (if label is empty, or not found) the last entry in
// r.History — mirrors latestSnapshot's identical fallback rule. Returns
// false if r is unavailable or has no history at all.
func latestPeriodRatios(r ratios.Result, label string) (ratios.PeriodRatios, bool) {
	if !r.Available || len(r.History) == 0 {
		return ratios.PeriodRatios{}, false
	}
	if label != "" {
		for _, p := range r.History {
			if string(p.Period) == label {
				return p, true
			}
		}
		return ratios.PeriodRatios{}, false
	}
	return r.History[len(r.History)-1], true
}

// latestInventoryPeriod returns the last entry in r.Periods — this
// package does not have a chronology-ordering mechanism of its own for
// inventory.PeriodSummary (accounting/inventory does not expose
// PeriodMeta-equivalent ordering on Result), so "last in Periods' own
// input order" is the best-effort answer, matching this package's
// documented label-less fallback limitation elsewhere (e.g.
// latestSnapshot).
func latestInventoryPeriod(r inventory.Result) (inventory.PeriodSummary, bool) {
	if len(r.Periods) == 0 {
		return inventory.PeriodSummary{}, false
	}
	return r.Periods[len(r.Periods)-1], true
}

// ar90PlusShare returns the share of AR portfolio open amount in buckets
// at/beyond 90 days past due, reading ar.PortfolioSummary.Buckets by
// BucketCode — this package never invents its own bucket boundaries; it
// only sums whatever buckets the source itself defined with a BucketCode
// this package recognizes as "90 or more."
func ar90PlusShare(r ar.Result) (float64, bool) {
	total := r.PortfolioSummary.TotalOpenReceivables
	if total == 0 {
		return 0, false
	}
	for _, b := range r.PortfolioSummary.Buckets {
		if b.BucketCode == bucket90PlusCode {
			return b.Amount / total, true
		}
	}
	return 0, false
}

func ap90PlusShare(r ap.Result) (float64, bool) {
	total := r.PortfolioSummary.TotalOpenPayables
	if total == 0 {
		return 0, false
	}
	for _, b := range r.PortfolioSummary.Buckets {
		if b.BucketCode == bucket90PlusCode {
			return b.Amount / total, true
		}
	}
	return 0, false
}

// bucket90PlusCode is ar/ap's own documented default terminal-bucket code
// for "91 days or more past due" (accounting/ar/buckets.go,
// accounting/ap/buckets.go's DefaultBuckets: CURRENT/1_30/31_60/61_90/
// 91_PLUS) — the closest available bucket to "over 90 days." ar/ap's own
// BucketDefinition is caller-configurable (both packages allow arbitrary
// bucket boundaries), so this package recognizes only this documented
// default code — a caller using custom bucket codes gets (0, false),
// never a silently wrong "0% over 90 days," from
// ar90PlusShare/ap90PlusShare.
const bucket90PlusCode = "91_PLUS"

func findingsFromARFlags(r ar.Result, period string) ([]Insight, []ActionItem) {
	var insights []Insight
	var actions []ActionItem
	for _, f := range r.Flags {
		insights = append(insights, Insight{
			Code: string(f.Code), Category: string(SectionWorkingCapital),
			Severity: SeverityMedium, Title: "AR flag", Statement: f.Message,
			Period: period, EntityRef: f.CustomerID,
			SourceModule: "ar", SourceCode: string(f.Code),
			SourceRefs: []SourceRef{{Module: "ar", Code: string(f.Code), Ref: f.CustomerID, Period: period}},
		})
	}
	if r.Concentration.TotalAR.Available {
		// Concentration facts feed REVENUE/action generation via
		// synthesis.go and section_revenue.go; nothing further here.
	}
	return insights, actions
}

func findingsFromAPFlags(r ap.Result, period string) ([]Insight, []ActionItem) {
	var insights []Insight
	var actions []ActionItem
	for _, f := range r.Flags {
		insights = append(insights, Insight{
			Code: string(f.Code), Category: string(SectionWorkingCapital),
			Severity: SeverityMedium, Title: "AP flag", Statement: f.Message,
			Period: period, EntityRef: f.SupplierID,
			SourceModule: "ap", SourceCode: string(f.Code),
			SourceRefs: []SourceRef{{Module: "ap", Code: string(f.Code), Ref: f.SupplierID, Period: period}},
		})
	}
	return insights, actions
}
