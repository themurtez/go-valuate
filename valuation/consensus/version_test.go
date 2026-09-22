package consensus

import "testing"

func TestCalculate_ResultCarriesFormulaVersion(t *testing.T) {
	res := Calculate([]Input{{Method: "A", Value: 100}}, Options{})
	if res.FormulaVersion != FormulaVersion {
		t.Errorf("Result.FormulaVersion = %q, want %q", res.FormulaVersion, FormulaVersion)
	}
	if FormulaVersion == "" {
		t.Error("FormulaVersion constant must not be empty")
	}
}
