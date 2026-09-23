package ar

import "sort"

// AmountValue represents a figure that may or may not be calculable,
// mirroring analytics/concentration.ConcentrationValue and
// analytics/metrics.MetricValue's identical availability convention:
// Available distinguishes "computed to be exactly 0" from "cannot be
// computed because a required input is absent" (e.g. percent-of-total-AR
// when total AR is zero — see the task's explicit "percentage unavailable,
// not NaN/Inf" instruction).
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
	// InvoiceCount is the number of receivables contributing to this
	// bucket.
	InvoiceCount int `json:"invoice_count"`
}

// CreditSummary reports customer credit memo / negative-balance exposure
// separately from invoice aging, per the package doc's "credit memos"
// section default policy of never silently netting.
type CreditSummary struct {
	// TotalCreditBalance is the sum of OpenAmount across all included
	// credit-memo-type receivables (a negative or zero number).
	TotalCreditBalance float64 `json:"total_credit_balance"`
	// CreditMemoCount is the number of included credit-memo-type
	// receivables.
	CreditMemoCount int `json:"credit_memo_count"`
	// NettingApplied echoes the CreditNettingPolicy actually used for
	// PortfolioSummary/CustomerSummary aggregate totals.
	NettingApplied CreditNettingPolicy `json:"netting_applied"`
}

// PortfolioSummary is the overall-portfolio aging result — section 10.
type PortfolioSummary struct {
	// TotalGrossReceivables is the sum of OriginalAmount across all
	// included, non-credit-memo receivables.
	TotalGrossReceivables float64 `json:"total_gross_receivables"`
	// TotalOpenReceivables is the sum of OpenAmount across all included
	// receivables (after CreditNettingPolicy is applied).
	TotalOpenReceivables float64 `json:"total_open_receivables"`
	// CurrentAmount is the OpenAmount total in the CURRENT-equivalent
	// bucket (the bucket whose MinDaysPastDue == 0), if the resolved
	// bucket schema has one.
	CurrentAmount AmountValue `json:"current_amount"`
	// Buckets is one BucketAmount per resolved BucketDefinition, in
	// ascending MinDaysPastDue order.
	Buckets []BucketAmount `json:"buckets"`
	// OverdueTotal is TotalOpenReceivables minus CurrentAmount (every
	// non-current bucket summed).
	OverdueTotal float64 `json:"overdue_total"`
	// PercentOverdue is OverdueTotal / TotalOpenReceivables. Unavailable if
	// TotalOpenReceivables is zero.
	PercentOverdue AmountValue `json:"percent_overdue"`
	// WeightedAvgDaysPastDue is the OpenAmount-weighted average
	// DaysPastDue across all included receivables. Unavailable if
	// TotalOpenReceivables is zero.
	WeightedAvgDaysPastDue AmountValue `json:"weighted_avg_days_past_due"`
	// OldestOpenReceivableDays is the maximum DaysPastDue among included
	// receivables with OpenAmount != 0. Unavailable if there are none.
	OldestOpenReceivableDays AmountValue `json:"oldest_open_receivable_days"`
	// OpenInvoiceCount is the number of included receivables with nonzero
	// OpenAmount.
	OpenInvoiceCount int `json:"open_invoice_count"`
	// OverdueInvoiceCount is the number of included receivables with
	// DaysPastDue > 0 and nonzero OpenAmount.
	OverdueInvoiceCount int `json:"overdue_invoice_count"`
	// CustomerCount is the number of distinct CustomerID values among
	// included receivables.
	CustomerCount int `json:"customer_count"`
	// DisputedAmount is the sum of OpenAmount across included receivables
	// with Status == StatusDisputed.
	DisputedAmount float64 `json:"disputed_amount"`
	// WrittenOffAmount is the sum of OpenAmount across included
	// receivables with Status == StatusWrittenOff. Zero (not Unavailable)
	// when none are supplied — written-off receivables are excluded from
	// aging by default, so this is simply their total when present.
	WrittenOffAmount float64 `json:"written_off_amount"`
	// Credits summarizes credit-memo exposure.
	Credits CreditSummary `json:"credits"`
}

// CustomerSummary is one customer's deterministic aging summary — section
// 11.
type CustomerSummary struct {
	CustomerID   string  `json:"customer_id"`
	CustomerName string  `json:"customer_name,omitempty"`
	OpenAmount   float64 `json:"open_amount"`
	// Buckets is one BucketAmount per resolved BucketDefinition, in
	// ascending MinDaysPastDue order, with Percent computed against this
	// customer's own OpenAmount.
	Buckets                []BucketAmount `json:"buckets"`
	OverdueTotal           float64        `json:"overdue_total"`
	PercentOverdue         AmountValue    `json:"percent_overdue"`
	WeightedAvgDaysPastDue AmountValue    `json:"weighted_avg_days_past_due"`
	OldestInvoiceDays      AmountValue    `json:"oldest_invoice_days"`
	InvoiceCount           int            `json:"invoice_count"`
	OverdueInvoiceCount    int            `json:"overdue_invoice_count"`
	DisputedAmount         float64        `json:"disputed_amount"`
	CreditAmount           float64        `json:"credit_amount"`
	// Materiality assesses this customer's OverdueTotal against Options'
	// resolved absolute/percent-of-total-AR materiality thresholds — see
	// MaterialityAssessment (section 22). Distinguishes a minor overdue
	// customer balance from a material one.
	Materiality MaterialityAssessment `json:"materiality"`
}

