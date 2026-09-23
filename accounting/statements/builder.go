package statements

import (
	"sort"
	"strings"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/reconciliation"
)

// Source selects which of the two convergent input paths (task section 2)
// a Build call uses. Exactly one of Input's corresponding fields is read,
// selected by this value — Build does not attempt to infer which path was
// intended from which fields happen to be non-empty, since a caller could
// legitimately supply ledger data for provenance alongside an imported TB
// it actually wants built from.
type Source string

const (
	// SourceLedger builds from Input.Chart/Input.Entries (or
	// Input.OpeningBalances), replaying journal activity via
	// ledger.CalculateBalances for each requested period.
	SourceLedger Source = "LEDGER"
	// SourceImportedTrialBalance builds from Input.ImportedTrialBalances,
	// one ledger.NormalizedTrialBalance per requested period, with no
	// journal detail at all.
	SourceImportedTrialBalance Source = "IMPORTED_TRIAL_BALANCE"
)

// StatementSelection controls which statement(s) a Build call attempts.
type StatementSelection string

const (
	SelectionBoth        StatementSelection = "BOTH"
	SelectionIncomeOnly  StatementSelection = "INCOME_STATEMENT_ONLY"
	SelectionBalanceOnly StatementSelection = "BALANCE_SHEET_ONLY"
)

func (s StatementSelection) wantsIncome() bool {
	return s == "" || s == SelectionBoth || s == SelectionIncomeOnly
}

func (s StatementSelection) wantsBalance() bool {
	return s == "" || s == SelectionBoth || s == SelectionBalanceOnly
}

// Input bundles everything Build needs. Exactly one source path is used,
// selected by Source — see Source's doc comment.
type Input struct {
	// Source selects the ledger-derived or imported-TB path. Required.
	Source Source `json:"source"`

	// --- SourceLedger fields ---

	// Chart is the chart of accounts to build from. Required for
	// SourceLedger.
	Chart []ledger.Account `json:"chart,omitempty"`
	// Entries is the journal activity to replay. Required for
	// SourceLedger unless OpeningBalances alone is sufficient for every
	// requested period (an unusual but valid case — a caller with only
	// opening positions and no in-period activity).
	Entries []ledger.JournalEntry `json:"entries,omitempty"`
	// OpeningBalances supplies explicit starting positions, passed through
	// to ledger.BalanceOptions.Openings for every requested period — see
	// ledger.OpeningBalance's own doc comment for exactly how this
	// interacts with Range.StartDate.
	OpeningBalances []ledger.OpeningBalance `json:"opening_balances,omitempty"`
	// IncludeStatuses restricts which ledger.EntryStatus values count as
	// period activity, passed through to
	// ledger.BalanceOptions.IncludeStatuses. Zero value defaults to
	// ledger.PostedStatuses() exactly as accounting/ledger itself
	// defaults.
	IncludeStatuses []ledger.EntryStatus `json:"include_statuses,omitempty"`

	// --- SourceImportedTrialBalance fields ---

	// ImportedTrialBalances supplies one already-normalized trial balance
	// per period this build covers — see ledger.NormalizeTrialBalance,
	// which a caller runs upstream of this package (this package never
	// re-derives a NormalizedTrialBalance from a raw
	// ledger.TrialBalanceInput itself; it consumes the normalized
	// output). Required for SourceImportedTrialBalance, keyed by the same
	// financial.Period values as Periods.
	ImportedTrialBalances map[financial.Period]ledger.NormalizedTrialBalance `json:"imported_trial_balances,omitempty"`
	// ImportedChart supplies account metadata (Type, ParentID, Number,
	// Name, Active) for the accounts referenced in
	// ImportedTrialBalances, since ledger.NormalizedTrialBalance already
	// carries AccountType/Number/Name per line but not ParentID/Active —
	// this package needs ParentID for hierarchy policy (Section 14) and
	// Active for inactive-account issues. Required for
	// SourceImportedTrialBalance.
	ImportedChart []ledger.Account `json:"imported_chart,omitempty"`

	// --- Shared fields ---

	// Periods is the caller-selected set of reporting periods this build
	// covers. Required; Build never guesses a fiscal calendar — see the
	// task's explicit "caller supplies period selection" rule. For
	// SourceLedger, each period is resolved via a ledger.PeriodRange
	// matching ledger.JournalEntry.Period exactly (not a date range —
	// see periods.go); for SourceImportedTrialBalance, each period must
	// have a matching key in ImportedTrialBalances.
	Periods []financial.Period `json:"periods"`
	// ReportingCurrency is the single currency this build's
	// FinancialDataset is denominated in. Required whenever the
	// contributing accounts use more than one currency (see
	// IssueMixedCurrency); optional when every account shares one
	// currency, in which case that shared currency is used automatically.
	ReportingCurrency string `json:"reporting_currency,omitempty"`
	// Mappings is the caller's explicit account-mapping list — always
	// authoritative over any deterministic suggestion, per task section 4.
	Mappings []AccountMapping `json:"mappings,omitempty"`
	// Selection controls which statement(s) to attempt. Zero value is
	// SelectionBoth.
	Selection StatementSelection `json:"selection,omitempty"`
}

