package review

import (
	"fmt"
	"math"
	"reflect"
	"sort"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

// Source bundles every corrected-data input Apply might need to produce
// its output, so Apply's signature stays a small typed struct rather than
// several loose parameters or an `any`. Apply never mutates any field of
// Source — see the immutability guarantees documented on Apply itself.
type Source struct {
	// MappedLineItems is the classification-stage output Apply corrects
	// Code/Status on, in response to KindClassification/KindStructure
	// decisions. Matched to a decision's originating ReviewItem via
	// MappedLineItem.SourceID (== the row ID review items are keyed by).
	MappedLineItems []financial.MappedLineItem
	// Rows supplies the same OCR row/cell context Build consumed (see
	// BuildInput.Rows), so a KindOCRNumeric decision's override/accept/
	// reject can be resolved back to a concrete cell.
	Rows []RowContext
	// Adjustments is the adjustment set Apply corrects Included/Amount/
	// Reason on, in response to KindAdjustment decisions. Matched via
	// AdjustmentDecision's targeted item's AdjustmentPayload.AdjustmentID.
	Adjustments []adjustments.Adjustment
}

// AppliedDecision is one Decision that was successfully applied, carrying
// everything needed to answer, later, "what did the system originally
// propose, what did the user change, and what value was ultimately used" —
// see the package doc comment's audit/explainability goal. Deliberately
// carries no user ID or timestamp: this is a standalone deterministic
// library with no concept of who or when, by design (a future application
// layer owns that).
type AppliedDecision struct {
	// ItemID is the ReviewItem.ID this decision resolved.
	ItemID string `json:"item_id"`
	// Kind is the resolved item's Kind, copied for convenience so a caller
	// doesn't need to cross-reference the original Plan just to know what
	// kind of decision this was.
	Kind Kind `json:"kind"`
	// Action is the Decision.Action that was applied.
	Action Action `json:"action"`
	// OriginalProposal is a short display string for what Build originally
	// proposed (ReviewItem.ProposedValue at Build time).
	OriginalProposal string `json:"original_proposal,omitempty"`
	// FinalValue is a short display string for the value actually used
	// after applying this decision (e.g. the accepted code, the overridden
	// amount, the confirmed RowKind).
	FinalValue string `json:"final_value,omitempty"`
	// Reason is a deterministic, structured explanation of why this item
	// needed review in the first place (copied from ReviewItem.Reason at
	// Build time) — not why the user chose what they chose, which this
	// package has no visibility into beyond Action/payload.
	Reason string `json:"reason,omitempty"`
}

// InvalidDecision is one Decision Apply rejected, plus the Issue(s)
// explaining why.
type InvalidDecision struct {
	Decision Decision `json:"decision"`
	Issues   []Issue  `json:"issues"`
}

// ApplyResult is the output of Apply: which decisions were applied, which
// were rejected, which required items remain unresolved, and the resulting
// corrected domain structures.
type ApplyResult struct {
	// Applied is every Decision that was successfully applied, in Apply's
	// deterministic DOMAIN application order, not necessarily Decisions'
	// original slice order: every KindStructure decision is processed (and
	// so appears here) before every other kind, since a row's structural
	// RowKind determines whether a KindClassification decision against the
	// same row is even valid (see decisionPhase). Decisions within the same
	// phase preserve their original relative order. Each AppliedDecision
	// still carries its own ItemID/Kind, so original decision identity is
	// never lost even though array position no longer mirrors input order.
	Applied []AppliedDecision `json:"applied,omitempty"`
	// Invalid is every Decision that failed validation and was not
	// applied, in the same deterministic domain application order Applied
	// uses (see Applied's doc comment) — NOT necessarily Decisions'
	// original slice order.
	Invalid []InvalidDecision `json:"invalid,omitempty"`
	// UnresolvedRequired is every ReviewItem from the plan that has
	// Required == true and remains unresolved after applying every valid
	// decision, in Plan.Items' original order.
	UnresolvedRequired []ReviewItem `json:"unresolved_required,omitempty"`
	// Warnings carries non-fatal Issues (SeverityIssueWarning) surfaced
	// while applying otherwise-valid decisions.
	Warnings []Issue `json:"warnings,omitempty"`
	// Items is the corrected view of the plan's items: the same items
	// Build produced, with Status updated to reflect every applied
	// decision (StatusResolved/StatusRejected) or invalid decision
	// (StatusInvalidDecision), in the same deterministic order Plan.Items
	// used.
	Items []ReviewItem `json:"items"`
	// Summary aggregates Items post-decisions, using the exact same
	// Summary type Plan.Summary uses — see Summary's doc comment.
	Summary Summary `json:"summary"`

	// MappedLineItems is Source.MappedLineItems with every accepted
	// classification/structure decision applied (Code/Status corrected;
	// an ignored row's Status set to financial.RowStatusIgnored). A fresh
	// slice/copy — Source.MappedLineItems is never mutated in place.
	MappedLineItems []financial.MappedLineItem `json:"mapped_line_items,omitempty"`
	// Adjustments is Source.Adjustments with every accepted adjustment
	// decision applied (Included/Amount/Reason corrected). A fresh
	// slice/copy — Source.Adjustments is never mutated in place.
	Adjustments []adjustments.Adjustment `json:"adjustments,omitempty"`
	// CorrectedNumerics maps a KindOCRNumeric ReviewItem.ID to the final
	// numeric value Apply resolved for it (accepted parsed value, or an
	// override), for a caller that needs to re-run normalization/
	// classification with corrected cell values. A rejected numeric item
	// (Action == ActionReject) is intentionally absent from this map — its
	// row should be treated as having no usable value for that period,
	// exactly like a cell that never parsed at all.
	CorrectedNumerics map[string]float64 `json:"corrected_numerics,omitempty"`
	// CorrectedPeriods maps a KindPeriod ReviewItem.ID to the final
	// financial.Period Apply resolved for it (accepted proposed period, or
	// an override).
	CorrectedPeriods map[string]financial.Period `json:"corrected_periods,omitempty"`
}

// Apply deterministically applies decisions to plan/source and returns the
// resulting ApplyResult. Apply never mutates source, plan, or any element
// of decisions — see apply_immutability_test.go for explicit deep-copy-based
// proof of this for every field Apply reads.
//
// Apply does not return a Go error: an empty decisions slice is not an
// error (nothing gets applied and every required item stays unresolved —
// UnresolvedRequired reflects that), and every per-decision structural
// problem (unknown item ID, wrong payload kind, invalid code, ...) is a
// DECISION validation issue reported via ApplyResult.Invalid, not an Apply-
// call failure. There is no contract violation here with "no sensible
// partial result" severe enough to warrant a real error return, matching
// this repository's dominant Result.Available-plus-issues convention.
func Apply(source Source, plan Plan, decisions []Decision) ApplyResult {
	itemByID := make(map[string]ReviewItem, len(plan.Items))
	for _, it := range plan.Items {
		itemByID[it.ID] = it
	}

	// Deep-copy decisions so per-decision mutation-free validation can
	// freely read them without any risk of a caller-visible alias back to
	// the caller's own slice/pointers.
	decs := cloneDecisions(decisions)

	seen := make(map[string][]int, len(decs))
	for i, d := range decs {
		seen[d.ItemID] = append(seen[d.ItemID], i)
	}

	result := ApplyResult{
		MappedLineItems:   cloneMappedLineItems(source.MappedLineItems),
		Adjustments:       cloneAdjustments(source.Adjustments),
		CorrectedNumerics: make(map[string]float64),
		CorrectedPeriods:  make(map[string]financial.Period),
	}

	statusByID := make(map[string]Status, len(plan.Items))
	for _, it := range plan.Items {
		statusByID[it.ID] = it.Status
	}

	mappedIndex := indexMappedLineItemsBySourceID(result.MappedLineItems)
	adjustmentIndex := indexAdjustmentsByID(result.Adjustments)

	// Process decisions in deterministic DOMAIN order (see
	// decisionApplicationOrder) rather than caller slice order: a
	// KindStructure decision against a row must be validated/applied before
	// a KindClassification decision against the same row, since the row's
	// structural RowKind determines whether the classification decision is
	// even valid (see validateClassificationDecision). order[k] is the
	// index into decs (== the caller's original decisions slice position,
	// after cloning) to process k-th; every seen/duplicate lookup below
	// still keys off that ORIGINAL index, so duplicate/conflict detection
	// (always between decisions sharing one ItemID, hence one Kind, hence
	// one phase) is completely unaffected by this reordering.
	order := decisionApplicationOrder(decs, itemByID)

	for _, i := range order {
		d := decs[i]
		item, itemOK := itemByID[d.ItemID]

		var issues []Issue
		if dupIdxs := seen[d.ItemID]; len(dupIdxs) > 1 && !isDuplicateGroupIdentical(decs, dupIdxs) {
			issues = append(issues, Issue{ItemID: d.ItemID, Code: IssueConflictingDecision, Severity: SeverityIssueError,
				Message: fmt.Sprintf("item %q has %d conflicting decisions in this Apply call", d.ItemID, len(dupIdxs))})
		} else if len(dupIdxs) > 1 && dupIdxs[0] != i {
			// An exact duplicate repeat: only the first occurrence (by
			// ORIGINAL slice position, not processing order) applies; later
			// identical repeats are reported as duplicates too, for
			// visibility, but are not independently re-applied.
			issues = append(issues, Issue{ItemID: d.ItemID, Code: IssueDuplicateItemID, Severity: SeverityIssueWarning,
				Message: fmt.Sprintf("item %q has %d identical repeated decisions in this Apply call", d.ItemID, len(dupIdxs))})
		}

		if !itemOK {
			issues = append(issues, Issue{ItemID: d.ItemID, Code: IssueUnknownItemID, Severity: SeverityIssueError,
				Message: fmt.Sprintf("no review item with ID %q exists in this plan", d.ItemID)})
		} else {
			issues = append(issues, validateDecision(d, item, result.MappedLineItems, mappedIndex)...)
		}

		if HasErrors(issues) {
			result.Invalid = append(result.Invalid, InvalidDecision{Decision: d, Issues: issues})
			if itemOK {
				statusByID[d.ItemID] = StatusInvalidDecision
			}
			for _, iss := range issues {
				if iss.Severity == SeverityIssueWarning {
					result.Warnings = append(result.Warnings, iss)
				}
			}
			continue
		}
		for _, iss := range issues {
			result.Warnings = append(result.Warnings, iss)
		}

		// Skip re-applying a later identical duplicate: it was already
		// validated as non-conflicting above and flagged as a warning: the
		// FIRST occurrence (by ORIGINAL slice position) is the one that
		// actually mutates state, so behavior is deterministic and
		// independent of how many identical copies were supplied, and of
		// this loop's processing order.
		if dupIdxs := seen[d.ItemID]; len(dupIdxs) > 1 && dupIdxs[0] != i {
			continue
		}

		applied := applyDecision(d, item, &result, mappedIndex, adjustmentIndex)
		result.Applied = append(result.Applied, applied)
		statusByID[d.ItemID] = resolvedStatusFor(d.Action)
	}

	result.Items = make([]ReviewItem, len(plan.Items))
	for i, it := range plan.Items {
		it.Status = statusByID[it.ID]
		result.Items[i] = it
	}
	result.Summary = summarize(result.Items)

	for _, it := range result.Items {
		if it.Required && it.Status.IsUnresolved() {
			result.UnresolvedRequired = append(result.UnresolvedRequired, it)
		}
	}

	return result
}

// decisionPhase ranks a ReviewItem.Kind into a small, fixed processing
// phase, lowest first. This is what makes Apply's semantic result
// independent of the caller's decision slice order: within one Apply call,
// every decision in an earlier phase is fully validated and applied before
// any decision in a later phase is even validated.
//
// Only one real cross-kind dependency exists in this package today:
// KindStructure -> KindClassification. A row's structural financial.RowKind
// (HEADING/SUBTOTAL/TOTAL vs. NORMAL) determines whether a classification
// override against that same row is even valid (see
// validateClassificationDecision/IssueStructuralRowOverride) — so structure
// decisions must resolve first. No other kind reads or depends on state
// another kind's decision mutates: KindAdjustment only touches
// adjustments.Adjustment (keyed by AdjustmentID, never by row), KindPeriod/
// KindOCRNumeric/KindOCRText only touch CorrectedPeriods/CorrectedNumerics/
// display values, and KindValuationAssumption/KindReconciliation mutate no
// corrected domain structure at all. Inventing additional phases for those
// kinds would add ordering with no real dependency behind it, so they all
// share one "independent" phase below KindStructure and above nothing.
func decisionPhase(kind Kind) int {
	switch kind {
	case KindStructure:
		return 0
	default:
		return 1
	}
}

// decisionApplicationOrder returns, for decs, the indices into decs in the
// deterministic DOMAIN order Apply should process them: primarily by
// decisionPhase (see its doc comment) of the TARGETED ReviewItem's Kind,
// stable within a phase (ties broken by decs' own original order) so
// decisions that don't cross a real phase boundary keep behaving exactly as
// before this ordering was introduced. A decision whose ItemID doesn't
// resolve to a known item (itemByID) sorts into the default/independent
// phase — Apply's own unknown-item-ID validation reports that problem
// exactly as it always has, regardless of processing order.
func decisionApplicationOrder(decs []Decision, itemByID map[string]ReviewItem) []int {
	order := make([]int, len(decs))
	for i := range decs {
		order[i] = i
	}
	phaseOf := func(i int) int {
		if item, ok := itemByID[decs[i].ItemID]; ok {
			return decisionPhase(item.Kind)
		}
		return decisionPhase("")
	}
	sort.SliceStable(order, func(a, b int) bool {
		return phaseOf(order[a]) < phaseOf(order[b])
	})
	return order
}

// resolvedStatusFor maps an applied Action to the item Status it leaves
// behind.
func resolvedStatusFor(a Action) Status {
	if a == ActionReject {
		return StatusRejected
	}
	return StatusResolved
}

// isDuplicateGroupIdentical reports whether every decision at idxs is a
// byte-identical repeat of the first (same Action and same payload
// contents) — used to distinguish a harmless repeated submission (warning
// only) from a genuine conflicting duplicate (error, per section 15's
// "conflicting duplicate decisions" rule).
func isDuplicateGroupIdentical(decs []Decision, idxs []int) bool {
	first := decs[idxs[0]]
	for _, i := range idxs[1:] {
		if !decisionsEqual(first, decs[i]) {
			return false
		}
	}
	return true
}

// decisionsEqual performs a deep, dereferenced comparison of two Decisions'
// Action and payload contents. This deliberately does NOT use
// fmt.Sprintf("%+v", ...) equality: %v on a struct field that is itself a
// pointer prints the raw pointer ADDRESS rather than dereferencing it
// (fmt only auto-dereferences a pointer when it is the top-level argument,
// not a nested struct field) — two decisions built from separately
// allocated, byte-identical payload structs would then always compare
// unequal, which is exactly wrong for detecting a harmless repeated
// submission. reflect.DeepEqual correctly follows pointers to compare
// pointed-to values instead.
func decisionsEqual(a, b Decision) bool {
	if a.ItemID != b.ItemID || a.Action != b.Action {
		return false
	}
	return reflect.DeepEqual(a.Classification, b.Classification) &&
		reflect.DeepEqual(a.OCRNumeric, b.OCRNumeric) &&
		reflect.DeepEqual(a.Structure, b.Structure) &&
		reflect.DeepEqual(a.PeriodOverride, b.PeriodOverride) &&
		reflect.DeepEqual(a.Adjustment, b.Adjustment) &&
		reflect.DeepEqual(a.Assumption, b.Assumption)
}

// validateDecision implements section 15's decision-validation rules for a
// single (non-duplicate-conflict) Decision against its already-resolved
// ReviewItem. mappedLineItems/mappedIndex reflect the CURRENT
// (possibly-already-corrected-by-an-earlier-decision-in-this-same-Apply-
// call) state, so a classification decision can see whether an earlier
// KindStructure decision already turned this row's Kind back to
// RowKindNormal. Returns zero or more Issues; the decision is applied only
// if none of them is SeverityIssueError.
func validateDecision(d Decision, item ReviewItem, mappedLineItems []financial.MappedLineItem, mappedIndex map[string]int) []Issue {
	var issues []Issue

	if !actionAllowedForKind(item.Kind, d.Action) {
		issues = append(issues, Issue{ItemID: d.ItemID, Code: IssueInvalidAction, Severity: SeverityIssueError,
			Message: fmt.Sprintf("action %q is not valid for review kind %q", d.Action, item.Kind)})
		return issues
	}

	switch item.Kind {
	case KindClassification:
		issues = append(issues, validateClassificationDecision(d, item, mappedLineItems, mappedIndex)...)
	case KindOCRNumeric:
		issues = append(issues, validateOCRNumericDecision(d)...)
	case KindStructure:
		issues = append(issues, validateStructureDecision(d)...)
	case KindPeriod:
		issues = append(issues, validatePeriodDecision(d)...)
	case KindAdjustment:
		issues = append(issues, validateAdjustmentDecision(d)...)
	case KindValuationAssumption:
		issues = append(issues, validateAssumptionDecision(d)...)
	}

	return issues
}

// actionAllowedForKind is the fixed (Kind -> allowed Actions) table
// implementing section 13's per-kind decision menu.
func actionAllowedForKind(kind Kind, action Action) bool {
	switch kind {
	case KindClassification:
		return oneOf(action, ActionAccept, ActionOverride, ActionIgnore)
	case KindOCRText:
		return oneOf(action, ActionAccept, ActionOverride, ActionReject)
	case KindOCRNumeric:
		return oneOf(action, ActionAccept, ActionOverride, ActionReject)
	case KindPeriod:
		return oneOf(action, ActionAccept, ActionOverride)
	case KindStructure:
		return oneOf(action, ActionAccept, ActionOverride)
	case KindReconciliation:
		return oneOf(action, ActionAccept, ActionConfirm, ActionIgnore)
	case KindAdjustment:
		return oneOf(action, ActionAccept, ActionOverride, ActionIgnore)
	case KindValuationAssumption:
		return oneOf(action, ActionAccept, ActionOverride, ActionConfirm)
	default:
		return false
	}
}

func oneOf(action Action, allowed ...Action) bool {
	for _, a := range allowed {
		if action == a {
			return true
		}
	}
	return false
}

func validateClassificationDecision(d Decision, item ReviewItem, mappedLineItems []financial.MappedLineItem, mappedIndex map[string]int) []Issue {
	if d.Action != ActionOverride {
		return nil
	}
	if d.Classification == nil {
		return []Issue{{ItemID: d.ItemID, Code: IssueMissingPayload, Severity: SeverityIssueError,
			Message: "ACTION_OVERRIDE on a classification item requires a Classification payload"}}
	}
	if !financial.IsValidCode(d.Classification.Code) {
		return []Issue{{ItemID: d.ItemID, Code: IssueInvalidCode, Severity: SeverityIssueError,
			Message: fmt.Sprintf("%q is not a recognized canonical financial.Code", d.Classification.Code)}}
	}
	if isStructuralRowKind(effectiveRowKind(item, mappedLineItems, mappedIndex)) {
		return []Issue{{ItemID: d.ItemID, Code: IssueStructuralRowOverride, Severity: SeverityIssueError,
			Message: "this row is structural (HEADING, SUBTOTAL, or TOTAL); a classification override cannot assign it a financial account code — use a STRUCTURE decision to change its row kind to NORMAL first"}}
	}
	return nil
}

// effectiveRowKind reports the CURRENT financial.RowKind for a
// KindClassification item's underlying row: the corrected MappedLineItem's
// Kind when the row is present in mappedLineItems (reflecting any
// KindStructure decision already applied earlier in this same Apply call),
// falling back to the item's own Build-time snapshot
// (ReviewItem.Classification.Kind) when the row cannot be found there.
func effectiveRowKind(item ReviewItem, mappedLineItems []financial.MappedLineItem, mappedIndex map[string]int) financial.RowKind {
	if idx, ok := mappedIndex[item.SourceRowID]; ok {
		return mappedLineItems[idx].Kind
	}
	if item.Classification != nil {
		return item.Classification.Kind
	}
	return financial.RowKindNormal
}

// isStructuralRowKind reports whether kind is one of the three structural
// row kinds (HEADING, SUBTOTAL, TOTAL) that must never be silently turned
// back into an ordinary financial row by a classification decision — see
// IssueStructuralRowOverride.
func isStructuralRowKind(kind financial.RowKind) bool {
	switch kind {
	case financial.RowKindHeading, financial.RowKindSubtotal, financial.RowKindTotal:
		return true
	default:
		return false
	}
}

func validateOCRNumericDecision(d Decision) []Issue {
	if d.Action != ActionOverride {
		return nil
	}
	if d.OCRNumeric == nil {
		return []Issue{{ItemID: d.ItemID, Code: IssueMissingPayload, Severity: SeverityIssueError,
			Message: "ACTION_OVERRIDE on an OCR numeric item requires an OCRNumeric payload"}}
	}
	if math.IsNaN(d.OCRNumeric.Amount) || math.IsInf(d.OCRNumeric.Amount, 0) {
		return []Issue{{ItemID: d.ItemID, Code: IssueNonFiniteAmount, Severity: SeverityIssueError,
			Message: fmt.Sprintf("override amount %v is not finite", d.OCRNumeric.Amount)}}
	}
	return nil
}

func validateStructureDecision(d Decision) []Issue {
	if d.Action != ActionOverride {
		return nil
	}
	if d.Structure == nil {
		return []Issue{{ItemID: d.ItemID, Code: IssueMissingPayload, Severity: SeverityIssueError,
			Message: "ACTION_OVERRIDE on a structure item requires a Structure payload"}}
	}
	if !isKnownRowKind(d.Structure.RowKind) {
		return []Issue{{ItemID: d.ItemID, Code: IssueInvalidRowKind, Severity: SeverityIssueError,
			Message: fmt.Sprintf("%q is not one of the four known financial.RowKind values", d.Structure.RowKind)}}
	}
	return nil
}

func isKnownRowKind(k financial.RowKind) bool {
	switch k {
	case financial.RowKindNormal, financial.RowKindHeading, financial.RowKindSubtotal, financial.RowKindTotal:
		return true
	default:
		return false
	}
}

func validatePeriodDecision(d Decision) []Issue {
	if d.Action != ActionOverride {
		return nil
	}
	if d.PeriodOverride == nil {
		return []Issue{{ItemID: d.ItemID, Code: IssueMissingPayload, Severity: SeverityIssueError,
			Message: "ACTION_OVERRIDE on a period item requires a PeriodOverride payload"}}
	}
	if d.PeriodOverride.Period == "" {
		return []Issue{{ItemID: d.ItemID, Code: IssueMalformedPeriod, Severity: SeverityIssueError,
			Message: "period override must not be empty"}}
	}
	return nil
}

func validateAdjustmentDecision(d Decision) []Issue {
	if d.Action != ActionOverride && d.Action != ActionAccept {
		return nil
	}
	if d.Adjustment == nil {
		return []Issue{{ItemID: d.ItemID, Code: IssueMissingPayload, Severity: SeverityIssueError,
			Message: "this action on an adjustment item requires an Adjustment payload"}}
	}
	if d.Adjustment.NewAmount && (math.IsNaN(d.Adjustment.Amount) || math.IsInf(d.Adjustment.Amount, 0)) {
		return []Issue{{ItemID: d.ItemID, Code: IssueNonFiniteAmount, Severity: SeverityIssueError,
			Message: fmt.Sprintf("override amount %v is not finite", d.Adjustment.Amount)}}
	}
	return nil
}

func validateAssumptionDecision(d Decision) []Issue {
	if d.Action != ActionOverride {
		return nil
	}
	if d.Assumption == nil || d.Assumption.Value == nil {
		return []Issue{{ItemID: d.ItemID, Code: IssueMissingPayload, Severity: SeverityIssueError,
			Message: "ACTION_OVERRIDE on an assumption item requires an Assumption payload with a Value"}}
	}
	switch v := d.Assumption.Value.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return []Issue{{ItemID: d.ItemID, Code: IssueNonFiniteAmount, Severity: SeverityIssueError,
				Message: fmt.Sprintf("override value %v is not finite", v)}}
		}
	case bool:
		// Always valid.
	default:
		return []Issue{{ItemID: d.ItemID, Code: IssueIncompatibleDecision, Severity: SeverityIssueError,
			Message: fmt.Sprintf("assumption override value must be a number or a boolean, got %T", v)}}
	}
	return nil
}

