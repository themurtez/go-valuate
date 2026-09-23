package ap

import (
	"sort"

	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/financial"
)

// ConcentrationSummary bundles total-AP concentration (computed via
// analytics/concentration reuse) with overdue-balance concentration (this
// package's own top-N-by-overdue-balance ranking).
//
// Important semantic rule (task section 11): a large AP balance with one
// supplier does NOT by itself mean this business operationally depends on
// that supplier — it may simply reflect large recent purchase volume,
// long negotiated terms, or payment timing. This is AP/payment exposure
// concentration, not vendor dependency. True operational supplier
// dependency (single-sourced parts, switching cost, lead time risk)
// belongs in the separate, later Vendor Spend Analytics module — this
// package makes no dependency claim and Label says so explicitly.
type ConcentrationSummary struct {
	// TotalAP is analytics/concentration's own Result for one synthetic
	// "period" (this analysis's AsOfDate) built from each supplier's
	// total OpenAmount — see buildTotalAPConcentration. Preserves
	// supplier IDs, never names, as the concentration identity key.
	TotalAP concentration.Result `json:"total_ap"`
	// Overdue is the top-N ranking of suppliers by their OverdueTotal.
	Overdue []RankedSupplier `json:"overdue,omitempty"`
	// Overdue60Plus is the top-N ranking of suppliers by their 60+
	// balance.
	Overdue60Plus []RankedSupplier `json:"overdue_60_plus,omitempty"`
	// Overdue90Plus is the top-N ranking of suppliers by their 90+
	// balance.
	Overdue90Plus []RankedSupplier `json:"overdue_90_plus,omitempty"`
	// Label is a fixed, always-present disclaimer distinguishing AP
	// balance concentration from operational supplier dependency.
	Label string `json:"label"`
}

const concentrationLabel = "AP/payment exposure concentration by balance; not a measure of operational supplier dependency (single-sourcing, switching cost, lead time) — see Vendor Spend Analytics for that"

// RankedSupplier is one supplier's amount and share within a ranked list
// (overdue/60+/90+ concentration).
type RankedSupplier struct {
	SupplierID string      `json:"supplier_id"`
	Amount     float64     `json:"amount"`
	Share      AmountValue `json:"share"`
	Rank       int         `json:"rank"`
}

// apPeriod is the synthetic financial.Period this package uses when
// calling analytics/concentration.Calculate, since AP aging is a
// point-in-time (AsOfDate) analysis rather than a multi-period series.
const apPeriod financial.Period = "AS_OF"

// buildTotalAPConcentration adapts each supplier's total OpenAmount into
// analytics/concentration.Observation rows and reuses that package's
// share/HHI/top-N calculation rather than reimplementing HHI.
func buildTotalAPConcentration(suppliers []SupplierSummary, policy concentration.Policy) concentration.Result {
	obs := make([]concentration.Observation, 0, len(suppliers))
	for _, s := range suppliers {
		if s.OpenAmount <= 0 {
			continue
		}
		obs = append(obs, concentration.Observation{
			EntityKey: s.SupplierID,
			Period:    apPeriod,
			Amount:    s.OpenAmount,
		})
	}
	if len(obs) == 0 {
		return concentration.Result{}
	}
	in := concentration.Input{
		Basis:        concentration.BasisOther,
		Observations: obs,
		PeriodMeta: map[financial.Period]concentration.PeriodInfo{
			apPeriod: {Type: concentration.PeriodTypeFiscalYear, FiscalYear: 0},
		},
		Policy: policy,
	}
	return concentration.Calculate(in, concentration.Options{})
}

// rankByAmount ranks suppliers by the amount selector descending (ties
// broken by SupplierID ascending — deterministic), keeping only positive
// amounts, and returns the top n (n <= 0 means "no limit").
func rankByAmount(suppliers []SupplierSummary, amountOf func(SupplierSummary) float64, total float64, n int) []RankedSupplier {
	type pair struct {
		id     string
		amount float64
	}
	var pairs []pair
	for _, s := range suppliers {
		amt := amountOf(s)
		if amt <= 0 {
			continue
		}
		pairs = append(pairs, pair{id: s.SupplierID, amount: amt})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].amount != pairs[j].amount {
			return pairs[i].amount > pairs[j].amount
		}
		return pairs[i].id < pairs[j].id
	})
	if n > 0 && len(pairs) > n {
		pairs = pairs[:n]
	}
	out := make([]RankedSupplier, 0, len(pairs))
	for i, p := range pairs {
		var share AmountValue
		if total != 0 {
			share = AvailableAmount(p.amount / total)
		}
		out = append(out, RankedSupplier{SupplierID: p.id, Amount: p.amount, Share: share, Rank: i + 1})
	}
	return out
}

func supplierOverdueBucketTotal(s SupplierSummary, sortedBuckets []BucketDefinition, minDays int) float64 {
	var total float64
	for _, b := range s.Buckets {
		def := bucketDefByCode(sortedBuckets, b.BucketCode)
		if def == nil {
			continue
		}
		if def.MinDaysPastDue >= minDays {
			total += b.Amount
		}
	}
	return total
}

func buildConcentrationSummary(suppliers []SupplierSummary, sortedBuckets []BucketDefinition, overdueTotal, o60Total, o90Total float64, topN int, policy concentration.Policy) ConcentrationSummary {
	overdue := rankByAmount(suppliers, func(s SupplierSummary) float64 { return s.OverdueTotal }, overdueTotal, topN)
	o60 := rankByAmount(suppliers, func(s SupplierSummary) float64 { return supplierOverdueBucketTotal(s, sortedBuckets, 60) }, o60Total, topN)
	o90 := rankByAmount(suppliers, func(s SupplierSummary) float64 { return supplierOverdueBucketTotal(s, sortedBuckets, 90) }, o90Total, topN)

	return ConcentrationSummary{
		TotalAP:       buildTotalAPConcentration(suppliers, policy),
		Overdue:       overdue,
		Overdue60Plus: o60,
		Overdue90Plus: o90,
		Label:         concentrationLabel,
	}
}

// sumBucketsAtLeast sums BucketAmount.Amount across buckets whose
// definition has MinDaysPastDue >= minDays.
func sumBucketsAtLeast(buckets []BucketAmount, sortedBuckets []BucketDefinition, minDays int) float64 {
	var total float64
	for _, b := range buckets {
		def := bucketDefByCode(sortedBuckets, b.BucketCode)
		if def != nil && def.MinDaysPastDue >= minDays {
			total += b.Amount
		}
	}
	return total
}

// concentrationTopN is the fixed top-N cutoff used for overdue/60+/90+
// concentration ranking (RankedSupplier lists) — kept generous since
// callers can truncate client-side; 10 mirrors
// analytics/concentration.DefaultPolicy's largest default cutoff.
const concentrationTopN = 10