// Options configures Build's policy decisions — everything that isn't
// itself financial data.
type Options struct {
	// Mapping configures the mapping-suggestion stage — see
	// MappingOptions.
	Mapping MappingOptions
	// UnmappedPolicy controls how a material unmapped account affects
	// dataset availability. Zero value is PolicyAllowUnmappedWithWarning.
	UnmappedPolicy UnmappedPolicy
	// Materiality configures what counts as a material unmapped balance
	// for UnmappedPolicy purposes — see MaterialityPolicy.
	Materiality MaterialityPolicy
	// Hierarchy controls parent/child double-counting policy. Zero value
	// is HierarchyLeafOnly, currently the only implemented policy.
	Hierarchy HierarchyPolicy
	// InactivePostingPolicy mirrors
	// ledger.ValidateOptions.InactivePostingPolicy for entries validation
	// when Input.Source == SourceLedger. Zero value is
	// ledger.InactivePostingWarning.
	InactivePostingPolicy ledger.InactivePostingPolicy
	// BalanceTolerance is the tolerance passed to
	// financial/reconciliation.Run's balance-sheet check (see
	// reconcile.go). Zero value uses reconciliation.DefaultTolerance.
	BalanceTolerance reconciliation.Tolerance
	// TrialBalanceTolerance is the tolerance used when building a
	// ledger-derived ledger.TrialBalance internally, and when checking
	// SourceImportedTrialBalance's own NormalizedTrialBalance.Balanced —
	// see IssueSourceTrialBalanceUnbalanced. Zero value uses 0.01 (one
	// cent), a conservative default for accumulated float rounding.
	TrialBalanceTolerance float64
}

func (o Options) trialBalanceTolerance() float64 {
	if o.TrialBalanceTolerance == 0 {
		return 0.01
	}
	return o.TrialBalanceTolerance
}

// MaterialityPolicy configures IssueUnmappedAccount materiality, reusing
// review.IsMaterial's exact semantics (absolute threshold OR percent of a
// reference amount) rather than inventing a parallel accounting
// materiality standard — see the task's explicit "use simple
// deterministic logic... do not create an accounting materiality
// standard" instruction. This package supplies its own small
// MaterialityPolicy type (rather than importing review.Policy wholesale)
// because review.Policy carries many unrelated OCR/classification/
// reconciliation fields this package has no use for; only the two
// materiality-relevant fields are mirrored here, and review.IsMaterial
// itself is called directly with them — see issues.go's materiality
// helper.
type MaterialityPolicy struct {
	// AbsoluteThreshold mirrors review.Policy.MaterialAmountThreshold. 0
	// (default) means "not applied" for this leg.
	AbsoluteThreshold float64
	// PercentOfTotalAssets is the fraction of total assets (e.g. 0.01 =
	// 1%) below which an unmapped balance-sheet account's balance is
	// immaterial, when total assets are known. 0 (default) means "not
	// applied."
	PercentOfTotalAssets float64
	// PercentOfRevenue is the fraction of total revenue below which an
	// unmapped income-statement account's balance is immaterial, when
	// revenue is known. 0 (default) means "not applied."
	PercentOfRevenue float64
}

