// Package statements builds canonical financial statements — and, most
// importantly, a financial.FinancialDataset — from accounting/ledger data,
// by mapping a chart of accounts to the financial.Code taxonomy and
// aggregating balances into presentation-neutral statement models. It is
// the deterministic bridge described in accounting/ledger's own package
// doc comment:
//
//	accounting/ledger
//	      ↓
//	account mapping
//	      ↓
//	financial statement builder   (this package)
//	      ↓
//	financial.FinancialDataset
//
// This package depends on accounting/ledger, financial, and
// financial/classification (for optional deterministic mapping
// suggestions only — see mapping.go). accounting/ledger never depends on
// this package or on financial: the dependency direction is strictly
// ledger -> statements -> financial, exactly as accounting/ledger's own
// doc comment states.
//
// Like every other package in this repository, it contains no
// persistence, HTTP/API, auth, UI, background jobs, QuickBooks/Xero
// integration, AI/LLM, tax logic, external benchmark data, or OCR. It
// consumes and produces plain Go structs that are JSON-compatible, and
// every exported function is pure: no I/O, no mutation of caller-owned
// input, no package-global mutable state — see immutability_test.go and
// determinism_test.go.
//
// # Two convergent input paths
//
// Build accepts EITHER a ledger-derived source (an accounting/ledger
// chart of accounts plus journal entries or a pre-built trial balance) OR
// an imported-trial-balance source (accounting/ledger's own normalized
// TrialBalance/NormalizedTrialBalance, with no journal detail at all) —
// see Input and resolveAccountBalances in builder.go. Both paths converge
// on the exact same internal balance representation before mapping or
// statement construction begins, so there is only one mapping/calculation
// pipeline in this package, never two that could silently drift apart.
//
// # What this package does not do
//
// It does not post to the ledger, modify retained earnings, infer
// one-to-many account splits, fetch FX rates, consolidate multiple
// entities, guess a fiscal calendar, or accept AI-proposed mappings — see
// each relevant file's doc comment for the specific boundary and the
// repository README's "What this project intentionally does not contain"
// section, which this package follows exactly.
package statements

import "github.com/themurtez/go-valuate/financial"

// MappingSource identifies how an AccountMapping's FinancialCode was
// determined. Mirrors the precedence classification.Source already
// establishes for row-level classification, at the account level.
type MappingSource string

const (
	// MappingSourceExplicit means a caller supplied this mapping directly
	// (via Input.Mappings or a resolved MappingTemplate rule) and it takes
	// precedence over any deterministic suggestion — see mapping.go's
	// "explicit mappings are authoritative" rule.
	MappingSourceExplicit MappingSource = "EXPLICIT"
	// MappingSourceDeterministicSuggestion means Build's optional
	// deterministic suggestion mechanism (MappingModeSuggestDeterministic)
	// proposed this mapping by adapting the account into a
	// financial.RawLineItem and running it through
	// financial/classification.Classify. A suggestion is never treated as
	// confirmed — see AccountMappingResult.Confirmed.
	MappingSourceDeterministicSuggestion MappingSource = "DETERMINISTIC_SUGGESTION"
	// MappingSourceUnmapped means no explicit mapping was supplied and
	// either suggestions were not requested (MappingModeExplicitOnly, the
	// default) or the suggestion mechanism could not justify one
	// (classification.SourceUnknown, or the row was excluded outright —
	// see resolveOneMapping in mapping.go).
	MappingSourceUnmapped MappingSource = "UNMAPPED"
)

// SignTreatment controls how a mapped account's raw ledger balance is
// converted into the canonical financial.NormalizedItem amount stored in
// the FinancialDataset — see signs.go for the exact transformation.
//
// financial's existing sign convention (documented in
// financial/metrics/income_statement.go and used by
// financial/reconciliation's contra-asset handling for
// financial.CodeBsAccumDepreciation) stores every canonical amount as a
// positive "as reported" magnitude: a revenue code holds how much
// revenue there was, an expense code holds how much was spent, and a
// contra-asset code (accumulated depreciation) ALSO holds a positive
// magnitude, with downstream formulas explicitly subtracting it rather
// than expecting it pre-negated. SignTreatment is this package's
// mechanism for reaching that same positive-magnitude convention
// starting from a raw ledger balance, which is signed debit-positive/
// credit-negative (see ledger's own package doc comment).
type SignTreatment string

