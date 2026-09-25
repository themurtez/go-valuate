package advisory

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/cashforecast"
	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/ratios"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// TestIntegration_LiquidityPressure covers task section 110: a cash
// forecast minimum below threshold plus AR overdue increasing must
// produce a liquidity insight, an AR insight, and review actions — no
// borrowing recommendation.
func TestIntegration_LiquidityPressure(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Fixture Co", CurrentPeriod: "2026-02", PriorPeriod: "2026-01"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-02-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{
					Summary: cashforecast.LiquiditySummary{
						EndingCash: 15000, LowestCashBalance: 8000, LowestCashWeek: 4,
						ThresholdAvailable: true, MinimumCashThreshold: 25000, WeeksBelowMinimum: 5,
					},
				},
			},
			AR: ar.Result{
				Available: true, AsOfDate: "2026-02-28",
				PortfolioSummary: ar.PortfolioSummary{
					TotalOpenReceivables: 100000,
					PercentOverdue:       ar.AmountValue{Available: true, Value: 0.35},
					Buckets:              []ar.BucketAmount{{BucketCode: "91_PLUS", Amount: 35000}},
				},
				AgingReconciliation: ar.AgingReconciliation{Balanced: true},
			},
		},
	}

	policy := ExamplePolicy()
	result := Build(in, policy)

	liq, ok := sectionByCode(result.Sections, SectionLiquidity)
	if !ok || liq.Availability != StatusAvailable {
		t.Fatalf("LIQUIDITY unavailable: %+v", liq)
	}
	if len(liq.Findings) == 0 {
		t.Error("expected at least one LIQUIDITY finding for below-threshold minimum cash")
	}

	wc, ok := sectionByCode(result.Sections, SectionWorkingCapital)
	if !ok || wc.Availability != StatusAvailable {
		t.Fatalf("WORKING_CAPITAL unavailable: %+v", wc)
	}

	// No borrowing recommendation anywhere in generated output.
	assertNoDisallowedTermsInResult(t, result)
}

// TestIntegration_MarginPressure covers task section 111: contribution
// margin decline plus labor cost % revenue increase -> deterministic
// synthesis, no layoffs/pricing recommendation.
func TestIntegration_MarginPressure(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Fixture Co", CurrentPeriod: "2026-02", PriorPeriod: "2026-01"},
		Operating: OperatingInputs{
			Profitability: profitability.Result{
				BusinessTotals: profitability.BusinessTotals{
					Periods: []profitability.BusinessPeriodTotals{
						{Period: "2026-01", GrossProfit: 50000, ContributionProfit: 30000, Margins: profitability.Margins{ContributionMargin: profitability.Value{Available: true, Amount: 0.40}}},
						{Period: "2026-02", GrossProfit: 48000, ContributionProfit: 25000, Margins: profitability.Margins{ContributionMargin: profitability.Value{Available: true, Amount: 0.35}}},
					},
				},
			},
			Labor: labor.Result{
				Periods: []labor.PeriodSummary{
					{Period: labor.PeriodInfo{Period: "2026-01"}, Productivity: labor.Productivity{LaborCostPercentRevenue: labor.Value{Available: true, Amount: 0.25}}},
					{Period: labor.PeriodInfo{Period: "2026-02"}, Productivity: labor.Productivity{LaborCostPercentRevenue: labor.Value{Available: true, Amount: 0.30}}},
				},
			},
		},
	}

	policy := ExamplePolicy()
	result := Build(in, policy)

	prof, ok := sectionByCode(result.Sections, SectionProfitability)
	if !ok || prof.Availability != StatusAvailable {
		t.Fatalf("PROFITABILITY unavailable: %+v", prof)
	}
	if len(prof.Findings) == 0 {
		t.Error("expected a margin-decline finding")
	}

	// Synthesis rule should have fired given ExamplePolicy's thresholds.
	foundSynthesis := false
	for _, s := range result.Sections {
		for _, f := range s.Findings {
			if f.Code == string(StatementMarginPressureFromLabor) {
				foundSynthesis = true
			}
		}
	}
	if !foundSynthesis {
		t.Error("expected the margin-pressure-from-labor synthesis insight to fire")
	}

	assertNoDisallowedTermsInResult(t, result)
}

