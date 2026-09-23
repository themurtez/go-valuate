package ap

// PaymentTiming is per-portfolio average days-to-pay and early/on-time/
// late behavior, computed from Input.Payments matched to their Payable's
// BillDate/DueDate/TermsDays.
type PaymentTiming struct {
	Available bool `json:"available"`
	// AverageDaysToPay is the mean of (Payment.Date - Payable.BillDate)
	// across every SupplierPayment matched to a known Payable, in days.
	AverageDaysToPay AmountValue `json:"average_days_to_pay"`
	// AverageDaysBeyondTerms is the mean of (Payment.Date -
	// Payable.DueDate) where the matched Payable has a DueDate (i.e.
	// payment delay beyond contractual due date, which can be negative
	// for early payment).
	AverageDaysBeyondTerms AmountValue `json:"average_days_beyond_terms"`
	// PercentPaidOnOrBeforeDue is the fraction of matched payments where
	// Payment.Date <= Payable.DueDate.
	PercentPaidOnOrBeforeDue AmountValue `json:"percent_paid_on_or_before_due"`
	// PercentPaidLate is the fraction of matched payments where
	// Payment.Date > Payable.DueDate.
	PercentPaidLate     AmountValue `json:"percent_paid_late"`
	MatchedPaymentCount int         `json:"matched_payment_count"`
}

func calculatePaymentTiming(payments []SupplierPayment, payablesByID map[string]Payable, excludedPayments map[string]bool) PaymentTiming {
	var daysToPaySum float64
	var daysToPayCount int
	var beyondTermsSum float64
	var beyondTermsCount int
	var onOrBeforeCount, lateCount int

	for _, sp := range payments {
		if excludedPayments[sp.ID] {
			continue
		}
		p, ok := payablesByID[sp.PayableID]
		if !ok {
			continue
		}
		if !p.BillDate.IsZero() {
			daysToPaySum += sp.Date.Sub(p.BillDate).Hours() / 24
			daysToPayCount++
		}
		if !p.DueDate.IsZero() {
			beyondTermsSum += sp.Date.Sub(p.DueDate).Hours() / 24
			beyondTermsCount++
			if sp.Date.After(p.DueDate) {
				lateCount++
			} else {
				onOrBeforeCount++
			}
		}
	}

	if daysToPayCount == 0 {
		return PaymentTiming{}
	}

	pt := PaymentTiming{Available: true, MatchedPaymentCount: daysToPayCount}
	pt.AverageDaysToPay = AvailableAmount(daysToPaySum / float64(daysToPayCount))
	if beyondTermsCount > 0 {
		pt.AverageDaysBeyondTerms = AvailableAmount(beyondTermsSum / float64(beyondTermsCount))
		pt.PercentPaidOnOrBeforeDue = AvailableAmount(float64(onOrBeforeCount) / float64(beyondTermsCount))
		pt.PercentPaidLate = AvailableAmount(float64(lateCount) / float64(beyondTermsCount))
	}
	return pt
}

// TermsAnalysis is the contractual-terms report.
type TermsAnalysis struct {
	Available bool `json:"available"`
	// AverageTermsDays is the simple mean of TermsDays across included
	// payables with TermsDays > 0.
	AverageTermsDays AmountValue `json:"average_terms_days"`
	// WeightedAverageTermsDays weights each payable's TermsDays by its
	// OriginalAmount.
	WeightedAverageTermsDays AmountValue `json:"weighted_average_terms_days"`
	// PaymentTiming is the actual observed payment behavior, when
	// Input.Payments supplies it, for distinguishing long contractual
	// terms from genuinely late payment behavior (task section 9's "this
	// distinguishes long terms from chronic lateness" instruction).
	PaymentTiming PaymentTiming `json:"payment_timing"`
}

func calculateTermsAnalysis(rows []payableAging, timing PaymentTiming) TermsAnalysis {
	var sum float64
	var count int
	var weightedSum float64
	var weightTotal float64

	for _, row := range rows {
		if !row.includedInAgg || row.isCredit || row.p.TermsDays <= 0 {
			continue
		}
		sum += float64(row.p.TermsDays)
		count++
		weightedSum += float64(row.p.TermsDays) * row.p.OriginalAmount
		weightTotal += row.p.OriginalAmount
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

// PaymentMetrics is the portfolio-level payment-history report, available
// only where underlying snapshot/payment data was supplied.
type PaymentMetrics struct {
	Available bool `json:"available"`
	// AmountPaid is the sum of Input.Payments amounts (all matched
	// payments, portfolio-wide).
	AmountPaid AmountValue `json:"amount_paid"`
	// OverdueTrend/Overdue60Trend/Overdue90Trend characterize AgingTrend's
	// first-vs-last direction for each respective series.
	OverdueTrend     string      `json:"overdue_trend,omitempty"`
	Overdue60Trend   string      `json:"overdue_60_trend,omitempty"`
	Overdue90Trend   string      `json:"overdue_90_trend,omitempty"`
	AverageDaysToPay AmountValue `json:"average_days_to_pay"`
	// SuppliersImproving/SuppliersDeteriorating count suppliers whose
	// individual DaysPastDue position improved/worsened between the last
	// snapshot and the current analysis (via MigrationResult).
	SuppliersImproving     int `json:"suppliers_improving"`
	SuppliersDeteriorating int `json:"suppliers_deteriorating"`
}

func calculatePaymentMetrics(payments []SupplierPayment, excludedPayments map[string]bool, agingTrend AgingTrend, timing PaymentTiming, migration MigrationResult) PaymentMetrics {
	available := len(payments) > 0 || agingTrend.Available || migration.Available
	if !available {
		return PaymentMetrics{}
	}

	pm := PaymentMetrics{Available: true}

	var paid float64
	var any bool
	for _, sp := range payments {
		if excludedPayments[sp.ID] {
			continue
		}
		paid += sp.Amount
		any = true
	}
	if any {
		pm.AmountPaid = AvailableAmount(paid)
	}

	if agingTrend.Available && len(agingTrend.Points) >= 2 {
		var overdue, o60, o90 []float64
		for _, pt := range agingTrend.Points {
			overdue = append(overdue, pt.OverdueTotal)
			o60 = append(o60, pt.Overdue60Plus)
			o90 = append(o90, pt.Overdue90Plus)
		}
		pm.OverdueTrend = trendDirection(overdue)
		pm.Overdue60Trend = trendDirection(o60)
		pm.Overdue90Trend = trendDirection(o90)
	}

	pm.AverageDaysToPay = timing.AverageDaysToPay

	if migration.Available {
		pm.SuppliersImproving = migration.SuppliersImproving
		pm.SuppliersDeteriorating = migration.SuppliersDeteriorating
	}

	return pm
}

// countLatePaymentsBySupplier counts, per supplier, how many matched
// payments were made after the payable's DueDate — used by
// FlagRepeatedLatePayment.
func countLatePaymentsBySupplier(payments []SupplierPayment, excluded map[string]bool, payablesByID map[string]Payable) map[string]int {
	counts := map[string]int{}
	for _, sp := range payments {
		if excluded[sp.ID] {
			continue
		}
		p, ok := payablesByID[sp.PayableID]
		if !ok || p.DueDate.IsZero() {
			continue
		}
		if sp.Date.After(p.DueDate) {
			counts[sp.SupplierID]++
		}
	}
	return counts
}
