package ai

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func candidateRows() []SourceRow {
	return []SourceRow{
		{RowID: "row-1", Period: "2025", Label: "Officer Compensation", Code: financial.CodeOpexOwnerComp, Amount: 220000},
		{RowID: "row-2", Period: "2025", Label: "Product Sales", Code: financial.CodeRevProduct, Amount: 500000},
		{RowID: "row-3", Period: "2025", Label: "Vehicle Lease", Code: financial.CodeOpexVehicle, Amount: 9600},
		{RowID: "row-4", Period: "2025", Label: "Total Operating Expenses", RowKind: financial.RowKindTotal, Amount: 400000},
		{RowID: "row-5", Period: "2025", Label: "Ambiguous OCR Fee", Code: financial.CodeOpexOther, Amount: 500, AmbiguousOCR: true},
	}
}

func TestSelectCandidates_CodeRulesDisabledByDefault(t *testing.T) {
	got := SelectCandidates(candidateRows(), CandidatePolicy{})
	if len(got) != 0 {
		t.Fatalf("expected no candidates with EnableCodeRules false and no explicit IDs, got %d: %+v", len(got), got)
	}
}

func TestSelectCandidates_CodeRules_MatchesOwnerCompAndVehicle(t *testing.T) {
	got := SelectCandidates(candidateRows(), CandidatePolicy{EnableCodeRules: true})
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates (owner comp + vehicle), got %d: %+v", len(got), got)
	}
	for _, r := range got {
		if r.RowID != "row-1" && r.RowID != "row-3" {
			t.Errorf("unexpected candidate row selected: %s", r.RowID)
		}
	}
}

func TestSelectCandidates_ExcludesStructuralRows(t *testing.T) {
	rows := []SourceRow{{RowID: "row-total", Period: "2025", Code: financial.CodeOpexOwnerComp, RowKind: financial.RowKindTotal, Amount: 1000}}
	got := SelectCandidates(rows, CandidatePolicy{EnableCodeRules: true})
	if len(got) != 0 {
		t.Fatalf("expected structural row excluded, got %+v", got)
	}
}

func TestSelectCandidates_ExcludesAmbiguousOCRRows(t *testing.T) {
	rows := []SourceRow{{RowID: "row-amb", Period: "2025", Code: financial.CodeOpexOwnerComp, AmbiguousOCR: true, Amount: 1000}}
	got := SelectCandidates(rows, CandidatePolicy{EnableCodeRules: true})
	if len(got) != 0 {
		t.Fatalf("expected ambiguous OCR row excluded, got %+v", got)
	}
}

func TestSelectCandidates_ExplicitRowIDs_AlwaysIncluded(t *testing.T) {
	rows := candidateRows()
	got := SelectCandidates(rows, CandidatePolicy{ExplicitRowIDs: []string{"row-2"}})
	if len(got) != 1 || got[0].RowID != "row-2" {
		t.Fatalf("expected explicit row-2 included even though it's revenue, got %+v", got)
	}
}

func TestSelectCandidates_ExplicitRowIDs_StillExcludesStructuralAndAmbiguous(t *testing.T) {
	rows := candidateRows()
	got := SelectCandidates(rows, CandidatePolicy{ExplicitRowIDs: []string{"row-4", "row-5"}})
	if len(got) != 0 {
		t.Fatalf("expected explicit structural/ambiguous rows still excluded, got %+v", got)
	}
}

func TestSelectCandidates_MaxRows(t *testing.T) {
	rows := candidateRows()
	got := SelectCandidates(rows, CandidatePolicy{EnableCodeRules: true, MaxRows: 1})
	if len(got) != 1 {
		t.Fatalf("expected MaxRows=1 truncation, got %d", len(got))
	}
}

func TestSelectCandidates_ExplicitRowsPrecedeCodeRuleRowsForTruncation(t *testing.T) {
	rows := candidateRows()
	got := SelectCandidates(rows, CandidatePolicy{EnableCodeRules: true, ExplicitRowIDs: []string{"row-2"}, MaxRows: 1})
	if len(got) != 1 || got[0].RowID != "row-2" {
		t.Fatalf("expected explicit row-2 to survive truncation ahead of code-rule matches, got %+v", got)
	}
}

func TestSelectCandidates_NeverMutatesInput(t *testing.T) {
	rows := candidateRows()
	orig := make([]SourceRow, len(rows))
	copy(orig, rows)
	_ = SelectCandidates(rows, CandidatePolicy{EnableCodeRules: true})
	for i := range rows {
		if rows[i] != orig[i] {
			t.Fatalf("SelectCandidates mutated input row %d", i)
		}
	}
}
