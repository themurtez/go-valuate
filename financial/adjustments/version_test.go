package adjustments

import (
	"testing"

	"github.com/themurtez/go-valuate/financial/metrics"
)

func TestApply_ResultCarriesSemanticsVersion(t *testing.T) {
	snapshot := metrics.Snapshot{Period: "2025", EBITDA: metrics.AvailableValue(500000)}
	res := Apply(snapshot, nil)
	if res.SemanticsVersion != SemanticsVersion {
		t.Errorf("Result.SemanticsVersion = %q, want %q", res.SemanticsVersion, SemanticsVersion)
	}
	if SemanticsVersion == "" {
		t.Error("SemanticsVersion constant must not be empty")
	}
}