const (
	// SignNatural (the default when a mapping does not set this field)
	// converts the raw ledger balance using the mapped account's own
	// ledger.AccountType via ledger.NormalBalance: an account in its
	// normal position (e.g. a REVENUE account with a credit balance)
	// produces a positive canonical amount, exactly like
	// ledger.Balance.DisplayBalance. This is correct for the overwhelming
	// majority of accounts, which are not contra accounts — see signs.go.
	SignNatural SignTreatment = "NATURAL"
	// SignNormal treats the raw balance as already representing the
	// canonical magnitude directly (RawBalance, unmodified) — for a caller
	// who has a specific reason to bypass the account-type-driven
	// NATURAL flip (rare; NATURAL already produces the same result as
	// NORMAL for every non-contra account).
	SignNormal SignTreatment = "NORMAL"
	// SignInvert flips SignNatural's result — the mechanism for a contra
	// account. Example: "Accumulated Depreciation" is a ledger ASSET
	// account (so NormalBalance is DEBIT), but its accounts normally carry
	// a CREDIT balance; SignInvert on a mapping to
	// financial.CodeBsAccumDepreciation converts that natural-for-a-
	// contra-credit balance into the positive magnitude
	// financial/reconciliation and financial/metrics already expect for
	// that code.
	SignInvert SignTreatment = "INVERT"
)

// resolvedSignTreatment returns t if it is a recognized, non-empty value,
// otherwise SignNatural — the documented default.
func resolvedSignTreatment(t SignTreatment) SignTreatment {
	switch t {
	case SignNormal, SignInvert, SignNatural:
		return t
	default:
		return SignNatural
	}
}

// AllocationRule is one caller-supplied split of a single ledger account
// into a fraction of a canonical financial.Code — see mapping.go's
// one-to-many section. AllocationRule only exists inside
// AccountMapping.Allocations; a mapping with no Allocations uses its own
// single FinancialCode/StatementType/SignTreatment for 100% of the
// account's balance (the V1 default one-account-to-one-code rule).
type AllocationRule struct {
	// FinancialCode is the canonical code this fraction of the account's
	// balance maps to.
	FinancialCode financial.Code `json:"financial_code"`
	// StatementType is the statement this fraction belongs to.
	StatementType financial.StatementType `json:"statement_type"`
	// Percent is this allocation's fraction of the account's balance, in
	// [0, 1] (e.g. 0.70 for 70%). Every AccountMapping.Allocations slice's
	// Percent values must sum to 1.0 within a small fixed tolerance — see
	// validateAllocations in validate.go. Never inferred: a caller who
	// wants a one-to-many split must supply every fraction explicitly.
	Percent float64 `json:"percent"`
	// SignTreatment controls this allocation's own sign conversion,
	// independent of any other allocation on the same account. Zero value
	// resolves to SignNatural, exactly like AccountMapping.SignTreatment.
	SignTreatment SignTreatment `json:"sign_treatment,omitempty"`
}

// AccountMapping is a portable, caller-owned (or template-resolved)
// instruction for how one ledger account's balance contributes to the
// canonical financial.FinancialDataset. AccountMapping is plain domain
// data: it carries no persistence identity of its own (no mapping ID,
// no client ID, no storage timestamp) — a future application layer is
// expected to add those around this type, not into it, per the task's
// explicit "no persistence" architectural rule.
type AccountMapping struct {
	// AccountID is the ledger.Account.ID this mapping applies to.
	// Required.
	AccountID string `json:"account_id"`
	// FinancialCode is the canonical taxonomy code this account maps to.
	// Required unless Allocations is non-empty (one-to-many mode — see
	// AllocationRule), in which case FinancialCode/StatementType/
	// SignTreatment on the mapping itself are ignored in favor of each
	// AllocationRule's own fields.
	FinancialCode financial.Code `json:"financial_code,omitempty"`
	// StatementType is the statement FinancialCode belongs to. Required
	// alongside FinancialCode (ignored when Allocations is set). Kept as
	// its own field, rather than derived purely from FinancialCode, so
	// mapValidation can flag a caller-supplied mismatch (see
	// IssueInvalidMapping) instead of silently trusting the taxonomy
	// lookup.
	StatementType financial.StatementType `json:"statement_type,omitempty"`
	// SignTreatment controls the raw-balance-to-canonical-amount sign
	// conversion for this mapping. Zero value resolves to SignNatural.
	// Ignored when Allocations is set (each AllocationRule carries its
	// own).
	SignTreatment SignTreatment `json:"sign_treatment,omitempty"`
	// Allocations, when non-empty, splits this account's balance across
	// multiple canonical codes per the task's explicit one-to-many policy:
	// only supported with caller-supplied percentages summing to 1.0.
	// Empty (the V1 default) means this account maps to exactly one code
	// (FinancialCode/StatementType/SignTreatment above).
	Allocations []AllocationRule `json:"allocations,omitempty"`
	// Source identifies how this mapping was determined. Required;
	// MappingSourceUnmapped is the only value Build itself ever assigns
	// when it cannot resolve a mapping — a caller constructing an
	// AccountMapping by hand should set MappingSourceExplicit.
	Source MappingSource `json:"source"`
	// Confirmed is true when a human/caller has explicitly accepted this
	// mapping, as opposed to it merely being a proposal. A caller-supplied
	// explicit mapping (Source == MappingSourceExplicit) is always treated
	// as authoritative regardless of Confirmed — see mapping.go — but
	// Confirmed is preserved as a separate signal for a future review UI
	// distinguishing "I typed this in and it's final" from "I accepted a
	// suggestion." Build sets Confirmed = false on every
	// DETERMINISTIC_SUGGESTION mapping it produces itself; a caller who
	// wants a suggestion promoted to fully confirmed does so explicitly
	// (see the mapping-review integration test).
	Confirmed bool `json:"confirmed"`
	// ClassificationReason carries classification.Result.Reason verbatim
	// when Source == MappingSourceDeterministicSuggestion, for review-UI
	// display. Empty for explicit/unmapped mappings.
	ClassificationReason string `json:"classification_reason,omitempty"`
	// ClassificationConfidence carries classification.Result.Confidence
	// verbatim when Source == MappingSourceDeterministicSuggestion.
	ClassificationConfidence float64 `json:"classification_confidence,omitempty"`
}

