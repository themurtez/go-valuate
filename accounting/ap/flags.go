package ap

import "sort"

// FlagCode is a stable identifier for one deterministic supplier/portfolio
// risk signal. This package never derives a hidden supplier-risk or
// default-probability model; every flag is a simple, documented threshold
// comparison the caller can fully see and override via Thresholds.
type FlagCode string

const (
	FlagHighOverduePercent       FlagCode = "HIGH_OVERDUE_PERCENT"
	FlagLarge60PlusBalance       FlagCode = "LARGE_60_PLUS_BALANCE"
	FlagLarge90PlusBalance       FlagCode = "LARGE_90_PLUS_BALANCE"
	FlagHighAPConcentration      FlagCode = "HIGH_AP_CONCENTRATION"
	FlagHighOverdueConcentration FlagCode = "HIGH_OVERDUE_CONCENTRATION"
	FlagDPODeterioration         FlagCode = "DPO_DETERIORATION"
	FlagAgingDeterioration       FlagCode = "AGING_DETERIORATION"
	FlagRepeatedLatePayment      FlagCode = "REPEATED_LATE_PAYMENT"
	FlagLargeDisputedBalance     FlagCode = "LARGE_DISPUTED_BALANCE"
	FlagNearTermPaymentPressure  FlagCode = "NEAR_TERM_PAYMENT_PRESSURE"
	FlagControlAccountMismatch   FlagCode = "CONTROL_ACCOUNT_MISMATCH"
	FlagVendorCreditReview       FlagCode = "VENDOR_CREDIT_REVIEW"
)

// flagCodeOrder fixes FlagCode declaration order for deterministic Flags
// sorting.
var flagCodeOrder = []FlagCode{
	FlagHighOverduePercent,
	FlagLarge60PlusBalance,
	FlagLarge90PlusBalance,
	FlagHighAPConcentration,
	FlagHighOverdueConcentration,
	FlagDPODeterioration,
	FlagAgingDeterioration,
	FlagRepeatedLatePayment,
	FlagLargeDisputedBalance,
	FlagNearTermPaymentPressure,
	FlagControlAccountMismatch,
	FlagVendorCreditReview,
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
	SupplierID string   `json:"supplier_id,omitempty"`
	Value      float64  `json:"value,omitempty"`
	Threshold  float64  `json:"threshold,omitempty"`
}

// Thresholds configures every flag's trigger point. All caller-adjustable;
// the zero value resolves to DefaultThresholds.
type Thresholds struct {
	// HighOverduePercent triggers FlagHighOverduePercent when a supplier's
	// PercentOverdue exceeds this fraction (e.g. 0.5 for 50%). Default 0.5.
	HighOverduePercent float64 `json:"high_overdue_percent,omitempty"`
	// Large60PlusBalance/Large90PlusBalance trigger their respective flags
	// when a supplier's 60+/90+ balance exceeds this absolute dollar
	// amount. Default 0 (disabled) unless MaterialityThreshold/
	// MaterialityPercentOfAP resolves a value — see resolveThresholds.
	Large60PlusBalance float64 `json:"large_60_plus_balance,omitempty"`
	Large90PlusBalance float64 `json:"large_90_plus_balance,omitempty"`
	// HighAPConcentrationShare triggers FlagHighAPConcentration when
	// ConcentrationSummary.TotalAP's largest-entity share exceeds this
	// fraction. Default 0.25 (25%).
	HighAPConcentrationShare float64 `json:"high_ap_concentration_share,omitempty"`
	// SupplierOverdueConcentrationShare triggers
	// FlagHighOverdueConcentration when a single supplier's share of total
	// OverdueTotal exceeds this fraction. Default 0.25.
	SupplierOverdueConcentrationShare float64 `json:"supplier_overdue_concentration_share,omitempty"`
	// DPODeteriorationDays triggers FlagDPODeterioration when
	// DPOHistory.FirstVsLastChange exceeds this many days. Default 5.
	DPODeteriorationDays float64 `json:"dpo_deterioration_days,omitempty"`
	// AgingDeteriorationPercent triggers FlagAgingDeterioration when
	// AgingTrend's overdue total increased by more than this fraction
	// first-vs-last. Default 0.15 (15%).
	AgingDeteriorationPercent float64 `json:"aging_deterioration_percent,omitempty"`
	// RepeatedLatePaymentCount triggers FlagRepeatedLatePayment for a
	// supplier with at least this many payments beyond terms
	// (SupplierPayment.Date after the matched Payable.DueDate). Default 3.
	RepeatedLatePaymentCount int `json:"repeated_late_payment_count,omitempty"`
	// LargeDisputedBalance triggers FlagLargeDisputedBalance when a
	// supplier's DisputedAmount exceeds this absolute dollar amount.
	// Default 0 (disabled) unless resolved from materiality.
	LargeDisputedBalance float64 `json:"large_disputed_balance,omitempty"`
	// VendorCreditReview triggers FlagVendorCreditReview when a
	// supplier's VendorCreditAmount (in magnitude) exceeds this absolute
	// dollar amount. Default 0 (disabled) unless resolved from
	// materiality.
	VendorCreditReview float64 `json:"vendor_credit_review,omitempty"`
	// NearTermCoverageMin triggers FlagNearTermPaymentPressure when
	// PaymentPressure's 7-day window NearTermCoverage is available and
	// below this ratio (e.g. 1.0 meaning liquidity does not fully cover
	// AP due in 7 days). Default 1.0.
	NearTermCoverageMin float64 `json:"near_term_coverage_min,omitempty"`
}

