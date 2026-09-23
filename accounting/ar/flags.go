package ar

import "sort"

// FlagCode is a stable identifier for one deterministic customer/portfolio
// risk signal — section 20. This package never derives a hidden
// credit-risk model; every flag is a simple, documented threshold
// comparison the caller can fully see and override via Thresholds.
type FlagCode string

const (
	FlagHighOverduePercent           FlagCode = "HIGH_OVERDUE_PERCENT"
	FlagLarge60PlusBalance           FlagCode = "LARGE_60_PLUS_BALANCE"
	FlagLarge90PlusBalance           FlagCode = "LARGE_90_PLUS_BALANCE"
	FlagHighARConcentration          FlagCode = "HIGH_AR_CONCENTRATION"
	FlagCustomerOverdueConcentration FlagCode = "CUSTOMER_OVERDUE_CONCENTRATION"
	FlagDSODeterioration             FlagCode = "DSO_DETERIORATION"
	FlagAgingDeterioration           FlagCode = "AGING_DETERIORATION"
	FlagRepeatedLatePayment          FlagCode = "REPEATED_LATE_PAYMENT"
	FlagLargeDisputedBalance         FlagCode = "LARGE_DISPUTED_BALANCE"
	FlagCreditBalanceReview          FlagCode = "CREDIT_BALANCE_REVIEW"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting (section 35).
var flagCodeOrder = []FlagCode{
	FlagHighOverduePercent,
	FlagLarge60PlusBalance,
	FlagLarge90PlusBalance,
	FlagHighARConcentration,
	FlagCustomerOverdueConcentration,
	FlagDSODeterioration,
	FlagAgingDeterioration,
	FlagRepeatedLatePayment,
	FlagLargeDisputedBalance,
	FlagCreditBalanceReview,
}

func flagRank(c FlagCode) int {
	for i, fc := range flagCodeOrder {
		if fc == c {
			return i
		}
	}
	return len(flagCodeOrder)
}

// Flag is one deterministic risk signal Calculate triggered.
type Flag struct {
	Code       FlagCode `json:"code"`
	Message    string   `json:"message"`
	CustomerID string   `json:"customer_id,omitempty"`
	Value      float64  `json:"value,omitempty"`
	Threshold  float64  `json:"threshold,omitempty"`
}

// Thresholds configures every flag's trigger point. All caller-adjustable
// per the task's "thresholds must be caller-configurable" instruction; the
// zero value resolves to DefaultThresholds.
type Thresholds struct {
	// HighOverduePercent triggers FlagHighOverduePercent when a customer's
	// PercentOverdue exceeds this fraction (e.g. 0.5 for 50%). Default 0.5.
	HighOverduePercent float64 `json:"high_overdue_percent,omitempty"`
	// Large60PlusBalance/Large90PlusBalance trigger their respective flags
	// when a customer's 60+/90+ balance exceeds this absolute dollar
	// amount. Default 0 (disabled) unless MaterialityThreshold/
	// MaterialityPercentOfAR resolves a value — see resolveThresholds.
	Large60PlusBalance float64 `json:"large_60_plus_balance,omitempty"`
	Large90PlusBalance float64 `json:"large_90_plus_balance,omitempty"`
	// HighARConcentrationShare triggers FlagHighARConcentration when
	// ConcentrationSummary.TotalAR's largest-entity share exceeds this
	// fraction. Default 0.25 (25%).
	HighARConcentrationShare float64 `json:"high_ar_concentration_share,omitempty"`
	// CustomerOverdueConcentrationShare triggers
	// FlagCustomerOverdueConcentration when a single customer's share of
	// total OverdueTotal exceeds this fraction. Default 0.25.
	CustomerOverdueConcentrationShare float64 `json:"customer_overdue_concentration_share,omitempty"`
	// DSODeteriorationDays triggers FlagDSODeterioration when
	// DSOHistory.FirstVsLastChange exceeds this many days. Default 5.
	DSODeteriorationDays float64 `json:"dso_deterioration_days,omitempty"`
	// AgingDeteriorationPercent triggers FlagAgingDeterioration when
	// AgingTrend's overdue total increased by more than this fraction
	// first-vs-last. Default 0.15 (15%).
	AgingDeteriorationPercent float64 `json:"aging_deterioration_percent,omitempty"`
	// RepeatedLatePaymentCount triggers FlagRepeatedLatePayment for a
	// customer with at least this many payments beyond terms (Payment.Date
	// after the matched Receivable.DueDate). Default 3.
	RepeatedLatePaymentCount int `json:"repeated_late_payment_count,omitempty"`
	// LargeDisputedBalance triggers FlagLargeDisputedBalance when a
	// customer's DisputedAmount exceeds this absolute dollar amount.
	// Default 0 (disabled) unless resolved from materiality.
	LargeDisputedBalance float64 `json:"large_disputed_balance,omitempty"`
	// CreditBalanceReview triggers FlagCreditBalanceReview when a
	// customer's CreditAmount (in magnitude) exceeds this absolute dollar
	// amount. Default 0 (disabled) unless resolved from materiality.
	CreditBalanceReview float64 `json:"credit_balance_review,omitempty"`
}

// DefaultThresholds returns this package's baseline flag trigger points.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HighOverduePercent:                0.5,
		HighARConcentrationShare:          0.25,
		CustomerOverdueConcentrationShare: 0.25,
		DSODeteriorationDays:              5,
		AgingDeteriorationPercent:         0.15,
		RepeatedLatePaymentCount:          3,
	}
}

