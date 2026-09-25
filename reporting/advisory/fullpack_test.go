package advisory

import (
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ap"
	"github.com/themurtez/go-valuate/accounting/ar"
	"github.com/themurtez/go-valuate/accounting/cashforecast"
	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/inventory"
	"github.com/themurtez/go-valuate/accounting/labor"
	"github.com/themurtez/go-valuate/accounting/profitability"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
	"github.com/themurtez/go-valuate/accounting/vendorspend"
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/covenants"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/forecast"
	"github.com/themurtez/go-valuate/analytics/kpi"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/financial"
	fmetrics "github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/transactions/acquisition"
	"github.com/themurtez/go-valuate/transactions/salereadiness"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/consensus"
)

// fullyPopulatedInput builds an Input touching every section this
// package builds — task section 109's "healthy/ready company" integration
// fixture, extended to also exercise the sections that fixture alone
// would leave untouched (financial performance, forecast, revenue,
// transaction readiness, valuation, vendor spend). Every source value is
// deliberately unremarkable/positive so the resulting pack has no
// blocking actions — task section 109's expected outcome.
func fullyPopulatedInput() Input {
	return Input{
		Company: CompanyContext{
			CompanyID: "CO-1", CompanyName: "Fixture Holdings LLC", ReportingCurrency: "USD",
			CurrentPeriod: "2026-02", PriorPeriod: "2026-01", FiscalYear: 2026,
			IndustryLabel: "Professional Services", EntityScope: ScopeCompany,
		},
		Periods: []PeriodInfo{
			{Code: "2026-01", Label: "January 2026", Sequence: 1, IsPrior: true},
			{Code: "2026-02", Label: "February 2026", Sequence: 2, IsCurrent: true},
		},
		Financial: FinancialInputs{
			Metrics: fmetrics.Result{
				FormulaVersion: "1.0.0",
				Snapshots: []fmetrics.Snapshot{
					{
						Period:       financial.Period("2026-02"),
						TotalRevenue: fmetrics.MetricValue{Available: true, Value: 500000},
						GrossProfit:  fmetrics.MetricValue{Available: true, Value: 300000},
						GrossMargin:  fmetrics.MetricValue{Available: true, Value: 0.60},
						EBITDA:       fmetrics.MetricValue{Available: true, Value: 120000},
						EBITDAMargin: fmetrics.MetricValue{Available: true, Value: 0.24},
						NetIncome:    fmetrics.MetricValue{Available: true, Value: 90000},
					},
				},
			},
			Debt: debt.Result{
				Available: true,
				BaseCase: debt.CoverageResult{
					TotalDebtBalance: debt.Value{Available: true, Amount: 400000},
					DSCR:             debt.Value{Available: true, Amount: 1.8},
					NetDebtToEBITDA:  debt.Value{Available: true, Amount: 2.1},
				},
				Capacity: debt.MaximumCapacity{Headroom: debt.Value{Available: true, Amount: 250000}},
			},
			Covenants: covenants.Result{
				Available: true,
				Tests: []covenants.TestResult{
					{
						CovenantID: "MIN_DSCR", Metric: covenants.MetricDSCR, Operator: covenants.OperatorGTE, Threshold: 1.25,
						Period: financial.Period("2026-02"), Actual: covenants.Value{Available: true, Amount: 1.8},
						Status: covenants.StatusPass, WarningBufferStatus: covenants.WarningBufferOutsideBuffer,
						Explanation: "DSCR of 1.8 exceeds the required minimum of 1.25",
						Headroom:    covenants.Value{Available: true, Amount: 0.55},
					},
				},
			},
			RevenueQuality: revenuequality.Result{
				Available: true,
				TotalRevenueHistory: []revenuequality.PeriodRevenue{
					{Period: "2026-02", RecurringPercent: revenuequality.RevenueValue{Available: true, Value: 0.70}},
				},
			},
			Concentration: concentration.Result{
				Available: true,
				History: []concentration.PeriodConcentration{
					{Period: "2026-02", LargestEntityShare: concentration.ConcentrationValue{Available: true, Value: 0.12}, HHI: concentration.ConcentrationValue{Available: true, Value: 800}},
				},
			},
			Forecast: forecast.Result{
				Available: true,
				ScenarioResults: []forecast.ScenarioResult{
					{
						Name: "Base", Type: forecast.ScenarioTypeBase,
						ProjectedPeriods: []forecast.PeriodPL{
							{Period: "2026-03", TotalRevenue: forecast.ForecastValue{Available: true, Value: 510000}, EBITDA: forecast.ForecastValue{Available: true, Value: 122000}},
							{Period: "2026-12", TotalRevenue: forecast.ForecastValue{Available: true, Value: 600000}, EBITDA: forecast.ForecastValue{Available: true, Value: 150000}},
						},
					},
				},
			},
		},
		Operating: OperatingInputs{
			AR: ar.Result{
				Available: true, AsOfDate: "2026-02-28",
				DSO: ar.DSOResult{Available: true, Value: 38.5},
				PortfolioSummary: ar.PortfolioSummary{
					TotalOpenReceivables: 80000,
					PercentOverdue:       ar.AmountValue{Available: true, Value: 0.08},
					Buckets:              []ar.BucketAmount{{BucketCode: "91_PLUS", Amount: 3000}},
				},
				AgingReconciliation:          ar.AgingReconciliation{Balanced: true},
				ControlAccountReconciliation: ar.ControlAccountReconciliation{Available: true, Reconciled: true},
			},
			AP: ap.Result{
				Available: true, AsOfDate: "2026-02-28",
				DPO: ap.DPOResult{Available: true, Value: 32.0},
				PortfolioSummary: ap.PortfolioSummary{
					TotalOpenPayables: 60000,
					PercentOverdue:    ap.AmountValue{Available: true, Value: 0.05},
				},
				AgingReconciliation:          ap.AgingReconciliation{Balanced: true},
				ControlAccountReconciliation: ap.ControlAccountReconciliation{Available: true, Reconciled: true},
			},
			Inventory: inventory.Result{
				Available: true, AsOfDate: "2026-02-28",
				Portfolio:      inventory.PortfolioSummary{TotalInventoryValue: 45000},
				Periods:        []inventory.PeriodSummary{{Period: inventory.PeriodInfo{Period: "2026-02"}, Turnover: inventory.TurnoverResult{Available: true, Value: 6.2}, DIO: inventory.DIOResult{Available: true, Value: 58}}},
				Aging:          inventory.AgingSummary{Available: true, SlowMovingPercent: inventory.Value{Available: true, Amount: 0.04}},
				Reconciliation: inventory.ReconciliationSummary{Available: true, AllReconciled: true},
			},
			Labor: labor.Result{
				Periods: []labor.PeriodSummary{
					{Period: labor.PeriodInfo{Period: "2026-01"}, LaborCostBridge: labor.LaborCostBridge{TotalLaborCost: 150000}, Productivity: labor.Productivity{LaborCostPercentRevenue: labor.Value{Available: true, Amount: 0.30}, RevenuePerFTE: labor.Value{Available: true, Amount: 62500}}, FTE: labor.FTESummary{Available: true, FTE: 8}},
					{Period: labor.PeriodInfo{Period: "2026-02"}, LaborCostBridge: labor.LaborCostBridge{TotalLaborCost: 148000}, Productivity: labor.Productivity{LaborCostPercentRevenue: labor.Value{Available: true, Amount: 0.296}, RevenuePerFTE: labor.Value{Available: true, Amount: 62500}}, FTE: labor.FTESummary{Available: true, FTE: 8}, Headcount: labor.Headcount{Available: true, EndingHeadcount: 8}},
				},
			},
			Profitability: profitability.Result{
				BusinessTotals: profitability.BusinessTotals{
					Periods: []profitability.BusinessPeriodTotals{
						{Period: "2026-01", GrossProfit: 295000, ContributionProfit: 200000, Margins: profitability.Margins{ContributionMargin: profitability.Value{Available: true, Amount: 0.42}}},
						{Period: "2026-02", GrossProfit: 300000, ContributionProfit: 205000, Margins: profitability.Margins{ContributionMargin: profitability.Value{Available: true, Amount: 0.41}}},
					},
				},
			},
			VendorSpend: vendorspend.Result{
				Available: true, Periods: []string{"2026-02"},
				Bridge:        vendorspend.SpendBridge{NetSpend: 200000},
				Concentration: vendorspend.SpendConcentration{Available: true, Top1: vendorspend.Value{Available: true, Value: 0.18}},
			},
			CashForecast: cashforecast.Result{
				Available: true, ForecastStartDate: "2026-03-01", HorizonWeeks: 13,
				OpeningPosition: cashforecast.OpeningPosition{UnrestrictedCash: 150000},
				BaseScenario: cashforecast.ScenarioResult{
					Summary: cashforecast.LiquiditySummary{
						EndingCash: 180000, LowestCashBalance: 120000, LowestCashWeek: 3,
						ThresholdAvailable: true, MinimumCashThreshold: 50000,
					},
				},
			},
		},
		Close: CloseInputs{
			Reconciliation: reconciliation.Result{
				AccountID: "OPERATING-CASH", AsOfDate: "2026-02-28", Status: reconciliation.StatusReconciled,
			},
			CloseQuality: closequality.Result{Status: closequality.StatusReady},
			CloseChecklist: closechecklist.Result{
				PeriodID: "2026-02", Readiness: closechecklist.ChecklistReadyToClose,
				Completion: closechecklist.Completion{RequiredCompletionPercent: 100},
			},
		},
		Transaction: TransactionInputs{
			SaleReadiness: salereadiness.Result{Available: true},
			Acquisition: acquisition.Result{
				Available: true,
				Multiples: acquisition.PriceMultiples{PriceToEBITDA: acquisition.Value{Available: true, Amount: 5.2}},
				Consensus: acquisition.ConsensusComparison{Premium: acquisition.Value{Available: true, Amount: 50000}, PremiumPercent: acquisition.Value{Available: true, Amount: 0.05}},
				Returns:   acquisition.ReturnMetrics{CashOnCashReturn: acquisition.Value{Available: true, Amount: 0.18}},
			},
		},
		Valuation: ValuationInputs{
			Consensus: consensus.Result{
				Available: true, Basis: valuation.ValueTypeEnterprise, WeightsValid: true,
				Statistics: consensus.Statistics{Count: 3, WeightedMean: 1200000, SimpleMean: 1180000},
				Range:      consensus.Range{Min: 1000000, Max: 1400000},
				Dispersion: consensus.Dispersion{Score: 82},
			},
		},
		KPIValues: []kpi.KPIResult{
			{
				Code: "GROSS_MARGIN", Period: "2026-02",
				Value: kpi.Value{Available: true, Amount: 0.60}, Unit: kpi.Unit{Kind: kpi.UnitPercent},
				TargetEvaluation: kpi.TargetEvaluation{TargetAvailable: true, TargetMet: true},
			},
		},
	}
}

