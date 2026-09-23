package cashforecast

import "time"

// Input bundles every caller-supplied cash-event source Calculate needs.
// Every slice here is optional except OpeningCash (or CashAccounts) and
// ForecastStartDate; Calculate computes whatever subset of the analysis
// the supplied input supports.
type Input struct {
	// ForecastStartDate is the explicit date Week 1 is measured from.
	// Required — Calculate returns Result{Available: false} with an issue
	// if it is the zero time.Time. Never defaulted to time.Now(); see the
	// package doc comment.
	ForecastStartDate time.Time `json:"forecast_start_date"`

	OpeningCash  OpeningCash   `json:"opening_cash"`
	CashAccounts []CashAccount `json:"cash_accounts,omitempty"`

	// Events is every directly-supplied CashFlowEvent — manual entries,
	// or events a caller has already built from its own AR/AP/payroll/tax
	// systems. Combined with every event this package's own adapters/
	// generators produce (RecurringRules, ARCollections, APPayments,
	// Payroll, Tax, DebtService, Capex, Financing) into one base event
	// set.
	Events []CashFlowEvent `json:"events,omitempty"`

	RecurringRules []RecurringRule `json:"recurring_rules,omitempty"`

	ARSources     []ARReceivableSource     `json:"ar_sources,omitempty"`
	ARCollections []ARCollectionAssumption `json:"ar_collections,omitempty"`

	APSources []APPayableSource `json:"ap_sources,omitempty"`
	APPlans   []APPaymentPlan   `json:"ap_plans,omitempty"`

	Payroll     []PayrollEvent     `json:"payroll,omitempty"`
	Tax         []TaxEvent         `json:"tax,omitempty"`
	DebtService []DebtServiceEvent `json:"debt_service,omitempty"`
	Capex       []CapexEvent       `json:"capex,omitempty"`
	Financing   []FinancingEvent   `json:"financing,omitempty"`

	Facilities []CreditFacility `json:"facilities,omitempty"`

	// Scenarios is zero or more named what-if cases evaluated in addition
	// to the base case, in the order supplied.
	Scenarios []Scenario `json:"scenarios,omitempty"`

	// ARSnapshotDate/APSnapshotDate are optional as-of dates for ARSources/
	// APSources, used only for Coverage staleness reporting.
	ARSnapshotDate time.Time `json:"ar_snapshot_date,omitempty"`
	APSnapshotDate time.Time `json:"ap_snapshot_date,omitempty"`
}

// Options configures Calculate's behavior. All fields are optional; the
// zero value resolves to documented safe defaults.
type Options struct {
	// HorizonWeeks is the forecast horizon in weeks. Zero resolves to
	// DefaultHorizonWeeks (13); must be in [1, MaxHorizonWeeks] if set —
	// see the task's section 2.
	HorizonWeeks int `json:"horizon_weeks,omitempty"`
	// WeekAlignment controls Week 1's start date relative to
	// ForecastStartDate. Zero resolves to AlignCallerStartDate.
	WeekAlignment WeekAlignment `json:"week_alignment,omitempty"`

	ReportingCurrency string `json:"reporting_currency,omitempty"`

	MinimumCash MinimumCashPolicy `json:"minimum_cash"`

	// PayAPOnDueDate opts into the section 16/54 due-date adapter: every
	// APSources payable not already covered by an explicit APPlans entry
	// generates an outflow event on its own due date. Default false — see
	// buildAPDueDateEvents's doc comment.
	PayAPOnDueDate bool `json:"pay_ap_on_due_date,omitempty"`

	RequiredInputs RequiredInputs  `json:"required_inputs"`
	Staleness      StalenessPolicy `json:"staleness"`
	FlagThresholds FlagThresholds  `json:"flag_thresholds"`
}