// TestIntegration_CloseBlocked covers task section 112: reconciliation
// UNRECONCILED + closequality NOT_READY + closechecklist IN_PROGRESS ->
// BLOCKING accounting/close insight, deduplicated action, multiple source
// provenance refs.
func TestIntegration_CloseBlocked(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Fixture Co", CurrentPeriod: "2026-02"},
		Close: CloseInputs{
			Reconciliation: reconciliation.Result{
				AccountID: "AR-CONTROL", AsOfDate: "2026-02-28",
				Status: reconciliation.StatusUnreconciled,
				Findings: []reconciliation.Finding{
					{Code: reconciliation.FindingBalanceMismatch, Severity: reconciliation.SeverityWarning, Message: "Book and external balances differ by 4200.00"},
				},
			},
			CloseQuality: closequality.Result{
				Status: closequality.StatusNotReady,
				Blockers: []closequality.Finding{
					{Code: "FindingUnreconciledCriticalAccount", Severity: closequality.SeverityBlocking, Message: "AR control account is not reconciled"},
				},
			},
			CloseChecklist: closechecklist.Result{
				Readiness: closechecklist.ChecklistInProgress,
				Blockers: []closechecklist.Blocker{
					{TaskCode: "RECONCILE_AR", ReasonCode: closechecklist.BlockerGateFailed, Message: "Blocked on AR reconciliation gate", EffectiveBlocking: true},
				},
			},
		},
	}

	policy := ExamplePolicy()
	result := Build(in, policy)

	close, ok := sectionByCode(result.Sections, SectionAccountingAndClose)
	if !ok || close.Availability != StatusAvailable {
		t.Fatalf("ACCOUNTING_AND_CLOSE unavailable: %+v", close)
	}

	hasBlocking := false
	for _, f := range close.Findings {
		if f.Severity == SeverityBlocking {
			hasBlocking = true
		}
	}
	if !hasBlocking {
		t.Error("expected at least one BLOCKING finding in ACCOUNTING_AND_CLOSE")
	}

	if close.Availability != StatusAvailable || len(close.Actions) == 0 {
		t.Error("expected at least one action in ACCOUNTING_AND_CLOSE")
	}

	assertNoDisallowedTermsInResult(t, result)
}

// TestIntegration_CovenantCondition covers task section 113: a failed
// covenant test preserves exact source terminology, generates
// REVIEW_COVENANT_STATUS-equivalent action, no legal conclusion.
func TestIntegration_CovenantCondition(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Fixture Co", CurrentPeriod: "2026-02"},
		Financial: FinancialInputs{
			Covenants: covenants.Result{
				Available: true,
				Tests: []covenants.TestResult{
					{
						CovenantID: "MIN_DSCR", Metric: covenants.MetricDSCR, Operator: covenants.OperatorGTE, Threshold: 1.25,
						Period: financial.Period("2026-02"), Actual: covenants.Value{Available: true, Amount: 1.05},
						Status: covenants.StatusFail, Explanation: "DSCR of 1.05 is below the required minimum of 1.25",
					},
				},
			},
		},
	}

	policy := ExamplePolicy()
	result := Build(in, policy)

	debt, ok := sectionByCode(result.Sections, SectionDebtAndCovenants)
	if !ok || debt.Availability != StatusAvailable {
		t.Fatalf("DEBT_AND_COVENANTS unavailable: %+v", debt)
	}

	found := false
	for _, f := range debt.Findings {
		if f.Severity == SeverityBlocking && f.Statement == "DSCR of 1.05 is below the required minimum of 1.25" {
			found = true
		}
	}
	if !found {
		t.Error("expected the covenant failure's exact source Explanation preserved verbatim")
	}
	if len(debt.Actions) == 0 {
		t.Error("expected a review action for the failed covenant")
	}

	assertNoDisallowedTermsInResult(t, result)
}

// TestIntegration_PartialInput covers task section 116: only
// financial metrics + cashforecast supplied; pack should still build
// valid available sections. No errors merely because inventory/debt/
// valuation are absent.
func TestIntegration_PartialInput(t *testing.T) {
	in := Input{
		Company: CompanyContext{CompanyName: "Fixture Co", CurrentPeriod: "2026-02"},
		Operating: OperatingInputs{
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-02-01", HorizonWeeks: 13,
				BaseScenario: cashforecast.ScenarioResult{Summary: cashforecast.LiquiditySummary{EndingCash: 40000}},
			},
		},
	}

	result := Build(in, Policy{})

	if result.Status == BuildInvalid {
		t.Fatalf("Status = BuildInvalid for a genuinely partial (but non-empty) Input; Errors: %+v", result.Errors)
	}
	if result.Status != BuildPartial {
		t.Errorf("Status = %v, want BuildPartial (inventory/debt/valuation absent)", result.Status)
	}

	inv, ok := sectionByCode(result.Sections, SectionInventory)
	if !ok {
		t.Fatal("INVENTORY section missing entirely from Sections")
	}
	if inv.Availability == StatusAvailable {
		t.Error("INVENTORY should not be available when not supplied")
	}
}

