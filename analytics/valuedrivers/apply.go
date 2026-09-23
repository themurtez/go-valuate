package valuedrivers

import (
	"fmt"
	"strings"

	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/applicability"
	"github.com/themurtez/go-valuate/valuation/dcf"
	"github.com/themurtez/go-valuate/valuation/netassets"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
)

// cloneRequest deep-copies req so a Driver's mutation never reaches the
// caller's own Input.BaselineRequest or any value reachable from it — see
// the package doc comment on no mutation of caller input. Every pointer
// field req.SDE/EBITDA/Capitalization/DCF/NetAssets/Applicability is
// copied to a new backing value (not just the pointer), and every slice
// within those (DCF.ForecastPeriods, NetAssets.Assets/Liabilities,
// Applicability.Methods) is independently copied so appending or
// mutating an element never aliases the original. No Driver in this
// package's DriverType set mutates Applicability today (it is only ever
// read by orchestrator.Execute's own filtering) — it is still cloned so
// this guarantee holds unconditionally rather than only for the fields a
// Driver happens to touch as of this writing.
func cloneRequest(req orchestrator.Request) orchestrator.Request {
	out := req
	out.SelectedMethods = append([]valuation.Code(nil), req.SelectedMethods...)

	if req.Applicability != nil {
		v := *req.Applicability
		v.Methods = append([]applicability.Result(nil), req.Applicability.Methods...)
		out.Applicability = &v
	}

	if req.SDE != nil {
		v := *req.SDE
		out.SDE = &v
	}
	if req.EBITDA != nil {
		v := *req.EBITDA
		out.EBITDA = &v
	}
	if req.Capitalization != nil {
		v := *req.Capitalization
		out.Capitalization = &v
	}
	if req.DCF != nil {
		v := *req.DCF
		v.ForecastPeriods = append([]dcf.ForecastPeriod(nil), req.DCF.ForecastPeriods...)
		out.DCF = &v
	}
	if req.NetAssets != nil {
		v := *req.NetAssets
		v.Assets = append([]netassets.AssetItem(nil), req.NetAssets.Assets...)
		v.Liabilities = append([]netassets.LiabilityItem(nil), req.NetAssets.Liabilities...)
		out.NetAssets = &v
	}
	return out
}

// applyOutcome is what applyDriver returns for a single (driver, method)
// pair: the Linkage to record, plus, when Status is LinkageApplied, the
// ChangedInput entries describing exactly what field(s) moved.
type applyOutcome struct {
	linkage Linkage
	changed []ChangedInput
}

// notApplicable builds a LinkageNotApplicable applyOutcome with detail.
func notApplicable(method valuation.Code, detail string) applyOutcome {
	return applyOutcome{linkage: Linkage{Method: method, Status: LinkageNotApplicable, Detail: detail}}
}

// excluded builds a LinkageMethodExcluded applyOutcome.
func excluded(method valuation.Code) applyOutcome {
	return applyOutcome{linkage: Linkage{
		Method: method, Status: LinkageMethodExcluded,
		Detail: "no Input was supplied for this method in the baseline Request",
	}}
}

// applied builds a LinkageApplied applyOutcome for a single changed field.
func applied(method valuation.Code, driverID, field string, before, after float64, detail string) applyOutcome {
	return applyOutcome{
		linkage: Linkage{Method: method, Status: LinkageApplied, Detail: detail},
		changed: []ChangedInput{{Method: method, Field: field, Before: before, After: after, DriverID: driverID}},
	}
}

