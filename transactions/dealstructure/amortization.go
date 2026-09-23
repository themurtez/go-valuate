package dealstructure

import "math"

// validateTrancheTerms reports every structural problem with t as Issues
// (ref identifies which entry t came from), and whether t is usable at
// all for amortization. usable == false means: Amount < 0,
// AnnualInterestRate < 0, AmortizationYears <= 0, TermYears <= 0,
// TermYears > AmortizationYears, an unrecognized Frequency,
// InterestOnlyYears < 0, InterestOnlyYears >= AmortizationYears, or a
// BalloonAmount that is negative or exceeds what Amount amortizing over
// AmortizationYears could leave outstanding at TermYears (a balloon
// larger than the loan's own amortizing balance at that point is not a
// coherent input — see balloonCeiling).
func validateTrancheTerms(t DebtTranche, ref string) (issues []Issue, usable bool) {
	usable = true

	if t.Amount < 0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "amount must be >= 0", Tranche: ref})
		usable = false
	}
	if t.AnnualInterestRate < 0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "annual interest rate must be >= 0", Tranche: ref})
		usable = false
	}
	if t.AmortizationYears <= 0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "amortization years must be > 0", Tranche: ref})
		usable = false
	}
	if t.TermYears <= 0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "term years must be > 0", Tranche: ref})
		usable = false
	} else if t.AmortizationYears > 0 && t.TermYears > t.AmortizationYears {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "term years must be <= amortization years", Tranche: ref})
		usable = false
	}
	if _, ok := paymentsPerYear(t.Frequency); !ok {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "frequency must be one of \"monthly\", \"quarterly\", or \"annual\"", Tranche: ref})
		usable = false
	}
	if t.InterestOnlyYears < 0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "interest-only years must be >= 0", Tranche: ref})
		usable = false
	} else if t.AmortizationYears > 0 && t.InterestOnlyYears >= t.AmortizationYears {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "interest-only years must be less than amortization years", Tranche: ref})
		usable = false
	}
	if t.BalloonAmount < 0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "balloon amount must be >= 0", Tranche: ref})
		usable = false
	}

	if usable && t.BalloonAmount > 0 {
		ceiling := balloonCeiling(t)
		// A small epsilon tolerates floating-point accumulation in
		// balloonCeiling's own amortization simulation.
		if t.BalloonAmount > ceiling+0.01 {
			issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityError, Message: "balloon amount exceeds the outstanding balance the loan's amortization schedule would leave at term end", Tranche: ref})
			usable = false
		}
	}

	if usable && t.AnnualInterestRate > 1.0 {
		issues = append(issues, Issue{Code: IssueInvalidTrancheTerms, Severity: SeverityWarning, Message: "annual interest rate is above 1.0 (100%); verify it was supplied as a decimal (0.08 for 8%), not a whole-number percent", Tranche: ref})
	}

	return issues, usable
}

// balloonCeiling returns the outstanding principal balance a
// zero-balloon amortization of t would leave at t.TermYears — the
// largest balloon amount t.BalloonAmount could coherently specify (a
// balloon cannot exceed what the loan would still owe at that point
// under its own amortization schedule). This reads the final period's
// Balloon field, not EndingBalance: buildPeriods always resolves the
// final period's EndingBalance to exactly 0 by moving whatever remains
// into that period's Balloon (this is what correctly produces a
// TermYears-driven "loan comes due" payoff even with BalloonAmount
// unset — see buildPeriods's doc comment), so EndingBalance can never be
// used to recover the pre-payoff outstanding balance.
func balloonCeiling(t DebtTranche) float64 {
	unballooned := t
	unballooned.BalloonAmount = 0
	sched := amortize(unballooned)
	if len(sched.Periods) == 0 {
		return 0
	}
	return sched.Periods[len(sched.Periods)-1].Balloon
}

