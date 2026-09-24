package kpi

import "sort"

// ThresholdBand is one caller-defined labeled range — task section 23.
// Labels are always whatever the caller supplied verbatim; this package
// never invents evaluative labels ("good"/"bad"/"healthy") or colors of
// its own (task section 23/41's neutral-output-boundary rule).
type ThresholdBand struct {
	// Label is caller-supplied, carried through verbatim (e.g. "LOW",
	// "MID", "HIGH", or any caller-chosen string).
	Label string `json:"label"`
	// Min/Max define this band's range. Min is inclusive; Max is
	// exclusive — task section 23's "define exact inclusive/exclusive
	// boundary semantics" instruction: a value v is in this band when
	// Min <= v < Max, EXCEPT the single band (if any) whose Max is the
	// overall maximum across all bands supplied, which is Min <= v <= Max
	// (closed on both ends) so the top band's own upper boundary value is
	// actually reachable — task section 23's worked example "80+ HIGH"
	// requires 80 itself to land in HIGH, not fall through unbanded.
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// validateBands checks bands for structural validity and overlaps — task
// section 23's "reject overlapping bands" instruction. Two bands overlap
// when their [Min, Max) ranges (both using the half-open convention; see
// ThresholdBand's doc comment for the single closed-top-band exception,
// which does not by itself create an overlap since it only extends
// inclusivity at the point Max, already excluded from every other band's
// own half-open range) intersect. Returns ok=false if any band has
// Min >= Max (a degenerate/empty band) or any two bands overlap.
func validateBands(bands []ThresholdBand) (ok bool, overlapping bool, degenerate bool) {
	for _, b := range bands {
		if b.Min >= b.Max {
			return false, false, true
		}
	}
	sorted := make([]ThresholdBand, len(bands))
	copy(sorted, bands)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Min < sorted[j].Min })
	for i := 1; i < len(sorted); i++ {
		if sorted[i].Min < sorted[i-1].Max {
			return false, true, false
		}
	}
	return true, false, false
}

// bandFor returns the ThresholdBand containing value, and whether one was
// found. bands is assumed already validated (validateBands returned ok).
// The top band (max Max across all bands) is treated as closed on both
// ends — see ThresholdBand's doc comment.
func bandFor(bands []ThresholdBand, value float64) (ThresholdBand, bool) {
	if len(bands) == 0 {
		return ThresholdBand{}, false
	}
	topMax := bands[0].Max
	for _, b := range bands {
		if b.Max > topMax {
			topMax = b.Max
		}
	}
	for _, b := range bands {
		if value >= b.Min && value < b.Max {
			return b, true
		}
		if b.Max == topMax && value == topMax {
			return b, true
		}
	}
	return ThresholdBand{}, false
}