// applyDriver mutates req in place (req must already be a clone — see
// cloneRequest) according to driver, and returns one applyOutcome per
// method orchestrator.Request can name (SDE, EBITDA, Capitalization, DCF,
// NetAssets, in that fixed order — matching orchestrator.Execute's own
// order), plus any DriverIssues raised validating driver itself.
//
// Every DriverType branch below states, in its own case, exactly which
// method(s) it can ever touch — a method not mentioned in a given
// driver.Type's case always resolves to LinkageNotApplicable for that
// type, never silently skipped without a recorded Linkage (see the
// package doc comment).
func applyDriver(req *orchestrator.Request, driver Driver) (outcomes []applyOutcome, issues []DriverIssue) {
	if driver.ID == "" {
		return nil, []DriverIssue{{
			Code: IssueMissingDriverID, Severity: DriverSeverityError,
			Message: "driver has no ID; it was skipped and contributed no mutation",
		}}
	}

	switch driver.Type {
	case DriverRevenueGrowth:
		return applyRevenueGrowth(req, driver)
	case DriverMarginChange:
		return applyMarginChange(req, driver)
	case DriverSDEChange:
		return applySDEChange(req, driver)
	case DriverMultipleChange:
		return applyMultipleChange(req, driver)
	case DriverCapRateChange:
		return applyCapRateChange(req, driver)
	case DriverDiscountRateChange:
		return applyDiscountRateChange(req, driver)
	case DriverDebtChange:
		return applyDebtChange(req, driver)
	case DriverWorkingCapitalChange:
		return applyWorkingCapitalChange(req, driver)
	case DriverOwnerCompensationAdjustment:
		return applyOwnerCompensationAdjustment(req, driver)
	case DriverCustomerLossImpact:
		return applyCustomerLossImpact(req, driver)
	case DriverMethodMultipleRule:
		return applyMethodMultipleRule(req, driver)
	default:
		return nil, []DriverIssue{{
			Code: IssueUnrecognizedDriverType, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q has an empty or unrecognized type %q; it was skipped and contributed no mutation", driver.ID, driver.Type),
		}}
	}
}

// allMethodsNotApplicable builds one LinkageNotApplicable applyOutcome per
// method this DriverType has no documented linkage to at all — used by
// every branch below to fill in the methods its own switch does not
// otherwise report on, so applyDriver always returns exactly one
// applyOutcome per orchestrator method regardless of DriverType.
func allMethodsNotApplicable(except map[valuation.Code]bool, detail string) []applyOutcome {
	var out []applyOutcome
	for _, m := range methodOrder {
		if except[m] {
			continue
		}
		out = append(out, notApplicable(m, detail))
	}
	return out
}

// methodOrder mirrors valuation/report's identical fixed display order,
// which itself mirrors orchestrator.Execute's fixed evaluation order.
var methodOrder = []valuation.Code{
	valuation.CodeSDEMultiple,
	valuation.CodeEBITDAMultiple,
	valuation.CodeCapitalizationOfEarnings,
	valuation.CodeDCF,
	valuation.CodeAdjustedNetAssetValue,
}

func applyRevenueGrowth(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.RevenueGrowth == nil {
		return nil, missingParams(driver, "revenue_growth")
	}
	factor := 1 + driver.RevenueGrowth.GrowthPercent
	detail := fmt.Sprintf("multiplied by (1 + %.4f) = %.4fx for %.2f%% revenue growth", driver.RevenueGrowth.GrowthPercent, factor, driver.RevenueGrowth.GrowthPercent*100)

	var out []applyOutcome

	if req.SDE != nil {
		before := req.SDE.MaintainableSDE
		req.SDE.MaintainableSDE = before * factor
		out = append(out, applied(valuation.CodeSDEMultiple, driver.ID, "maintainable_sde", before, req.SDE.MaintainableSDE, detail))
	} else {
		out = append(out, excluded(valuation.CodeSDEMultiple))
	}

	if req.EBITDA != nil {
		before := req.EBITDA.MaintainableEBITDA
		req.EBITDA.MaintainableEBITDA = before * factor
		out = append(out, applied(valuation.CodeEBITDAMultiple, driver.ID, "maintainable_ebitda", before, req.EBITDA.MaintainableEBITDA, detail))
	} else {
		out = append(out, excluded(valuation.CodeEBITDAMultiple))
	}

	if req.Capitalization != nil {
		before := req.Capitalization.MaintainableEarnings
		req.Capitalization.MaintainableEarnings = before * factor
		out = append(out, applied(valuation.CodeCapitalizationOfEarnings, driver.ID, "maintainable_earnings", before, req.Capitalization.MaintainableEarnings, detail))
	} else {
		out = append(out, excluded(valuation.CodeCapitalizationOfEarnings))
	}

	if req.DCF != nil && len(req.DCF.ForecastPeriods) > 0 {
		var changed []ChangedInput
		for i := range req.DCF.ForecastPeriods {
			before := req.DCF.ForecastPeriods[i].FreeCashFlow
			req.DCF.ForecastPeriods[i].FreeCashFlow = before * factor
			changed = append(changed, ChangedInput{
				Method: valuation.CodeDCF, Field: fmt.Sprintf("forecast_periods[%d].free_cash_flow", i),
				Before: before, After: req.DCF.ForecastPeriods[i].FreeCashFlow, DriverID: driver.ID,
			})
		}
		out = append(out, applyOutcome{
			linkage: Linkage{Method: valuation.CodeDCF, Status: LinkageApplied, Detail: detail + " to every forecast period's free cash flow"},
			changed: changed,
		})
	} else if req.DCF != nil {
		out = append(out, notApplicable(valuation.CodeDCF, "DCF input has no forecast periods to scale"))
	} else {
		out = append(out, excluded(valuation.CodeDCF))
	}

	out = append(out, notApplicable(valuation.CodeAdjustedNetAssetValue, "revenue growth has no documented linkage to the adjusted net asset value method"))

	return out, nil
}

func applyMarginChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.MarginChange == nil {
		return nil, missingParams(driver, "margin_change")
	}
	if driver.MarginChange.RevenueBase <= 0 {
		return nil, []DriverIssue{{
			Code: IssueMissingRevenueBase, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q is a margin change but RevenueBase was <= 0; it was skipped and contributed no mutation", driver.ID),
		}}
	}
	dollarDelta := driver.MarginChange.MarginPointsDelta * driver.MarginChange.RevenueBase
	detail := fmt.Sprintf("added %.2f margin points x %.2f revenue base = %.2f", driver.MarginChange.MarginPointsDelta, driver.MarginChange.RevenueBase, dollarDelta)

	out := []applyOutcome{
		addToEarningsField(req.SDE != nil, valuation.CodeSDEMultiple, driver.ID, "maintainable_sde", dollarDelta, detail, func() *float64 {
			if req.SDE == nil {
				return nil
			}
			return &req.SDE.MaintainableSDE
		}),
		addToEarningsField(req.EBITDA != nil, valuation.CodeEBITDAMultiple, driver.ID, "maintainable_ebitda", dollarDelta, detail, func() *float64 {
			if req.EBITDA == nil {
				return nil
			}
			return &req.EBITDA.MaintainableEBITDA
		}),
		addToEarningsField(req.Capitalization != nil, valuation.CodeCapitalizationOfEarnings, driver.ID, "maintainable_earnings", dollarDelta, detail, func() *float64 {
			if req.Capitalization == nil {
				return nil
			}
			return &req.Capitalization.MaintainableEarnings
		}),
		notApplicable(valuation.CodeDCF, "margin change has no documented linkage to DCF (no revenue base to scale forecast cash flow against); use DriverRevenueGrowth or DriverCustomerLossImpact for DCF effects"),
		notApplicable(valuation.CodeAdjustedNetAssetValue, "margin change has no documented linkage to the adjusted net asset value method"),
	}
	return out, nil
}

// addToEarningsField centralizes the "add a dollar delta to one method's
// maintainable-earnings-shaped field, or report exclusion/not-applicable"
// pattern shared by applyMarginChange, applyOwnerCompensationAdjustment,
// and applyCustomerLossImpact.
func addToEarningsField(hasInput bool, method valuation.Code, driverID, field string, delta float64, detail string, fieldPtr func() *float64) applyOutcome {
	if !hasInput {
		return excluded(method)
	}
	p := fieldPtr()
	before := *p
	*p = before + delta
	return applied(method, driverID, field, before, *p, detail)
}

func applySDEChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.SDEChange == nil {
		return nil, missingParams(driver, "sde_change")
	}
	var issues []DriverIssue
	if driver.SDEChange.AmountDelta != 0 && driver.SDEChange.PercentDelta != 0 {
		issues = append(issues, DriverIssue{
			Code: IssueBothDeltaKindsSet, Severity: DriverSeverityWarning, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q set both AmountDelta and PercentDelta; both were applied (amount first, then percent), which is unusual — verify this was intended", driver.ID),
		})
	}

	out := allMethodsNotApplicable(map[valuation.Code]bool{valuation.CodeSDEMultiple: true},
		"SDE change has no documented linkage to this method; it targets Maintainable SDE only")

	if req.SDE == nil {
		return append([]applyOutcome{excluded(valuation.CodeSDEMultiple)}, out...), issues
	}

	before := req.SDE.MaintainableSDE
	after := before + driver.SDEChange.AmountDelta
	after *= 1 + driver.SDEChange.PercentDelta
	req.SDE.MaintainableSDE = after
	detail := fmt.Sprintf("applied amount delta %.2f then percent delta %.4f to maintainable SDE", driver.SDEChange.AmountDelta, driver.SDEChange.PercentDelta)

	return append([]applyOutcome{applied(valuation.CodeSDEMultiple, driver.ID, "maintainable_sde", before, after, detail)}, out...), issues
}

func applyMultipleChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.MultipleChange == nil {
		return nil, missingParams(driver, "multiple_change")
	}
	if len(driver.MultipleChange.Methods) == 0 {
		return nil, []DriverIssue{{
			Code: IssueNoTargetMethod, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q is a multiple change but named no target Methods; it was skipped and contributed no mutation", driver.ID),
		}}
	}

	wantsSDE, wantsEBITDA := false, false
	for _, m := range driver.MultipleChange.Methods {
		switch m {
		case MultipleChangeSDE:
			wantsSDE = true
		case MultipleChangeEBITDA:
			wantsEBITDA = true
		}
	}

	sdeOutcome := multipleOutcome(req.SDE != nil, valuation.CodeSDEMultiple, driver.ID, wantsSDE, driver.MultipleChange.ChangeDelta, driver.MultipleChange.NewValue, func() *float64 {
		if req.SDE == nil {
			return nil
		}
		return &req.SDE.Multiple
	})
	ebitdaOutcome := multipleOutcome(req.EBITDA != nil, valuation.CodeEBITDAMultiple, driver.ID, wantsEBITDA, driver.MultipleChange.ChangeDelta, driver.MultipleChange.NewValue, func() *float64 {
		if req.EBITDA == nil {
			return nil
		}
		return &req.EBITDA.Multiple
	})

	out := []applyOutcome{
		sdeOutcome, ebitdaOutcome,
		notApplicable(valuation.CodeCapitalizationOfEarnings, "multiple change targets Input.Multiple, which this method has no equivalent field for (see DriverCapRateChange)"),
		notApplicable(valuation.CodeDCF, "multiple change has no documented linkage to DCF (see DriverDiscountRateChange)"),
		notApplicable(valuation.CodeAdjustedNetAssetValue, "multiple change has no documented linkage to the adjusted net asset value method"),
	}
	return out, nil
}

// multipleOutcome applies a delta-or-replacement change to a single
// method's Multiple field, or reports why it did not.
func multipleOutcome(hasInput bool, method valuation.Code, driverID string, wanted bool, delta, newValue float64, fieldPtr func() *float64) applyOutcome {
	if !wanted {
		return notApplicable(method, "this driver's Methods list did not name this method")
	}
	if !hasInput {
		return excluded(method)
	}
	p := fieldPtr()
	before := *p
	after := before + delta
	detail := fmt.Sprintf("multiple adjusted by delta %.4f", delta)
	if newValue != 0 {
		after = newValue
		detail = fmt.Sprintf("multiple replaced with %.4f", newValue)
	}
	*p = after
	return applied(method, driverID, "multiple", before, after, detail)
}

func applyCapRateChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.CapRateChange == nil {
		return nil, missingParams(driver, "cap_rate_change")
	}
	out := allMethodsNotApplicable(map[valuation.Code]bool{valuation.CodeCapitalizationOfEarnings: true},
		"cap rate change has no documented linkage to this method; it targets the capitalization rate only")

	if req.Capitalization == nil {
		return append([]applyOutcome{excluded(valuation.CodeCapitalizationOfEarnings)}, out...), nil
	}
	before := req.Capitalization.CapitalizationRate
	after := before + driver.CapRateChange.ChangeDelta
	detail := fmt.Sprintf("capitalization rate adjusted by delta %.4f", driver.CapRateChange.ChangeDelta)
	if driver.CapRateChange.NewValue != 0 {
		after = driver.CapRateChange.NewValue
		detail = fmt.Sprintf("capitalization rate replaced with %.4f", driver.CapRateChange.NewValue)
	}
	req.Capitalization.CapitalizationRate = after

	return append([]applyOutcome{applied(valuation.CodeCapitalizationOfEarnings, driver.ID, "capitalization_rate", before, after, detail)}, out...), nil
}

func applyDiscountRateChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.DiscountRateChange == nil {
		return nil, missingParams(driver, "discount_rate_change")
	}
	p := driver.DiscountRateChange
	if p.DiscountRateDelta == 0 && p.DiscountRateNewValue == 0 && p.TerminalGrowthRateDelta == 0 && p.TerminalGrowthRateNewValue == 0 {
		return nil, []DriverIssue{{
			Code: IssueNoRateChange, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q is a discount rate change but every field was zero; it was skipped and contributed no mutation", driver.ID),
		}}
	}

	out := allMethodsNotApplicable(map[valuation.Code]bool{valuation.CodeDCF: true},
		"discount rate change has no documented linkage to this method; it targets DCF's discount rate and terminal growth rate only")

	if req.DCF == nil {
		return append([]applyOutcome{excluded(valuation.CodeDCF)}, out...), nil
	}

	var changed []ChangedInput
	var details []string

	if p.DiscountRateDelta != 0 || p.DiscountRateNewValue != 0 {
		before := req.DCF.DiscountRate
		after := before + p.DiscountRateDelta
		d := fmt.Sprintf("discount rate adjusted by delta %.4f", p.DiscountRateDelta)
		if p.DiscountRateNewValue != 0 {
			after = p.DiscountRateNewValue
			d = fmt.Sprintf("discount rate replaced with %.4f", p.DiscountRateNewValue)
		}
		req.DCF.DiscountRate = after
		changed = append(changed, ChangedInput{Method: valuation.CodeDCF, Field: "discount_rate", Before: before, After: after, DriverID: driver.ID})
		details = append(details, d)
	}
	if p.TerminalGrowthRateDelta != 0 || p.TerminalGrowthRateNewValue != 0 {
		before := req.DCF.TerminalGrowthRate
		after := before + p.TerminalGrowthRateDelta
		d := fmt.Sprintf("terminal growth rate adjusted by delta %.4f", p.TerminalGrowthRateDelta)
		if p.TerminalGrowthRateNewValue != 0 {
			after = p.TerminalGrowthRateNewValue
			d = fmt.Sprintf("terminal growth rate replaced with %.4f", p.TerminalGrowthRateNewValue)
		}
		req.DCF.TerminalGrowthRate = after
		changed = append(changed, ChangedInput{Method: valuation.CodeDCF, Field: "terminal_growth_rate", Before: before, After: after, DriverID: driver.ID})
		details = append(details, d)
	}

	return append([]applyOutcome{{
		linkage: Linkage{Method: valuation.CodeDCF, Status: LinkageApplied, Detail: joinDetails(details)},
		changed: changed,
	}}, out...), nil
}

func applyDebtChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.DebtChange == nil {
		return nil, missingParams(driver, "debt_change")
	}
	return applyDebtDelta(req, driver.ID, driver.DebtChange), nil
}

// applyDebtDelta is the shared engine behind both applyDebtChange and
// applyWorkingCapitalChange (which translates its own BridgeField/Amount
// into an equivalent DebtChangeParams — see bridgeFieldToDebtParams — and
// calls straight through to this function, rather than building a
// synthetic Driver to re-enter applyDebtChange). Parameterized on only
// the data it actually needs, so neither caller has to fabricate
// Driver fields (Label, Type, other Params) this function never reads.
func applyDebtDelta(req *orchestrator.Request, driverID string, p *DebtChangeParams) []applyOutcome {
	return []applyOutcome{
		bridgeOutcome(req.SDE != nil, req.SDE != nil && req.SDE.EquityBridge.Requested, valuation.CodeSDEMultiple, driverID, p, sdeBridgeAccessors(req)),
		bridgeOutcome(req.EBITDA != nil, req.EBITDA != nil && req.EBITDA.EquityBridge.Requested, valuation.CodeEBITDAMultiple, driverID, p, ebitdaBridgeAccessors(req)),
		notApplicable(valuation.CodeCapitalizationOfEarnings, "debt change has no documented linkage to this method (capitalization of earnings has no equity bridge field)"),
		bridgeOutcome(req.DCF != nil, req.DCF != nil && req.DCF.EquityBridge.Requested, valuation.CodeDCF, driverID, p, dcfBridgeAccessors(req)),
		notApplicable(valuation.CodeAdjustedNetAssetValue, "debt change has no documented linkage to the adjusted net asset value method"),
	}
}