// DatasetAvailability distinguishes the states task section 37 requires:
// never encoded by an empty slice alone.
type DatasetAvailability string

const (
	// AvailabilityNotRequested means the caller's Input.Selection did not
	// ask for this statement.
	AvailabilityNotRequested DatasetAvailability = "NOT_REQUESTED"
	// AvailabilityUnavailable means the statement was requested but no
	// data existed to build it (e.g. SelectionBoth requested but the
	// source chart has zero balance-sheet accounts at all).
	AvailabilityUnavailable DatasetAvailability = "UNAVAILABLE"
	// AvailabilityBuilt means the statement built with no blocking
	// problems.
	AvailabilityBuilt DatasetAvailability = "BUILT"
	// AvailabilityBuiltWithWarnings means the statement built, but with at
	// least one warning-severity issue (e.g. a material unmapped account
	// under PolicyAllowUnmappedWithWarning).
	AvailabilityBuiltWithWarnings DatasetAvailability = "BUILT_WITH_WARNINGS"
	// AvailabilityInvalid means the statement could not be trusted — e.g.
	// PolicyFailOnUnmappedMaterial tripped, or a structural error (mixed
	// currency with no reporting currency resolved) blocked construction.
	AvailabilityInvalid DatasetAvailability = "INVALID"
)

// Result is Build's top-level output.
type Result struct {
	SchemaVersion           string `json:"schema_version"`
	StatementFormulaVersion string `json:"statement_formula_version"`
	MappingContractVersion  string `json:"mapping_contract_version"`

	// IncomeStatement is the built income statement model, when
	// requested/available. See IncomeStatementAvailability for its
	// availability state.
	IncomeStatement             *Statement          `json:"income_statement,omitempty"`
	IncomeStatementAvailability DatasetAvailability `json:"income_statement_availability"`
	// BalanceSheet is the built balance sheet model, when
	// requested/available.
	BalanceSheet             *Statement          `json:"balance_sheet,omitempty"`
	BalanceSheetAvailability DatasetAvailability `json:"balance_sheet_availability"`

	// Dataset is the primary bridge output: the combined
	// financial.FinancialDataset built from every mapped NORMAL account
	// across both statements — see dataset.go.
	Dataset             financial.FinancialDataset `json:"dataset"`
	DatasetAvailability DatasetAvailability        `json:"dataset_availability"`

	// Mappings is the full per-account mapping review contract — see
	// AccountMappingResult.
	Mappings []AccountMappingResult `json:"mappings"`
	// Coverage summarizes Mappings — see MappingCoverage.
	Coverage MappingCoverage `json:"coverage"`

	// Reconciliation is the balance-sheet reconciliation outcome for every
	// period built — see reconcile.go. Empty when the balance sheet was
	// not built.
	Reconciliation []BalanceSheetReconciliation `json:"reconciliation,omitempty"`

	// Issues is every structured finding from every stage of this build:
	// mapping validation, statement construction, and reconciliation.
	Issues []Issue `json:"issues,omitempty"`
}