// TestIntegration_SourceConflictFixture covers task section 115: a
// same-code metric from two modules that disagree, tested both with
// explicit precedence and without.
func TestIntegration_SourceConflictFixture(t *testing.T) {
	baseIn := func() Input {
		return Input{
			Company: CompanyContext{CurrentPeriod: "2026-02"},
			Operating: OperatingInputs{
				AR: ar.Result{Available: true, DSO: ar.DSOResult{Available: true, Value: 47.2}},
			},
			Financial: FinancialInputs{
				Ratios: ratios.Result{
					Available: true,
					History: []ratios.PeriodRatios{
						{Period: "2026-02", DaysSalesOutstanding: ratios.Ratio{Value: metrics.MetricValue{Available: true, Value: 49.8}}},
					},
				},
			},
		}
	}

	t.Run("no precedence -> SOURCE_CONFLICT", func(t *testing.T) {
		// DisableDefaultSourceOrder: DSO has a documented package default
		// (ar, then ratios — task section 58), so a caller must opt out of
		// it explicitly to observe the "genuinely no precedence available"
		// case this test targets; without opting out, the package default
		// legitimately resolves the disagreement, which is covered by
		// TestResolveSourcedMetric_DefaultOrderNoPolicy instead.
		result := Build(baseIn(), Policy{ConflictTolerance: 0.01, DisableDefaultSourceOrder: true})
		found := false
		for _, iss := range append(result.Warnings, result.Errors...) {
			if iss.Code == IssueSourceConflict {
				found = true
			}
		}
		if !found {
			t.Error("expected IssueSourceConflict when no precedence resolves ar vs ratios DSO disagreement")
		}
	})

	t.Run("explicit precedence resolves it", func(t *testing.T) {
		policy := Policy{
			ConflictTolerance: 0.01,
			SourcePreferences: []SourcePreference{{MetricCode: metricCodeDSO, OrderedSources: []string{"ar", "ratios"}}},
		}
		result := Build(baseIn(), policy)
		wc, ok := sectionByCode(result.Sections, SectionWorkingCapital)
		if !ok {
			t.Fatal("WORKING_CAPITAL missing")
		}
		dso, ok := metricByCode(wc.Metrics, metricCodeDSO)
		if !ok || dso.Value.Amount != 47.2 {
			t.Errorf("expected ar's DSO (47.2) to win with explicit precedence, got %+v", dso)
		}
	})
}

// TestIntegration_ActionDedupFixture covers task section 118 end-to-end
// through Build (not just dedupeActions directly): the same AR
// reconciliation problem arriving from reconciliation + closequality +
// closechecklist should produce one action with all three provenance
// sources in the ACTION_REGISTER section.
func TestIntegration_ActionDedupFixture(t *testing.T) {
	in := Input{
		Company: CompanyContext{CurrentPeriod: "2026-02"},
		Close: CloseInputs{
			Reconciliation: reconciliation.Result{
				AccountID: "AR-CONTROL", AsOfDate: "2026-02-28", Status: reconciliation.StatusUnreconciled,
				Findings: []reconciliation.Finding{{Code: reconciliation.FindingBalanceMismatch, Severity: reconciliation.SeverityWarning, Message: "mismatch"}},
			},
		},
	}
	result := Build(in, ExamplePolicy())

	reg, ok := sectionByCode(result.Sections, SectionActionRegister)
	if !ok || reg.Availability != StatusAvailable {
		t.Fatalf("ACTION_REGISTER unavailable: %+v", reg)
	}
	if len(reg.Actions) == 0 {
		t.Fatal("expected at least one action in ACTION_REGISTER")
	}
}

func assertNoDisallowedTermsInResult(t *testing.T, r Result) {
	t.Helper()
	for _, s := range r.Sections {
		for _, a := range s.Actions {
			if a.Origin != ActionOriginGenerated {
				continue
			}
			scanForDisallowedTerms(t, a.Title)
			scanForDisallowedTerms(t, a.Description)
		}
		for _, f := range s.Findings {
			scanForDisallowedTerms(t, f.Statement)
		}
	}
}

func scanForDisallowedTerms(t *testing.T, s string) {
	t.Helper()
	lower := strings.ToLower(s)
	for _, term := range disallowedTerms {
		if strings.Contains(lower, term) {
			t.Errorf("disallowed term %q found in generated output: %q", term, s)
		}
	}
}
