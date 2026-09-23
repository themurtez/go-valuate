// This file implements the deterministic scenario-transformation helpers
// the prompt requests: small, composable functions that each return a
// modified copy of an Assumptions value, never mutating the caller's
// original. A caller builds a downside/upside/custom Scenario by starting
// from a base Assumptions and applying one or more of these in sequence —
// e.g. ApplyRevenueShock(base, -0.15, 0, horizon) then
// ApplyOneTimeCostShock(...) — rather than this package guessing what
// "downside" means on its own (see the package doc comment's "does not
// predict assumptions" rule).
package forecast

import "github.com/themurtez/go-valuate/financial"

// cloneAssumptions returns a deep-enough copy of a so every transformation
// helper below can freely modify the result without aliasing slices the
// caller still holds a reference to. Slice-of-struct fields (Revenue, COGS,
// Opex, etc.) are copied element-by-element; CodeOverrides sub-slices are
// copied too, since ApplyRevenueShock/ApplyExpenseShock append to them.
func cloneAssumptions(a Assumptions) Assumptions {
	out := Assumptions{
		Revenue:                  append([]RevenuePeriodAssumption(nil), a.Revenue...),
		COGS:                     append([]COGSPeriodAssumption(nil), a.COGS...),
		Opex:                     append([]OpexPeriodAssumption(nil), a.Opex...),
		DepreciationAmortization: append([]DepreciationAmortizationAssumption(nil), a.DepreciationAmortization...),
		Capex:                    append([]CapexAssumption(nil), a.Capex...),
		WorkingCapital:           append([]WorkingCapitalPeriodAssumption(nil), a.WorkingCapital...),
		Tax:                      append([]TaxPeriodAssumption(nil), a.Tax...),
		DebtService:              append([]DebtServiceAssumption(nil), a.DebtService...),
	}
	for i, r := range out.Revenue {
		out.Revenue[i].CodeOverrides = append([]RevenueCodeAssumption(nil), r.CodeOverrides...)
	}
	for i, o := range out.Opex {
		out.Opex[i].CodeOverrides = append([]OpexCodeAssumption(nil), o.CodeOverrides...)
	}
	return out
}

// extendRevenue/extendCOGS/extendOpex/extendDebtService each extend a slice
// to at least n entries by appending zero-value entries, so a shock
// targeting a period beyond the slice's current length has somewhere to
// write — every Apply* helper below needs this since a caller may shock a
// period range wider than the Assumptions it started from.
func extendRevenue(s []RevenuePeriodAssumption, n int) []RevenuePeriodAssumption {
	for len(s) < n {
		s = append(s, RevenuePeriodAssumption{})
	}
	return s
}

func extendCOGS(s []COGSPeriodAssumption, n int) []COGSPeriodAssumption {
	for len(s) < n {
		s = append(s, COGSPeriodAssumption{})
	}
	return s
}

func extendOpex(s []OpexPeriodAssumption, n int) []OpexPeriodAssumption {
	for len(s) < n {
		s = append(s, OpexPeriodAssumption{})
	}
	return s
}

func extendDebtService(s []DebtServiceAssumption, n int) []DebtServiceAssumption {
	for len(s) < n {
		s = append(s, DebtServiceAssumption{})
	}
	return s
}

// periodRange normalizes a [fromPeriod, horizon) application window (1-based
// forecast period numbers, inclusive of fromPeriod) into the [0, horizon)
// slice-index range every Apply* helper loops over. fromPeriod <= 1 shocks
// every period from the first forward; fromPeriod > horizon shocks nothing
// (an empty range), which every helper here treats as a valid no-op rather
// than an error, since a caller composing several shocks across different
// windows may legitimately pass a window that ends up empty for a given
// horizon.
func periodRange(fromPeriod, horizon int) (start, end int) {
	start = fromPeriod - 1
	if start < 0 {
		start = 0
	}
	end = horizon
	if start > end {
		start = end
	}
	return start, end
}