// AgingReconciliation is the explicit self-check section 30 requires: the
// aging buckets must always sum to the total open receivables figure for
// valid input.
type AgingReconciliation struct {
	TotalOpenReceivables float64 `json:"total_open_receivables"`
	SumOfBuckets         float64 `json:"sum_of_buckets"`
	Difference           float64 `json:"difference"`
	Balanced             bool    `json:"balanced"`
}

// ControlAccountReconciliation is the optional subledger-vs-GL check —
// section 31. Available only if Options.ControlAccountBalance was
// supplied.
type ControlAccountReconciliation struct {
	Available             bool    `json:"available"`
	SubledgerBalance      float64 `json:"subledger_balance"`
	ControlAccountBalance float64 `json:"control_account_balance"`
	Difference            float64 `json:"difference"`
	Tolerance             float64 `json:"tolerance"`
	Reconciled            bool    `json:"reconciled"`
}

// buildBucketAmounts aggregates a set of receivableAging rows into one
// BucketAmount per resolved bucket, in ascending MinDaysPastDue order
// (deterministic — see section 35). total is the denominator for Percent;
// if it is zero, every Percent is Unavailable rather than NaN/Inf.
func buildBucketAmounts(rows []receivableAging, sortedBuckets []BucketDefinition, total float64) []BucketAmount {
	sums := make(map[string]float64, len(sortedBuckets))
	counts := make(map[string]int, len(sortedBuckets))
	for _, row := range rows {
		if !row.includedInAgg || row.isCredit {
			continue
		}
		sums[row.bucketCode] += row.r.OpenAmount
		counts[row.bucketCode]++
	}

	out := make([]BucketAmount, 0, len(sortedBuckets))
	for _, b := range sortedBuckets {
		amt := sums[b.Code]
		var pct AmountValue
		if total != 0 {
			pct = AvailableAmount(amt / total)
		}
		out = append(out, BucketAmount{BucketCode: b.Code, Amount: amt, Percent: pct, InvoiceCount: counts[b.Code]})
	}
	return out
}

// buildCustomerSummaries aggregates rows into one CustomerSummary per
// CustomerID, sorted by OpenAmount descending then CustomerID ascending —
// the documented default order (section 11/35).
func buildCustomerSummaries(rows []receivableAging, sortedBuckets []BucketDefinition) []CustomerSummary {
	type acc struct {
		name            string
		openAmount      float64
		weightedDaysSum float64
		oldestDays      int
		hasOpen         bool
		invoiceCount    int
		overdueCount    int
		disputed        float64
		credit          float64
		rows            []receivableAging
	}
	byCustomer := map[string]*acc{}
	var order []string

	for _, row := range rows {
		if !row.includedInAgg {
			continue
		}
		a, ok := byCustomer[row.r.CustomerID]
		if !ok {
			a = &acc{}
			byCustomer[row.r.CustomerID] = a
			order = append(order, row.r.CustomerID)
		}
		if row.r.CustomerName != "" {
			a.name = row.r.CustomerName
		}
		a.invoiceCount++
		if row.isCredit {
			a.credit += row.r.OpenAmount
			continue // credits do not contribute to bucket/day-past-due stats by default.
		}
		a.openAmount += row.r.OpenAmount
		a.weightedDaysSum += row.r.OpenAmount * float64(row.daysPastDue)
		if row.r.OpenAmount != 0 {
			a.hasOpen = true
			if row.daysPastDue > a.oldestDays {
				a.oldestDays = row.daysPastDue
			}
		}
		if row.daysPastDue > 0 && row.r.OpenAmount != 0 {
			a.overdueCount++
		}
		if row.r.Status == StatusDisputed {
			a.disputed += row.r.OpenAmount
		}
		a.rows = append(a.rows, row)
	}

	out := make([]CustomerSummary, 0, len(order))
	for _, cid := range order {
		a := byCustomer[cid]
		cs := CustomerSummary{
			CustomerID:          cid,
			CustomerName:        a.name,
			OpenAmount:          a.openAmount,
			Buckets:             buildBucketAmounts(a.rows, sortedBuckets, a.openAmount),
			InvoiceCount:        a.invoiceCount,
			OverdueInvoiceCount: a.overdueCount,
			DisputedAmount:      a.disputed,
			CreditAmount:        a.credit,
		}
		current := 0.0
		for _, b := range cs.Buckets {
			if bucketIsCurrent(sortedBuckets, b.BucketCode) {
				current += b.Amount
			}
		}
		cs.OverdueTotal = a.openAmount - current
		if a.openAmount != 0 {
			cs.PercentOverdue = AvailableAmount(cs.OverdueTotal / a.openAmount)
			cs.WeightedAvgDaysPastDue = AvailableAmount(a.weightedDaysSum / a.openAmount)
		}
		if a.hasOpen {
			cs.OldestInvoiceDays = AvailableAmount(float64(a.oldestDays))
		}
		out = append(out, cs)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].OpenAmount != out[j].OpenAmount {
			return out[i].OpenAmount > out[j].OpenAmount
		}
		return out[i].CustomerID < out[j].CustomerID
	})
	return out
}

// bucketIsCurrent reports whether code identifies the "current" bucket —
// the bucket (from sortedBuckets, ascending MinDaysPastDue) whose
// MinDaysPastDue == 0. Used to compute OverdueTotal without hard-coding
// the literal string "CURRENT" as a semantic identifier, per the task's
// "do not hard-code UI labels as semantic identifiers" instruction.
func bucketIsCurrent(sortedBuckets []BucketDefinition, code string) bool {
	for _, b := range sortedBuckets {
		if b.Code == code {
			return b.MinDaysPastDue == 0
		}
	}
	return false
}
