package ar

import (
	"sort"
	"time"
)

// WriteOff is a historical write-off record — section 24. This package
// never infers bad-debt expense from current AR; write-off reporting is
// purely a pass-through summarization of caller-supplied records.
type WriteOff struct {
	ID           string    `json:"id"`
	ReceivableID string    `json:"receivable_id,omitempty"`
	CustomerID   string    `json:"customer_id"`
	Date         time.Time `json:"date"`
	Amount       float64   `json:"amount"`
	Period       string    `json:"period,omitempty"`
}

// WriteOffByPeriod is one period's aggregated write-off total.
type WriteOffByPeriod struct {
	Period string  `json:"period"`
	Amount float64 `json:"amount"`
	Count  int     `json:"count"`
}

// WriteOffByCustomer is one customer's aggregated write-off history.
type WriteOffByCustomer struct {
	CustomerID string  `json:"customer_id"`
	Amount     float64 `json:"amount"`
	Count      int     `json:"count"`
}

// WriteOffSummary is the section-24 write-off report. Available only if
// Input.WriteOffs is non-empty.
type WriteOffSummary struct {
	Available   bool                 `json:"available"`
	TotalAmount float64              `json:"total_amount"`
	ByPeriod    []WriteOffByPeriod   `json:"by_period,omitempty"`
	ByCustomer  []WriteOffByCustomer `json:"by_customer,omitempty"`
	// WriteOffRate is TotalAmount / denominator, when a caller-supplied
	// denominator (opening AR or sales) is available via
	// Options.WriteOffRateDenominator. Unavailable otherwise — this
	// package never guesses a denominator.
	WriteOffRate AmountValue `json:"write_off_rate"`
}

func calculateWriteOffSummary(writeOffs []WriteOff, denominator *float64) WriteOffSummary {
	if len(writeOffs) == 0 {
		return WriteOffSummary{}
	}
	var total float64
	byPeriod := map[string]*WriteOffByPeriod{}
	var periodOrder []string
	byCustomer := map[string]*WriteOffByCustomer{}
	var customerOrder []string

	for _, w := range writeOffs {
		if isNonFinite(w.Amount) {
			continue
		}
		total += w.Amount
		if p, ok := byPeriod[w.Period]; ok {
			p.Amount += w.Amount
			p.Count++
		} else {
			byPeriod[w.Period] = &WriteOffByPeriod{Period: w.Period, Amount: w.Amount, Count: 1}
			periodOrder = append(periodOrder, w.Period)
		}
		if c, ok := byCustomer[w.CustomerID]; ok {
			c.Amount += w.Amount
			c.Count++
		} else {
			byCustomer[w.CustomerID] = &WriteOffByCustomer{CustomerID: w.CustomerID, Amount: w.Amount, Count: 1}
			customerOrder = append(customerOrder, w.CustomerID)
		}
	}

	sort.Strings(periodOrder)
	byPeriodOut := make([]WriteOffByPeriod, 0, len(periodOrder))
	for _, p := range periodOrder {
		byPeriodOut = append(byPeriodOut, *byPeriod[p])
	}

	sort.SliceStable(customerOrder, func(i, j int) bool {
		ci, cj := byCustomer[customerOrder[i]], byCustomer[customerOrder[j]]
		if ci.Amount != cj.Amount {
			return ci.Amount > cj.Amount
		}
		return customerOrder[i] < customerOrder[j]
	})
	byCustomerOut := make([]WriteOffByCustomer, 0, len(customerOrder))
	for _, c := range customerOrder {
		byCustomerOut = append(byCustomerOut, *byCustomer[c])
	}

	summary := WriteOffSummary{Available: true, TotalAmount: total, ByPeriod: byPeriodOut, ByCustomer: byCustomerOut}
	if denominator != nil && *denominator != 0 {
		summary.WriteOffRate = AvailableAmount(total / *denominator)
	}
	return summary
}

// PaymentTiming is per-customer or portfolio-level average days-to-pay,
// computed from Input.Payments matched to their Receivable's InvoiceDate
// — section 17.
type PaymentTiming struct {
	Available bool `json:"available"`
	// AverageDaysToPay is the mean of (Payment.Date - Receivable.InvoiceDate)
	// across every Payment matched to a known Receivable, in days.
	AverageDaysToPay AmountValue `json:"average_days_to_pay"`
	// AverageDaysBeyondTerms is the mean of (Payment.Date - Receivable.DueDate)
	// where Receivable.TermsDays > 0 (i.e. payment delay beyond contractual
	// terms, which can be negative for early payment).
	AverageDaysBeyondTerms AmountValue `json:"average_days_beyond_terms"`
	MatchedPaymentCount    int         `json:"matched_payment_count"`
}

