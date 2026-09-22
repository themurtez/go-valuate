package metrics

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCalculate_ResultCarriesFormulaVersion(t *testing.T) {
	ds := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "2025", Amount: 100000},
		},
	}
	res := Calculate(ds, Options{})
	if res.FormulaVersion != FormulaVersion {
		t.Errorf("Result.FormulaVersion = %q, want %q", res.FormulaVersion, FormulaVersion)
	}
	if FormulaVersion == "" {
		t.Error("FormulaVersion constant must not be empty")
	}
}