// amortize derives an AmortizationSchedule from t. The caller is
// responsible for having validated t via validateTrancheTerms; amortize
// never runs on a DebtTranche that failed validation (Build skips
// invalid entries entirely — see buildDebtSchedules).
//
// The level-payment formula used is the standard fixed-rate amortizing
// loan formula:
//
//	payment = principal x rate / (1 - (1 + rate)^-n)
//
// where rate is the periodic interest rate (AnnualInterestRate /
// paymentsPerYear) and n is the total number of amortizing payments
// (AmortizationYears x paymentsPerYear) — identical to
// analytics/debt.Amortize's formula (see this package's doc comment for
// why that function is not reused directly). If rate is exactly zero,
// payment is simply principal / n (straight-line, no interest).
//
// When t.BalloonAmount is nonzero, the payment is instead sized so that
// amortizing t.Amount at this same periodic rate leaves exactly
// t.BalloonAmount outstanding after n payments (a standard "amortized as
// if it were an N-year loan, but the balance is due as a term-shorter
// balloon" structure):
//
//	payment = (principal - balloon x (1+rate)^-n) x rate / (1 - (1+rate)^-n)
//
// which reduces to the ordinary formula above when balloon is 0.
//
// The schedule runs from origination through t.TermYears (not
// necessarily all of AmortizationYears — see DebtTranche.TermYears's doc
// comment): if TermYears < AmortizationYears, the loan comes due at
// TermYears with its remaining amortizing balance payable as a
// term-driven balloon, in addition to (or, more precisely, combined
// with) any BalloonAmount already sized into the payment — the final
// period's Balloon is always whatever balance actually remains after
// that period's regular payment, which already reflects both effects.
func amortize(t DebtTranche) AmortizationSchedule {
	ppy, ok := paymentsPerYear(t.Frequency)
	if !ok {
		return AmortizationSchedule{Terms: t}
	}

	periodicRate := t.AnnualInterestRate / float64(ppy)
	n := t.AmortizationYears * float64(ppy)
	termPayments := int(math.Round(t.TermYears * float64(ppy)))

	// When BalloonAmount is explicitly set, the payment must be sized so
	// the balance reaches exactly BalloonAmount at TermYears (not at the
	// full AmortizationYears horizon) — see DebtTranche.BalloonAmount's
	// doc comment. When BalloonAmount is unset, the payment is always
	// sized on the full AmortizationYears schedule (a normal amortizing
	// payment); if TermYears < AmortizationYears in that case, the loan
	// simply comes due with whatever balance that payment naturally
	// leaves at TermYears — an implicit, unsized balloon (see
	// buildPeriods's doc comment) — rather than being re-sized around a
	// target.
	payment := levelPayment(t.Amount, 0, periodicRate, n)
	if t.BalloonAmount > 0 {
		payment = levelPayment(t.Amount, t.BalloonAmount, periodicRate, float64(termPayments))
	}

	sched := AmortizationSchedule{
		Terms:                               t,
		PaymentsPerYear:                     ppy,
		PeriodicPrincipalAndInterestPayment: payment,
		SteadyStateAnnualDebtService:        payment * float64(ppy),
		BalloonPayment:                      t.BalloonAmount,
	}

	if t.InterestOnlyYears > 0 {
		sched.PeriodicInterestOnlyPayment = t.Amount * periodicRate
	}
	ioPeriods := int(math.Round(t.InterestOnlyYears * float64(ppy)))

	sched.Periods = buildPeriods(t.Amount, periodicRate, payment, sched.PeriodicInterestOnlyPayment, ioPeriods, termPayments)

	sched.FirstYearAnnualDebtService = firstYearDebtService(sched.Periods, ppy)

	return sched
}

