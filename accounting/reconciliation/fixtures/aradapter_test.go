package fixtures

import (
	"testing"
	"time"

	"github.com/themurtez/go-valuate/accounting/ap"
	apfixtures "github.com/themurtez/go-valuate/accounting/ap/fixtures"
	"github.com/themurtez/go-valuate/accounting/ar"
	arfixtures "github.com/themurtez/go-valuate/accounting/ar/fixtures"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

func asOfDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// TestARAdapter_ExactControlMatch uses a real ar.Result (control balance
// equal to the subledger total) to confirm ARSubledgerBookBalance/
// ARControlExternalBalance produce a RECONCILED result — task section 60:
// "exact... control-account cases," using the real package's own
// fixtures/results, not a recalculated stand-in.
func TestARAdapter_ExactControlMatch(t *testing.T) {
	receivables := arfixtures.HealthyPortfolio()
	var total float64
	for _, r := range receivables {
		total += r.OpenAmount
	}
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{
		AsOfDate:              asOfDate("2025-07-10"),
		ControlAccountBalance: &total,
	})
	if !result.Available || !result.ControlAccountReconciliation.Available {
		t.Fatalf("expected ar.Result available with control reconciliation, got %+v", result)
	}

	book := reconciliation.ARSubledgerBookBalance(result, "2025-07-10")
	external := reconciliation.ARControlExternalBalance(result, "2025-07-10")

	in := reconciliation.Input{
		AccountID:         "AR-SUBLEDGER",
		ExternalAccountID: "GL-1100-AR",
		AsOfDate:          "2025-07-10",
		Type:              reconciliation.TypeARControl,
		BookBalance:       book,
		ExternalBalance:   external,
		Policy:            reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	}
	r := reconciliation.Calculate(in)
	if r.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected RECONCILED for exact AR control match, got %s (equation=%+v)", r.Status, r.Equation)
	}
}

// TestARAdapter_MismatchCase uses ar/fixtures.GLSubledgerMismatch (a real
// sibling fixture, not a recalculated stand-in) to confirm the adapter
// correctly surfaces a genuine AR-control mismatch.
func TestARAdapter_MismatchCase(t *testing.T) {
	receivables, controlBalance := arfixtures.GLSubledgerMismatch()
	result := ar.Calculate(ar.Input{Receivables: receivables}, ar.Options{
		AsOfDate:              asOfDate("2025-07-10"),
		ControlAccountBalance: &controlBalance,
	})
	if !result.ControlAccountReconciliation.Available || result.ControlAccountReconciliation.Reconciled {
		t.Fatalf("expected ar's own control reconciliation to show a genuine mismatch, got %+v", result.ControlAccountReconciliation)
	}

	book := reconciliation.ARSubledgerBookBalance(result, "2025-07-10")
	external := reconciliation.ARControlExternalBalance(result, "2025-07-10")

	in := reconciliation.Input{
		AccountID:       "AR-SUBLEDGER",
		AsOfDate:        "2025-07-10",
		Type:            reconciliation.TypeARControl,
		BookBalance:     book,
		ExternalBalance: external,
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	}
	r := reconciliation.Calculate(in)
	if r.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected UNRECONCILED for the mismatch fixture, got %s", r.Status)
	}
}

// TestAPAdapter_ExactControlMatch mirrors TestARAdapter_ExactControlMatch
// for AP.
func TestAPAdapter_ExactControlMatch(t *testing.T) {
	payables := apfixtures.HealthyPayables()
	var total float64
	for _, p := range payables {
		total += p.OpenAmount
	}
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:              asOfDate("2025-07-10"),
		ControlAccountBalance: &total,
	})
	if !result.Available || !result.ControlAccountReconciliation.Available {
		t.Fatalf("expected ap.Result available with control reconciliation, got %+v", result)
	}

	book := reconciliation.APSubledgerBookBalance(result, "2025-07-10")
	external := reconciliation.APControlExternalBalance(result, "2025-07-10")

	in := reconciliation.Input{
		AccountID:       "AP-SUBLEDGER",
		AsOfDate:        "2025-07-10",
		Type:            reconciliation.TypeAPControl,
		BookBalance:     book,
		ExternalBalance: external,
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	}
	r := reconciliation.Calculate(in)
	if r.Status != reconciliation.StatusReconciled {
		t.Fatalf("expected RECONCILED for exact AP control match, got %s (equation=%+v)", r.Status, r.Equation)
	}
}

// TestAPAdapter_MismatchCase mirrors TestARAdapter_MismatchCase for AP.
func TestAPAdapter_MismatchCase(t *testing.T) {
	payables, controlBalance := apfixtures.GLSubledgerMismatch()
	result := ap.Calculate(ap.Input{Payables: payables}, ap.Options{
		AsOfDate:              asOfDate("2025-07-10"),
		ControlAccountBalance: &controlBalance,
	})
	if !result.ControlAccountReconciliation.Available || result.ControlAccountReconciliation.Reconciled {
		t.Fatalf("expected ap's own control reconciliation to show a genuine mismatch, got %+v", result.ControlAccountReconciliation)
	}

	book := reconciliation.APSubledgerBookBalance(result, "2025-07-10")
	external := reconciliation.APControlExternalBalance(result, "2025-07-10")

	in := reconciliation.Input{
		AccountID:       "AP-SUBLEDGER",
		AsOfDate:        "2025-07-10",
		Type:            reconciliation.TypeAPControl,
		BookBalance:     book,
		ExternalBalance: external,
		Policy:          reconciliation.MatchingPolicy{AmountTolerance: 0.01},
	}
	r := reconciliation.Calculate(in)
	if r.Status != reconciliation.StatusUnreconciled {
		t.Fatalf("expected UNRECONCILED for the mismatch fixture, got %s", r.Status)
	}
}

// TestARAdapter_UnavailableWhenResultUnavailable confirms the adapter
// never fabricates a balance when the upstream ar.Result itself was not
// Available.
func TestARAdapter_UnavailableWhenResultUnavailable(t *testing.T) {
	_, ok := reconciliation.ARSubledgerBalance(ar.Result{})
	if ok {
		t.Fatalf("expected unavailable balance for an unavailable ar.Result")
	}
}

// TestARAdapter_BookItemsFromReceivables confirms
// BookItemsFromARReceivables preserves signed OpenAmount (including a
// credit memo's negative figure) without reinterpreting it.
func TestARAdapter_BookItemsFromReceivables(t *testing.T) {
	receivables := arfixtures.HealthyPortfolio()
	items := reconciliation.BookItemsFromARReceivables(receivables)
	if len(items) != len(receivables) {
		t.Fatalf("expected 1 BookItem per Receivable, got %d vs %d", len(items), len(receivables))
	}
	for i, it := range items {
		if it.SignedAmount() != receivables[i].OpenAmount {
			t.Errorf("expected item %d's signed amount to preserve OpenAmount %v, got %v", i, receivables[i].OpenAmount, it.SignedAmount())
		}
	}
}

// TestAPAdapter_BookItemsFromPayables mirrors the AR test for AP.
func TestAPAdapter_BookItemsFromPayables(t *testing.T) {
	payables := apfixtures.HealthyPayables()
	items := reconciliation.BookItemsFromAPPayables(payables)
	if len(items) != len(payables) {
		t.Fatalf("expected 1 BookItem per Payable, got %d vs %d", len(items), len(payables))
	}
	for i, it := range items {
		if it.SignedAmount() != payables[i].OpenAmount {
			t.Errorf("expected item %d's signed amount to preserve OpenAmount %v, got %v", i, payables[i].OpenAmount, it.SignedAmount())
		}
	}
}