// applyDecision performs the actual state mutation for one valid decision
// against result's corrected copies, and returns the AppliedDecision audit
// record for it.
func applyDecision(d Decision, item ReviewItem, result *ApplyResult, mappedIndex map[string]int, adjustmentIndex map[string]int) AppliedDecision {
	applied := AppliedDecision{
		ItemID:           d.ItemID,
		Kind:             item.Kind,
		Action:           d.Action,
		OriginalProposal: item.ProposedValue,
		Reason:           item.Reason,
	}

	switch item.Kind {
	case KindClassification:
		applied.FinalValue = applyClassificationDecision(d, item, result, mappedIndex)
	case KindStructure:
		applied.FinalValue = applyStructureDecision(d, item, result, mappedIndex)
	case KindOCRNumeric:
		applied.FinalValue = applyOCRNumericDecision(d, item, result)
	case KindPeriod:
		applied.FinalValue = applyPeriodDecision(d, item, result)
	case KindAdjustment:
		applied.FinalValue = applyAdjustmentDecision(d, item, result, adjustmentIndex)
	default:
		// KindOCRText, KindReconciliation, KindValuationAssumption:
		// acknowledgment-only kinds with no corrected domain structure of
		// their own to mutate in Source/ApplyResult beyond the Status
		// change already recorded by the caller in Apply.
		applied.FinalValue = item.ProposedValue
	}
	return applied
}