// isAllocated reports whether m uses one-to-many allocation mode.
func (m AccountMapping) isAllocated() bool {
	return len(m.Allocations) > 0
}

// MappingMode controls whether Build attempts deterministic mapping
// suggestions for accounts the caller did not explicitly map — see
// mapping.go.
type MappingMode string

const (
	// MappingExplicitOnly (the default, used when the zero value is
	// supplied) never proposes a mapping for an account the caller did
	// not explicitly map: every such account's AccountMappingResult has
	// Source == MappingSourceUnmapped. This is the safe default per the
	// task's explicit "default behavior should remain safe" instruction.
	MappingExplicitOnly MappingMode = "MAPPING_EXPLICIT_ONLY"
	// MappingSuggestDeterministic additionally runs the deterministic
	// suggestion mechanism (adapting each unmapped account into a
	// financial.RawLineItem and calling
	// financial/classification.Classify) for every account without an
	// explicit mapping — see suggestMapping in mapping.go. A suggestion
	// never counts as Confirmed and never silently becomes part of the
	// FinancialDataset; it is exposed on AccountMappingResult for a future
	// caller to confirm (see the mapping-review integration test).
	MappingSuggestDeterministic MappingMode = "MAPPING_SUGGEST_DETERMINISTIC"
)

// resolvedMappingMode returns m if recognized, otherwise
// MappingExplicitOnly.
func resolvedMappingMode(m MappingMode) MappingMode {
	if m == MappingSuggestDeterministic {
		return m
	}
	return MappingExplicitOnly
}

// UnmappedPolicy controls how Build treats a material unmapped account
// when building the final FinancialDataset — see issues.go's materiality
// section and IssueUnmappedAccount.
type UnmappedPolicy string

const (
	// PolicyAllowUnmappedWithWarning (the default) still builds whatever
	// FinancialDataset it can from mapped accounts, but flags every
	// unmapped account as a warning-severity issue and marks
	// Result.DatasetAvailability as StatementBuiltWithWarnings rather than
	// StatementBuiltSuccessfully when at least one unmapped account is
	// material (see MaterialityPolicy).
	PolicyAllowUnmappedWithWarning UnmappedPolicy = "ALLOW_UNMAPPED_WITH_WARNING"
	// PolicyFailOnUnmappedMaterial refuses to produce a FinancialDataset
	// (Result.DatasetAvailability = StatementInvalid) if any material
	// unmapped account exists — see IssueUnmappedAccount's Severity under
	// this policy.
	PolicyFailOnUnmappedMaterial UnmappedPolicy = "FAIL_ON_UNMAPPED_MATERIAL"
)

// resolvedUnmappedPolicy returns p if recognized, otherwise
// PolicyAllowUnmappedWithWarning.
func resolvedUnmappedPolicy(p UnmappedPolicy) UnmappedPolicy {
	if p == PolicyFailOnUnmappedMaterial {
		return p
	}
	return PolicyAllowUnmappedWithWarning
}

// HierarchyPolicy controls how Build treats an account that has both
// direct postings and child accounts in the chart hierarchy — see
// hierarchy hint in builder.go's leaf-posting section.
type HierarchyPolicy string

const (
	// HierarchyLeafOnly (the default) is the only mode this package
	// implements in V1: a parent account's own direct balance is included
	// in the statement exactly like any other mapped account, and each
	// child's balance is separately included via its own mapping — no
	// automatic rollup aggregation is performed by this package (that is
	// accounting/ledger.BuildRollups' job, one layer down, and mixing the
	// two would double count — see the task's explicit "never aggregate
	// both rolled-up parent total and child leaf totals" rule). A caller
	// wanting a parent's rolled-up total to be the ONLY figure in the
	// statement should map only the parent and leave every child account
	// unmapped (or explicitly excluded), never map both to the same code.
	HierarchyLeafOnly HierarchyPolicy = "HIERARCHY_LEAF_ONLY"
)