// resolveThresholds merges t over DefaultThresholds field by field (a
// zero-valued field takes the default), then applies materiality as the
// fallback for the two balance-based thresholds that have no fixed
// numeric default (Large60PlusBalance/Large90PlusBalance/
// LargeDisputedBalance/CreditBalanceReview scale with portfolio size, so
// their sensible default is "material," not a fixed dollar figure).
func resolveThresholds(t Thresholds, materialityAbs float64) Thresholds {
	d := DefaultThresholds()
	if t.HighOverduePercent == 0 {
		t.HighOverduePercent = d.HighOverduePercent
	}
	if t.HighARConcentrationShare == 0 {
		t.HighARConcentrationShare = d.HighARConcentrationShare
	}
	if t.CustomerOverdueConcentrationShare == 0 {
		t.CustomerOverdueConcentrationShare = d.CustomerOverdueConcentrationShare
	}
	if t.DSODeteriorationDays == 0 {
		t.DSODeteriorationDays = d.DSODeteriorationDays
	}
	if t.AgingDeteriorationPercent == 0 {
		t.AgingDeteriorationPercent = d.AgingDeteriorationPercent
	}
	if t.RepeatedLatePaymentCount == 0 {
		t.RepeatedLatePaymentCount = d.RepeatedLatePaymentCount
	}
	if t.Large60PlusBalance == 0 {
		t.Large60PlusBalance = materialityAbs
	}
	if t.Large90PlusBalance == 0 {
		t.Large90PlusBalance = materialityAbs
	}
	if t.LargeDisputedBalance == 0 {
		t.LargeDisputedBalance = materialityAbs
	}
	if t.CreditBalanceReview == 0 {
		t.CreditBalanceReview = materialityAbs
	}
	return t
}

// flagInputs bundles everything computeFlags needs, kept as one struct so
// the function signature stays readable.
type flagInputs struct {
	customers         []CustomerSummary
	sortedBuckets     []BucketDefinition
	concentration     ConcentrationSummary
	overdueTotal      float64
	o60Total          float64
	o90Total          float64
	dsoHistory        DSOHistory
	agingTrend        AgingTrend
	latePaymentCounts map[string]int // customerID -> count of payments beyond terms
	thresholds        Thresholds
}

