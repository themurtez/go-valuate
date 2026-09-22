package ai

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
)

func TestBuildRequests_SplitsByMaxCandidateRows(t *testing.T) {
	rows := []SourceRow{
		{RowID: "row-1", Period: "2025", Amount: 1},
		{RowID: "row-2", Period: "2025", Amount: 2},
		{RowID: "row-3", Period: "2025", Amount: 3},
	}
	reqs := BuildRequests(rows, adjustments.AllTypes(), "", nil, nil, BatchPolicy{MaxCandidateRows: 2})
	if len(reqs) != 2 {
		t.Fatalf("expected 2 chunks for 3 rows at MaxCandidateRows=2, got %d", len(reqs))
	}
	if len(reqs[0].Candidates) != 2 || len(reqs[1].Candidates) != 1 {
		t.Fatalf("expected chunk sizes [2,1], got [%d,%d]", len(reqs[0].Candidates), len(reqs[1].Candidates))
	}
}

func TestBuildRequests_PreservesSourceIdentityAcrossChunks(t *testing.T) {
	rows := []SourceRow{
		{RowID: "row-1", Period: "2025", Amount: 1},
		{RowID: "row-2", Period: "2025", Amount: 2},
		{RowID: "row-3", Period: "2025", Amount: 3},
	}
	reqs := BuildRequests(rows, adjustments.AllTypes(), "", nil, nil, BatchPolicy{MaxCandidateRows: 2})
	var seen []string
	for _, req := range reqs {
		for _, c := range req.Candidates {
			seen = append(seen, c.RowID)
		}
	}
	want := []string{"row-1", "row-2", "row-3"}
	if len(seen) != len(want) {
		t.Fatalf("expected %v, got %v", want, seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("expected order-preserving identity %v, got %v", want, seen)
		}
	}
}

func TestBuildRequests_SplitsByCharacterBudget(t *testing.T) {
	longLabel := strings.Repeat("x", 500)
	rows := []SourceRow{
		{RowID: "row-1", Period: "2025", Label: longLabel, Amount: 1},
		{RowID: "row-2", Period: "2025", Label: longLabel, Amount: 2},
	}
	reqs := BuildRequests(rows, adjustments.AllTypes(), "", nil, nil, BatchPolicy{MaxRequestCharacters: 600})
	if len(reqs) < 2 {
		t.Fatalf("expected the character budget to force at least 2 chunks, got %d", len(reqs))
	}
}

func TestBuildRequests_SingleOversizedRowNeverDropped(t *testing.T) {
	longLabel := strings.Repeat("x", 10000)
	rows := []SourceRow{{RowID: "row-1", Period: "2025", Label: longLabel, Amount: 1}}
	reqs := BuildRequests(rows, adjustments.AllTypes(), "", nil, nil, BatchPolicy{MaxRequestCharacters: 100})
	if len(reqs) != 1 || len(reqs[0].Candidates) != 1 {
		t.Fatalf("expected the oversized row to form its own chunk rather than being dropped, got %+v", reqs)
	}
}

func TestBuildRequests_EmptyCandidates_NoRequests(t *testing.T) {
	reqs := BuildRequests(nil, adjustments.AllTypes(), "", nil, nil, BatchPolicy{})
	if len(reqs) != 0 {
		t.Fatalf("expected no requests for an empty candidate set, got %d", len(reqs))
	}
}

func TestBuildRequests_ContextAndMultiYearScopedPerChunk(t *testing.T) {
	rows := []SourceRow{
		{RowID: "row-1", Period: "2025", Amount: 1},
		{RowID: "row-2", Period: "2025", Amount: 2},
	}
	ctxByRow := map[string][]ContextRow{
		"row-1": {{Label: "Nearby Row"}},
		"row-2": {{Label: "Another Nearby Row"}},
	}
	myByRow := map[string][]MultiYearValue{
		"row-1": {{Period: "2024", Amount: 900}},
	}
	reqs := BuildRequests(rows, adjustments.AllTypes(), "", ctxByRow, myByRow, BatchPolicy{MaxCandidateRows: 1})
	if len(reqs) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(reqs))
	}
	if _, ok := reqs[0].ContextRows["row-1"]; !ok {
		t.Errorf("expected chunk 0 to carry row-1's context rows")
	}
	if _, ok := reqs[0].ContextRows["row-2"]; ok {
		t.Errorf("expected chunk 0 to NOT carry row-2's context rows (wrong chunk)")
	}
	if _, ok := reqs[0].MultiYearValues["row-1"]; !ok {
		t.Errorf("expected chunk 0 to carry row-1's multi-year values")
	}
}

// --- Privacy tests -------------------------------------------------------

// TestPrivacy_RequestOnlyContainsAllowedFields inspects a marshaled Request
// and asserts it contains only the fields section 7 allows: row id, period,
// label, parent label, code, statement type, amount, row kind, plus the
// caller-supplied allowed types/industry context/context rows/multi-year
// values. No customer/user identity, bank details, tax IDs, or unrelated
// fields.
func TestPrivacy_RequestOnlyContainsAllowedFields(t *testing.T) {
	req := Request{
		Candidates: []SourceRow{
			{RowID: "row-1", Period: "2025", Label: "Officer Compensation", ParentLabel: "Operating Expenses", Code: financial.CodeOpexOwnerComp, StatementType: financial.StatementIncomeStatement, Amount: 220000},
		},
		AllowedTypes:    BuildAllowedTypes(adjustments.AllTypes()),
		IndustryContext: "residential HVAC contractor",
	}
	forbidden := []string{
		"customer", "user_id", "userId", "bank", "tax_id", "taxId", "ssn", "account_number", "email", "phone",
	}
	data := mustMarshalRequest(t, req)
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(data), strings.ToLower(f)) {
			t.Errorf("request payload unexpectedly contains forbidden field/substring %q: %s", f, data)
		}
	}
}

func TestPrivacy_ContextRowsNeverCarryAmounts(t *testing.T) {
	// ContextRow's own type has no Amount field at all — this test pins that
	// structural guarantee via JSON round-trip so a future field addition
	// cannot silently reintroduce one without a visible test failure here.
	cr := ContextRow{Label: "Nearby Row", ParentLabel: "Section"}
	data := mustMarshalAny(t, cr)
	if strings.Contains(strings.ToLower(data), "amount") {
		t.Fatalf("ContextRow must never carry an amount field, got %s", data)
	}
}

func mustMarshalRequest(t *testing.T, req Request) string {
	t.Helper()
	return mustMarshalAny(t, req)
}
