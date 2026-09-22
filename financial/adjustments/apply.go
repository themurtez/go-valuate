package adjustments

import (
	"github.com/themurtez/go-valuate/financial/metrics"
)

// SkipReason explains why an Adjustment did not participate in a bridge.
type SkipReason string

const (
	// SkipNotIncluded means Adjustment.Included was false.
	SkipNotIncluded SkipReason = "not_included"
	// SkipWrongPeriod means the adjustment's Period does not match the
	// snapshot being adjusted.
	SkipWrongPeriod SkipReason = "wrong_period"
	// SkipNotTargeted means the adjustment's resolved Targets does not
	// include the bridge being built (e.g. an SDE-only owner compensation
	// normalization skipped when building the EBITDA bridge).
	SkipNotTargeted SkipReason = "not_targeted"
	// SkipInvalid means the adjustment failed Validate (ambiguous
	// effect/targets, non-finite amount, etc.) and cannot be safely
	// applied.
	SkipInvalid SkipReason = "invalid"
	// SkipBaseMetricUnavailable means the bridge's starting metric
	// (EBITDA or SDE) is unavailable for this period, so no adjustment can
	// be layered on top of it.
	SkipBaseMetricUnavailable SkipReason = "base_metric_unavailable"
)

// Skipped is one Adjustment that did not participate in a bridge, plus why.
type Skipped struct {
	Adjustment Adjustment `json:"adjustment"`
	Reason     SkipReason `json:"reason"`
	Detail     string     `json:"detail,omitempty"`
}

// AppliedLine is one Adjustment that did participate in a bridge, plus the
// signed amount it contributed (positive = increased the metric, negative
// = decreased it) — the one place this package surfaces a signed number,
// since a bridge display needs one, but it is always derived from
// Amount+Effect, never supplied directly by a caller.
type AppliedLine struct {
	Adjustment Adjustment `json:"adjustment"`
	// SignedAmount is +Amount for EffectIncrease, -Amount for
	// EffectDecrease.
	SignedAmount float64 `json:"signed_amount"`
}

// Bridge is a transparent walk from a base metric to a normalized metric:
// the starting value, every applied adjustment line, and the resulting
// total. Used for both the EBITDA bridge and the SDE bridge (see
// Result.EBITDABridge/SDEBridge).
type Bridge struct {
	// Target is which metric this bridge is for.
	Target Target `json:"target"`
	// Period is the period this bridge was computed for.
	Period string `json:"period"`
	// BaseAvailable is false if the underlying metrics.Snapshot value
	// (EBITDA or SDE) was itself unavailable — in that case every other
	// field is zero-value and Applied/Skipped both reflect
	// SkipBaseMetricUnavailable for anything that would have targeted this
	// bridge.
	BaseAvailable bool `json:"base_available"`
	// BaseValue is the starting metrics.Snapshot value (EBITDA.Value or
	// SDE.Value) this bridge adjusts from. Meaningful only when
	// BaseAvailable.
	BaseValue float64 `json:"base_value"`
	// Applied lists every adjustment that contributed to this bridge, in
	// the order supplied to Apply.
	Applied []AppliedLine `json:"applied,omitempty"`
	// TotalAdjustment is the sum of every AppliedLine.SignedAmount.
	TotalAdjustment float64 `json:"total_adjustment"`
	// NormalizedValue is BaseValue + TotalAdjustment. Meaningful only when
	// BaseAvailable.
	NormalizedValue float64 `json:"normalized_value"`
}

// Result is the output of Apply: the original metrics.Snapshot, both
// bridges, every adjustment that was skipped (across either bridge) with
// its reason, and any warnings surfaced during application (e.g. from
// Validate).
type Result struct {
	// Period is the period Apply was run for.
	Period string `json:"period"`
	// OriginalSnapshot is the unmodified metrics.Snapshot Apply was given.
	// Apply never mutates it.
	OriginalSnapshot metrics.Snapshot `json:"original_snapshot"`
	// EBITDABridge is the normalized-EBITDA bridge: EBITDA plus confirmed
	// TargetEBITDA adjustments. See BridgeEBITDA.
	EBITDABridge Bridge `json:"ebitda_bridge"`
	// SDEBridge is the normalized-SDE bridge: SDE plus confirmed TargetSDE
	// adjustments. See BridgeSDE.
	SDEBridge Bridge `json:"sde_bridge"`
	// Skipped lists every adjustment that did not contribute to at least
	// one bridge it targeted. No deduplication is performed — an
	// adjustment targeting both EBITDA and SDE that is skipped from both
	// appears once per bridge it was skipped from, since the reason can
	// differ per bridge (e.g. owner compensation normalization is
	// structurally SkipNotTargeted for EBITDA but may apply cleanly to
	// SDE).
	Skipped []Skipped `json:"skipped,omitempty"`
	// Warnings carries non-fatal Issues found by Validate (SeverityWarning)
	// plus any other advisory notes Apply itself generates.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries fatal Issues found by Validate (SeverityError) for
	// adjustments that were consequently skipped with SkipInvalid.
	Errors []Issue `json:"errors,omitempty"`
}

