package closequality

// deriveStatus implements the documented, deterministic readiness rule
// (Prompt 42 section 27):
//
//  1. If no *substantive* dimension could be assessed (every dimension
//     other than DATA_COMPLETENESS is UNASSESSED) AND no required input
//     is known to be missing (no FindingMissingRequiredInput), Status is
//     UNASSESSED — there is genuinely insufficient basis for any
//     determination: no data to check, and no declared policy saying
//     anything ought to have been supplied. DATA_COMPLETENESS itself is
//     excluded from the "substantive dimension" check because it is a
//     meta-dimension about input coverage, always assessable (see
//     mineDataCompleteness) — but a MISSING_REQUIRED_INPUT finding it
//     produced is itself a substantive, actionable fact ("required input
//     X is missing"), so it independently satisfies having a basis for
//     NOT_READY even when literally nothing else could be assessed. This
//     matches the spec's stated preference: "missing required input ->
//     NOT_READY; no required dimensions configured / insufficient basis
//     -> UNASSESSED."
//  2. Else if any BLOCKING finding exists among Blockers, Status is
//     NOT_READY.
//  3. Else if any WARNING finding exists among Warnings, Status is
//     READY_WITH_WARNINGS.
//  4. Else Status is READY.
func deriveStatus(dc DimensionCoverage, blockers, warnings []Finding) Status {
	substantiveAssessed := false
	for _, d := range dc.AssessedDimensions {
		if d != DimensionDataCompleteness {
			substantiveAssessed = true
			break
		}
	}
	if !substantiveAssessed && !hasFindingCode(blockers, FindingMissingRequiredInput) && !hasFindingCode(warnings, FindingMissingRequiredInput) {
		return StatusUnassessed
	}
	if len(blockers) > 0 {
		return StatusNotReady
	}
	if len(warnings) > 0 {
		return StatusReadyWithWarnings
	}
	return StatusReady
}

func hasFindingCode(findings []Finding, code FindingCode) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