// TestIntegration_HealthyReadyCompany covers task section 109: a
// COMPLETE-leaning pack with no blocking actions and factual positive/
// stable highlights — never labeling the company "healthy" itself (this
// test asserts the package's own output contains no such subjective
// label, only factual figures).
func TestIntegration_HealthyReadyCompany(t *testing.T) {
	result := Build(fullyPopulatedInput(), ExamplePolicy())

	if result.Status == BuildInvalid {
		t.Fatalf("Status = BuildInvalid for a fully populated Input; Errors: %+v", result.Errors)
	}

	for _, s := range result.Sections {
		for _, a := range s.Actions {
			if a.Blocking {
				t.Errorf("unexpected BLOCKING action in a healthy-fixture pack: %+v", a)
			}
		}
	}

	// This package must never itself assert "healthy" as a label.
	assertNoDisallowedTermsInResult(t, result)
	for _, s := range result.Sections {
		for _, h := range s.Highlights {
			if containsWord(h.Statement, "healthy") || containsWord(h.Title, "healthy") {
				t.Errorf("package must never label the company 'healthy' itself: %+v", h)
			}
		}
	}

	// Spot-check every section actually built content given this fully
	// populated fixture (raises confidence beyond the narrower per-section
	// unit tests, and confirms no section panics/silently drops with a
	// realistic multi-module Input).
	for _, code := range []SectionCode{
		SectionLiquidity, SectionFinancialPerf, SectionWorkingCapital, SectionRevenue,
		SectionProfitability, SectionLabor, SectionInventory, SectionVendorSpend,
		SectionDebtAndCovenants, SectionAccountingAndClose, SectionForecastAndOutlook,
		SectionValuation, SectionTransactionReady, SectionKPI, SectionActionRegister,
	} {
		s, ok := sectionByCode(result.Sections, code)
		if !ok {
			t.Errorf("section %s missing entirely from Result.Sections", code)
			continue
		}
		if s.Availability != StatusAvailable {
			t.Errorf("section %s = %s, want StatusAvailable given the fully populated fixture", code, s.Availability)
		}
	}
}

func containsWord(s, word string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(word))
}