// levelPayment returns the periodic principal-and-interest payment that
// amortizes principal down to exactly balloon after n payments at
// periodicRate. balloon == 0 reduces to the ordinary fully-amortizing
// formula.
func levelPayment(principal, balloon, periodicRate, n float64) float64 {
	if periodicRate == 0 {
		return (principal - balloon) / n
	}
	discountedBalloon := balloon * math.Pow(1+periodicRate, -n)
	return (principal - discountedBalloon) * periodicRate / (1 - math.Pow(1+periodicRate, -n))
}

// buildPeriods runs the declining-balance simulation period by period
// from origination through termPayments (t.TermYears x paymentsPerYear),
// applying interestOnlyPayment for the first ioPeriods periods (balance
// unchanged) and the level principal-and-interest payment thereafter.
// The final period always fully resolves the tranche, regardless of
// which regime it falls in: it pays off (or pays down to, if a balloon
// was sized into the payment) whatever balance remains, recorded as
// that period's Balloon. This correctly captures three cases as the
// same underlying event — "the tranche comes due with some balance
// still remaining, so that balance is paid as a balloon": an explicit
// BalloonAmount sized into the payment, a TermYears-driven early payoff
// of an otherwise still-amortizing loan, and a tranche whose entire term
// falls inside its own interest-only period (ioPeriods >= termPayments,
// e.g. a 2-year interest-only bridge loan with TermYears == 2), where no
// amortization ever occurs and the full original principal is payable
// at maturity.
func buildPeriods(principal, periodicRate, payment, interestOnlyPayment float64, ioPeriods, termPayments int) []PeriodDebtService {
	if termPayments <= 0 {
		return nil
	}

	periods := make([]PeriodDebtService, 0, termPayments)
	balance := principal

	for p := 1; p <= termPayments; p++ {
		interest := balance * periodicRate
		isFinal := p == termPayments

		if p <= ioPeriods && !isFinal {
			periods = append(periods, PeriodDebtService{
				PeriodNumber:  p,
				Payment:       interestOnlyPayment,
				Interest:      interest,
				EndingBalance: balance,
			})
			continue
		}

		if p <= ioPeriods {
			// The tranche comes due at TermYears while still inside its
			// interest-only period (TermYears <= InterestOnlyYears): no
			// amortization has occurred, so the entire remaining
			// principal is payable as a balloon at maturity, in addition
			// to this period's ordinary interest-only payment — rather
			// than left silently outstanding with no resolution.
			periods = append(periods, PeriodDebtService{
				PeriodNumber:  p,
				Payment:       interestOnlyPayment + balance,
				Interest:      interest,
				Balloon:       balance,
				EndingBalance: 0,
			})
			balance = 0
			continue
		}

		principalPortion := payment - interest

		if isFinal {
			// Pay off whatever balance actually remains (the ordinary
			// scheduled principal portion, plus any residual from a
			// sized-in balloon or a TermYears cutoff before full
			// amortization) as this period's balloon, rather than the
			// schedule's normal principal portion.
			balloon := balance - principalPortion
			if balloon < 0 {
				// Rounding-level overshoot on the final scheduled
				// payment of a non-balloon loan; clamp so the ending
				// balance never goes negative.
				principalPortion = balance
				balloon = 0
			}
			total := principalPortion + interest + balloon
			periods = append(periods, PeriodDebtService{
				PeriodNumber:  p,
				Payment:       total,
				Interest:      interest,
				Principal:     principalPortion,
				Balloon:       balloon,
				EndingBalance: 0,
			})
			balance = 0
			continue
		}

		balance -= principalPortion
		periods = append(periods, PeriodDebtService{
			PeriodNumber:  p,
			Payment:       payment,
			Interest:      interest,
			Principal:     principalPortion,
			EndingBalance: balance,
		})
	}

	return periods
}

// firstYearDebtService sums Payment across periods 1..ppy (or every
// period, if the schedule is shorter than a full year).
func firstYearDebtService(periods []PeriodDebtService, ppy int) float64 {
	var sum float64
	for i, p := range periods {
		if i >= ppy {
			break
		}
		sum += p.Payment
	}
	return sum
}
