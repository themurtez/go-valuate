package management

import (
	"testing"

	"github.com/themurtez/go-valuate/analytics/anomalies"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/qoe"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

func TestCalculate_EmptyInput(t *testing.T) {
	report := Calculate(Input{})
	if report.Available {
		t.Fatalf("expected Available == false for a zero-value Input")
	}
	if report.FormulaVersion != FormulaVersion {
		t.Errorf("FormulaVersion = %q, want %q", report.FormulaVersion, FormulaVersion)
	}
	if !HasErrors(report.Errors) {
		t.Errorf("expected an error Issue for a zero-value Input")
	}
	if report.Coverage.TotalModules != 0 || report.Coverage.AvailableModules != 0 || len(report.Coverage.MissingModules) != 0 {
		t.Errorf("expected zero-value Coverage when Available is false, got %+v", report.Coverage)
	}
}

func TestCalculate_FullFixture_Available(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.Available {
		t.Fatalf("expected Available, errors=%+v", report.Errors)
	}
	if HasErrors(report.Errors) {
		t.Errorf("expected no errors for the full fixture, got %+v", report.Errors)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("expected no warnings for the full fixture (every module supplied), got %+v", report.Warnings)
	}
}

func TestCalculate_HistoricalSeries_ChronologicalOrder(t *testing.T) {
	report := Calculate(fullFixture())
	periods := report.HistoricalSeries.Periods
	if len(periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(periods))
	}
	if periods[0].Period != "FY2023" || periods[1].Period != "FY2024" {
		t.Fatalf("expected chronological order [FY2023, FY2024], got [%s, %s]", periods[0].Period, periods[1].Period)
	}
}

// TestCalculate_HistoricalSeries_FromDatasetFallback proves
// HistoricalSeries recomputes metrics.Calculate itself when Metrics is
// empty but Dataset carries items — a caller supplying only a
// FinancialDataset (per Prompt 34's accepted-input list) still gets a
// historical series, not silence.
func TestCalculate_HistoricalSeries_FromDatasetFallback(t *testing.T) {
	dataset := financial.FinancialDataset{
		Currency: "USD",
		Items: []financial.NormalizedItem{
			{Code: financial.CodeRevProduct, Period: "FY2024", Amount: 1_000_000},
		},
	}
	report := Calculate(Input{Dataset: dataset})
	if !report.Available {
		t.Fatalf("expected Available, errors=%+v", report.Errors)
	}
	if !report.HistoricalSeries.Available {
		t.Fatalf("expected HistoricalSeries.Available from a Dataset-only Input")
	}
	if len(report.HistoricalSeries.Periods) != 1 {
		t.Fatalf("expected 1 period, got %d", len(report.HistoricalSeries.Periods))
	}
	if got := report.HistoricalSeries.Periods[0].TotalRevenue; !got.Available || got.Amount != 1_000_000 {
		t.Errorf("TotalRevenue = %+v, want {true 1000000}", got)
	}
}

func TestCalculate_ProfitabilitySeries_FallsBackToMetricsWhenRatiosUnavailable(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics, PeriodMeta: fullFixture().PeriodMeta}
	report := Calculate(in)
	if !report.ProfitabilitySeries.Available {
		t.Fatalf("expected ProfitabilitySeries.Available from Metrics alone")
	}
	if report.ProfitabilitySeries.Source != "metrics" {
		t.Errorf("Source = %q, want %q", report.ProfitabilitySeries.Source, "metrics")
	}
	if len(report.ProfitabilitySeries.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(report.ProfitabilitySeries.Periods))
	}
	last := report.ProfitabilitySeries.Periods[1]
	if !last.EBITDAMargin.Available || last.EBITDAMargin.Amount != 0.14 {
		t.Errorf("EBITDAMargin = %+v, want {true 0.14}", last.EBITDAMargin)
	}
	if last.ReturnOnAssets.Available {
		t.Errorf("expected ReturnOnAssets unavailable from the metrics-only fallback (ratios-only field)")
	}
}

func TestCalculate_ProfitabilitySeries_PrefersRatios(t *testing.T) {
	report := Calculate(fullFixture())
	if report.ProfitabilitySeries.Source != "ratios" {
		t.Errorf("Source = %q, want %q", report.ProfitabilitySeries.Source, "ratios")
	}
	last := report.ProfitabilitySeries.Periods[len(report.ProfitabilitySeries.Periods)-1]
	if !last.ReturnOnAssets.Available {
		t.Errorf("expected ReturnOnAssets available when Ratios is supplied")
	}
}