// Calculate derives a full Result from in under opts. It never mutates
// in.Events, in.RecurringRules, in.ARSources, in.ARCollections,
// in.APSources, in.APPlans, in.Payroll, in.Tax, in.DebtService, in.Capex,
// in.Financing, in.Facilities, in.Scenarios, in.CashAccounts, or any
// slice/map within them, and performs no I/O — see immutability_test.go.
func Calculate(in Input, opts Options) Result {
	var issues []Issue

	result := Result{
		SchemaVersion:     SchemaVersion,
		FormulaVersion:    FormulaVersion,
		WeekAlignment:     resolvedAlignment(opts.WeekAlignment),
		ReportingCurrency: opts.ReportingCurrency,
	}

	if in.ForecastStartDate.IsZero() {
		issues = append(issues, Issue{Code: IssueInvalidForecastStart, Severity: SeverityError,
			Message: "ForecastStartDate is required"})
		result.Issues = issues
		return result
	}
	result.ForecastStartDate = in.ForecastStartDate.Format("2006-01-02")

	horizonWeeks := resolvedHorizonWeeks(opts.HorizonWeeks)
	if opts.HorizonWeeks < 0 || opts.HorizonWeeks > MaxHorizonWeeks {
		issues = append(issues, Issue{Code: IssueInvalidHorizon, Severity: SeverityError,
			Message: "HorizonWeeks must be between 1 and MaxHorizonWeeks"})
		result.Issues = issues
		return result
	}
	result.HorizonWeeks = horizonWeeks

	weeks := buildWeekBounds(in.ForecastStartDate, opts.WeekAlignment, horizonWeeks)
	result.ForecastEndDate = weeks[len(weeks)-1].EndDate.Format("2006-01-02")
	horizonStart, horizonEnd := weeks[0].StartDate, weeks[len(weeks)-1].EndDate

	openingIssues := validateOpeningCash(in.OpeningCash, in.CashAccounts, opts.ReportingCurrency)
	issues = append(issues, openingIssues...)
	openingPosition := resolveOpeningPosition(in.OpeningCash, in.CashAccounts, opts.ReportingCurrency)
	result.OpeningPosition = openingPosition

	validScenarios, scenarioIssues := validateScenarios(in.Scenarios)
	issues = append(issues, scenarioIssues...)

	facilities, facilityIssues := validateFacilities(in.Facilities)
	issues = append(issues, facilityIssues...)

	// Assemble the base event set from every source, in a fixed order so
	// generation (and therefore duplicate-ID detection) is deterministic
	// regardless of which sources are populated.
	var baseEvents []CashFlowEvent
	baseEvents = append(baseEvents, in.Events...)

	for _, rule := range in.RecurringRules {
		ruleIssues := validateRecurringRule(rule)
		issues = append(issues, ruleIssues...)
		if HasErrors(ruleIssues) {
			continue
		}
		baseEvents = append(baseEvents, generateRecurringEvents(rule, horizonStart, horizonEnd)...)
	}

	arEvents, arIssues := buildAREvents(in.ARCollections, in.ARSources)
	issues = append(issues, arIssues...)
	baseEvents = append(baseEvents, arEvents...)

	apEvents, apIssues := buildAPEvents(in.APPlans, in.APSources)
	issues = append(issues, apIssues...)
	baseEvents = append(baseEvents, apEvents...)

	if opts.PayAPOnDueDate {
		baseEvents = append(baseEvents, buildAPDueDateEvents(in.APSources, in.APPlans)...)
	}

	baseEvents = append(baseEvents, buildPayrollEvents(in.Payroll)...)
	baseEvents = append(baseEvents, buildTaxEvents(in.Tax)...)
	baseEvents = append(baseEvents, buildDebtServiceEvents(in.DebtService)...)
	baseEvents = append(baseEvents, buildCapexEvents(in.Capex)...)

	financingEvents, financingIssues := buildFinancingEvents(in.Financing)
	issues = append(issues, financingIssues...)
	baseEvents = append(baseEvents, financingEvents...)

	validBase, eventIssues := validateAndDedupeEvents(baseEvents)
	issues = append(issues, eventIssues...)

	inRange, beforeStart, beyondHorizon := partitionEventsByRange(validBase, horizonStart, horizonEnd)
	for _, e := range beforeStart {
		issues = append(issues, Issue{Code: IssueEventBeforeForecastStart, Severity: SeverityWarning,
			Message: "event Date is before ForecastStartDate; excluded from weekly totals", EventID: e.ID})
	}
	for _, e := range beyondHorizon {
		issues = append(issues, Issue{Code: IssueEventBeyondHorizon, Severity: SeverityWarning,
			Message: "event Date is after the forecast horizon; excluded from weekly totals", EventID: e.ID})
	}
	result.BeforeStart = summarizeEvents(beforeStart)
	result.BeyondHorizon = BeyondHorizonSummary{Count: len(beyondHorizon), Amount: sumEventAmounts(beyondHorizon)}

	minimumThreshold, haveThreshold := resolvedMinimumCashThreshold(opts.MinimumCash, openingPosition.AccountReserveTotal)

	buildOne := func(events []CashFlowEvent, label string) ScenarioResult {
		weeklyForecast, detailed := buildWeeklyForecast(weeks, openingPosition.UnrestrictedCash, events, minimumThreshold, haveThreshold)
		summary := buildLiquiditySummary(weeklyForecast, minimumThreshold, haveThreshold, facilities)
		return ScenarioResult{Label: label, Weekly: weeklyForecast, Summary: summary, DetailedSchedule: detailed}
	}

	// filterForScenario keeps only events that apply to label (empty
	// ScenarioTags applies everywhere, including base) — see
	// appliesToScenario. Applied to the base case too, not just named
	// scenarios, so a caller-pre-tagged Input.Events entry never leaks
	// into base.
	filterForScenario := func(events []CashFlowEvent, label string) []CashFlowEvent {
		out := make([]CashFlowEvent, 0, len(events))
		for _, e := range events {
			if appliesToScenario(e, label) {
				out = append(out, e)
			}
		}
		return out
	}

	result.BaseScenario = buildOne(filterForScenario(inRange, BaseScenarioLabel), BaseScenarioLabel)

	// Only scenarios that individually passed validateScenarios are
	// processed here — one invalid scenario (bad Label, duplicate, etc.)
	// must never silently drop every other, otherwise-valid, scenario
	// from the Result. See validateScenarios' doc comment.
	for _, sc := range validScenarios {
		scenarioBaseEvents := applyScenario(validBase, sc)
		// Re-validate after transformation: a caller-supplied
		// ScaleFactor (e.g. negative, by mistake) or an ADD_EVENT
		// transform can produce an event that no longer satisfies the
		// same invariants (Amount >= 0, finite) validateAndDedupeEvents
		// already enforces on the base event set — those invariants must
		// hold for scenario-transformed events too, not just the
		// original ones.
		scenarioValidEvents, scenarioValidationIssues := validateAndDedupeEvents(scenarioBaseEvents)
		for i := range scenarioValidationIssues {
			scenarioValidationIssues[i].SourceID = sc.Label
		}
		issues = append(issues, scenarioValidationIssues...)
		scenarioInRange, scenarioBeforeStart, scenarioBeyondHorizon := partitionEventsByRange(scenarioValidEvents, horizonStart, horizonEnd)
		for _, e := range scenarioBeforeStart {
			issues = append(issues, Issue{Code: IssueEventBeforeForecastStart, Severity: SeverityWarning,
				Message: "event Date is before ForecastStartDate; excluded from weekly totals", EventID: e.ID, SourceID: sc.Label})
		}
		for _, e := range scenarioBeyondHorizon {
			issues = append(issues, Issue{Code: IssueEventBeyondHorizon, Severity: SeverityWarning,
				Message: "event Date is after the forecast horizon; excluded from weekly totals", EventID: e.ID, SourceID: sc.Label})
		}
		scenarioResult := buildOne(scenarioInRange, sc.Label)
		scenarioResult.DeltaVsBase = buildDeltaVsBase(scenarioResult.Weekly, result.BaseScenario.Weekly)
		result.Scenarios = append(result.Scenarios, scenarioResult)
	}

	result.UnscheduledAR = unscheduledARAmount(in.ARSources, in.ARCollections)
	result.UnscheduledAP = unscheduledAPAmount(in.APSources, in.APPlans, opts.PayAPOnDueDate)

	var coverageIssues []Issue
	result.Coverage, coverageIssues = buildCoverage(in, opts, openingPosition, result.UnscheduledAR, result.UnscheduledAP)
	issues = append(issues, coverageIssues...)
	for _, category := range result.Coverage.RequiredInputsMissing {
		issues = append(issues, Issue{Code: IssueMissingRequiredInput, Severity: SeverityWarning,
			Message: "a category Options.RequiredInputs marked required was not supplied", SourceID: category})
	}

	result.Available = true
	result.Issues = dedupeAndSortIssues(issues)
	result.Flags = computeFlags(flagInputs{
		result:     result,
		facilities: facilities,
		thresholds: resolveFlagThresholds(opts.FlagThresholds),
	})

	return result
}

