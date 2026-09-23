package closequality_test

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/closequality/fixtures"
)

// prohibitedTerms are words this package's own generated Finding/Issue
// text must never contain — this is a bookkeeping/close-quality
// diagnostic, not an audit opinion or a fraud engine (see package doc
// comment). This also guards against a prohibited term leaking through
// from an upstream journaldiagnostics Message, which that package's own
// safety_test.go already independently guarantees never happens at the
// source.
var prohibitedTerms = []string{
	"fraud", "fraudulent", "theft", "embezzlement", "stolen", "manipulation", "misconduct",
	"misstatement", "unreliable books", "accountant error", "audit opinion",
}

func scanForProhibitedLanguage(t *testing.T, label, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, term := range prohibitedTerms {
		if strings.Contains(lower, term) {
			t.Errorf("%s contains prohibited term %q: %q", label, term, text)
		}
	}
}

// TestSafety_NoProhibitedLanguageInFindings scans every Finding message
// across a broad mix of scenarios for prohibited language.
func TestSafety_NoProhibitedLanguageInFindings(t *testing.T) {
	messyInput := fixtures.CleanCloseInput()
	messyInput.AR = fixtures.ARResultControlMismatch()
	messyInput.AP = fixtures.APResultControlMismatch()
	messyInput.Ledger = fixtures.UnbalancedLedger()
	messyInput.LedgerValidation = nil
	messyInput.CloseTasks = fixtures.BlockedCloseTasks()
	messyInput.Reconciliations = fixtures.UnreconciledCashStatus()

	scenarios := []closequality.Result{
		closequality.Calculate(fixtures.CleanCloseInput(), fixtures.CleanPolicy()),
		closequality.Calculate(messyInput, fixtures.CleanPolicy()),
	}

	seen := map[closequality.FindingCode]bool{}
	for _, r := range scenarios {
		for _, f := range append(append(append([]closequality.Finding{}, r.Blockers...), r.Warnings...), r.Information...) {
			scanForProhibitedLanguage(t, "Finding "+string(f.Code)+" Message", f.Message)
			seen[f.Code] = true
		}
	}
	if len(seen) < 5 {
		t.Errorf("expected these scenarios to exercise at least 5 distinct FindingCodes, got %d: %v", len(seen), seen)
	}
}

// TestSafety_NoProhibitedLanguageInIssues scans Issue messages.
func TestSafety_NoProhibitedLanguageInIssues(t *testing.T) {
	in := fixtures.CleanCloseInput()
	in.Ledger = fixtures.UnbalancedLedger()
	in.LedgerValidation = nil
	policy := fixtures.CleanPolicy()
	policy.AccountExpectations = []closequality.AccountExpectation{{}} // invalid: no AccountID

	result := closequality.Calculate(in, policy)
	for _, iss := range result.Issues {
		scanForProhibitedLanguage(t, "Issue "+string(iss.Code)+" Message", iss.Message)
	}
}