func calculatePaymentTiming(payments []Payment, receivablesByID map[string]Receivable, excludedPayments map[string]bool) PaymentTiming {
	var daysToPaySum float64
	var daysToPayCount int
	var beyondTermsSum float64
	var beyondTermsCount int

	for _, p := range payments {
		if excludedPayments[p.ID] {
			continue
		}
		r, ok := receivablesByID[p.ReceivableID]
		if !ok {
			continue
		}
		if !r.InvoiceDate.IsZero() {
			daysToPaySum += p.Date.Sub(r.InvoiceDate).Hours() / 24
			daysToPayCount++
		}
		if !r.DueDate.IsZero() {
			beyondTermsSum += p.Date.Sub(r.DueDate).Hours() / 24
			beyondTermsCount++
		}
	}

	if daysToPayCount == 0 {
		return PaymentTiming{}
	}

	pt := PaymentTiming{Available: true, MatchedPaymentCount: daysToPayCount}
	pt.AverageDaysToPay = AvailableAmount(daysToPaySum / float64(daysToPayCount))
	if beyondTermsCount > 0 {
		pt.AverageDaysBeyondTerms = AvailableAmount(beyondTermsSum / float64(beyondTermsCount))
	}
	return pt
}

// TermsAnalysis is the section-32 contractual-terms report.
type TermsAnalysis struct {
	Available bool `json:"available"`
	// AverageTermsDays is the simple mean of TermsDays across included
	// receivables with TermsDays > 0.
	AverageTermsDays AmountValue `json:"average_terms_days"`
	// WeightedAverageTermsDays weights each receivable's TermsDays by its
	// OriginalAmount.
	WeightedAverageTermsDays AmountValue `json:"weighted_average_terms_days"`
	// PaymentTiming is the actual observed payment behavior, when
	// Input.Payments supplies it, for distinguishing long contractual
	// terms from genuinely late collections (section 32).
	PaymentTiming PaymentTiming `json:"payment_timing"`
}

func calculateTermsAnalysis(rows []receivableAging, timing PaymentTiming) TermsAnalysis {
	var sum float64
	var count int
	var weightedSum float64
	var weightTotal float64

	for _, row := range rows {
		if !row.includedInAgg || row.isCredit || row.r.TermsDays <= 0 {
			continue
		}
		sum += float64(row.r.TermsDays)
		count++
		weightedSum += float64(row.r.TermsDays) * row.r.OriginalAmount
		weightTotal += row.r.OriginalAmount
	}

	if count == 0 {
		return TermsAnalysis{PaymentTiming: timing}
	}

	ta := TermsAnalysis{Available: true, PaymentTiming: timing}
	ta.AverageTermsDays = AvailableAmount(sum / float64(count))
	if weightTotal != 0 {
		ta.WeightedAverageTermsDays = AvailableAmount(weightedSum / weightTotal)
	}
	return ta
}

// CollectionMetrics is the section-16 collection-trend report, available
// only where the underlying snapshot/payment data was supplied.
type CollectionMetrics struct {
	Available bool `json:"available"`
	// AmountCollected is the sum of Input.Payments amounts (all matched
	// payments, portfolio-wide).
	AmountCollected AmountValue `json:"amount_collected"`
	// CollectionRate is AmountCollected / (prior-period TotalOpen +
	// current-period new invoicing), when Options.PriorPeriodOpenAR is
	// supplied; otherwise unavailable. Kept intentionally simple (V1): this
	// package does not attempt to reconstruct a full AR roll-forward.
	CollectionRate AmountValue `json:"collection_rate"`
	// OverdueTrend/Overdue60Trend/Overdue90Trend characterize AgingTrend's
	// first-vs-last direction for each respective series.
	OverdueTrend     string      `json:"overdue_trend,omitempty"`
	Overdue60Trend   string      `json:"overdue_60_trend,omitempty"`
	Overdue90Trend   string      `json:"overdue_90_trend,omitempty"`
	AverageDaysToPay AmountValue `json:"average_days_to_pay"`
	// CustomersImproving/CustomersDeteriorating count customers whose
	// individual DaysPastDue position improved/worsened between the last
	// snapshot and the current analysis (via MigrationResult).
	CustomersImproving     int `json:"customers_improving"`
	CustomersDeteriorating int `json:"customers_deteriorating"`
}

func trendDirection(points []float64) string {
	if len(points) < 2 {
		return ""
	}
	change := points[len(points)-1] - points[0]
	switch {
	case change > 0:
		return "deteriorating"
	case change < 0:
		return "improving"
	default:
		return "flat"
	}
}

func calculateCollectionMetrics(payments []Payment, excludedPayments map[string]bool, agingTrend AgingTrend, timing PaymentTiming, migration MigrationResult) CollectionMetrics {
	available := len(payments) > 0 || agingTrend.Available || migration.Available
	if !available {
		return CollectionMetrics{}
	}

	cm := CollectionMetrics{Available: true}

	var collected float64
	var any bool
	for _, p := range payments {
		if excludedPayments[p.ID] {
			continue
		}
		collected += p.Amount
		any = true
	}
	if any {
		cm.AmountCollected = AvailableAmount(collected)
	}

	if agingTrend.Available && len(agingTrend.Points) >= 2 {
		var overdue, o60, o90 []float64
		for _, pt := range agingTrend.Points {
			overdue = append(overdue, pt.OverdueTotal)
			o60 = append(o60, pt.Overdue60Plus)
			o90 = append(o90, pt.Overdue90Plus)
		}
		cm.OverdueTrend = trendDirection(overdue)
		cm.Overdue60Trend = trendDirection(o60)
		cm.Overdue90Trend = trendDirection(o90)
	}

	cm.AverageDaysToPay = timing.AverageDaysToPay

	if migration.Available {
		cm.CustomersImproving = migration.CustomersImproving
		cm.CustomersDeteriorating = migration.CustomersDeteriorating
	}

	return cm
}
