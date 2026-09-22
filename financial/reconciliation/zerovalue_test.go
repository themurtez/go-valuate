package reconciliation

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

// TestZeroValue_Options_RunDoesNotPanic proves reconciliation.Options{}
// (no Tolerance, no Reported) is a safe zero value: Run falls back to
// DefaultTolerance, and every reported-vs-reconstructed check simply
// returns NOT_APPLICABLE (never a fabricated comparison or a panic on a
// nil Reported map).
func TestZeroValue_Options_RunDoesNotPanic(t *testing.T) {
	dataset := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 100000},
		},
	}
	result := Run(dataset, Options{})
	if len(result.Checks) == 0 {
		t.Fatal("expected at least the dataset-integrity checks to run under zero-value Options")
	}
	for _, c := range result.Checks {
		if c.Code == CheckGrossProfit && c.Status != StatusNotApplicable {
			t.Errorf("expected GROSS_PROFIT_RECONCILES to be NOT_APPLICABLE with no Options.Reported supplied, got %s", c.Status)
		}
	}
}

// TestZeroValue_Dataset_RunDoesNotPanic proves an entirely zero-value
// financial.FinancialDataset{} (no Currency, no Items) does not panic — it
// produces a DATASET_NOT_EMPTY WARNING (an empty dataset is unusual but
// not itself malformed data — see the package's PASS/WARNING/FAIL/
// NOT_APPLICABLE status design), never a crash on a nil Items slice or an
// empty Currency string.
func TestZeroValue_Dataset_RunDoesNotPanic(t *testing.T) {
	result := Run(financial.FinancialDataset{}, Options{})
	found := false
	for _, c := range result.Checks {
		if c.Code == CheckDatasetNotEmpty {
			found = true
			if c.Status != StatusWarning {
				t.Errorf("expected DATASET_NOT_EMPTY to be WARNING for a genuinely empty dataset, got %s", c.Status)
			}
		}
	}
	if !found {
		t.Fatal("expected a DATASET_NOT_EMPTY check to run even for a zero-value dataset")
	}
}
