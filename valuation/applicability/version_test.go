package applicability

import (
	"testing"

	"github.com/themurtez/go-valuate/valuation/profile"
)

func TestCalculate_ResultsCarriesRulesVersion(t *testing.T) {
	res := Calculate(profile.Profile{})
	if res.RulesVersion != RulesVersion {
		t.Errorf("Results.RulesVersion = %q, want %q", res.RulesVersion, RulesVersion)
	}
	if RulesVersion == "" {
		t.Error("RulesVersion constant must not be empty")
	}
}
