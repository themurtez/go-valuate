package tabular

import "strings"

// SheetCandidate is one worksheet's metadata considered during sheet
// selection.
type SheetCandidate struct {
	Index int
	Name  string
	Empty bool
	// PlausibilityScore is a caller-computed heuristic for how likely this
	// sheet is to contain the primary financial statement, e.g. derived
	// from row/column count and whether DetectStatementType found a
	// signal. Higher is more plausible.
	PlausibilityScore int
}

// SheetSelection is the outcome of SelectSheet.
type SheetSelection struct {
	// SelectedIndex is the chosen sheet's index, or -1 if none could be
	// selected.
	SelectedIndex int
	// Ambiguous is true when more than one non-empty sheet scored equally
	// highest, meaning the choice is not determinable without caller
	// input.
	Ambiguous bool
	// Candidates lists every non-empty sheet considered.
	Candidates []SheetCandidate
}

// SelectSheet picks the single most plausible sheet among candidates.
// Rules, applied in order:
//  1. If requestedName is non-empty, select the sheet with that exact name
//     (case-sensitive) if present; SelectedIndex is -1 if no such sheet
//     exists (caller reports this as a fatal error, since an explicit
//     request that can't be satisfied should never silently fall back).
//  2. Otherwise, among non-empty sheets, select the one with the highest
//     PlausibilityScore. If exactly one sheet has the highest score,
//     select it. If more than one sheet ties for the highest score,
//     Ambiguous is true and SelectedIndex is -1 — this package never
//     silently guesses among equally plausible sheets (see the ingestion
//     contract's workbook/sheet-handling section).
//  3. If there are no non-empty sheets at all, SelectedIndex is -1.
func SelectSheet(candidates []SheetCandidate, requestedName string) SheetSelection {
	if requestedName != "" {
		for _, c := range candidates {
			if c.Name == requestedName {
				return SheetSelection{SelectedIndex: c.Index, Candidates: candidates}
			}
		}
		return SheetSelection{SelectedIndex: -1, Candidates: candidates}
	}

	var nonEmpty []SheetCandidate
	for _, c := range candidates {
		if !c.Empty {
			nonEmpty = append(nonEmpty, c)
		}
	}
	if len(nonEmpty) == 0 {
		return SheetSelection{SelectedIndex: -1, Candidates: candidates}
	}

	best := nonEmpty[0]
	tieCount := 1
	for _, c := range nonEmpty[1:] {
		if c.PlausibilityScore > best.PlausibilityScore {
			best = c
			tieCount = 1
		} else if c.PlausibilityScore == best.PlausibilityScore {
			tieCount++
		}
	}

	if tieCount > 1 {
		return SheetSelection{SelectedIndex: -1, Ambiguous: true, Candidates: candidates}
	}
	return SheetSelection{SelectedIndex: best.Index, Candidates: candidates}
}

// ScoreSheetName gives a small plausibility bonus to sheet names that look
// like they hold a financial statement (vs. e.g. "Notes", "Cover Page",
// "Assumptions"). Purely a tiebreaker signal, combined by the xlsx package
// with row/column density into PlausibilityScore.
func ScoreSheetName(name string) int {
	lower := strings.ToLower(name)
	for _, sig := range []string{"p&l", "p & l", "profit", "income statement", "balance sheet", "cash flow", "financials", "statement"} {
		if strings.Contains(lower, sig) {
			return 10
		}
	}
	for _, sig := range []string{"notes", "cover", "assumptions", "instructions", "readme", "summary"} {
		if strings.Contains(lower, sig) {
			return -5
		}
	}
	return 0
}