// computeFlags evaluates every FlagCode rule and returns triggered flags
// sorted by FlagCode declaration order, then CustomerID (section 35/20).
func computeFlags(in flagInputs) []Flag {
	var flags []Flag
	t := in.thresholds

	for _, c := range in.customers {
		if c.PercentOverdue.Available && c.PercentOverdue.Value > t.HighOverduePercent {
			flags = append(flags, Flag{Code: FlagHighOverduePercent, CustomerID: c.CustomerID, Value: c.PercentOverdue.Value, Threshold: t.HighOverduePercent,
				Message: "customer overdue percentage exceeds threshold"})
		}
		bal60 := customerOverdueBucketTotal(c, in.sortedBuckets, 60)
		if t.Large60PlusBalance > 0 && bal60 > t.Large60PlusBalance {
			flags = append(flags, Flag{Code: FlagLarge60PlusBalance, CustomerID: c.CustomerID, Value: bal60, Threshold: t.Large60PlusBalance,
				Message: "customer 60+ day balance exceeds threshold"})
		}
		bal90 := customerOverdueBucketTotal(c, in.sortedBuckets, 90)
		if t.Large90PlusBalance > 0 && bal90 > t.Large90PlusBalance {
			flags = append(flags, Flag{Code: FlagLarge90PlusBalance, CustomerID: c.CustomerID, Value: bal90, Threshold: t.Large90PlusBalance,
				Message: "customer 90+ day balance exceeds threshold"})
		}
		if t.LargeDisputedBalance > 0 && c.DisputedAmount > t.LargeDisputedBalance {
			flags = append(flags, Flag{Code: FlagLargeDisputedBalance, CustomerID: c.CustomerID, Value: c.DisputedAmount, Threshold: t.LargeDisputedBalance,
				Message: "customer disputed balance exceeds threshold"})
		}
		if t.CreditBalanceReview > 0 && -c.CreditAmount > t.CreditBalanceReview {
			flags = append(flags, Flag{Code: FlagCreditBalanceReview, CustomerID: c.CustomerID, Value: c.CreditAmount, Threshold: t.CreditBalanceReview,
				Message: "customer credit balance exceeds threshold and warrants review"})
		}
		if in.overdueTotal != 0 {
			share := c.OverdueTotal / in.overdueTotal
			if share > t.CustomerOverdueConcentrationShare {
				flags = append(flags, Flag{Code: FlagCustomerOverdueConcentration, CustomerID: c.CustomerID, Value: share, Threshold: t.CustomerOverdueConcentrationShare,
					Message: "customer share of total overdue balance exceeds threshold"})
			}
		}
		if count := in.latePaymentCounts[c.CustomerID]; count >= t.RepeatedLatePaymentCount && t.RepeatedLatePaymentCount > 0 {
			flags = append(flags, Flag{Code: FlagRepeatedLatePayment, CustomerID: c.CustomerID, Value: float64(count), Threshold: float64(t.RepeatedLatePaymentCount),
				Message: "customer has repeated late payments"})
		}
	}

	if in.concentration.TotalAR.Available && len(in.concentration.TotalAR.History) > 0 {
		latest := in.concentration.TotalAR.History[len(in.concentration.TotalAR.History)-1]
		if latest.LargestEntityShare.Available && latest.LargestEntityShare.Value > t.HighARConcentrationShare {
			flags = append(flags, Flag{Code: FlagHighARConcentration, Value: latest.LargestEntityShare.Value, Threshold: t.HighARConcentrationShare,
				Message: "largest customer's share of total AR exceeds threshold"})
		}
	}

	if in.dsoHistory.Available && in.dsoHistory.FirstVsLastChange.Available && in.dsoHistory.FirstVsLastChange.Value > t.DSODeteriorationDays {
		flags = append(flags, Flag{Code: FlagDSODeterioration, Value: in.dsoHistory.FirstVsLastChange.Value, Threshold: t.DSODeteriorationDays,
			Message: "DSO increased more than threshold days first-vs-last"})
	}

	if in.agingTrend.Available && len(in.agingTrend.Points) >= 2 {
		first := in.agingTrend.Points[0].OverdueTotal
		last := in.agingTrend.Points[len(in.agingTrend.Points)-1].OverdueTotal
		if first != 0 {
			change := (last - first) / first
			if change > t.AgingDeteriorationPercent {
				flags = append(flags, Flag{Code: FlagAgingDeterioration, Value: change, Threshold: t.AgingDeteriorationPercent,
					Message: "overdue total increased more than threshold percent first-vs-last"})
			}
		}
	}

	sort.SliceStable(flags, func(i, j int) bool {
		ri, rj := flagRank(flags[i].Code), flagRank(flags[j].Code)
		if ri != rj {
			return ri < rj
		}
		return flags[i].CustomerID < flags[j].CustomerID
	})
	return flags
}
