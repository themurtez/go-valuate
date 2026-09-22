package qoe

import "testing"

func TestCalculate_ResultCarriesFormulaVersion(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{})
	if res.FormulaVersion != FormulaVersion {
		t.Errorf("Result.FormulaVersion = %q, want %q", res.FormulaVersion, FormulaVersion)
	}
	if FormulaVersion == "" {
		t.Error("FormulaVersion constant must not be empty")
	}
}

func TestComputeScore_CarriesScoreVersion(t *testing.T) {
	ds := loadFixtureDataset(t, "normalized_manufacturer_multi_year.json")
	res := Calculate(Input{Dataset: ds, PeriodMeta: threeYearMeta()}, Options{ComputeScore: true})
	if res.Score == nil {
		t.Fatal("expected Score to be populated")
	}
	if res.Score.Version != ScoreVersion {
		t.Errorf("Score.Version = %q, want %q", res.Score.Version, ScoreVersion)
	}
	if ScoreVersion == "" {
		t.Error("ScoreVersion constant must not be empty")
	}
}
