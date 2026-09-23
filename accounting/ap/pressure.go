package ap

import "time"

// PaymentPressureInput is the optional caller-supplied liquidity context
// used to compute PaymentPressure. This package never fabricates a
// liquidity figure — PaymentPressure is unavailable unless a caller
// explicitly supplies this.
type PaymentPressureInput struct {
	// CashAvailable is the caller's current cash-on-hand figure.
	CashAvailable *float64 `json:"cash_available,omitempty"`
	// ExpectedNearTermInflows is the caller's own estimate of cash
	// expected to arrive within the near-term window (e.g. from AR
	// collections) — this package never computes or infers this figure
	// itself; it only consumes what the caller supplies.
	ExpectedNearTermInflows *float64 `json:"expected_near_term_inflows,omitempty"`
}

// PaymentPressureWindow is one horizon's AP-due-soon figure paired
// against caller-supplied liquidity, when available.
type PaymentPressureWindow struct {
	Days int `json:"days"`
	// APDue is the sum of open, non-credit payable OpenAmount with
	// DaysUntilDue <= Days (including anything already past due — a
	// near-term obligation regardless of whether it is technically
	// "already due").
	APDue float64 `json:"ap_due"`
	// NearTermCoverage is (CashAvailable + ExpectedNearTermInflows) /
	// APDue, when both liquidity inputs and a nonzero APDue are
	// available. Unavailable if PaymentPressureInput was not supplied or
	// APDue is zero.
	NearTermCoverage AmountValue `json:"near_term_coverage"`
	// Shortfall is APDue - (CashAvailable + ExpectedNearTermInflows), when
	// positive; reported as an AmountValue so "no shortfall" (available,
	// value <= 0 or Unavailable due to missing liquidity input) is
	// distinguishable from "shortfall of 0 dollars computed."
	Shortfall AmountValue `json:"shortfall"`
}

// PaymentPressure is section 13's optional near-term liquidity-pressure
// report. Available only when Options.PaymentPressure was supplied.
// Deliberately separate from any multi-week cash forecast — this package
// computes only known open AP against a single caller-supplied liquidity
// snapshot, never a projection of future cash flows.
type PaymentPressure struct {
	Available bool                    `json:"available"`
	Windows   []PaymentPressureWindow `json:"windows,omitempty"`
	// ExcludedDisputed reports whether disputed payables were excluded
	// from the APDue figures above, per
	// Options.ExcludeDisputedFromPaymentPressure.
	ExcludedDisputed bool `json:"excluded_disputed"`
}

// paymentPressureWindowDays are the fixed 7/14/30-day windows the task
// specifies.
var paymentPressureWindowDays = []int{7, 14, 30}

// buildPaymentPressure sums open AP due within each fixed window and
// compares it against caller-supplied liquidity, when supplied.
func buildPaymentPressure(rows []payableAging, asOf time.Time, input *PaymentPressureInput, excludeDisputed bool) PaymentPressure {
	if input == nil {
		return PaymentPressure{}
	}

	var liquidity float64
	haveLiquidity := false
	if input.CashAvailable != nil {
		liquidity += *input.CashAvailable
		haveLiquidity = true
	}
	if input.ExpectedNearTermInflows != nil {
		liquidity += *input.ExpectedNearTermInflows
		haveLiquidity = true
	}

	windows := make([]PaymentPressureWindow, 0, len(paymentPressureWindowDays))
	for _, days := range paymentPressureWindowDays {
		var due float64
		for _, row := range rows {
			if !row.includedInAgg || row.isCredit || row.p.OpenAmount == 0 {
				continue
			}
			if excludeDisputed && row.p.Status == StatusDisputed {
				continue
			}
			until := daysUntilDue(asOf, row.p.DueDate)
			if until <= days {
				due += row.p.OpenAmount
			}
		}
		w := PaymentPressureWindow{Days: days, APDue: due}
		if haveLiquidity {
			if due != 0 {
				w.NearTermCoverage = AvailableAmount(liquidity / due)
			}
			w.Shortfall = AvailableAmount(due - liquidity)
		}
		windows = append(windows, w)
	}

	return PaymentPressure{Available: true, Windows: windows, ExcludedDisputed: excludeDisputed}
}
