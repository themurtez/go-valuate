package labor_test

import (
	"testing"

	"github.com/themurtez/go-valuate/accounting/labor"
)

func TestVersions_EchoedOnResult(t *testing.T) {
	r := labor.Calculate(fullInput(), labor.DefaultPolicy())
	if r.SchemaVersion != labor.SchemaVersion {
		t.Errorf("Result.SchemaVersion = %q, want %q", r.SchemaVersion, labor.SchemaVersion)
	}
	if r.FormulaVersion != labor.FormulaVersion {
		t.Errorf("Result.FormulaVersion = %q, want %q", r.FormulaVersion, labor.FormulaVersion)
	}
	if labor.SchemaVersion == "" || labor.FormulaVersion == "" {
		t.Error("expected non-empty version constants")
	}
}