func applyClassificationDecision(d Decision, item ReviewItem, result *ApplyResult, mappedIndex map[string]int) string {
	idx, ok := mappedIndex[item.SourceRowID]
	if !ok {
		return item.ProposedValue
	}
	switch d.Action {
	case ActionAccept:
		return string(result.MappedLineItems[idx].Code)
	case ActionOverride:
		result.MappedLineItems[idx].Code = d.Classification.Code
		result.MappedLineItems[idx].Status = financial.RowStatusNormal
		return string(d.Classification.Code)
	case ActionIgnore:
		result.MappedLineItems[idx].Status = financial.RowStatusIgnored
		return string(financial.RowStatusIgnored)
	}
	return item.ProposedValue
}

func applyStructureDecision(d Decision, item ReviewItem, result *ApplyResult, mappedIndex map[string]int) string {
	idx, ok := mappedIndex[item.SourceRowID]
	if !ok {
		return item.ProposedValue
	}
	kind := item.Structure.ProposedKind
	if d.Action == ActionOverride {
		kind = d.Structure.RowKind
	}
	result.MappedLineItems[idx].Kind = kind
	result.MappedLineItems[idx].Status = rowKindToStatus(kind)
	return string(kind)
}

// rowKindToStatus translates an accepted/overridden financial.RowKind
// decision into the financial.RowStatus that downstream normalization
// consumes, consistent with how classification.Classify already maps
// Kind -> Status (see financial/classification/structural.go's
// detectStructuralStatus): RowKindHeading -> RowStatusIgnored,
// RowKindSubtotal -> RowStatusSubtotal, RowKindTotal -> RowStatusTotal,
// RowKindNormal -> RowStatusNormal. This ensures a caller-overridden
// structure decision flows downstream exactly as if classification itself
// had read that RowKind in the first place — headings/totals are never
// accidentally normalized.
func rowKindToStatus(kind financial.RowKind) financial.RowStatus {
	switch kind {
	case financial.RowKindHeading:
		return financial.RowStatusIgnored
	case financial.RowKindSubtotal:
		return financial.RowStatusSubtotal
	case financial.RowKindTotal:
		return financial.RowStatusTotal
	default:
		return financial.RowStatusNormal
	}
}

