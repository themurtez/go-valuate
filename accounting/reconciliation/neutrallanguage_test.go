package reconciliation

import (
	"math"
	"strings"
	"testing"
)

// forbiddenTerms locks the "neutral language" requirement (task section
// 57/74's non-goals list item "fraud detection"/"fraud conclusions"):
// no message, finding, or issue text anywhere in this package's source
// may use fraud/theft/suspicion/intent language. This mirrors
// journaldiagnostics's and vendorspend's identical permanent regression
// tests.
var forbiddenTerms = []string{
	"fraud", "fraudulent", "theft", "steal", "stolen", "embezzle",
	"suspicious", "suspect", "malicious", "criminal", "illegal",
	"intentional", "deliberate", "cover up", "scheme",
}

func TestNeutralLanguage_NoFraudTermsInMessages(t *testing.T) {
	messages := collectAllMessages()
	for _, m := range messages {
		lower := strings.ToLower(m)
		for _, term := range forbiddenTerms {
			if strings.Contains(lower, term) {
				t.Errorf("message %q contains forbidden term %q", m, term)
			}
		}
	}
}

// collectAllMessages runs a representative Calculate call exercising
// every Finding/Issue code path this package can produce, and returns
// every Message string produced.
func collectAllMessages() []string {
	var messages []string

	// Broad, deliberately messy input to hit as many code paths as
	// possible: duplicates, non-finite values, unmatched items,
	// ambiguity, reconciling items, mismatches.
	nanAmount := math.NaN()
	explained := 5.0
	in := Input{
		AccountID: "A",
		AsOfDate:  "2025-01-01",
		BookItems: []BookItem{
			{ItemID: "B1", Date: "2025-01-01", Amount: 100, Direction: DirectionOutflow},
			{ItemID: "B1", Date: "2025-01-01", Amount: 100, Direction: DirectionOutflow},
			{ItemID: "B2", Date: "2025-01-01", Amount: nanAmount, Direction: DirectionOutflow},
			{ItemID: "B3", Date: "2025-01-01", Amount: 500, Direction: DirectionOutflow},
			{ItemID: "B4", Date: "2025-01-01", Amount: 500, Direction: DirectionOutflow},
		},
		ExternalItems: []ExternalItem{
			{ItemID: "E1", Date: "2025-01-01", Amount: 500, Direction: DirectionOutflow},
			{ItemID: "E2", Date: "2025-01-01", Amount: 500, Direction: DirectionOutflow},
		},
		ConfirmedMatches: []ConfirmedMatch{
			{BookItemIDs: []string{"B3"}, ExternalItemIDs: []string{"UNKNOWN"}},
		},
		ReconcilingItems: []ReconcilingItem{
			{ItemID: "R1", Type: ReconcilingOutstandingCheck, Side: ReconcilingSideBook, Amount: -10, Date: "2024-01-01"},
			{ItemID: "R1", Type: ReconcilingOutstandingCheck, Side: ReconcilingSideBook, Amount: -20, Date: "2024-01-01"},
		},
		Policy: MatchingPolicy{
			StaleDaysThreshold:             1,
			Materiality:                    MaterialityPolicy{AbsoluteAmount: 0.01},
			EnableCompositeMatching:        true,
			MaxCompositeGroupSize:          2,
			MaxCandidatesPerItem:           1,
			MaxCompositeSearchCombinations: 10,
		},
		BookBalance:     BalanceInput{EndingBalance: &explained},
		ExternalBalance: BalanceInput{EndingBalance: floatPtrForTest(0)},
	}
	r := Calculate(in)

	for _, f := range r.Findings {
		messages = append(messages, f.Message)
	}
	for _, i := range r.Issues {
		messages = append(messages, i.Message)
	}
	return messages
}

func floatPtrForTest(v float64) *float64 { return &v }