// bridgeAccessors returns pointers to a single method's four
// EquityBridgeInput-shaped fields, letting bridgeOutcome/
// applyWorkingCapitalChange share one mutation path across sde/ebitda/dcf
// despite each having its own concrete EquityBridgeInput type.
type bridgeAccessors struct {
	excessCash, shortTermDebt, longTermDebt, otherDebt *float64
}

func sdeBridgeAccessors(req *orchestrator.Request) bridgeAccessors {
	if req.SDE == nil {
		return bridgeAccessors{}
	}
	return bridgeAccessors{
		excessCash: &req.SDE.EquityBridge.ExcessCash, shortTermDebt: &req.SDE.EquityBridge.ShortTermDebt,
		longTermDebt: &req.SDE.EquityBridge.LongTermDebt, otherDebt: &req.SDE.EquityBridge.OtherDebt,
	}
}

func ebitdaBridgeAccessors(req *orchestrator.Request) bridgeAccessors {
	if req.EBITDA == nil {
		return bridgeAccessors{}
	}
	return bridgeAccessors{
		excessCash: &req.EBITDA.EquityBridge.ExcessCash, shortTermDebt: &req.EBITDA.EquityBridge.ShortTermDebt,
		longTermDebt: &req.EBITDA.EquityBridge.LongTermDebt, otherDebt: &req.EBITDA.EquityBridge.OtherDebt,
	}
}

func dcfBridgeAccessors(req *orchestrator.Request) bridgeAccessors {
	if req.DCF == nil {
		return bridgeAccessors{}
	}
	return bridgeAccessors{
		excessCash: &req.DCF.EquityBridge.ExcessCash, shortTermDebt: &req.DCF.EquityBridge.ShortTermDebt,
		longTermDebt: &req.DCF.EquityBridge.LongTermDebt, otherDebt: &req.DCF.EquityBridge.OtherDebt,
	}
}

func bridgeOutcome(hasInput, bridgeRequested bool, method valuation.Code, driverID string, p *DebtChangeParams, acc bridgeAccessors) applyOutcome {
	if !hasInput {
		return excluded(method)
	}
	if !bridgeRequested {
		return notApplicable(method, "no EquityBridge was requested for this method in the baseline Request; debt driver has nothing to adjust")
	}

	var changed []ChangedInput
	var details []string
	move := func(field string, delta float64, target *float64) {
		if delta == 0 {
			return
		}
		before := *target
		*target += delta
		changed = append(changed, ChangedInput{Method: method, Field: field, Before: before, After: *target, DriverID: driverID})
		details = append(details, fmt.Sprintf("%s %+.2f", field, delta))
	}
	move("equity_bridge.excess_cash", p.ExcessCashDelta, acc.excessCash)
	move("equity_bridge.short_term_debt", p.ShortTermDebtDelta, acc.shortTermDebt)
	move("equity_bridge.long_term_debt", p.LongTermDebtDelta, acc.longTermDebt)
	move("equity_bridge.other_debt", p.OtherDebtDelta, acc.otherDebt)

	if len(changed) == 0 {
		// Every LinkageApplied outcome elsewhere in this file carries at
		// least one ChangedInput; a bridge that was requested and
		// reachable but received an all-zero delta set has nothing to
		// point to, so it is reported as not-applicable rather than a
		// LinkageApplied with no backing evidence of what changed.
		return notApplicable(method, "every debt-change delta was zero; no bridge field moved")
	}
	return applyOutcome{linkage: Linkage{Method: method, Status: LinkageApplied, Detail: joinDetails(details)}, changed: changed}
}

func applyWorkingCapitalChange(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.WorkingCapitalChange == nil {
		return nil, missingParams(driver, "working_capital_change")
	}
	p := driver.WorkingCapitalChange
	debtParams, ok := bridgeFieldToDebtParams(p.Field, p.Amount)
	if !ok {
		return nil, []DriverIssue{{
			Code: IssueInvalidBridgeField, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q named an empty or unrecognized BridgeField %q; it was skipped and contributed no mutation", driver.ID, p.Field),
		}}
	}
	return applyDebtDelta(req, driver.ID, &debtParams), nil
}

func bridgeFieldToDebtParams(field BridgeField, amount float64) (DebtChangeParams, bool) {
	switch field {
	case BridgeFieldExcessCash:
		return DebtChangeParams{ExcessCashDelta: amount}, true
	case BridgeFieldShortTermDebt:
		return DebtChangeParams{ShortTermDebtDelta: amount}, true
	case BridgeFieldLongTermDebt:
		return DebtChangeParams{LongTermDebtDelta: amount}, true
	case BridgeFieldOtherDebt:
		return DebtChangeParams{OtherDebtDelta: amount}, true
	default:
		return DebtChangeParams{}, false
	}
}