func applyOCRNumericDecision(d Decision, item ReviewItem, result *ApplyResult) string {
	switch d.Action {
	case ActionAccept:
		if item.OCRNumeric != nil && item.OCRNumeric.ParsedAmount != nil {
			result.CorrectedNumerics[item.ID] = *item.OCRNumeric.ParsedAmount
			return fmt.Sprintf("%v", *item.OCRNumeric.ParsedAmount)
		}
		return item.ProposedValue
	case ActionOverride:
		result.CorrectedNumerics[item.ID] = d.OCRNumeric.Amount
		return fmt.Sprintf("%v", d.OCRNumeric.Amount)
	case ActionReject:
		delete(result.CorrectedNumerics, item.ID)
		return "unavailable"
	}
	return item.ProposedValue
}

func applyPeriodDecision(d Decision, item ReviewItem, result *ApplyResult) string {
	switch d.Action {
	case ActionAccept:
		p := item.PeriodDetail.ProposedPeriod
		result.CorrectedPeriods[item.ID] = p
		return string(p)
	case ActionOverride:
		result.CorrectedPeriods[item.ID] = d.PeriodOverride.Period
		return string(d.PeriodOverride.Period)
	}
	return item.ProposedValue
}

func applyAdjustmentDecision(d Decision, item ReviewItem, result *ApplyResult, adjustmentIndex map[string]int) string {
	idx, ok := adjustmentIndex[item.Adjustment.AdjustmentID]
	if !ok {
		return item.ProposedValue
	}
	switch d.Action {
	case ActionAccept:
		if d.Adjustment != nil {
			result.Adjustments[idx].Included = d.Adjustment.Included
		}
	case ActionOverride:
		result.Adjustments[idx].Included = d.Adjustment.Included
		if d.Adjustment.NewAmount {
			result.Adjustments[idx].Amount = d.Adjustment.Amount
		}
		if d.Adjustment.Reason != "" {
			result.Adjustments[idx].Reason = d.Adjustment.Reason
		}
	case ActionIgnore:
		result.Adjustments[idx].Included = false
	}
	return fmt.Sprintf("included=%v amount=%v", result.Adjustments[idx].Included, result.Adjustments[idx].Amount)
}

