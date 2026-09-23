package ap

import "sort"

// AmountValue represents a figure that may or may not be calculable,
// mirroring analytics/concentration.ConcentrationValue and
// accounting/ar.AmountValue's identical availability convention:
// Available distinguishes "computed to be exactly 0" from "cannot be
// computed because a required input is absent" (e.g. percent-of-total-AP
// when total AP is zero).
type AmountValue struct {
	Available bool    `json:"available"`
	Value     float64 `json:"value"`
}

// Unavailable is the canonical zero-information AmountValue.
func Unavailable() AmountValue { return AmountValue{} }

// AvailableAmount reports an AmountValue for a successfully computed
// figure.
func AvailableAmount(v float64) AmountValue {
	return AmountValue{Available: true, Value: v}
}

// BucketAmount is one bucket's aging figure, reported as both a dollar
// amount and a percentage of the total it was computed against.
type BucketAmount struct {
	BucketCode string      `json:"bucket_code"`
	Amount     float64     `json:"amount"`
	Percent    AmountValue `json:"percent"`
	// BillCount is the number of payables contributing to this bucket.
	BillCount int `json:"bill_count"`
}

// VendorCreditSummary reports supplier vendor-credit / negative-balance
// exposure separately from bill aging, per the package doc's "vendor
// credits" section default policy of never silently netting.
type VendorCreditSummary struct {
	// TotalCreditBalance is the sum of OpenAmount across all included
	// vendor-credit-type payables (a negative or zero number).
	TotalCreditBalance float64 `json:"total_credit_balance"`
	// VendorCreditCount is the number of included vendor-credit-type
	// payables.
	VendorCreditCount int `json:"vendor_credit_count"`
	// NettingApplied echoes the CreditNettingPolicy actually used for
	// PortfolioSummary/SupplierSummary aggregate totals.
	NettingApplied CreditNettingPolicy `json:"netting_applied"`
}

// PortfolioSummary is the overall-portfolio aging result.
type PortfolioSummary struct {
	// TotalGrossPayables is the sum of OriginalAmount across all
	// included, non-vendor-credit payables.
	TotalGrossPayables float64 `json:"total_gross_payables"`
	// TotalOpenPayables is the sum of OpenAmount across all included
	// payables (after CreditNettingPolicy is applied).
	TotalOpenPayables float64 `json:"total_open_payables"`
	// CurrentAmount is the OpenAmount total in the CURRENT-equivalent
	// bucket (the bucket whose MinDaysPastDue == 0), if the resolved
	// bucket schema has one.
	CurrentAmount AmountValue `json:"current_amount"`
	// Buckets is one BucketAmount per resolved BucketDefinition, in
	// ascending MinDaysPastDue order.
	Buckets []BucketAmount `json:"buckets"`
	// OverdueTotal is TotalOpenPayables minus CurrentAmount (every
	// non-current bucket summed).
	OverdueTotal float64 `json:"overdue_total"`
	// PercentOverdue is OverdueTotal / TotalOpenPayables. Unavailable if
	// TotalOpenPayables is zero.
	PercentOverdue AmountValue `json:"percent_overdue"`
	// WeightedAvgDaysPastDue is the OpenAmount-weighted average
	// DaysPastDue across all included payables. Unavailable if
	// TotalOpenPayables is zero.
	WeightedAvgDaysPastDue AmountValue `json:"weighted_avg_days_past_due"`
	// OldestOpenBillDays is the maximum DaysPastDue among included
	// payables with OpenAmount != 0. Unavailable if there are none.
	OldestOpenBillDays AmountValue `json:"oldest_open_bill_days"`
	// OpenBillCount is the number of included payables with nonzero
	// OpenAmount.
	OpenBillCount int `json:"open_bill_count"`
	// OverdueBillCount is the number of included payables with
	// DaysPastDue > 0 and nonzero OpenAmount.
	OverdueBillCount int `json:"overdue_bill_count"`
	// SupplierCount is the number of distinct SupplierID values among
	// included payables.
	SupplierCount int `json:"supplier_count"`
	// DisputedAmount is the sum of OpenAmount across included payables
	// with Status == StatusDisputed.
	DisputedAmount float64 `json:"disputed_amount"`
	// VendorCredits summarizes vendor-credit exposure.
	VendorCredits VendorCreditSummary `json:"vendor_credits"`
}

// SupplierSummary is one supplier's deterministic aging summary.
type SupplierSummary struct {
	SupplierID   string  `json:"supplier_id"`
	SupplierName string  `json:"supplier_name,omitempty"`
	OpenAmount   float64 `json:"open_amount"`
	// Buckets is one BucketAmount per resolved BucketDefinition, in
	// ascending MinDaysPastDue order, with Percent computed against this
	// supplier's own OpenAmount.
	Buckets                []BucketAmount `json:"buckets"`
	OverdueTotal           float64        `json:"overdue_total"`
	PercentOverdue         AmountValue    `json:"percent_overdue"`
	WeightedAvgDaysPastDue AmountValue    `json:"weighted_avg_days_past_due"`
	OldestBillDays         AmountValue    `json:"oldest_bill_days"`
	BillCount              int            `json:"bill_count"`
	OverdueBillCount       int            `json:"overdue_bill_count"`
	DisputedAmount         float64        `json:"disputed_amount"`
	VendorCreditAmount     float64        `json:"vendor_credit_amount"`
	// Materiality assesses this supplier's OverdueTotal against Options'
	// resolved absolute/percent-of-total-AP materiality thresholds — see
	// MaterialityAssessment.
	Materiality MaterialityAssessment `json:"materiality"`
}