// Apply applies adjs to snapshot and returns both the normalized-EBITDA and
// normalized-SDE bridges. Apply never mutates snapshot or adjs.
//
// Owner compensation and double counting. financial/metrics' baseline SDE
// is defined as EBITDA + OwnerCompensation (see metrics' sde doc comment):
// EBITDA already had owner compensation deducted as an operating expense,
// and SDE adds it back because SDE is defined as the total benefit
// available to a single working owner. This means owner compensation is
// already fully reflected in the SDE baseline Apply starts from — an
// OwnerCompensationNormalization adjustment therefore represents a
// *replacement* of that baseline owner-benefit figure with a
// market-adjusted one (e.g. "the actual owner drew $180k; a hired GM would
// cost $90k; add back the $90k difference"), not a second, independent
// add-back of the full owner compensation figure. Callers are expected to
// supply Amount as that *difference*, not the raw compensation figure —
// this package cannot derive the difference itself since it has no
// visibility into what a market-rate replacement would cost. See the
// TestDoubleCountPrevention-style tests in apply_test.go, which assert
// that applying a TypeOwnerCompensationNormalization adjustment changes
// SDE by exactly its own Amount (signed by Effect) and not by
// OwnerCompensation's raw value.
//
// By default (see buildTypeRegistry), TypeOwnerCompensationNormalization
// targets SDE only, not EBITDA — EBITDA's baseline never added owner
// compensation back in the first place, so there is nothing to normalize
// in the EBITDA bridge; an adjustment of this type is reported as
// SkipNotTargeted for the EBITDA bridge rather than silently ignored.
func Apply(snapshot metrics.Snapshot, adjs []Adjustment) Result {
	issues := Validate(adjs, snapshot)
	invalidIDs := make(map[ID]bool)
	result := Result{Period: string(snapshot.Period), OriginalSnapshot: snapshot}
	for _, iss := range issues {
		if iss.Severity == SeverityError {
			result.Errors = append(result.Errors, iss)
			if iss.AdjustmentID != "" {
				invalidIDs[iss.AdjustmentID] = true
			}
		} else {
			result.Warnings = append(result.Warnings, iss)
		}
	}

	result.EBITDABridge = buildBridge(TargetEBITDA, snapshot, adjs, invalidIDs, &result.Skipped)
	result.SDEBridge = buildBridge(TargetSDE, snapshot, adjs, invalidIDs, &result.Skipped)

	return result
}

// buildBridge constructs the Bridge for a single target, appending every
// non-participating adjustment (for this target) to skipped.
func buildBridge(target Target, snapshot metrics.Snapshot, adjs []Adjustment, invalidIDs map[ID]bool, skipped *[]Skipped) Bridge {
	bridge := Bridge{Target: target, Period: string(snapshot.Period)}

	baseValue, baseAvailable := baseMetricValue(snapshot, target)
	bridge.BaseAvailable = baseAvailable
	bridge.BaseValue = baseValue

	total := 0.0
	for _, adj := range adjs {
		if !adj.Included {
			*skipped = append(*skipped, Skipped{Adjustment: adj, Reason: SkipNotIncluded})
			continue
		}
		if adj.Period != snapshot.Period {
			*skipped = append(*skipped, Skipped{Adjustment: adj, Reason: SkipWrongPeriod})
			continue
		}
		if !adj.appliesTo(target) {
			*skipped = append(*skipped, Skipped{Adjustment: adj, Reason: SkipNotTargeted})
			continue
		}
		if !baseAvailable {
			*skipped = append(*skipped, Skipped{Adjustment: adj, Reason: SkipBaseMetricUnavailable, Detail: string(target) + " is unavailable for this period"})
			continue
		}
		if invalidIDs[adj.ID] {
			*skipped = append(*skipped, Skipped{Adjustment: adj, Reason: SkipInvalid, Detail: "failed validation; see Result.Errors"})
			continue
		}

		signed, ok := adj.signedAmount()
		if !ok {
			// Ambiguous effect is already reported as a validation error
			// and this adjustment's ID is in invalidIDs; defensive only.
			*skipped = append(*skipped, Skipped{Adjustment: adj, Reason: SkipInvalid, Detail: "could not resolve effect"})
			continue
		}

		bridge.Applied = append(bridge.Applied, AppliedLine{Adjustment: adj, SignedAmount: signed})
		total += signed
	}

	bridge.TotalAdjustment = total
	if baseAvailable {
		bridge.NormalizedValue = baseValue + total
	}
	return bridge
}

// baseMetricValue returns the metrics.Snapshot value a bridge for target
// starts from.
func baseMetricValue(snapshot metrics.Snapshot, target Target) (value float64, available bool) {
	switch target {
	case TargetEBITDA:
		return snapshot.EBITDA.Value, snapshot.EBITDA.Available
	case TargetSDE:
		return snapshot.SDE.Value, snapshot.SDE.Available
	default:
		return 0, false
	}
}