// partitionEventsByRange splits events into three groups by Date relative
// to [rangeStart, rangeEnd]: in-range, before rangeStart, and after
// rangeEnd.
func partitionEventsByRange(events []CashFlowEvent, rangeStart, rangeEnd time.Time) (inRange, before, beyond []CashFlowEvent) {
	for _, e := range events {
		d := truncateToDate(e.Date)
		switch {
		case d.Before(truncateToDate(rangeStart)):
			before = append(before, e)
		case d.After(truncateToDate(rangeEnd)):
			beyond = append(beyond, e)
		default:
			inRange = append(inRange, e)
		}
	}
	return
}

func summarizeEvents(events []CashFlowEvent) BeforeStartSummary {
	return BeforeStartSummary{Count: len(events), Amount: sumEventAmounts(events)}
}

func sumEventAmounts(events []CashFlowEvent) float64 {
	var total float64
	for _, e := range events {
		total += e.Amount
	}
	return total
}

// dedupeAndSortIssues sorts issues deterministically. Issues are not
// deduplicated beyond what validateAndDedupeEvents/adapter functions
// already avoid emitting twice for the same cause — each Issue-emitting
// call site fires at most once per distinct problem instance.
func dedupeAndSortIssues(issues []Issue) []Issue {
	out := append([]Issue{}, issues...)
	sortIssues(out)
	return out
}