// ApplyRevenueShock returns a copy of a with every period's aggregate
// revenue growth rate in [fromPeriod, horizon] shifted by deltaGrowthRate
// (a decimal, e.g. -0.15 for a 15-point cut). Only applies to periods using
// RevenueMethodGrowthRate (the zero-value default); a period already using
// RevenueMethodFixedAmount is left untouched, since shifting a "growth
// rate" has no meaning against a fixed-dollar assumption — the caller
// should shock the fixed amount directly on the returned Assumptions
// instead for that case. horizon is the number of forecast periods a's
// slices are meant to cover; the returned Assumptions.Revenue is extended
// (with zero-value entries) to at least that length if it was shorter, so
// the shock has somewhere to apply within the full window even if the
// caller's base Assumptions left later periods unset.
func ApplyRevenueShock(a Assumptions, deltaGrowthRate float64, fromPeriod, horizon int) Assumptions {
	out := cloneAssumptions(a)
	out.Revenue = extendRevenue(out.Revenue, horizon)
	start, end := periodRange(fromPeriod, horizon)
	for i := start; i < end; i++ {
		if out.Revenue[i].Method == RevenueMethodFixedAmount {
			continue
		}
		out.Revenue[i].GrowthRate += deltaGrowthRate
	}
	return out
}

// ApplyMarginShock returns a copy of a with every period's gross-margin
// target in [fromPeriod, horizon] shifted by deltaMarginPoints (a decimal,
// e.g. -0.05 for a 5-point compression). Only applies to periods using
// COGSMethodGrossMarginPercent (the zero-value default); a period using
// COGSMethodGrowthRate or COGSMethodFixedAmount is left untouched, mirroring
// ApplyRevenueShock's identical "shock the field that's actually active"
// rule. The result is not clamped to [0, 1] — a caller stacking multiple
// margin shocks is responsible for checking the resulting
// GrossMarginPercent stays economically sensible; Calculate itself only
// warns (IssueImpliedZeroGrossMargin) on an exact 0, never on a negative
// margin target, per this package's "apply the assumption, don't
// second-guess it" design (see IssueNegativeProjectedValue's doc comment
// for the parallel rule on projected output).
func ApplyMarginShock(a Assumptions, deltaMarginPoints float64, fromPeriod, horizon int) Assumptions {
	out := cloneAssumptions(a)
	out.COGS = extendCOGS(out.COGS, horizon)
	start, end := periodRange(fromPeriod, horizon)
	for i := start; i < end; i++ {
		if out.COGS[i].Method == COGSMethodGrowthRate || out.COGS[i].Method == COGSMethodFixedAmount {
			continue
		}
		out.COGS[i].GrossMarginPercent += deltaMarginPoints
	}
	return out
}

// ApplyExpenseShock returns a copy of a with every period's aggregate opex
// growth rate in [fromPeriod, horizon] shifted by deltaGrowthRate. When
// codes is non-empty, the shock instead targets only those specific
// financial.Code values: for each, an existing OpexCodeAssumption override
// in that period has its GrowthRate shifted (if it already uses
// OpexMethodGrowthRate; left untouched otherwise, mirroring
// ApplyRevenueShock's fixed-amount exclusion), or a new
// OpexCodeAssumption{Code: c, GrowthRate: deltaGrowthRate} override is
// appended if none existed for that code in that period yet (since there is
// no existing per-code rate to shift from — the shock itself becomes the
// code's entire growth-rate assumption for that period, on top of whatever
// the aggregate rate would otherwise have applied). When codes is empty,
// the aggregate OpexPeriodAssumption.GrowthRate is shifted instead
// (identical structure to ApplyRevenueShock).
func ApplyExpenseShock(a Assumptions, deltaGrowthRate float64, codes []financial.Code, fromPeriod, horizon int) Assumptions {
	out := cloneAssumptions(a)
	out.Opex = extendOpex(out.Opex, horizon)
	start, end := periodRange(fromPeriod, horizon)

	if len(codes) == 0 {
		for i := start; i < end; i++ {
			if out.Opex[i].Method == OpexMethodFixedAmount {
				continue
			}
			out.Opex[i].GrowthRate += deltaGrowthRate
		}
		return out
	}

	for i := start; i < end; i++ {
		for _, code := range codes {
			out.Opex[i].CodeOverrides = shiftOrAddOpexOverride(out.Opex[i].CodeOverrides, code, deltaGrowthRate)
		}
	}
	return out
}