func applyOwnerCompensationAdjustment(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.OwnerCompensationAdjustment == nil {
		return nil, missingParams(driver, "owner_compensation_adjustment")
	}
	amount := driver.OwnerCompensationAdjustment.Amount
	detail := fmt.Sprintf("owner compensation addback of %.2f applied", amount)

	sdeOutcome := addToEarningsField(req.SDE != nil, valuation.CodeSDEMultiple, driver.ID, "maintainable_sde", amount, detail, func() *float64 {
		if req.SDE == nil {
			return nil
		}
		return &req.SDE.MaintainableSDE
	})

	if !driver.OwnerCompensationAdjustment.AppliesBeyondSDE {
		return append([]applyOutcome{sdeOutcome},
			notApplicable(valuation.CodeEBITDAMultiple, "AppliesBeyondSDE was false; this addback was applied to SDE only"),
			notApplicable(valuation.CodeCapitalizationOfEarnings, "AppliesBeyondSDE was false; this addback was applied to SDE only"),
			notApplicable(valuation.CodeDCF, "owner compensation change has no documented linkage to DCF"),
			notApplicable(valuation.CodeAdjustedNetAssetValue, "owner compensation change has no documented linkage to the adjusted net asset value method"),
		), nil
	}

	ebitdaOutcome := addToEarningsField(req.EBITDA != nil, valuation.CodeEBITDAMultiple, driver.ID, "maintainable_ebitda", amount, detail, func() *float64 {
		if req.EBITDA == nil {
			return nil
		}
		return &req.EBITDA.MaintainableEBITDA
	})
	capOutcome := addToEarningsField(req.Capitalization != nil, valuation.CodeCapitalizationOfEarnings, driver.ID, "maintainable_earnings", amount, detail, func() *float64 {
		if req.Capitalization == nil {
			return nil
		}
		return &req.Capitalization.MaintainableEarnings
	})

	return []applyOutcome{
		sdeOutcome, ebitdaOutcome, capOutcome,
		notApplicable(valuation.CodeDCF, "owner compensation change has no documented linkage to DCF"),
		notApplicable(valuation.CodeAdjustedNetAssetValue, "owner compensation change has no documented linkage to the adjusted net asset value method"),
	}, nil
}

func applyCustomerLossImpact(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.CustomerLossImpact == nil {
		return nil, missingParams(driver, "customer_loss_impact")
	}
	p := driver.CustomerLossImpact
	if p.EarningsMarginOnLostRevenue <= 0 {
		return nil, []DriverIssue{{
			Code: IssueMissingEarningsMargin, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q is a customer loss impact but EarningsMarginOnLostRevenue was <= 0; it was skipped and contributed no mutation", driver.ID),
		}}
	}

	revenueAtRisk := p.RevenueAtRiskAmount
	if revenueAtRisk == 0 {
		revenueAtRisk = p.RevenueBase * p.RevenueAtRiskPercent
	}
	earningsDelta := -(revenueAtRisk * p.EarningsMarginOnLostRevenue)
	detail := fmt.Sprintf("revenue at risk %.2f x earnings margin %.4f = earnings delta %.2f", revenueAtRisk, p.EarningsMarginOnLostRevenue, earningsDelta)

	out := []applyOutcome{
		addToEarningsField(req.SDE != nil, valuation.CodeSDEMultiple, driver.ID, "maintainable_sde", earningsDelta, detail, func() *float64 {
			if req.SDE == nil {
				return nil
			}
			return &req.SDE.MaintainableSDE
		}),
		addToEarningsField(req.EBITDA != nil, valuation.CodeEBITDAMultiple, driver.ID, "maintainable_ebitda", earningsDelta, detail, func() *float64 {
			if req.EBITDA == nil {
				return nil
			}
			return &req.EBITDA.MaintainableEBITDA
		}),
		addToEarningsField(req.Capitalization != nil, valuation.CodeCapitalizationOfEarnings, driver.ID, "maintainable_earnings", earningsDelta, detail, func() *float64 {
			if req.Capitalization == nil {
				return nil
			}
			return &req.Capitalization.MaintainableEarnings
		}),
	}

	if req.DCF != nil && len(req.DCF.ForecastPeriods) > 0 {
		var changed []ChangedInput
		for i := range req.DCF.ForecastPeriods {
			before := req.DCF.ForecastPeriods[i].FreeCashFlow
			req.DCF.ForecastPeriods[i].FreeCashFlow = before + earningsDelta
			changed = append(changed, ChangedInput{
				Method: valuation.CodeDCF, Field: fmt.Sprintf("forecast_periods[%d].free_cash_flow", i),
				Before: before, After: req.DCF.ForecastPeriods[i].FreeCashFlow, DriverID: driver.ID,
			})
		}
		out = append(out, applyOutcome{
			linkage: Linkage{Method: valuation.CodeDCF, Status: LinkageApplied, Detail: detail + " applied as a level shift to every forecast period's free cash flow"},
			changed: changed,
		})
	} else if req.DCF != nil {
		out = append(out, notApplicable(valuation.CodeDCF, "DCF input has no forecast periods to adjust"))
	} else {
		out = append(out, excluded(valuation.CodeDCF))
	}

	out = append(out, notApplicable(valuation.CodeAdjustedNetAssetValue, "customer loss impact has no documented linkage to the adjusted net asset value method"))
	return out, nil
}