func TestCalculate_LiquidityLeverageSeries_UnavailableWithoutRatios(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics}
	report := Calculate(in)
	if report.LiquidityLeverageSeries.Available {
		t.Errorf("expected LiquidityLeverageSeries unavailable without Ratios")
	}
}

func TestCalculate_CashFlowSeries_JoinsBridgeAndConversion(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.CashFlowSeries.Available {
		t.Fatalf("expected CashFlowSeries.Available")
	}
	last := report.CashFlowSeries.Periods[len(report.CashFlowSeries.Periods)-1]
	if !last.FreeCashFlow.Available || last.FreeCashFlow.Amount != 500_000 {
		t.Errorf("FreeCashFlow = %+v, want {true 500000}", last.FreeCashFlow)
	}
	if !last.EBITDAToFreeCashFlow.Available {
		t.Errorf("expected EBITDAToFreeCashFlow joined in from Conversion")
	}
	if !report.CashFlowSeries.MonthsOfRunway.Available {
		t.Errorf("expected MonthsOfRunway available from CashRunway")
	}
}

func TestCalculate_VarianceTables_OmitsEmptyTables(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.VarianceTables.Available {
		t.Fatalf("expected VarianceTables.Available")
	}
	for _, tbl := range report.VarianceTables.Tables {
		if tbl.Label == "Material Exceptions" {
			t.Errorf("expected no 'Material Exceptions' table when the fixture supplies none, got one with %d lines", len(tbl.Lines))
		}
	}
	foundAll, foundFavorable := false, false
	for _, tbl := range report.VarianceTables.Tables {
		switch tbl.Label {
		case "All Line Items":
			foundAll = true
		case "Top Favorable Variances":
			foundFavorable = true
		}
	}
	if !foundAll || !foundFavorable {
		t.Errorf("expected 'All Line Items' and 'Top Favorable Variances' tables, got %+v", report.VarianceTables.Tables)
	}
}

func TestCalculate_ForecastTables(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.ForecastTables.Available {
		t.Fatalf("expected ForecastTables.Available")
	}
	if len(report.ForecastTables.Scenarios) != 1 {
		t.Fatalf("expected 1 scenario, got %d", len(report.ForecastTables.Scenarios))
	}
	scenario := report.ForecastTables.Scenarios[0]
	if scenario.Name != "Base Case" || scenario.Type != "base" {
		t.Errorf("scenario = %+v, want Name=Base Case Type=base", scenario)
	}
	if len(scenario.Periods) != 2 {
		t.Fatalf("expected 2 forecast periods, got %d", len(scenario.Periods))
	}
	if !scenario.Periods[0].FreeCashFlow.Available || scenario.Periods[0].FreeCashFlow.Amount != 550_000 {
		t.Errorf("FreeCashFlow = %+v, want {true 550000} (joined from CashFlow by Period label)", scenario.Periods[0].FreeCashFlow)
	}
}

func TestCalculate_TopIssues_MergesAndSorts(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.TopIssues.Available {
		t.Fatalf("expected TopIssues.Available")
	}
	if report.TopIssues.Summary.Total != len(report.TopIssues.Issues) {
		t.Errorf("Summary.Total = %d, want %d", report.TopIssues.Summary.Total, len(report.TopIssues.Issues))
	}
	if !report.TopIssues.Summary.ReviewRecommended {
		t.Errorf("expected ReviewRecommended true when Total > 0")
	}

	// The fixture's covenant is StatusPass + WarningBufferWithinBuffer (a
	// near-breach), so it should surface as a warning-severity issue.
	foundCovenant := false
	for _, issue := range report.TopIssues.Issues {
		if issue.Source == TopIssueSourceCovenants {
			foundCovenant = true
			if issue.Severity != TopIssueSeverityWarning {
				t.Errorf("covenant near-breach severity = %s, want %s", issue.Severity, TopIssueSeverityWarning)
			}
		}
	}
	if !foundCovenant {
		t.Errorf("expected a covenants-sourced TopIssue for the near-breach fixture entry")
	}

	foundDebt := false
	for _, issue := range report.TopIssues.Issues {
		if issue.Source == TopIssueSourceDebt {
			foundDebt = true
			if issue.Code != string(debt.FlagBelowMinimumFixedChargeCoverage) {
				t.Errorf("debt issue Code = %q, want %q", issue.Code, debt.FlagBelowMinimumFixedChargeCoverage)
			}
		}
	}
	if !foundDebt {
		t.Errorf("expected a debt-sourced TopIssue for the fixture's Debt.Flags entry")
	}

	// Severity ordering: no severity should appear after a lower-urgency
	// one earlier in the slice.
	for i := 1; i < len(report.TopIssues.Issues); i++ {
		if severityRank(report.TopIssues.Issues[i].Severity) < severityRank(report.TopIssues.Issues[i-1].Severity) {
			t.Fatalf("Issues not ordered by severity at index %d: %+v", i, report.TopIssues.Issues)
		}
	}
}

