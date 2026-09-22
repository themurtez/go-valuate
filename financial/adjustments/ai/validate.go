package ai

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/financial/adjustments"
)

// ValidateSuggestions checks every entry in suggestions against req (the
// Request that supposedly produced them), enforcing every hard safety rule
// in this package's doc comment: source-bound row/period, exact-amount
// match, closed-set adjustment type, direction/effect compatibility,
// no structural or ambiguous-OCR source rows, no duplicates, no non-finite
// values.
//
// Returns, in suggestions' own input order, one entry per input suggestion:
// either the suggestion itself (valid) paired with a nil Issue, or a
// zero Suggestion paired with the rejecting Issue — so a caller can always
// recover which INPUT index a given outcome corresponds to. A rejected
// suggestion is never silently repaired (section 9): validation only ever
// accepts or rejects, never adjusts a value to make it pass.
func ValidateSuggestions(req Request, suggestions []Suggestion) []ValidatedSuggestion {
	rowByKey := make(map[rowKey]SourceRow, len(req.Candidates))
	for _, c := range req.Candidates {
		rowByKey[newRowKey(c.RowID, string(c.Period))] = c
	}
	allowedTypes := make(map[adjustments.Type]bool, len(req.AllowedTypes))
	for _, t := range req.AllowedTypes {
		allowedTypes[t.Type] = true
	}

	seenDup := make(map[dupKey]bool)
	out := make([]ValidatedSuggestion, len(suggestions))

	for i, s := range suggestions {
		if issue := validateOne(s, rowByKey, allowedTypes, seenDup); issue != nil {
			out[i] = ValidatedSuggestion{Issue: issue}
			continue
		}
		out[i] = ValidatedSuggestion{Suggestion: s, Valid: true}
		if s.RequiresUserInput {
			out[i].Issue = &Issue{
				SourceRowID: s.SourceRowID, Code: IssueMissingUserInput, Severity: SeverityWarning,
				Message: "suggestion requires a caller/accountant-supplied benchmark value before it can be applied",
			}
		}
	}
	return out
}

// ValidatedSuggestion pairs a Suggestion with its validation outcome.
type ValidatedSuggestion struct {
	// Suggestion is the original suggestion, populated only when Valid.
	Suggestion Suggestion
	// Valid is true when the suggestion passed every hard validation rule.
	// A Valid suggestion may still carry a non-nil, SeverityWarning Issue
	// (currently only IssueMissingUserInput) — check Issue.Severity, not
	// merely Issue's presence, to distinguish "rejected" from
	// "valid, but flagged."
	Valid bool
	// Issue is nil for a fully clean valid suggestion, a SeverityWarning
	// Issue for a valid-but-flagged suggestion (RequiresUserInput), or a
	// SeverityError Issue explaining why an invalid suggestion was rejected.
	Issue *Issue
}

// Rejected reports whether this outcome was rejected (Issue present with
// SeverityError) — the only case Valid is false.
func (v ValidatedSuggestion) Rejected() bool { return !v.Valid }

type rowKey struct {
	rowID  string
	period string
}

func newRowKey(rowID string, period string) rowKey { return rowKey{rowID, period} }

type dupKey struct {
	rowID  string
	period string
	typ    adjustments.Type
}