// AgingReconciliation is the explicit self-check this package always
// computes: the aging buckets must always sum to the total open payables
// figure for valid input.
type AgingReconciliation struct {
	TotalOpenPayables float64 `json:"total_open_payables"`
	SumOfBuckets      float64 `json:"sum_of_buckets"`
	Difference        float64 `json:"difference"`
	Balanced          bool    `json:"balanced"`
}

// ControlAccountReconciliation is the optional subledger-vs-GL check.
// Available only if Options.ControlAccountBalance was supplied.
type ControlAccountReconciliation struct {
	Available             bool    `json:"available"`
	SubledgerBalance      float64 `json:"subledger_balance"`
	ControlAccountBalance float64 `json:"control_account_balance"`
	Difference            float64 `json:"difference"`
	Tolerance             float64 `json:"tolerance"`
	Reconciled            bool    `json:"reconciled"`
}

// buildBucketAmounts aggregates a set of payableAging rows into one
// BucketAmount per resolved bucket, in ascending MinDaysPastDue order
// (deterministic). total is the denominator for Percent; if it is zero,
// every Percent is Unavailable rather than NaN/Inf.
func buildBucketAmounts(rows []payableAging, sortedBuckets []BucketDefinition, total float64) []BucketAmount {
	sums := make(map[string]float64, len(sortedBuckets))
	counts := make(map[string]int, len(sortedBuckets))
	for _, row := range rows {
		if !row.includedInAgg || row.isCredit {
			continue
		}
		sums[row.bucketCode] += row.p.OpenAmount
		counts[row.bucketCode]++
	}

	out := make([]BucketAmount, 0, len(sortedBuckets))
	for _, b := range sortedBuckets {
		amt := sums[b.Code]
		var pct AmountValue
		if total != 0 {
			pct = AvailableAmount(amt / total)
		}
		out = append(out, BucketAmount{BucketCode: b.Code, Amount: amt, Percent: pct, BillCount: counts[b.Code]})
	}
	return out
}

// buildSupplierSummaries aggregates rows into one SupplierSummary per
// SupplierID, sorted by OpenAmount descending then SupplierID ascending —
// the documented default order.
func buildSupplierSummaries(rows []payableAging, sortedBuckets []BucketDefinition) []SupplierSummary {
	type acc struct {
		name            string
		openAmount      float64
		weightedDaysSum float64
		oldestDays      int
		hasOpen         bool
		billCount       int
		overdueCount    int
		disputed        float64
		credit          float64
		rows            []payableAging
	}
	bySupplier := map[string]*acc{}
	var order []string

	for _, row := range rows {
		if !row.includedInAgg {
			continue
		}
		a, ok := bySupplier[row.p.SupplierID]
		if !ok {
			a = &acc{}
			bySupplier[row.p.SupplierID] = a
			order = append(order, row.p.SupplierID)
		}
		if row.p.SupplierName != "" {
			a.name = row.p.SupplierName
		}
		a.billCount++
		if row.isCredit {
			a.credit += row.p.OpenAmount
			continue // credits do not contribute to bucket/day-past-due stats by default.
		}
		a.openAmount += row.p.OpenAmount
		a.weightedDaysSum += row.p.OpenAmount * float64(row.daysPastDue)
		if row.p.OpenAmount != 0 {
			a.hasOpen = true
			if row.daysPastDue > a.oldestDays {
				a.oldestDays = row.daysPastDue
			}
		}
		if row.daysPastDue > 0 && row.p.OpenAmount != 0 {
			a.overdueCount++
		}
		if row.p.Status == StatusDisputed {
			a.disputed += row.p.OpenAmount
		}
		a.rows = append(a.rows, row)
	}

	out := make([]SupplierSummary, 0, len(order))
	for _, sid := range order {
		a := bySupplier[sid]
		ss := SupplierSummary{
			SupplierID:         sid,
			SupplierName:       a.name,
			OpenAmount:         a.openAmount,
			Buckets:            buildBucketAmounts(a.rows, sortedBuckets, a.openAmount),
			BillCount:          a.billCount,
			OverdueBillCount:   a.overdueCount,
			DisputedAmount:     a.disputed,
			VendorCreditAmount: a.credit,
		}
		current := 0.0
		for _, b := range ss.Buckets {
			if bucketIsCurrent(sortedBuckets, b.BucketCode) {
				current += b.Amount
			}
		}
		ss.OverdueTotal = a.openAmount - current
		if a.openAmount != 0 {
			ss.PercentOverdue = AvailableAmount(ss.OverdueTotal / a.openAmount)
			ss.WeightedAvgDaysPastDue = AvailableAmount(a.weightedDaysSum / a.openAmount)
		}
		if a.hasOpen {
			ss.OldestBillDays = AvailableAmount(float64(a.oldestDays))
		}
		out = append(out, ss)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OpenAmount != out[j].OpenAmount {
			return out[i].OpenAmount > out[j].OpenAmount
		}
		return out[i].SupplierID < out[j].SupplierID
	})
	return out
}
