package closequality

import "sort"

// sortFindings orders findings by Dimension (dimensionOrder) -> Code
// (findingOrder) -> first AccountID -> SourceCode, ascending, matching
// the documented deterministic-ordering rule. Never relies on Go map
// order.
func sortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if da, db := dimensionRank[a.Dimension], dimensionRank[b.Dimension]; da != db {
			return da < db
		}
		if ca, cb := findingRank[a.Code], findingRank[b.Code]; ca != cb {
			return ca < cb
		}
		aAcct, bAcct := firstOrEmpty(a.AccountIDs), firstOrEmpty(b.AccountIDs)
		if aAcct != bAcct {
			return aAcct < bAcct
		}
		return a.SourceCode < b.SourceCode
	})
}

func firstOrEmpty(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	return ss[0]
}

// dedupFindings merges findings that share a dedupKey (same Code,
// Dimension, and first AccountID — see keyFor), preserving the
// first-seen Finding's body and folding every subsequent duplicate's
// (SourceModule, SourceCode) into the survivor's AdditionalSources. It
// never merges findings whose evidence only superficially resembles
// each other; only an exact dedupKey match is merged (see
// docs/CLOSE_QUALITY.md's deduplication section for why this is
// deliberately conservative). findings must already be in a stable
// order (the mine* run order) before calling this, so the "first-seen"
// choice is itself deterministic.
func dedupFindings(findings []Finding) []Finding {
	if len(findings) == 0 {
		return findings
	}
	index := make(map[dedupKey]int, len(findings))
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		k := keyFor(f)
		if i, ok := index[k]; ok {
			survivor := &out[i]
			if f.SourceModule != "" {
				ref := SourceRef{SourceModule: f.SourceModule, SourceCode: f.SourceCode}
				if !containsSourceRef(survivor.AdditionalSources, ref) &&
					ref != (SourceRef{SourceModule: survivor.SourceModule, SourceCode: survivor.SourceCode}) {
					survivor.AdditionalSources = append(survivor.AdditionalSources, ref)
				}
			}
			// A duplicate at BLOCKING severity should not be silently
			// hidden behind a WARNING/INFO survivor — promote severity
			// to the worst of the two while keeping the first-seen body.
			if severityRank[f.Severity] < severityRank[survivor.Severity] {
				survivor.Severity = f.Severity
			}
			continue
		}
		index[k] = len(out)
		out = append(out, f)
	}
	return out
}

// sortDimensionsInPlace orders dims by dimensionOrder — the fixed,
// documented dimension output order (see dimensionOrder).
func sortDimensionsInPlace(dims []DimensionResult) {
	sort.SliceStable(dims, func(i, j int) bool {
		return dimensionRank[dims[i].Dimension] < dimensionRank[dims[j].Dimension]
	})
}

func containsSourceRef(refs []SourceRef, ref SourceRef) bool {
	for _, r := range refs {
		if r == ref {
			return true
		}
	}
	return false
}