func TestCalculate_TopIssues_PassingCovenantOmitted(t *testing.T) {
	in := fullFixture()
	in.Covenants.Tests[0].WarningBufferStatus = "outside_buffer"
	report := Calculate(in)
	for _, issue := range report.TopIssues.Issues {
		if issue.Source == TopIssueSourceCovenants {
			t.Fatalf("expected no covenant issue for a passing test safely outside its buffer, got %+v", issue)
		}
	}
}

func TestCalculate_TopIssues_UnavailableWithoutContributingModules(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics}
	report := Calculate(in)
	if report.TopIssues.Available {
		t.Errorf("expected TopIssues unavailable without Anomalies/QoE/Covenants")
	}
}

func TestCalculate_ExecutiveSummary_KPIsAndChange(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.ExecutiveSummary.Available {
		t.Fatalf("expected ExecutiveSummary.Available")
	}
	if report.ExecutiveSummary.Period != "FY2024" {
		t.Errorf("Period = %s, want FY2024", report.ExecutiveSummary.Period)
	}

	var revenueKPI *KPI
	for i := range report.ExecutiveSummary.KPIs {
		if report.ExecutiveSummary.KPIs[i].Label == "Total Revenue" {
			revenueKPI = &report.ExecutiveSummary.KPIs[i]
		}
	}
	if revenueKPI == nil {
		t.Fatalf("expected a Total Revenue KPI, got %+v", report.ExecutiveSummary.KPIs)
	}
	if revenueKPI.Value.Amount != 5_000_000 {
		t.Errorf("Value.Amount = %v, want 5000000", revenueKPI.Value.Amount)
	}
	if !revenueKPI.PriorValue.Available || revenueKPI.PriorValue.Amount != 4_000_000 {
		t.Errorf("PriorValue = %+v, want {true 4000000}", revenueKPI.PriorValue)
	}
	wantChange := (5_000_000.0 - 4_000_000.0) / 4_000_000.0
	if !revenueKPI.Change.Available || revenueKPI.Change.Amount != wantChange {
		t.Errorf("Change = %+v, want {true %v}", revenueKPI.Change, wantChange)
	}
}

func TestCalculate_ExecutiveSummary_OmitsUnavailableKPIs(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics, PeriodMeta: fullFixture().PeriodMeta}
	report := Calculate(in)
	for _, k := range report.ExecutiveSummary.KPIs {
		if k.Label == "Indicated Value" {
			t.Errorf("expected no Indicated Value KPI without Consensus, got %+v", k)
		}
		if !k.Value.Available {
			t.Errorf("KPI %q included with an unavailable Value; ExecutiveSummary.KPIs must omit rather than include unavailable", k.Label)
		}
	}
}

