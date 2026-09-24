package closechecklist_test

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closechecklist/fixtures"
)

// bannedTerms are misconduct/performance-judgment words this package's
// Finding/Blocker/Issue messages must never contain — section 38.
var bannedTerms = []string{
	"fail to", "failed to", "neglect", "poor performance", "bad close",
	"incompetent", "careless", "lazy", "irresponsible", "blame",
	"should have", "did not bother", "mistake by", "error by",
}

func TestNeutralLanguage_NoMisconductOrPerformanceJudgment(t *testing.T) {
	in := fixtures.CleanInstance()
	// Force a variety of findings/blockers to exist.
	in.Gates = fixtures.GatesWithFailedBankReconciliation()
	states := fixtures.CleanCloseTaskStates()
	for i := range states {
		if states[i].TaskCode == "controller_review" {
			states[i].SignOffs = nil
		}
	}
	in.TaskStates = states

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	check := func(msg string) {
		lower := strings.ToLower(msg)
		for _, term := range bannedTerms {
			if strings.Contains(lower, term) {
				t.Errorf("message contains banned term %q: %q", term, msg)
			}
		}
	}

	for _, b := range res.Blockers {
		check(b.Message)
	}
	for _, f := range res.Findings {
		check(f.Message)
	}
	for _, tr := range res.Tasks {
		for _, b := range tr.Blockers {
			check(b.Message)
		}
		for _, f := range tr.Findings {
			check(f.Message)
		}
	}
	for _, pc := range res.PeriodConsistency {
		check(pc.Message)
	}
	for _, iss := range res.Issues {
		check(iss.Message)
	}
}

// TestNeutralLanguage_MessagesAreFactual spot-checks that the two
// example messages from the task spec's section 38 are the kind of
// factual, subject-object-condition phrasing this package produces.
func TestNeutralLanguage_MessagesAreFactual(t *testing.T) {
	in := fixtures.CleanInstance()
	in.Gates = fixtures.GatesWithFailedBankReconciliation()

	res := closechecklist.Calculate(in, closechecklist.DefaultPolicy())

	foundGateMessage := false
	for _, b := range res.Blockers {
		if b.TaskCode == "bank_rec" && strings.Contains(b.Message, "gate") && strings.Contains(b.Message, "FAIL") {
			foundGateMessage = true
		}
	}
	if !foundGateMessage {
		t.Errorf("expected a factual gate-failure blocker message referencing the gate and its status, got %+v", res.Blockers)
	}
}
