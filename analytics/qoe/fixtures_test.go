package qoe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/earnings"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// loadFixtureDataset loads a financial.FinancialDataset from
// fixtures/<name>, mirroring financial/adjustments/fixtures_test.go's
// helper of the same name/shape.
func loadFixtureDataset(t *testing.T, name string) financial.FinancialDataset {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var ds financial.FinancialDataset
	if err := json.Unmarshal(b, &ds); err != nil {
		t.Fatalf("unmarshaling fixture %s: %v", name, err)
	}
	return ds
}

// loadAdjustmentFixtures loads fixtures/adjustments_by_business_type.json,
// keyed by business archetype ("hvac", "agency", "manufacturer", "saas") —
// every entry in this fixture is Period: "2025" only.
func loadAdjustmentFixtures(t *testing.T) map[string][]adjustments.Adjustment {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "adjustments_by_business_type.json"))
	if err != nil {
		t.Fatalf("reading adjustments fixture: %v", err)
	}
	var byArchetype map[string][]adjustments.Adjustment
	if err := json.Unmarshal(b, &byArchetype); err != nil {
		t.Fatalf("unmarshaling adjustments fixture: %v", err)
	}
	return byArchetype
}

// threeYearMeta builds a metrics.PeriodInfo map for the three consecutive
// fiscal years "2023", "2024", "2025", exactly matching every
// normalized_*_multi_year.json fixture's period set.
func threeYearMeta() map[financial.Period]metrics.PeriodInfo {
	return map[financial.Period]metrics.PeriodInfo{
		"2023": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2023},
		"2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
		"2025": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2025},
	}
}

// maintainableFromHistory runs financial/earnings.Calculate over history's
// NormalizedEBITDA or NormalizedSDE series (selected by useSDE) under
// StrategySimpleAverage — the simplest strategy, sufficient for exercising
// Calculate's maintainable-earnings wiring without this test file needing
// to cover every earnings.Strategy itself (that is earnings' own package's
// job).
func maintainableFromHistory(history []PeriodFigures, useSDE bool) earnings.Result {
	obs := make([]earnings.Observation, 0, len(history))
	for _, pf := range history {
		v := pf.NormalizedEBITDA
		if useSDE {
			v = pf.NormalizedSDE
		}
		obs = append(obs, earnings.Observation{
			Period:     string(pf.Period),
			PeriodType: earnings.PeriodTypeFiscalYear,
			Value:      v.Value,
			Available:  v.Available,
		})
	}
	return earnings.Calculate(obs, earnings.Options{Strategy: earnings.StrategySimpleAverage})
}

// earningsResultAt builds a minimal available earnings.Result carrying
// exactly value, for tests that need to force a specific maintainable-
// earnings figure (e.g. to exercise a near-zero/negative flag) without
// running the full runQoE two-pass derivation.
func earningsResultAt(value float64) earnings.Result {
	return earnings.Result{Available: true, Value: value}
}

// adjustmentAt builds a single Included adjustments.Adjustment with a
// generated Reason, for tests that only care about ID/Period/Type/Amount/
// Effect. effect defaults to the type's registered default when omitted
// (matching how a caller normally leaves Effect unset for a built-in
// type — see adjustments.Adjustment.Effect's own doc comment).
func adjustmentAt(id, period string, typ adjustments.Type, amount float64, effect ...adjustments.Effect) adjustments.Adjustment {
	adj := adjustments.Adjustment{
		ID:       adjustments.ID(id),
		Period:   financial.Period(period),
		Type:     typ,
		Amount:   amount,
		Reason:   "test fixture adjustment",
		Included: true,
	}
	if len(effect) > 0 {
		adj.Effect = effect[0]
	}
	return adj
}

// runQoE is the standard end-to-end path a test uses: build history via a
// first Calculate pass (Input.MaintainableEBITDA/SDE left zero), derive
// maintainable earnings from that history, then re-run Calculate with
// maintainable earnings supplied — mirroring how a real caller would
// sequence financial/earnings.Calculate (which itself needs qoe's
// normalized-EBITDA/SDE series as its Observations) ahead of a final QoE
// read. Both passes use identical Dataset/PeriodMeta/Adjustments, so
// History is byte-identical between them.
func runQoE(t *testing.T, dataset financial.FinancialDataset, meta map[financial.Period]metrics.PeriodInfo, adjs []adjustments.Adjustment, opts Options) Result {
	t.Helper()
	first := Calculate(Input{Dataset: dataset, PeriodMeta: meta, Adjustments: adjs}, opts)
	maintainableEBITDA := maintainableFromHistory(first.History, false)
	maintainableSDE := maintainableFromHistory(first.History, true)
	return Calculate(Input{
		Dataset:            dataset,
		PeriodMeta:         meta,
		Adjustments:        adjs,
		MaintainableEBITDA: maintainableEBITDA,
		MaintainableSDE:    maintainableSDE,
	}, opts)
}