func TestCalculate_ChartSeries_IncludesForecastTail(t *testing.T) {
	report := Calculate(fullFixture())
	if !report.ChartSeries.Available {
		t.Fatalf("expected ChartSeries.Available")
	}
	var revenueSeries *ChartSeries
	for i := range report.ChartSeries.Series {
		if report.ChartSeries.Series[i].Label == "Total Revenue" {
			revenueSeries = &report.ChartSeries.Series[i]
		}
	}
	if revenueSeries == nil {
		t.Fatalf("expected a Total Revenue chart series")
	}
	if len(revenueSeries.Points) != 4 { // 2 historical + 2 forecast
		t.Fatalf("expected 4 points (2 historical + 2 forecast), got %d: %+v", len(revenueSeries.Points), revenueSeries.Points)
	}
	if revenueSeries.Points[0].IsForecast {
		t.Errorf("expected the first point to be historical, not forecast")
	}
	if !revenueSeries.Points[len(revenueSeries.Points)-1].IsForecast {
		t.Errorf("expected the last point to be forecast")
	}
	if revenueSeries.Source != "metrics+forecast" {
		t.Errorf("Source = %q, want %q", revenueSeries.Source, "metrics+forecast")
	}
}

func TestCalculate_ChartSeries_NoForecastTailWithoutForecast(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics, PeriodMeta: fullFixture().PeriodMeta}
	report := Calculate(in)
	for _, s := range report.ChartSeries.Series {
		if s.Label == "Total Revenue" {
			for _, p := range s.Points {
				if p.IsForecast {
					t.Errorf("expected no forecast points without Forecast input, got %+v", p)
				}
			}
		}
	}
}

// TestCalculate_ChartSeries_ForecastOnlySourceLabel proves a chart series
// built from Forecast alone (no Metrics, no Dataset) is labeled "forecast",
// never "metrics+forecast" — a prior version of chartFromHistorical always
// set Source to "metrics+forecast" once any forecast points existed,
// regardless of whether HistoricalSeries actually contributed anything.
func TestCalculate_ChartSeries_ForecastOnlySourceLabel(t *testing.T) {
	in := Input{Forecast: fullFixture().Forecast}
	report := Calculate(in)
	if report.HistoricalSeries.Available {
		t.Fatalf("test setup invalid: expected HistoricalSeries unavailable with no Metrics/Dataset")
	}

	var revenueSeries *ChartSeries
	for i := range report.ChartSeries.Series {
		if report.ChartSeries.Series[i].Label == "Total Revenue" {
			revenueSeries = &report.ChartSeries.Series[i]
		}
	}
	if revenueSeries == nil {
		t.Fatalf("expected a Total Revenue chart series from Forecast alone")
	}
	if revenueSeries.Source != "forecast" {
		t.Errorf("Source = %q, want %q (no metrics data contributed any point)", revenueSeries.Source, "forecast")
	}
	for _, p := range revenueSeries.Points {
		if !p.IsForecast {
			t.Errorf("expected every point to be IsForecast with no HistoricalSeries, got %+v", p)
		}
	}
}

func TestCalculate_Coverage_CountsAndMissingModules(t *testing.T) {
	in := Input{Metrics: fullFixture().Metrics, Ratios: fullFixture().Ratios}
	report := Calculate(in)
	if report.Coverage.TotalModules != 12 {
		t.Errorf("TotalModules = %d, want 12", report.Coverage.TotalModules)
	}
	if report.Coverage.AvailableModules != 2 {
		t.Errorf("AvailableModules = %d, want 2", report.Coverage.AvailableModules)
	}
	wantPercent := 2.0 / 12.0
	if report.Coverage.CoveragePercent != wantPercent {
		t.Errorf("CoveragePercent = %v, want %v", report.Coverage.CoveragePercent, wantPercent)
	}
	if !report.Coverage.WithMetrics || !report.Coverage.WithRatios {
		t.Errorf("expected WithMetrics and WithRatios true, got %+v", report.Coverage)
	}
	if report.Coverage.WithConsensus {
		t.Errorf("expected WithConsensus false (not supplied)")
	}
	found := false
	for _, m := range report.Coverage.MissingModules {
		if m == "QoE" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'QoE' in MissingModules, got %+v", report.Coverage.MissingModules)
	}
	if len(report.Coverage.MissingModules) != report.Coverage.TotalModules-report.Coverage.AvailableModules {
		t.Errorf("len(MissingModules) = %d, want %d", len(report.Coverage.MissingModules), report.Coverage.TotalModules-report.Coverage.AvailableModules)
	}
}