// Build is this package's single top-level entry point (task section 35):
// it resolves account balances from either convergent input path,
// resolves and validates account mappings, builds the requested
// statement(s), constructs the combined FinancialDataset, and reconciles
// the balance sheet. Build never mutates any part of input.
func Build(input Input, opts Options) Result {
	result := Result{
		SchemaVersion:               SchemaVersion,
		StatementFormulaVersion:     StatementFormulaVersion,
		MappingContractVersion:      MappingContractVersion,
		IncomeStatementAvailability: AvailabilityNotRequested,
		BalanceSheetAvailability:    AvailabilityNotRequested,
		DatasetAvailability:         AvailabilityNotRequested,
	}

	var issues []Issue

	if len(input.Periods) == 0 {
		issues = append(issues, Issue{
			Code:     IssueMissingPeriod,
			Severity: SeverityError,
			Message:  "no periods supplied",
		})
		result.Issues = issues
		result.DatasetAvailability = AvailabilityInvalid
		return result
	}

	chart, balances, sourceIssues := resolveAccountBalances(input, opts)
	issues = append(issues, sourceIssues...)

	currency, currencyIssue := resolveReportingCurrency(input.ReportingCurrency, chart, balances)
	if currencyIssue != nil {
		issues = append(issues, *currencyIssue)
	}

	mappingResults, mappingIssues := resolveMappings(chart, latestBalanceByAccount(balances), input.Mappings, opts.Mapping)
	issues = append(issues, mappingIssues...)

	coverage := computeCoverage(mappingResults)
	result.Mappings = mappingResults
	result.Coverage = coverage

	selection := input.Selection

	var incomeStmt, balanceStmt *Statement
	if selection.wantsIncome() {
		incomeStmt, result.IncomeStatementAvailability = buildIncomeStatement(chart, balances, mappingResults, input.Periods, opts)
		result.IncomeStatement = incomeStmt
	}
	if selection.wantsBalance() {
		balanceStmt, result.BalanceSheetAvailability = buildBalanceSheet(chart, balances, mappingResults, input.Periods, opts)
		result.BalanceSheet = balanceStmt
	}

	dataset, datasetIssues := buildDataset(chart, balances, mappingResults, currency, opts)
	issues = append(issues, datasetIssues...)
	result.Dataset = dataset

	unmappedIssues := evaluateUnmappedMateriality(mappingResults, dataset, opts)
	issues = append(issues, unmappedIssues...)

	if result.BalanceSheetAvailability == AvailabilityBuilt || result.BalanceSheetAvailability == AvailabilityBuiltWithWarnings {
		recon, reconIssues := reconcileBalanceSheet(dataset, input.Periods, opts)
		result.Reconciliation = recon
		issues = append(issues, reconIssues...)
	}

	result.DatasetAvailability = deriveDatasetAvailability(dataset, issues)
	result.Issues = dedupeAndSortIssues(issues)
	return result
}

// resolvedBalance is the internal, source-agnostic representation both
// convergent input paths (SourceLedger and SourceImportedTrialBalance)
// normalize into before mapping or statement construction — see the
// package doc comment's "two convergent input paths" section. This is the
// single point where the two paths become indistinguishable to every
// downstream function in this package.
type resolvedBalance struct {
	accountID      string
	period         financial.Period
	rawBalance     float64 // debit-positive/credit-negative, ledger's own convention
	currency       string
	sourceEntryIDs []string // provenance — journal entry IDs contributing (SourceLedger only)
}

// resolveAccountBalances runs the selected input path and returns a
// synthetic ledger.ChartOfAccounts (built from whichever chart the source
// supplies) plus every resolvedBalance for every requested period. Never
// mutates input.
func resolveAccountBalances(input Input, opts Options) (ledger.ChartOfAccounts, []resolvedBalance, []Issue) {
	switch input.Source {
	case SourceImportedTrialBalance:
		return resolveFromImportedTrialBalance(input, opts)
	default: // SourceLedger, and the zero value (SourceLedger is the more common/expected default)
		return resolveFromLedger(input, opts)
	}
}

// resolveFromLedger implements the SourceLedger path: for each requested
// period, filter input.Entries to that ledger.Period exactly (via
// ledger.PeriodRange.Periods, never a date range — see the task's
// explicit "caller supplies period selection" rule, applied literally
// rather than resolved into a date window this package would have to
// guess fiscal boundaries for) and compute ledger.CalculateBalances.
func resolveFromLedger(input Input, opts Options) (ledger.ChartOfAccounts, []resolvedBalance, []Issue) {
	chart := ledger.BuildChartOfAccounts(input.Chart)
	var issues []Issue

	balOpts := ledger.BalanceOptions{
		Openings:        input.OpeningBalances,
		IncludeStatuses: input.IncludeStatuses,
	}

	var out []resolvedBalance
	for _, period := range input.Periods {
		periodOpts := balOpts
		periodOpts.Range = ledger.PeriodRange{Periods: []string{string(period)}}

		balances := ledger.CalculateBalances(chart, input.Entries, periodOpts)
		for _, b := range balances {
			out = append(out, resolvedBalance{
				accountID:  b.AccountID,
				period:     period,
				rawBalance: b.RawBalance,
				currency:   b.Currency,
			})
		}
	}

	tb := ledger.BuildTrialBalance(chart, input.Entries, ledger.BalanceOptions{
		Openings:        input.OpeningBalances,
		IncludeStatuses: input.IncludeStatuses,
	}, ledger.ModeEndingBalances, opts.trialBalanceTolerance())
	if !tb.Balanced {
		issues = append(issues, Issue{
			Code:     IssueSourceTrialBalanceUnbalanced,
			Severity: SeverityError,
			Message:  "source ledger's overall trial balance does not balance within tolerance",
		})
	}

	return chart, out, issues
}