func applyMethodMultipleRule(req *orchestrator.Request, driver Driver) ([]applyOutcome, []DriverIssue) {
	if driver.MethodMultipleRule == nil {
		return nil, missingParams(driver, "method_multiple_rule")
	}
	p := driver.MethodMultipleRule
	if p.NewMultiple <= 0 {
		return nil, []DriverIssue{{
			Code: IssueNonPositiveMultiple, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q supplied a non-positive NewMultiple %.4f; it was skipped and contributed no mutation", driver.ID, p.NewMultiple),
		}}
	}

	if p.Method != MultipleChangeSDE && p.Method != MultipleChangeEBITDA {
		return nil, []DriverIssue{{
			Code: IssueNoTargetMethod, Severity: DriverSeverityError, DriverID: driver.ID,
			Message: fmt.Sprintf("driver %q has an empty or unrecognized target Method %q; it was skipped and contributed no mutation", driver.ID, p.Method),
		}}
	}

	notApplicableRest := []applyOutcome{
		notApplicable(valuation.CodeCapitalizationOfEarnings, "method multiple rule has no documented linkage to this method"),
		notApplicable(valuation.CodeDCF, "method multiple rule has no documented linkage to this method"),
		notApplicable(valuation.CodeAdjustedNetAssetValue, "method multiple rule has no documented linkage to this method"),
	}

	method, other, target, hasInput := valuation.CodeSDEMultiple, valuation.CodeEBITDAMultiple, (*float64)(nil), false
	if p.Method == MultipleChangeSDE {
		hasInput = req.SDE != nil
		if hasInput {
			target = &req.SDE.Multiple
		}
	} else {
		method, other = valuation.CodeEBITDAMultiple, valuation.CodeSDEMultiple
		hasInput = req.EBITDA != nil
		if hasInput {
			target = &req.EBITDA.Multiple
		}
	}

	otherOutcome := notApplicable(other, "this driver's Method named a different multiple method")
	if !hasInput {
		return append([]applyOutcome{excluded(method), otherOutcome}, notApplicableRest...), nil
	}

	detail := fmt.Sprintf("multiple replaced with caller-supplied %.4f", p.NewMultiple)
	if p.TriggerLabel != "" {
		detail = fmt.Sprintf("%s (rule: %s = %s)", detail, p.TriggerLabel, p.TriggerValue)
	}
	before := *target
	*target = p.NewMultiple

	return append([]applyOutcome{
		applied(method, driver.ID, "multiple", before, p.NewMultiple, detail), otherOutcome,
	}, notApplicableRest...), nil
}

func missingParams(driver Driver, field string) []DriverIssue {
	return []DriverIssue{{
		Code: IssueMissingDriverParams, Severity: DriverSeverityError, DriverID: driver.ID,
		Message: fmt.Sprintf("driver %q has type %q but its %s parameters were nil; it was skipped and contributed no mutation", driver.ID, driver.Type, field),
	}}
}

func joinDetails(details []string) string {
	return strings.Join(details, "; ")
}