func TestCalculate_Coverage_FullFixtureIsFullyCovered(t *testing.T) {
	report := Calculate(fullFixture())
	if report.Coverage.AvailableModules != report.Coverage.TotalModules {
		t.Errorf("AvailableModules = %d, want %d (full fixture)", report.Coverage.AvailableModules, report.Coverage.TotalModules)
	}
	if report.Coverage.CoveragePercent != 1.0 {
		t.Errorf("CoveragePercent = %v, want 1.0", report.Coverage.CoveragePercent)
	}
	if len(report.Coverage.MissingModules) != 0 {
		t.Errorf("expected no MissingModules for the full fixture, got %+v", report.Coverage.MissingModules)
	}
}

func TestCalculate_ModuleVersions_EchoesEachSibling(t *testing.T) {
	report := Calculate(fullFixture())
	if report.Versions.FormulaVersion != FormulaVersion {
		t.Errorf("Versions.FormulaVersion = %q, want %q", report.Versions.FormulaVersion, FormulaVersion)
	}
	if len(report.Versions.Modules) != 13 {
		t.Fatalf("expected 13 module version entries, got %d", len(report.Versions.Modules))
	}
	byName := make(map[string]ModuleVersion, len(report.Versions.Modules))
	for _, m := range report.Versions.Modules {
		byName[m.Module] = m
	}
	if byName["qoe"].Version != "1.0.0" {
		t.Errorf("qoe version = %q, want 1.0.0", byName["qoe"].Version)
	}
	if byName["consensus"].Version != "1.0.0" {
		t.Errorf("consensus version = %q, want 1.0.0", byName["consensus"].Version)
	}
}

func TestCalculate_ModuleVersions_EmptyVersionWhenUnavailable(t *testing.T) {
	report := Calculate(Input{Metrics: fullFixture().Metrics})
	byName := make(map[string]ModuleVersion, len(report.Versions.Modules))
	for _, m := range report.Versions.Modules {
		byName[m.Module] = m
	}
	if byName["ratios"].Version != "" {
		t.Errorf("expected empty ratios version when Ratios unavailable, got %q", byName["ratios"].Version)
	}
	// Every module entry is always present, regardless of availability.
	if _, ok := byName["ratios"]; !ok {
		t.Errorf("expected a 'ratios' entry even when unavailable")
	}
}

func TestCalculate_Warnings_OneUnavailableIssuePerMissingModule(t *testing.T) {
	report := Calculate(Input{Metrics: fullFixture().Metrics})
	if HasErrors(report.Warnings) {
		t.Errorf("HasErrors should only match Errors, not Warnings")
	}
	wantCodes := map[IssueCode]bool{
		IssueRatiosUnavailable:         false,
		IssueCashFlowUnavailable:       false,
		IssueWorkingCapitalUnavailable: false,
		IssueQoEUnavailable:            false,
		IssueVarianceUnavailable:       false,
		IssueForecastUnavailable:       false,
		IssueAnomaliesUnavailable:      false,
		IssueConcentrationUnavailable:  false,
		IssueRevenueQualityUnavailable: false,
		IssueDebtUnavailable:           false,
		IssueCovenantsUnavailable:      false,
		IssueConsensusUnavailable:      false,
	}
	for _, w := range report.Warnings {
		if _, ok := wantCodes[w.Code]; ok {
			wantCodes[w.Code] = true
		}
		if w.Severity != IssueSeverityWarning {
			t.Errorf("issue %s has severity %s, want warning", w.Code, w.Severity)
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Errorf("expected a warning Issue with code %s", code)
		}
	}
}