// shiftOrAddOpexOverride mirrors ApplyExpenseShock's per-code doc comment:
// the first matching CodeOverrides entry (first-entry-wins precedence,
// mirroring opexOverrideIndex) has its GrowthRate shifted if it's a
// growth-rate override, or a new override is appended if none exists yet.
func shiftOrAddOpexOverride(overrides []OpexCodeAssumption, code financial.Code, deltaGrowthRate float64) []OpexCodeAssumption {
	for i, ov := range overrides {
		if ov.Code != code {
			continue
		}
		if ov.Method == OpexMethodFixedAmount {
			return overrides
		}
		overrides[i].GrowthRate += deltaGrowthRate
		return overrides
	}
	return append(overrides, OpexCodeAssumption{Code: code, GrowthRate: deltaGrowthRate})
}

// ApplyCustomerLossShock returns a copy of a modeling the sudden loss of a
// fraction of revenue (lossFraction, a decimal, e.g. 0.2 for losing 20% of
// revenue) starting at fromPeriod and persisting through horizon — modeled
// as a permanent step-down in the affected periods' revenue base via
// RevenueMethodFixedAmount at (1 - lossFraction) x what the period would
// otherwise have projected to, computed by first running the unshocked a
// through projectRevenue period-by-period so the fixed amount reflects
// whatever compounding/mix the caller's own assumptions already imply,
// not merely lossFraction applied once to a static base. baseRevenue is the
// historical base period's PeriodPL (Result.Base.PL from a prior Calculate
// call, or any PeriodPL with RevenueLines/TotalRevenue populated) the shock
// compounds forward from — the same role Input.Dataset's base period plays
// inside Calculate itself, supplied explicitly here since this is a
// standalone helper with no access to a Calculate call's internal state.
//
// After fromPeriod, every subsequent period's RevenuePeriodAssumption.
// Method is left as originally supplied — the loss is a one-time step down
// in the level of revenue, not a change to the ongoing growth rate, so
// periods after fromPeriod continue compounding (at their own
// already-specified growth rates) from the newly-lowered base.
func ApplyCustomerLossShock(a Assumptions, lossFraction float64, baseRevenue PeriodPL, fromPeriod, horizon int) Assumptions {
	out := cloneAssumptions(a)
	out.Revenue = extendRevenue(out.Revenue, horizon)
	start, _ := periodRange(fromPeriod, horizon)
	if start >= horizon {
		return out
	}

	prev := baseRevenue
	for i := 0; i < horizon; i++ {
		if i < start {
			lines, total, _ := projectRevenue(out.Revenue[i], prev, "", "")
			prev = PeriodPL{RevenueLines: lines, TotalRevenue: total}
			continue
		}
		if i == start {
			lines, total, _ := projectRevenue(out.Revenue[i], prev, "", "")
			if total.Available {
				out.Revenue[i] = RevenuePeriodAssumption{
					Method:      RevenueMethodFixedAmount,
					FixedAmount: total.Value * (1 - lossFraction),
				}
			}
			prev = PeriodPL{RevenueLines: lines, TotalRevenue: AvailableValue(total.Value * (1 - lossFraction))}
			continue
		}
		lines, total, _ := projectRevenue(out.Revenue[i], prev, "", "")
		prev = PeriodPL{RevenueLines: lines, TotalRevenue: total}
	}
	return out
}

