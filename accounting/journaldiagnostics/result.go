package journaldiagnostics

import (
	"sort"

	"github.com/themurtez/go-valuate/accounting/ledger"
)

// Result is this package's top-level, presentation-neutral output. It
// never chooses a UI layout or narrative — it returns structured findings,
// summaries, and availability so the calling application decides
// presentation. See the package doc comment for the non-fraud/no-composite-
// score boundary this Result honors throughout.
type Result struct {
	Period string `json:"period"`

	Coverage               Coverage               `json:"coverage"`
	PopulationSummary      PopulationSummary      `json:"population_summary"`
	SourceSummary          SourceSummary          `json:"source_summary"`
	AccountActivitySummary AccountActivitySummary `json:"account_activity_summary"`

	Findings         []Finding        `json:"findings,omitempty"`
	DuplicateGroups  []DuplicateGroup `json:"duplicate_groups,omitempty"`
	ReversalSummary  ReversalSummary  `json:"reversal_summary"`
	PeriodEndSummary PeriodEndSummary `json:"period_end_summary"`

	RuleAvailability RuleAvailability `json:"rule_availability"`

	// CrossPeriodComparison is populated only when Calculate is called via
	// CalculateWithPrior — see compareToPrior and section 35.
	CrossPeriodComparison CrossPeriodComparison `json:"cross_period_comparison,omitempty"`

	// Issues covers analysis/input problems, distinct from Findings (a
	// journal-entry anomaly). See IssueCode.
	Issues []Issue `json:"issues,omitempty"`
	// LedgerIssues carries through the raw ledger.Issue values from
	// ledger.ValidateEntries, for a caller that wants the full structural
	// detail behind IssueLedgerValidationFailed rather than just the
	// summary Issue.
	LedgerIssues []ledger.Issue `json:"ledger_issues,omitempty"`

	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`
}

// nonFiniteThresholds scans p for any NaN/Inf numeric field, returning an
// Issue per offending field name (not the value itself — a NaN doesn't
// serialize meaningfully into a message). The offending rule reads as
// disabled downstream since every rule's own resolve()/gate already treats
// a non-positive/zero threshold as "off," and isNonFinite fields are
// normalized to 0 before any rule reads them — see sanitizePolicy.
func nonFiniteThresholds(p Policy) []Issue {
	fields := map[string]float64{
		"material_amount": p.MaterialAmount, "round_dollar_min_amount": p.RoundDollarMinAmount,
		"large_entry_absolute_threshold": p.LargeEntryAbsoluteThreshold, "mad_multiplier": p.MADMultiplier,
		"duplicate_window_days": float64(p.DuplicateWindowDays), "repeated_amount_min_amount": p.RepeatedAmountMinAmount,
		"approval_threshold": p.ApprovalThreshold, "threshold_cluster_lower_percent": p.ThresholdClusterLowerPercent,
	}
	var issues []Issue
	for name, v := range fields {
		if isNonFinite(v) {
			issues = append(issues, Issue{
				Code: IssueNonFiniteThreshold, Severity: IssueSeverityError,
				Message: "policy field " + name + " is not a finite number; the rules that depend on it are disabled",
			})
		}
	}
	return issues
}

// sanitizePolicy replaces any NaN/Inf numeric field in p with 0 (disabling
// the rules that depend on it) so no non-finite value ever reaches a
// comparison or gets serialized into a Finding/Evidence — money-safety
// rule applied to Policy itself, not just entry amounts.
func sanitizePolicy(p Policy) Policy {
	zeroIfNonFinite := func(v float64) float64 {
		if isNonFinite(v) {
			return 0
		}
		return v
	}
	p.MaterialAmount = zeroIfNonFinite(p.MaterialAmount)
	p.RoundDollarMinAmount = zeroIfNonFinite(p.RoundDollarMinAmount)
	p.LargeEntryAbsoluteThreshold = zeroIfNonFinite(p.LargeEntryAbsoluteThreshold)
	p.MADMultiplier = zeroIfNonFinite(p.MADMultiplier)
	p.RepeatedAmountMinAmount = zeroIfNonFinite(p.RepeatedAmountMinAmount)
	p.ApprovalThreshold = zeroIfNonFinite(p.ApprovalThreshold)
	p.ThresholdClusterLowerPercent = zeroIfNonFinite(p.ThresholdClusterLowerPercent)
	if p.ThresholdClusterLowerPercent < 0 || p.ThresholdClusterLowerPercent > 1 {
		p.ThresholdClusterLowerPercent = 0 // INVALID_POLICY surfaced separately below; disable rather than misapply
	}
	return p
}

func validatePolicy(p Policy) []Issue {
	var issues []Issue
	if p.ThresholdClusterLowerPercent != 0 && (p.ThresholdClusterLowerPercent < 0 || p.ThresholdClusterLowerPercent > 1) {
		issues = append(issues, Issue{
			Code: IssueInvalidPolicy, Severity: IssueSeverityError,
			Message: "policy field threshold_cluster_lower_percent must be within [0, 1]",
		})
	}
	negFields := map[string]float64{
		"material_amount": p.MaterialAmount, "period_end_days": float64(p.PeriodEndDays),
		"round_dollar_min_amount": p.RoundDollarMinAmount, "large_entry_absolute_threshold": p.LargeEntryAbsoluteThreshold,
		"mad_multiplier": p.MADMultiplier, "min_baseline_observations": float64(p.MinBaselineObservations),
		"rare_account_max_historical_entries": float64(p.RareAccountMaxHistoricalEntries),
		"duplicate_window_days":               float64(p.DuplicateWindowDays),
		"min_repeated_amount_count":           float64(p.MinRepeatedAmountCount),
		"repeated_amount_min_amount":          p.RepeatedAmountMinAmount,
		"rapid_reversal_days":                 float64(p.RapidReversalDays),
		"approval_threshold":                  p.ApprovalThreshold,
		"threshold_cluster_min_count":         float64(p.ThresholdClusterMinCount),
	}
	for name, v := range negFields {
		if !isNonFinite(v) && v < 0 {
			issues = append(issues, Issue{
				Code: IssueInvalidPolicy, Severity: IssueSeverityError,
				Message: "policy field " + name + " must not be negative",
			})
		}
	}
	return issues
}

// Calculate runs every diagnostic rule in this package against l, using
// metadata and window, per policy. It does not mutate l, metadata, window,
// or policy. See the package doc comment for the full behavior contract
// (non-fraud boundary, determinism, no hidden current-date dependency).
func Calculate(l ledger.Ledger, metadata []EntryMetadata, window PeriodWindow, policy Policy) Result {
	var issues []Issue

	if !window.Valid() {
		issues = append(issues, Issue{
			Code: IssueInvalidPeriod, Severity: IssueSeverityError,
			Message: "period window requires both StartDate and EndDate, with StartDate <= EndDate",
		})
	}

	issues = append(issues, validatePolicy(policy)...)
	issues = append(issues, nonFiniteThresholds(policy)...)
	policy = sanitizePolicy(policy).resolve()

	entryIDs := make(map[string]bool, len(l.Entries))
	for _, e := range l.Entries {
		entryIDs[e.ID] = true
	}
	metaByID, metaIssues := buildMetadataIndex(metadata, entryIDs)
	issues = append(issues, metaIssues...)

	all, excludedCount, ledgerIssues := buildPopulation(l, metaByID, policy)
	if ledger.HasErrors(ledgerIssues) {
		issues = append(issues, Issue{
			Code: IssueLedgerValidationFailed, Severity: IssueSeverityWarning,
			Message: "ledger validation reported structural errors; affected entries were excluded from statistical diagnostics — see LedgerIssues",
		})
	}

	popSummary, coverage := computePopulationSummary(l.Entries, all, excludedCount)
	sourceSummary := computeSourceSummary(all, popSummary.TotalAnalyzedAmount)
	accountActivity := computeAccountActivitySummary(all)
	chart := l.Chart()

	var findings []Finding
	var avail RuleAvailability

	f, st := findMaterialManualEntries(all, policy)
	findings = append(findings, f...)
	avail.MaterialManualEntry = st

	f, st = findPeriodEndFindings(all, window, policy)
	findings = append(findings, f...)
	avail.PeriodEnd = st
	periodEndSummary, _ := computePeriodEndSummary(all, window, policy, popSummary.TotalAnalyzedAmount)

	f, st = findPostCloseFindings(all, window)
	findings = append(findings, f...)
	avail.PostClose = st

	f, st = findWeekendFindings(all, policy)
	findings = append(findings, f...)
	avail.Weekend = st

	f, st = findOutsideBusinessHoursFindings(all, policy)
	findings = append(findings, f...)
	avail.OutsideBusinessHours = st

	f, st = findRoundDollarFindings(all, policy)
	findings = append(findings, f...)
	avail.RoundDollar = st

	f, st = findLargeEntryAbsoluteFindings(all, policy)
	findings = append(findings, f...)
	avail.LargeEntryAbsolute = st

	f, st, insufficientAccounts := findAccountRelativeLargeEntryFindings(all, policy)
	findings = append(findings, f...)
	avail.AccountRelativeLargeEntry = st
	for _, acct := range insufficientAccounts {
		issues = append(issues, Issue{
			Code: IssueInsufficientBaseline, Severity: IssueSeverityWarning,
			Message:   "account " + acct + " has fewer than the configured minimum baseline observations; account-relative large-entry testing unavailable for it this run",
			AccountID: acct,
		})
	}

	f, st = findRareAndNewAccountFindings(all, policy)
	findings = append(findings, f...)
	avail.RareAccountActivity = st
	avail.NewAccountActivity = st

	f, st = findOppositeNormalBalanceFindings(all, chart, policy)
	findings = append(findings, f...)
	avail.OppositeNormalBalanceMovement = st

	f, st = findManualRevenueEquityFindings(all, chart)
	findings = append(findings, f...)
	avail.ManualRevenueEquity = st

	f, st = findSensitiveAccountFindings(all, policy)
	findings = append(findings, f...)
	avail.SensitiveAccountEntry = st

	var groups []DuplicateGroup
	f, groups, avail.ExactDuplicateEntry, avail.PossibleDuplicateEntry = findDuplicateFindings(all, policy)
	findings = append(findings, f...)

	f, st = findRepeatedIdenticalAmountFindings(all, policy)
	findings = append(findings, f...)
	avail.RepeatedIdenticalAmount = st

	reversalSummary := computeReversalSummary(all, policy)
	f, st = findReversalFindings(all, policy)
	findings = append(findings, f...)
	avail.Reversals = st

	f, st = findPeriodEndEarlyReversalFindings(all, window, policy)
	findings = append(findings, f...)
	avail.PeriodEndEarlyReversal = st

	f, st = findThresholdClusterFindings(all, policy)
	findings = append(findings, f...)
	avail.ThresholdCluster = st

	f, st = findSplitEntryClusterFindings(all, policy)
	findings = append(findings, f...)
	avail.SplitEntryCluster = st

	f, st = findBlankDescriptionFindings(all)
	findings = append(findings, f...)
	avail.BlankDescription = st

	f, st = findGenericDescriptionFindings(all, policy)
	findings = append(findings, f...)
	avail.GenericDescription = st

	f, st = findMissingReferenceFindings(all, policy)
	findings = append(findings, f...)
	avail.MissingReference = st

	f, st = findPreparerApproverFindings(all, policy)
	findings = append(findings, f...)
	avail.PreparerApproverDiagnostics = st

	f, st = findRareAccountCombinationFindings(all, policy)
	findings = append(findings, f...)
	avail.RareAccountCombination = st

	avail.CrossPeriodComparison = RuleUnavailable // set to RuleAvailable by CalculateWithPrior only

	return Result{
		Period:                 window.Period,
		Coverage:               coverage,
		PopulationSummary:      popSummary,
		SourceSummary:          sourceSummary,
		AccountActivitySummary: accountActivity,
		Findings:               sortFindings(findings),
		DuplicateGroups:        sortDuplicateGroups(groups),
		ReversalSummary:        reversalSummary,
		PeriodEndSummary:       periodEndSummary,
		RuleAvailability:       avail,
		Issues:                 issues,
		LedgerIssues:           ledgerIssues,
		SchemaVersion:          SchemaVersion,
		FormulaVersion:         FormulaVersion,
	}
}

// CalculateWithPrior runs Calculate and additionally populates
// Result.CrossPeriodComparison against a caller-supplied prior-period
// Result — see section 35 and compareToPrior. The prior Result is read-only
// comparison input, never mutated, and this package never fabricates the
// prior period's data itself.
func CalculateWithPrior(l ledger.Ledger, metadata []EntryMetadata, window PeriodWindow, policy Policy, prior Result) Result {
	r := Calculate(l, metadata, window, policy)
	r.CrossPeriodComparison = compareToPrior(r, prior)
	r.RuleAvailability.CrossPeriodComparison = RuleAvailable
	return r
}

// sortDuplicateGroups orders groups by (date, first entry ID) — see the
// package doc's determinism section.
func sortDuplicateGroups(groups []DuplicateGroup) []DuplicateGroup {
	out := make([]DuplicateGroup, len(groups))
	copy(out, groups)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].EarliestDate != out[j].EarliestDate {
			return out[i].EarliestDate < out[j].EarliestDate
		}
		fi, fj := "", ""
		if len(out[i].EntryIDs) > 0 {
			fi = out[i].EntryIDs[0]
		}
		if len(out[j].EntryIDs) > 0 {
			fj = out[j].EntryIDs[0]
		}
		return fi < fj
	})
	return out
}
