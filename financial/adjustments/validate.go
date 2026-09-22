package adjustments

import (
	"fmt"
	"math"
	"sort"

	"github.com/themurtez/go-valuate/financial/metrics"
)

// IssueSeverity distinguishes a validation problem that must block
// application (SeverityError) from one that is worth surfacing to a human
// but does not by itself invalidate the adjustment (SeverityWarning) —
// mirroring reconciliation.Status's PASS/WARNING/FAIL split rather than
// collapsing every problem into a single boolean.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of validation problem,
// analogous to reconciliation.CheckCode.
type IssueCode string

const (
	IssueMissingID             IssueCode = "MISSING_ID"
	IssueDuplicateID           IssueCode = "DUPLICATE_ID"
	IssueMissingPeriod         IssueCode = "MISSING_PERIOD"
	IssueUnknownPeriod         IssueCode = "UNKNOWN_PERIOD"
	IssueMissingType           IssueCode = "MISSING_TYPE"
	IssueNonFiniteAmount       IssueCode = "NON_FINITE_AMOUNT"
	IssueMissingReason         IssueCode = "MISSING_REASON"
	IssueAmbiguousEffect       IssueCode = "AMBIGUOUS_EFFECT"
	IssueAmbiguousTargets      IssueCode = "AMBIGUOUS_TARGETS"
	IssueIncompatibleTarget    IssueCode = "INCOMPATIBLE_TARGET"
	IssueUnavailableBaseMetric IssueCode = "UNAVAILABLE_BASE_METRIC"
	IssueSuspiciousDuplicate   IssueCode = "SUSPICIOUS_DUPLICATE"
)

// Issue is a single validation finding for one Adjustment.
type Issue struct {
	// AdjustmentID identifies the Adjustment this issue was found on. Empty
	// only for IssueDuplicateID, which is reported once per colliding ID
	// group (see Validate) rather than once per adjustment.
	AdjustmentID ID            `json:"adjustment_id,omitempty"`
	Code         IssueCode     `json:"code"`
	Severity     IssueSeverity `json:"severity"`
	Message      string        `json:"message"`
}

// Validate checks a set of Adjustments for internal consistency against a
// metrics.Snapshot they will be applied to (see Apply). It never mutates
// its inputs and performs no I/O.
//
// Validate deliberately does NOT reject a negative-earnings-effect
// adjustment, and does not reject Amount == 0 or Amount < 0 by itself —
// non-negative-magnitude is enforced structurally by IssueNonFiniteAmount
// only checking finiteness, not sign, per this package's explicit design
// requirement that legitimate adjustments may reduce normalized earnings
// (see TypeNonOperatingIncome/TypeUnusualGain). A negative Amount (as
// opposed to a positive Amount with EffectDecrease) is unusual but not
// rejected outright, since a caller migrating from a signed-delta
// convention may still supply one; it is passed through arithmetic as-is
// and its sign simply combines with Effect.
func Validate(adjs []Adjustment, snapshot metrics.Snapshot) []Issue {
	var issues []Issue

	seenIDs := make(map[ID][]int)
	seenDuplicateKey := make(map[string][]int)

	for i, adj := range adjs {
		if adj.ID == "" {
			issues = append(issues, Issue{Code: IssueMissingID, Severity: SeverityError, Message: fmt.Sprintf("adjustment at index %d has no ID", i)})
		} else {
			seenIDs[adj.ID] = append(seenIDs[adj.ID], i)
		}

		if adj.Period == "" {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueMissingPeriod, Severity: SeverityError, Message: "adjustment has no period"})
		} else if adj.Period != snapshot.Period {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueUnknownPeriod, Severity: SeverityError, Message: fmt.Sprintf("adjustment period %q does not match snapshot period %q", adj.Period, snapshot.Period)})
		}

		if adj.Type == "" {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueMissingType, Severity: SeverityError, Message: "adjustment has no type"})
		}

		if math.IsNaN(adj.Amount) || math.IsInf(adj.Amount, 0) {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueNonFiniteAmount, Severity: SeverityError, Message: fmt.Sprintf("amount %v is not finite", adj.Amount)})
		}

		if adj.Reason == "" {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueMissingReason, Severity: SeverityWarning, Message: "adjustment has no reason/description"})
		}

		if _, ok := adj.resolvedEffect(); !ok {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueAmbiguousEffect, Severity: SeverityError, Message: fmt.Sprintf("type %q has no default effect; Effect must be set explicitly", adj.Type)})
		}

		targets := adj.resolvedTargets()
		if len(targets) == 0 {
			issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueAmbiguousTargets, Severity: SeverityError, Message: fmt.Sprintf("type %q has no default targets; Targets must be set explicitly", adj.Type)})
		}
		for _, target := range targets {
			if target != TargetEBITDA && target != TargetSDE {
				issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueIncompatibleTarget, Severity: SeverityError, Message: fmt.Sprintf("unrecognized target %q", target)})
			}
		}

		// Reported as a warning, not an error: Apply itself already skips
		// this adjustment precisely (SkipBaseMetricUnavailable) without
		// needing to treat the adjustment as structurally invalid — the
		// problem is the snapshot's data availability, not this
		// adjustment's shape.
		if adj.Included {
			for _, target := range targets {
				if base, ok := baseMetricAvailable(snapshot, target); adj.Period == snapshot.Period && !ok {
					issues = append(issues, Issue{AdjustmentID: adj.ID, Code: IssueUnavailableBaseMetric, Severity: SeverityWarning, Message: fmt.Sprintf("target %q base metric is unavailable for period %q: %s", target, snapshot.Period, base)})
				}
			}
		}

		// Suspicious duplicate detection: same period + type + amount +
		// effect is very likely the same real-world adjustment entered
		// twice (e.g. a double form submission), as opposed to two
		// distinct adjustments that legitimately share a type (two
		// different one-time expenses of different amounts are fine).
		effect, _ := adj.resolvedEffect()
		key := fmt.Sprintf("%s|%s|%v|%s", adj.Period, adj.Type, adj.Amount, effect)
		seenDuplicateKey[key] = append(seenDuplicateKey[key], i)
	}

	for id, idxs := range seenIDs {
		if len(idxs) > 1 {
			issues = append(issues, Issue{AdjustmentID: id, Code: IssueDuplicateID, Severity: SeverityError, Message: fmt.Sprintf("ID %q is used by %d adjustments; IDs must be unique", id, len(idxs))})
		}
	}

	for _, idxs := range seenDuplicateKey {
		if len(idxs) > 1 {
			for _, i := range idxs {
				issues = append(issues, Issue{AdjustmentID: adjs[i].ID, Code: IssueSuspiciousDuplicate, Severity: SeverityWarning, Message: fmt.Sprintf("another adjustment in this set has the same period, type, amount, and effect; possible duplicate entry (%d matching adjustments)", len(idxs))})
			}
		}
	}

	sort.SliceStable(issues, func(i, j int) bool { return issues[i].Code < issues[j].Code })
	return issues
}

// baseMetricAvailable reports whether the metrics.Snapshot value a bridge
// for target starts from is available, and a label to explain which one
// when it is not.
func baseMetricAvailable(snapshot metrics.Snapshot, target Target) (label string, ok bool) {
	switch target {
	case TargetEBITDA:
		return "EBITDA", snapshot.EBITDA.Available
	case TargetSDE:
		return "SDE", snapshot.SDE.Available
	default:
		return string(target), false
	}
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated in valuation (HasErrors over valuation.Issue)
// and review (HasErrors over review.Issue) rather than shared — see
// valuation.HasErrors's doc comment for the full rationale.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