// ApplyOneTimeCostShock returns a copy of a with a single additional
// one-time expense of amount added to exactly one period (period, a
// 1-based forecast period number) via an OpexCodeAssumption override on
// code, using OpexMethodFixedAmount so the shock itself lands exactly once.
// If an override for code already exists in that period, its amount is
// increased by amount rather than replaced, so stacking two one-time-cost
// shocks on the same code and period accumulates rather than clobbers.
//
// Without further correction, this one-time spike would still become part
// of period+1's "prior period opex" to grow from (a growth-rate or
// fixed-amount override in one period always feeds the next period's
// compounding base — see projectOpex), permanently inflating every
// subsequent period's trajectory by a cost that was only ever meant to hit
// once. So when period+1 is within horizon and has no CodeOverrides entry
// of its own for code yet, this function also inserts one there, using
// OpexMethodExcludeAmount with ExcludeFromBase equal to the total one-time
// amount shocked into period so far (accumulated across repeated calls,
// exactly like the FixedAmount override in period itself) — see that
// method's doc comment for the exact arithmetic this produces. A caller
// who has already supplied their own override for code in period+1 has
// that respected as-is: this function only fills a gap, never overwrites
// an explicit caller choice.
func ApplyOneTimeCostShock(a Assumptions, amount float64, code financial.Code, period, horizon int) Assumptions {
	out := cloneAssumptions(a)
	out.Opex = extendOpex(out.Opex, horizon)
	i := period - 1
	if i < 0 || i >= horizon {
		return out
	}

	total := amount
	if idx, ok := findOpexCodeOverride(out.Opex[i].CodeOverrides, code); ok && out.Opex[i].CodeOverrides[idx].Method == OpexMethodFixedAmount {
		out.Opex[i].CodeOverrides[idx].FixedAmount += amount
		total = out.Opex[i].CodeOverrides[idx].FixedAmount
	} else {
		out.Opex[i].CodeOverrides = append(out.Opex[i].CodeOverrides, OpexCodeAssumption{
			Code: code, Method: OpexMethodFixedAmount, FixedAmount: amount,
		})
	}

	next := i + 1
	if next < horizon {
		if _, exists := findOpexCodeOverride(out.Opex[next].CodeOverrides, code); !exists {
			out.Opex[next].CodeOverrides = append(out.Opex[next].CodeOverrides, OpexCodeAssumption{
				Code:            code,
				Method:          OpexMethodExcludeAmount,
				GrowthRate:      out.Opex[next].GrowthRate,
				ExcludeFromBase: total,
			})
		} else if idx, _ := findOpexCodeOverride(out.Opex[next].CodeOverrides, code); out.Opex[next].CodeOverrides[idx].Method == OpexMethodExcludeAmount {
			// A prior call already inserted the auto-pin for this exact
			// code/period pair (repeated ApplyOneTimeCostShock calls
			// targeting the same period) — keep it in sync with the
			// accumulated total rather than leaving it stale.
			out.Opex[next].CodeOverrides[idx].ExcludeFromBase = total
		}
	}

	return out
}

// findOpexCodeOverride returns the index of the first CodeOverrides entry
// for code, and whether one exists — mirroring opexOverrideIndex's
// first-entry-wins precedence without building a full index for a single
// lookup.
func findOpexCodeOverride(overrides []OpexCodeAssumption, code financial.Code) (int, bool) {
	for i, ov := range overrides {
		if ov.Code == code {
			return i, true
		}
	}
	return 0, false
}

// ApplyDebtRateShock returns a copy of a with every period's debt-service
// InterestRate in [fromPeriod, horizon] shifted by deltaRate (a decimal,
// e.g. 0.02 for a 200bps increase). Only applies to periods where
// InterestRate and BeginningBalance are both already available (see
// DebtServiceAssumption's doc comment: Interest is recomputed from those
// two only when both are present) — a period using a flat caller-supplied
// Interest figure with no rate/balance is left untouched, since there is no
// rate for this helper to shift.
func ApplyDebtRateShock(a Assumptions, deltaRate float64, fromPeriod, horizon int) Assumptions {
	out := cloneAssumptions(a)
	out.DebtService = extendDebtService(out.DebtService, horizon)
	start, end := periodRange(fromPeriod, horizon)
	for i := start; i < end; i++ {
		if !out.DebtService[i].InterestRate.Available || !out.DebtService[i].BeginningBalance.Available {
			continue
		}
		out.DebtService[i].InterestRate = AvailableValue(out.DebtService[i].InterestRate.Value + deltaRate)
	}
	return out
}