// DefaultThresholds returns this package's baseline flag trigger points.
func DefaultThresholds() Thresholds {
	return Thresholds{
		HighOverduePercent:                0.5,
		HighAPConcentrationShare:          0.25,
		SupplierOverdueConcentrationShare: 0.25,
		DPODeteriorationDays:              5,
		AgingDeteriorationPercent:         0.15,
		RepeatedLatePaymentCount:          3,
		NearTermCoverageMin:               1.0,
	}
}

// resolveThresholds merges t over DefaultThresholds field by field (a
// zero-valued field takes the default), then applies materiality as the
// fallback for the balance-based thresholds that have no fixed numeric
// default (they scale with portfolio size, so their sensible default is
// "material," not a fixed dollar figure).
func resolveThresholds(t Thresholds, materialityAbs float64) Thresholds {
	d := DefaultThresholds()
	if t.HighOverduePercent == 0 {
		t.HighOverduePercent = d.HighOverduePercent
	}
	if t.HighAPConcentrationShare == 0 {
		t.HighAPConcentrationShare = d.HighAPConcentrationShare
	}
	if t.SupplierOverdueConcentrationShare == 0 {
		t.SupplierOverdueConcentrationShare = d.SupplierOverdueConcentrationShare
	}
	if t.DPODeteriorationDays == 0 {
		t.DPODeteriorationDays = d.DPODeteriorationDays
	}
	if t.AgingDeteriorationPercent == 0 {
		t.AgingDeteriorationPercent = d.AgingDeteriorationPercent
	}
	if t.RepeatedLatePaymentCount == 0 {
		t.RepeatedLatePaymentCount = d.RepeatedLatePaymentCount
	}
	if t.NearTermCoverageMin == 0 {
		t.NearTermCoverageMin = d.NearTermCoverageMin
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
	if t.VendorCreditReview == 0 {
		t.VendorCreditReview = materialityAbs
	}
	return t
}

// flagInputs bundles everything computeFlags needs, kept as one struct so
// the function signature stays readable.
type flagInputs struct {
	suppliers         []SupplierSummary
	sortedBuckets     []BucketDefinition
	concentration     ConcentrationSummary
	overdueTotal      float64
	o60Total          float64
	o90Total          float64
	dpoHistory        DPOHistory
	agingTrend        AgingTrend
	latePaymentCounts map[string]int // supplierID -> count of payments beyond terms
	paymentPressure   PaymentPressure
	controlReconciled ControlAccountReconciliation
	thresholds        Thresholds
}

// computeFlags evaluates every FlagCode rule and returns triggered flags
// sorted by FlagCode declaration order, then SupplierID.
func computeFlags(in flagInputs) []Flag {
	var flags []Flag
	t := in.thresholds

	for _, s := range in.suppliers {
		if s.PercentOverdue.Available && s.PercentOverdue.Value > t.HighOverduePercent {
			flags = append(flags, Flag{Code: FlagHighOverduePercent, SupplierID: s.SupplierID, Value: s.PercentOverdue.Value, Threshold: t.HighOverduePercent,
				Message: "supplier overdue percentage exceeds threshold"})
		}
		bal60 := supplierOverdueBucketTotal(s, in.sortedBuckets, 60)
		if t.Large60PlusBalance > 0 && bal60 > t.Large60PlusBalance {
			flags = append(flags, Flag{Code: FlagLarge60PlusBalance, SupplierID: s.SupplierID, Value: bal60, Threshold: t.Large60PlusBalance,
				Message: "supplier 60+ day balance exceeds threshold"})
		}
		bal90 := supplierOverdueBucketTotal(s, in.sortedBuckets, 90)
		if t.Large90PlusBalance > 0 && bal90 > t.Large90PlusBalance {
			flags = append(flags, Flag{Code: FlagLarge90PlusBalance, SupplierID: s.SupplierID, Value: bal90, Threshold: t.Large90PlusBalance,
				Message: "supplier 90+ day balance exceeds threshold"})
		}
		if t.LargeDisputedBalance > 0 && s.DisputedAmount > t.LargeDisputedBalance {
			flags = append(flags, Flag{Code: FlagLargeDisputedBalance, SupplierID: s.SupplierID, Value: s.DisputedAmount, Threshold: t.LargeDisputedBalance,
				Message: "supplier disputed balance exceeds threshold"})
		}
		if t.VendorCreditReview > 0 && -s.VendorCreditAmount > t.VendorCreditReview {
			flags = append(flags, Flag{Code: FlagVendorCreditReview, SupplierID: s.SupplierID, Value: s.VendorCreditAmount, Threshold: t.VendorCreditReview,
				Message: "supplier vendor credit balance exceeds threshold and warrants review"})
		}
		if in.overdueTotal != 0 {
			share := s.OverdueTotal / in.overdueTotal
			if share > t.SupplierOverdueConcentrationShare {
				flags = append(flags, Flag{Code: FlagHighOverdueConcentration, SupplierID: s.SupplierID, Value: share, Threshold: t.SupplierOverdueConcentrationShare,
					Message: "supplier share of total overdue balance exceeds threshold"})
			}
		}
		if count := in.latePaymentCounts[s.SupplierID]; count >= t.RepeatedLatePaymentCount && t.RepeatedLatePaymentCount > 0 {
			flags = append(flags, Flag{Code: FlagRepeatedLatePayment, SupplierID: s.SupplierID, Value: float64(count), Threshold: float64(t.RepeatedLatePaymentCount),
				Message: "supplier has repeated late payments"})
		}
	}

	if in.concentration.TotalAP.Available && len(in.concentration.TotalAP.History) > 0 {
		latest := in.concentration.TotalAP.History[len(in.concentration.TotalAP.History)-1]
		if latest.LargestEntityShare.Available && latest.LargestEntityShare.Value > t.HighAPConcentrationShare {
			flags = append(flags, Flag{Code: FlagHighAPConcentration, Value: latest.LargestEntityShare.Value, Threshold: t.HighAPConcentrationShare,
				Message: "largest supplier's share of total AP exceeds threshold"})
		}
	}

	if in.dpoHistory.Available && in.dpoHistory.FirstVsLastChange.Available && in.dpoHistory.FirstVsLastChange.Value > t.DPODeteriorationDays {
		flags = append(flags, Flag{Code: FlagDPODeterioration, Value: in.dpoHistory.FirstVsLastChange.Value, Threshold: t.DPODeteriorationDays,
			Message: "DPO increased more than threshold days first-vs-last"})
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

	if in.paymentPressure.Available {
		for _, w := range in.paymentPressure.Windows {
			if w.Days != 7 {
				continue
			}
			if w.NearTermCoverage.Available && w.NearTermCoverage.Value < t.NearTermCoverageMin {
				flags = append(flags, Flag{Code: FlagNearTermPaymentPressure, Value: w.NearTermCoverage.Value, Threshold: t.NearTermCoverageMin,
					Message: "near-term (7-day) liquidity coverage of AP due is below threshold"})
			}
		}
	}

	if in.controlReconciled.Available && !in.controlReconciled.Reconciled {
		flags = append(flags, Flag{Code: FlagControlAccountMismatch, Value: in.controlReconciled.Difference, Threshold: in.controlReconciled.Tolerance,
			Message: "subledger AP total does not match supplied GL control account balance"})
	}

	sort.SliceStable(flags, func(i, j int) bool {
		ri, rj := flagRank(flags[i].Code), flagRank(flags[j].Code)
		if ri != rj {
			return ri < rj
		}
		return flags[i].SupplierID < flags[j].SupplierID
	})
	return flags
}