// TestSeverityTranslation_LosslessRoundTrip proves the anomalies/qoe
// severity-translation helpers cover every enum value defined by their
// source packages, so a future new severity level would be caught by this
// test rather than silently defaulting to TopIssueSeverityInfo.
func TestSeverityTranslation_LosslessRoundTrip(t *testing.T) {
	anomalyCases := []struct {
		in   anomalies.AnomalySeverity
		want TopIssueSeverity
	}{
		{anomalies.AnomalySeverityInfo, TopIssueSeverityInfo},
		{anomalies.AnomalySeverityWarning, TopIssueSeverityWarning},
		{anomalies.AnomalySeverityCritical, TopIssueSeverityCritical},
	}
	for _, c := range anomalyCases {
		if got := anomalySeverityToTopIssue(c.in); got != c.want {
			t.Errorf("anomalySeverityToTopIssue(%s) = %s, want %s", c.in, got, c.want)
		}
	}

	qoeCases := []struct {
		in   qoe.FlagSeverity
		want TopIssueSeverity
	}{
		{qoe.FlagSeverityInfo, TopIssueSeverityInfo},
		{qoe.FlagSeverityWarning, TopIssueSeverityWarning},
		{qoe.FlagSeverityCritical, TopIssueSeverityCritical},
	}
	for _, c := range qoeCases {
		if got := qoeFlagSeverityToTopIssue(c.in); got != c.want {
			t.Errorf("qoeFlagSeverityToTopIssue(%s) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestOrderedSnapshots_FallsBackToOriginalOrderWhenMetaIncomplete(t *testing.T) {
	snapshots := []metrics.Snapshot{
		{Period: "FY2024"},
		{Period: "FY2023"},
	}
	meta := map[financial.Period]metrics.PeriodInfo{
		"FY2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
		// FY2023 deliberately missing.
	}
	got, issue := orderedSnapshots(snapshots, meta)
	if got[0].Period != "FY2024" || got[1].Period != "FY2023" {
		t.Fatalf("expected original order preserved on incomplete meta, got [%s, %s]", got[0].Period, got[1].Period)
	}
	if issue == nil || issue.Code != IssueNoPeriodMetaForHistoricalOrder || issue.Severity != IssueSeverityWarning {
		t.Fatalf("expected a IssueNoPeriodMetaForHistoricalOrder warning Issue, got %+v", issue)
	}
}

// TestCalculate_HistoricalSeries_WarnsOnIncompletePeriodMeta proves the
// fallback Issue orderedSnapshots raises actually surfaces on Report.Warnings,
// not just from the unit-level helper — the end-to-end path a caller
// would actually observe.
func TestCalculate_HistoricalSeries_WarnsOnIncompletePeriodMeta(t *testing.T) {
	in := Input{
		Metrics: metrics.Result{
			FormulaVersion: "1.0.0",
			Snapshots: []metrics.Snapshot{
				{Period: "FY2024", TotalRevenue: metrics.AvailableValue(1)},
				{Period: "FY2023", TotalRevenue: metrics.AvailableValue(1)},
			},
		},
		PeriodMeta: map[financial.Period]metrics.PeriodInfo{
			"FY2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
			// FY2023 deliberately missing.
		},
	}
	report := Calculate(in)
	if !report.HistoricalSeries.Available {
		t.Fatalf("expected HistoricalSeries.Available")
	}
	found := false
	for _, w := range report.Warnings {
		if w.Code == IssueNoPeriodMetaForHistoricalOrder {
			found = true
		}
	}
	if !found {
		t.Errorf("expected IssueNoPeriodMetaForHistoricalOrder in Report.Warnings, got %+v", report.Warnings)
	}
}

func TestOrderedSnapshots_SortsChronologically(t *testing.T) {
	snapshots := []metrics.Snapshot{
		{Period: "FY2024"},
		{Period: "FY2023"},
	}
	meta := map[financial.Period]metrics.PeriodInfo{
		"FY2024": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2024},
		"FY2023": {Type: metrics.PeriodTypeFiscalYear, FiscalYear: 2023},
	}
	got, issue := orderedSnapshots(snapshots, meta)
	if issue != nil {
		t.Errorf("expected no fallback Issue when PeriodMeta fully covers every period, got %+v", issue)
	}
	if got[0].Period != "FY2023" || got[1].Period != "FY2024" {
		t.Fatalf("expected chronological order, got [%s, %s]", got[0].Period, got[1].Period)
	}
}

func TestValue_Constructors(t *testing.T) {
	if v := Unavailable(); v.Available || v.Amount != 0 {
		t.Errorf("Unavailable() = %+v, want zero value", v)
	}
	if v := AvailableValue(42); !v.Available || v.Amount != 42 {
		t.Errorf("AvailableValue(42) = %+v, want {true 42}", v)
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors(nil) {
		t.Errorf("HasErrors(nil) = true, want false")
	}
	if HasErrors([]Issue{{Severity: IssueSeverityWarning}}) {
		t.Errorf("HasErrors with only a warning = true, want false")
	}
	if !HasErrors([]Issue{{Severity: IssueSeverityWarning}, {Severity: IssueSeverityError}}) {
		t.Errorf("HasErrors with an error present = false, want true")
	}
}