func validateOne(s Suggestion, rowByKey map[rowKey]SourceRow, allowedTypes map[adjustments.Type]bool, seenDup map[dupKey]bool) *Issue {
	if s.SourceRowID == "" {
		return &Issue{Code: IssueUnknownSourceRow, Severity: SeverityError,
			Message: "suggestion has no source_row_id"}
	}

	row, ok := rowByKey[newRowKey(s.SourceRowID, string(s.Period))]
	if !ok {
		if _, existsAnyPeriod := findAnyPeriod(rowByKey, s.SourceRowID); existsAnyPeriod {
			return &Issue{SourceRowID: s.SourceRowID, Code: IssueWrongPeriod, Severity: SeverityError,
				Message: fmt.Sprintf("suggestion period %q does not match any candidate period for row %q", s.Period, s.SourceRowID)}
		}
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueUnknownSourceRow, Severity: SeverityError,
			Message: fmt.Sprintf("suggestion references row_id %q, which is not in the request's candidates", s.SourceRowID)}
	}

	if isStructuralSourceRow(row) {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueStructuralSourceRow, Severity: SeverityError,
			Message: "referenced source row is structural (heading/subtotal/total); never a valid adjustment target"}
	}
	if row.AmbiguousOCR {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueAmbiguousSourceAmount, Severity: SeverityError,
			Message: "referenced source row's amount is unconfirmed OCR-ambiguous data"}
	}

	if math.IsNaN(s.Amount) || math.IsInf(s.Amount, 0) {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueNonFiniteAmount, Severity: SeverityError,
			Message: fmt.Sprintf("amount %v is not finite", s.Amount)}
	}
	if s.Confidence != nil {
		c := *s.Confidence
		if math.IsNaN(c) || math.IsInf(c, 0) || c < 0 || c > 1 {
			return &Issue{SourceRowID: s.SourceRowID, Code: IssueNonFiniteAmount, Severity: SeverityError,
				Message: fmt.Sprintf("confidence %v is not a finite value in [0, 1]", c)}
		}
	}

	// Section 9: suggested amount must EXACTLY equal the source row's own
	// amount under MVP rules. No tolerance, no partial-amount support (no
	// existing adjustments.Type in this repository grants a caller-approved
	// partial amount), so an amount mismatch of any size is rejected.
	if s.Amount != row.Amount {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueInventedAmount, Severity: SeverityError,
			Message: fmt.Sprintf("suggested amount %v does not match source row amount %v", s.Amount, row.Amount)}
	}

	if s.AdjustmentType == "" || !allowedTypes[s.AdjustmentType] {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueInvalidAdjustmentType, Severity: SeverityError,
			Message: fmt.Sprintf("adjustment type %q is not in the request's allowed closed set", s.AdjustmentType)}
	}

	if issue := validateDirection(s); issue != nil {
		return issue
	}

	key := dupKey{s.SourceRowID, string(s.Period), s.AdjustmentType}
	if seenDup[key] {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueDuplicateSuggestion, Severity: SeverityError,
			Message: fmt.Sprintf("duplicate suggestion for row %q, period %q, type %q", s.SourceRowID, s.Period, s.AdjustmentType)}
	}
	seenDup[key] = true

	return nil
}

// validateDirection enforces section 9's "direction incompatible with
// deterministic adjustment semantics" rejection rule: when AdjustmentType
// has a fixed adjustments.TypeMeta.DefaultEffect, a suggested Direction that
// resolves to the OPPOSITE adjustments.Effect is rejected outright — the AI
// may not contradict a type whose sign is a hard product rule (e.g.
// suggesting DECREASE_EARNINGS for adjustments.TypeOneTimeExpense, which by
// definition only ever adds an expense back). A type with no fixed default
// effect (TypeCustom, TypeRelatedPartyRentAdjustment) accepts either
// Direction, since the real-world sign genuinely depends on the specific
// case (see adjustments.TypeRelatedPartyRentAdjustment's own doc comment).
func validateDirection(s Suggestion) *Issue {
	effect, ok := s.Direction.toEffect()
	if !ok {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueIncompatibleDirection, Severity: SeverityError,
			Message: fmt.Sprintf("direction %q is not a recognized value", s.Direction)}
	}

	meta, known := adjustments.LookupType(s.AdjustmentType)
	if !known || meta.DefaultEffect == "" {
		return nil
	}
	if effect != meta.DefaultEffect {
		return &Issue{SourceRowID: s.SourceRowID, Code: IssueIncompatibleDirection, Severity: SeverityError,
			Message: fmt.Sprintf("direction %q (effect %q) is incompatible with adjustment type %q, whose only valid effect is %q", s.Direction, effect, s.AdjustmentType, meta.DefaultEffect)}
	}
	return nil
}

func findAnyPeriod(rowByKey map[rowKey]SourceRow, rowID string) (SourceRow, bool) {
	for k, row := range rowByKey {
		if k.rowID == rowID {
			return row, true
		}
	}
	return SourceRow{}, false
}
