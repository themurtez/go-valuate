package report

import "testing"

func TestBuild_ReportCarriesSchemaVersion(t *testing.T) {
	rep := Build(BuildInput{})
	if rep.SchemaVersion != SchemaVersion {
		t.Errorf("Report.SchemaVersion = %q, want %q", rep.SchemaVersion, SchemaVersion)
	}
	if SchemaVersion == "" {
		t.Error("SchemaVersion constant must not be empty")
	}
}
