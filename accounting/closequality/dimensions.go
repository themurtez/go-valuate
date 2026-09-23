package closequality

// Dimension is a fixed close-quality assessment axis. Not every dimension
// is assessable for every Input — a dimension whose required upstream
// module is absent (and not required by Policy) reports
// [DimensionUnassessed] rather than being silently omitted, so a caller
// can always see the full fixed list and why any dimension is missing.
type Dimension string

const (
	DimensionLedgerIntegrity             Dimension = "LEDGER_INTEGRITY"
	DimensionTrialBalanceIntegrity       Dimension = "TRIAL_BALANCE_INTEGRITY"
	DimensionFinancialStatementIntegrity Dimension = "FINANCIAL_STATEMENT_INTEGRITY"
	DimensionARControl                   Dimension = "AR_CONTROL"
	DimensionAPControl                   Dimension = "AP_CONTROL"
	DimensionJournalReview               Dimension = "JOURNAL_REVIEW"
	DimensionBalanceSheetQuality         Dimension = "BALANCE_SHEET_QUALITY"
	DimensionAccountBalancePlausibility  Dimension = "ACCOUNT_BALANCE_PLAUSIBILITY"
	DimensionReconciliationCoverage      Dimension = "RECONCILIATION_COVERAGE"
	DimensionCloseTaskCompletion         Dimension = "CLOSE_TASK_COMPLETION"
	DimensionDataCompleteness            Dimension = "DATA_COMPLETENESS"
	DimensionPeriodLockPostCloseActivity Dimension = "PERIOD_LOCK_POST_CLOSE_ACTIVITY"
)

// dimensionOrder is the fixed, documented evaluation and output order for
// every dimension-keyed slice/map in Result — never Go map order.
var dimensionOrder = []Dimension{
	DimensionLedgerIntegrity,
	DimensionTrialBalanceIntegrity,
	DimensionFinancialStatementIntegrity,
	DimensionARControl,
	DimensionAPControl,
	DimensionJournalReview,
	DimensionBalanceSheetQuality,
	DimensionAccountBalancePlausibility,
	DimensionReconciliationCoverage,
	DimensionCloseTaskCompletion,
	DimensionDataCompleteness,
	DimensionPeriodLockPostCloseActivity,
}

var dimensionRank = func() map[Dimension]int {
	m := make(map[Dimension]int, len(dimensionOrder))
	for i, d := range dimensionOrder {
		m[d] = i
	}
	return m
}()

// DimensionStatus is one dimension's own rollup status, independent of
// the overall Result.Status (which additionally folds in cross-dimension
// policy such as required-input-missing handling).
type DimensionStatus string

const (
	DimensionPass       DimensionStatus = "PASS"
	DimensionWarning    DimensionStatus = "WARNING"
	DimensionBlocking   DimensionStatus = "BLOCKING"
	DimensionUnassessed DimensionStatus = "UNASSESSED"
)

// DimensionResult is one dimension's assessment: its own status, the
// Findings that drove it (already also present in Result.Blockers/
// Warnings/Information), a coverage note, and free-form typed evidence
// strings explaining the status when no Finding fully captures it (e.g.
// "no findings, ledger validation clean").
type DimensionResult struct {
	Dimension Dimension       `json:"dimension"`
	Status    DimensionStatus `json:"status"`
	// Assessed is false when Status == DimensionUnassessed because the
	// dimension's required upstream input was not supplied.
	Assessed bool `json:"assessed"`
	// FindingCodes lists, in Result-ordering order, the Codes of every
	// Finding attributed to this dimension (cross-reference into
	// Result.Blockers/Warnings/Information rather than duplicating full
	// Finding bodies here).
	FindingCodes []FindingCode `json:"finding_codes,omitempty"`
	// Evidence holds short factual notes about this dimension's
	// assessment that are not already captured by a Finding (e.g. "AR
	// aging reconciliation balanced: difference 0.00").
	Evidence []string `json:"evidence,omitempty"`
	// UnassessedReason explains, in neutral factual language, why this
	// dimension is DimensionUnassessed (e.g. "AR result not supplied").
	UnassessedReason string `json:"unassessed_reason,omitempty"`
}

// dimensionBuilder accumulates one dimension's findings/evidence as
// mine* functions run, then finalizes into a DimensionResult.
type dimensionBuilder struct {
	dimension Dimension
	assessed  bool
	reason    string
	evidence  []string
	codes     []FindingCode
	worst     DimensionStatus // "" until first finding recorded
}

func newDimensionBuilder(d Dimension) *dimensionBuilder {
	return &dimensionBuilder{dimension: d}
}

// unavailable marks the dimension unassessed for the given reason. It is
// a no-op if the dimension has already been marked assessed.
func (b *dimensionBuilder) unavailable(reason string) {
	if b.assessed {
		return
	}
	b.reason = reason
}

// markAssessed records that this dimension's required input was present,
// so it will report PASS (if no findings) or the worst finding severity.
func (b *dimensionBuilder) markAssessed() {
	b.assessed = true
}

func (b *dimensionBuilder) note(evidence string) {
	b.evidence = append(b.evidence, evidence)
}

func (b *dimensionBuilder) record(f Finding) {
	b.codes = append(b.codes, f.Code)
	status := severityToDimensionStatus(f.Severity)
	if b.worst == "" || dimensionStatusRank[status] < dimensionStatusRank[b.worst] {
		b.worst = status
	}
}

var dimensionStatusRank = map[DimensionStatus]int{
	DimensionBlocking:   0,
	DimensionWarning:    1,
	DimensionPass:       2,
	DimensionUnassessed: 3,
}

func severityToDimensionStatus(s Severity) DimensionStatus {
	switch s {
	case SeverityBlocking:
		return DimensionBlocking
	case SeverityWarning:
		return DimensionWarning
	default:
		return DimensionPass
	}
}

func (b *dimensionBuilder) build() DimensionResult {
	r := DimensionResult{
		Dimension:        b.dimension,
		Assessed:         b.assessed,
		FindingCodes:     b.codes,
		Evidence:         b.evidence,
		UnassessedReason: b.reason,
	}
	switch {
	case !b.assessed:
		r.Status = DimensionUnassessed
	case b.worst != "":
		r.Status = b.worst
	default:
		r.Status = DimensionPass
	}
	return r
}