// --- deep-copy helpers (immutability guarantees) ---------------------------

func cloneDecisions(decisions []Decision) []Decision {
	out := make([]Decision, len(decisions))
	for i, d := range decisions {
		cp := d
		if d.Classification != nil {
			v := *d.Classification
			cp.Classification = &v
		}
		if d.OCRNumeric != nil {
			v := *d.OCRNumeric
			cp.OCRNumeric = &v
		}
		if d.Structure != nil {
			v := *d.Structure
			cp.Structure = &v
		}
		if d.PeriodOverride != nil {
			v := *d.PeriodOverride
			cp.PeriodOverride = &v
		}
		if d.Adjustment != nil {
			v := *d.Adjustment
			cp.Adjustment = &v
		}
		if d.Assumption != nil {
			v := *d.Assumption
			cp.Assumption = &v
		}
		out[i] = cp
	}
	return out
}

func cloneMappedLineItems(items []financial.MappedLineItem) []financial.MappedLineItem {
	out := make([]financial.MappedLineItem, len(items))
	for i, it := range items {
		cp := it
		if it.Values != nil {
			cp.Values = make(map[financial.Period]float64, len(it.Values))
			for k, v := range it.Values {
				cp.Values[k] = v
			}
		}
		out[i] = cp
	}
	return out
}

func cloneAdjustments(adjs []adjustments.Adjustment) []adjustments.Adjustment {
	out := make([]adjustments.Adjustment, len(adjs))
	for i, a := range adjs {
		cp := a
		if a.Targets != nil {
			cp.Targets = append([]adjustments.Target(nil), a.Targets...)
		}
		out[i] = cp
	}
	return out
}

func indexMappedLineItemsBySourceID(items []financial.MappedLineItem) map[string]int {
	idx := make(map[string]int, len(items))
	for i, it := range items {
		idx[it.SourceID] = i
	}
	return idx
}

func indexAdjustmentsByID(adjs []adjustments.Adjustment) map[string]int {
	idx := make(map[string]int, len(adjs))
	for i, a := range adjs {
		idx[string(a.ID)] = i
	}
	return idx
}