// resolveFromImportedTrialBalance implements the SourceImportedTrialBalance
// path: for each requested period, look up the matching
// ledger.NormalizedTrialBalance and translate its lines directly into
// resolvedBalance (RawBalance) — no journal replay involved.
func resolveFromImportedTrialBalance(input Input, opts Options) (ledger.ChartOfAccounts, []resolvedBalance, []Issue) {
	chart := ledger.BuildChartOfAccounts(input.ImportedChart)
	var issues []Issue
	var out []resolvedBalance

	for _, period := range input.Periods {
		tb, ok := input.ImportedTrialBalances[period]
		if !ok {
			issues = append(issues, Issue{
				Code:     IssueMissingPeriod,
				Severity: SeverityError,
				Message:  "no imported trial balance supplied for period " + string(period),
				Period:   string(period),
			})
			continue
		}
		if !tb.Balanced {
			issues = append(issues, Issue{
				Code:     IssueSourceTrialBalanceUnbalanced,
				Severity: SeverityError,
				Message:  "imported trial balance for period " + string(period) + " does not balance within its own tolerance",
				Period:   string(period),
			})
		}
		for _, line := range tb.Lines {
			out = append(out, resolvedBalance{
				accountID:  line.AccountID,
				period:     period,
				rawBalance: line.RawBalance,
				currency:   line.Currency,
			})
		}
	}

	return chart, out, issues
}

// latestBalanceByAccount collapses resolvedBalance into one entry per
// account for AccountMappingResult.CurrentBalance/Currency display: the
// balance from the LAST period in balances' own (period-ascending, since
// resolveAccountBalances iterates input.Periods in caller order) input
// order. Used only for display purposes on the mapping-review contract —
// statement/dataset construction itself always uses every period's own
// resolvedBalance independently, never this collapsed view.
func latestBalanceByAccount(balances []resolvedBalance) map[string]resolvedBalance {
	out := make(map[string]resolvedBalance, len(balances))
	for _, b := range balances {
		out[b.accountID] = b // last write wins == last period in input order
	}
	return out
}

// resolveReportingCurrency determines the single currency a build's
// FinancialDataset is denominated in: input.ReportingCurrency if
// supplied, otherwise the shared currency across every account, if there
// is exactly one. Never fetches FX; returns an IssueMixedCurrency issue
// (not an error) if accounts disagree and no ReportingCurrency was
// supplied — see the package doc comment's currency-boundary note.
func resolveReportingCurrency(explicit string, chart ledger.ChartOfAccounts, balances []resolvedBalance) (string, *Issue) {
	if explicit != "" {
		return explicit, nil
	}

	seen := make(map[string]bool)
	for _, id := range chart.IDs() {
		acct, _ := chart.Lookup(id)
		if acct.Currency != "" {
			seen[acct.Currency] = true
		}
	}
	for _, b := range balances {
		if b.currency != "" {
			seen[b.currency] = true
		}
	}

	if len(seen) == 0 {
		return "", nil
	}
	if len(seen) == 1 {
		for c := range seen {
			return c, nil
		}
	}

	currencies := make([]string, 0, len(seen))
	for c := range seen {
		currencies = append(currencies, c)
	}
	sort.Strings(currencies)

	return "", &Issue{
		Code:     IssueMixedCurrency,
		Severity: SeverityError,
		Message:  "accounts use more than one currency (" + strings.Join(currencies, ", ") + ") and no ReportingCurrency was supplied",
	}
}
