package ar

import (
	"sort"

	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/financial"
)

// ConcentrationSummary bundles total-AR concentration (computed via
// analytics/concentration reuse — section 18) with overdue-balance
// concentration (this package's own top-N-by-overdue-balance ranking —
// section 19, which the task notes is "often more operationally useful
// than total AR concentration").
type ConcentrationSummary struct {
	// TotalAR is analytics/concentration's own Result for one synthetic
	// "period" (this analysis's AsOfDate) built from each customer's total
	// OpenAmount — see buildConcentrationObservations. Preserves customer
	// IDs, never names, as the concentration identity key (task section
	// 18).
	TotalAR concentration.Result `json:"total_ar"`
	// Overdue is the top-N ranking of customers by their OverdueTotal.
	Overdue []RankedCustomer `json:"overdue,omitempty"`
	// Overdue60Plus is the top-N ranking of customers by their 60+ balance.
	Overdue60Plus []RankedCustomer `json:"overdue_60_plus,omitempty"`
	// Overdue90Plus is the top-N ranking of customers by their 90+ balance.
	Overdue90Plus []RankedCustomer `json:"overdue_90_plus,omitempty"`
}

// RankedCustomer is one customer's amount and share within a ranked list
// (overdue/60+/90+ concentration).
type RankedCustomer struct {
	CustomerID string      `json:"customer_id"`
	Amount     float64     `json:"amount"`
	Share      AmountValue `json:"share"`
	Rank       int         `json:"rank"`
}

// arPeriod is the synthetic financial.Period this package uses when
// calling analytics/concentration.Calculate, since AR aging is a
// point-in-time (AsOfDate) analysis rather than a multi-period series.
// Concentration reuse here only needs a single period's cross-section —
// History/Trend/Scenarios naturally degrade to "not chronologically
// comparable," which is correct: there is only one snapshot.
const arPeriod financial.Period = "AS_OF"

// buildTotalARConcentration adapts each customer's total OpenAmount into
// analytics/concentration.Observation rows and reuses that package's
// share/HHI/top-N calculation, per the task's explicit "reuse
// analytics/concentration... do not reimplement HHI" instruction.
func buildTotalARConcentration(customers []CustomerSummary, policy concentration.Policy) concentration.Result {
	obs := make([]concentration.Observation, 0, len(customers))
	for _, c := range customers {
		if c.OpenAmount <= 0 {
			continue
		}
		obs = append(obs, concentration.Observation{
			EntityKey: c.CustomerID,
			Period:    arPeriod,
			Amount:    c.OpenAmount,
		})
	}
	if len(obs) == 0 {
		return concentration.Result{}
	}
	in := concentration.Input{
		Basis:        concentration.BasisOther,
		Observations: obs,
		PeriodMeta: map[financial.Period]concentration.PeriodInfo{
			arPeriod: {Type: concentration.PeriodTypeFiscalYear, FiscalYear: 0},
		},
		Policy: policy,
	}
	return concentration.Calculate(in, concentration.Options{})
}

// rankByAmount ranks customers by the amount selector descending (ties
// broken by CustomerID ascending — deterministic per section 35), keeping
// only positive amounts, and returns the top n (n <= 0 means "no limit").
func rankByAmount(customers []CustomerSummary, amountOf func(CustomerSummary) float64, total float64, n int) []RankedCustomer {
	type pair struct {
		id     string
		amount float64
	}
	var pairs []pair
	for _, c := range customers {
		amt := amountOf(c)
		if amt <= 0 {
			continue
		}
		pairs = append(pairs, pair{id: c.CustomerID, amount: amt})
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
	out := make([]RankedCustomer, 0, len(pairs))
	for i, p := range pairs {
		var share AmountValue
		if total != 0 {
			share = AvailableAmount(p.amount / total)
		}
		out = append(out, RankedCustomer{CustomerID: p.id, Amount: p.amount, Share: share, Rank: i + 1})
	}
	return out
}

func customerOverdueBucketTotal(c CustomerSummary, sortedBuckets []BucketDefinition, minDays int) float64 {
	var total float64
	for _, b := range c.Buckets {
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

func bucketDefByCode(defs []BucketDefinition, code string) *BucketDefinition {
	for i := range defs {
		if defs[i].Code == code {
			return &defs[i]
		}
	}
	return nil
}

func buildConcentrationSummary(customers []CustomerSummary, sortedBuckets []BucketDefinition, overdueTotal, o60Total, o90Total float64, topN int, policy concentration.Policy) ConcentrationSummary {
	overdue := rankByAmount(customers, func(c CustomerSummary) float64 { return c.OverdueTotal }, overdueTotal, topN)
	o60 := rankByAmount(customers, func(c CustomerSummary) float64 { return customerOverdueBucketTotal(c, sortedBuckets, 60) }, o60Total, topN)
	o90 := rankByAmount(customers, func(c CustomerSummary) float64 { return customerOverdueBucketTotal(c, sortedBuckets, 90) }, o90Total, topN)

	return ConcentrationSummary{
		TotalAR:       buildTotalARConcentration(customers, policy),
		Overdue:       overdue,
		Overdue60Plus: o60,
		Overdue90Plus: o90,
	}
}
