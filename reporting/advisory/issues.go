package advisory

// IssueSeverity distinguishes an input problem [Build] could not proceed
// past for some portion of the pack (IssueSeverityError) from one that is
// advisory only (IssueSeverityWarning) — this package's own two-severity
// model, matching every sibling package's identical convention (e.g.
// reporting/management.IssueSeverity).
type IssueSeverity string

const (
	IssueSeverityError   IssueSeverity = "error"
	IssueSeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Build-time input
// problem — task section 94. Only codes this package actually emits are
// defined (task section 94's "only define codes emitted" instruction).
type IssueCode string

const (
	// IssueInvalidInput means a caller-supplied Input field was
	// structurally invalid (e.g. a negative MaxExecutiveHighlights) in a
	// way Build could not proceed past for the affected section.
	IssueInvalidInput IssueCode = "INVALID_INPUT"
	// IssueInvalidPeriod means an Input.Periods entry was missing its
	// required Code, or two entries shared the same Code.
	IssueInvalidPeriod IssueCode = "INVALID_PERIOD"
	// IssuePeriodMismatch means two or more supplied source Results
	// represent different current periods — task section 88. Advisory
	// only; the affected sections still build, each labeled with its own
	// period.
	IssuePeriodMismatch IssueCode = "PERIOD_MISMATCH"
	// IssueAsOfDateMismatch means two or more point-in-time inputs (cash,
	// AR, AP, inventory, reconciliation, close) disagree on AsOfDate beyond
	// what Policy.RequireAlignedAsOfDates allows — task section 89.
	IssueAsOfDateMismatch IssueCode = "AS_OF_DATE_MISMATCH"
	// IssueCurrencyMismatch means two or more supplied monetary source
	// values report different currencies with no FX conversion available
	// — task section 90.
	IssueCurrencyMismatch IssueCode = "CURRENCY_MISMATCH"
	// IssueDuplicateSourceMetric means the same (SourceModule, SourceCode)
	// pair was read more than once while building one Section — a
	// programming-error guard, never expected in ordinary use.
	IssueDuplicateSourceMetric IssueCode = "DUPLICATE_SOURCE_METRIC"
	// IssueSourceConflict means two authoritative candidate values for the
	// same metric code disagreed beyond Policy.ConflictTolerance with no
	// SourcePreference resolving it — task section 19/46/59.
	IssueSourceConflict IssueCode = "SOURCE_CONFLICT"
	// IssueInvalidSourcePreference means a Policy.SourcePreferences entry
	// named an unrecognized module in OrderedSources, or had an empty
	// MetricCode.
	IssueInvalidSourcePreference IssueCode = "INVALID_SOURCE_PREFERENCE"
	// IssueInvalidPolicy means Policy itself was structurally invalid (a
	// negative threshold, an empty CategoryOrder entry).
	IssueInvalidPolicy IssueCode = "INVALID_POLICY"
	// IssueInvalidPriorityRule means a Policy.PriorityRules entry had an
	// empty ForcePriority or every Match field empty (matching nothing
	// specific — see PriorityRule.matches).
	IssueInvalidPriorityRule IssueCode = "INVALID_PRIORITY_RULE"
	// IssueInvalidActionOverride means a caller-supplied ActionItem
	// (ActionOriginCallerSupplied) had an empty ActionCode.
	IssueInvalidActionOverride IssueCode = "INVALID_ACTION_OVERRIDE"
	// IssueUnknownActionStatus means a caller-supplied ActionItem.Status
	// was not one of the defined ActionStatus values.
	IssueUnknownActionStatus IssueCode = "UNKNOWN_ACTION_STATUS"
	// IssueInvalidPriorResult means Input.Prior was supplied but its own
	// shape could not be reconciled against the current Result (e.g. an
	// empty PackVersions.SchemaVersion) — the prior-comparison sections are
	// then unavailable rather than built against an unusable baseline.
	IssueInvalidPriorResult IssueCode = "INVALID_PRIOR_RESULT"
	// IssueUnsupportedSourceVersion means a supplied source Result's own
	// version constant is empty or otherwise could not be echoed into
	// SourceVersions — advisory only, since this package never refuses to
	// compose a fact merely because its source's version string is
	// missing.
	IssueUnsupportedSourceVersion IssueCode = "UNSUPPORTED_SOURCE_VERSION"
)

// Issue is a single Build-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`

	// SourceModule/SourceCode/Period/EntityRef identify which fact (if any)
	// this Issue concerns, using the same provenance shape as every other
	// composed fact — never a bare string blob a caller has to parse.
	SourceModule string `json:"source_module,omitempty"`
	SourceCode   string `json:"source_code,omitempty"`
	Period       string `json:"period,omitempty"`
	EntityRef    string `json:"entity_ref,omitempty"`
}

// HasErrors reports whether any Issue in issues has IssueSeverityError.
//
// Intentionally duplicated from every sibling package's identical
// HasErrors rather than shared — see
// transactions/salereadiness.HasErrors's doc comment for the full
// rationale (each package's Issue is a distinct Go type with no common
// interface worth introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == IssueSeverityError {
			return true
		}
	}
	return false
}
