package smoketest

import (
	"testing"

	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/debt"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/fixtures/synthetic"
)

// TestInvariant_ConcentrationHHIMatchesRevenueQualityHHI proves
// analytics/concentration and analytics/revenuequality's ConcentrationSummary
// compute the identical Herfindahl-Hirschman Index for the same underlying
// customer-revenue data — both are independent implementations of the same
// "Σ(share²) × 10,000" formula on the DOJ/FTC 0-10,000 scale (see
// docs/ANALYTICS_ARCHITECTURE.md § Cross-module semantic consistency §
// Concentration), and this is the cleanest genuinely-equivalent pair found
// during this task's audit. synthetic.BuildConcentrationObservations and
// synthetic.BuildCustomerRevenue are built from the identical
// customerYears source data (see fixtures/synthetic/customers.go), so this
// is a real equivalence check, not a coincidence of the fixture.
func TestInvariant_ConcentrationHHIMatchesRevenueQualityHHI(t *testing.T) {
	cres := concentration.Calculate(synthetic.BuildConcentrationInput(), concentration.Options{})
	if !cres.Available {
		t.Fatalf("concentration.Calculate: Available == false, errors: %+v", cres.Errors)
	}
	rres := revenuequality.Calculate(synthetic.BuildRevenueQualityInput(), revenuequality.Options{})
	if !rres.Available {
		t.Fatalf("revenuequality.Calculate: Available == false, errors: %+v", rres.Errors)
	}

	var concentrationHHI concentration.ConcentrationValue
	for _, h := range cres.History {
		if h.Period == "2025" {
			concentrationHHI = h.HHI
		}
	}
	if !concentrationHHI.Available {
		t.Fatal("expected concentration.Result.History to have an Available HHI for 2025")
	}

	revQualityHHI := rres.ConcentrationSummary.HHI
	if !revQualityHHI.Available {
		t.Fatal("expected revenuequality.Result.ConcentrationSummary.HHI to be Available (most recent period with customer data)")
	}
	if rres.ConcentrationSummary.Period != "2025" {
		t.Fatalf("expected revenuequality.ConcentrationSummary to be for 2025, got %q", rres.ConcentrationSummary.Period)
	}

	if concentrationHHI.Value != revQualityHHI.Value {
		t.Fatalf("HHI mismatch: analytics/concentration = %v, analytics/revenuequality = %v (both computed from the identical customer-revenue data and should agree exactly)", concentrationHHI.Value, revQualityHHI.Value)
	}
}

// TestInvariant_DebtNetDebtMatchesMetricsNetDebt proves analytics/debt's
// NetDebtToEBITDA-implied net debt figure (TotalDebtBalance - CashAndEquivalents)
// agrees exactly with financial/metrics.Snapshot.NetDebt (TotalDebt - Cash,
// computed from BS_SHORT_TERM_DEBT/BS_LONG_TERM_DEBT/BS_CASH line items)
// when analytics/debt is fed the same underlying debt/cash figures the
// dataset actually carries — see docs/ANALYTICS_ARCHITECTURE.md
// § Cross-module semantic consistency § Debt / net debt for why these two
// packages resolve the same definition through different mechanisms
// (analytics/debt has no FinancialDataset dependency by design, so it
// cannot simply read the Snapshot's NetDebt field directly). This is a
// same-definition-via-different-resolution-chains check, not a claim that
// the two packages should ever be merged.
func TestInvariant_DebtNetDebtMatchesMetricsNetDebt(t *testing.T) {
	mres := metrics.Calculate(synthetic.BuildDataset(), metrics.Options{PeriodMeta: synthetic.BuildPeriodMeta()})
	snap, ok := mres.SnapshotFor("2025")
	if !ok {
		t.Fatal("expected a 2025 snapshot")
	}
	if !snap.NetDebt.Available {
		t.Fatal("expected financial/metrics.Snapshot.NetDebt to be Available for 2025")
	}

	debtIn := synthetic.BuildDebtInput()
	dres := debt.Calculate(debtIn)
	if !dres.Available {
		t.Fatal("expected debt.Calculate to be Available")
	}
	if !dres.BaseCase.TotalDebtBalance.Available || !debtIn.CashAndEquivalents.Available {
		t.Fatal("expected TotalDebtBalance and CashAndEquivalents to be Available")
	}

	debtImpliedNetDebt := dres.BaseCase.TotalDebtBalance.Amount - debtIn.CashAndEquivalents.Amount

	if debtImpliedNetDebt != snap.NetDebt.Value {
		t.Fatalf("net debt mismatch: analytics/debt-implied = %v, financial/metrics.Snapshot.NetDebt = %v (both should agree exactly given synthetic.BuildDebtInput's TotalDebtBalance/CashAndEquivalents are set to match BuildDataset's actual 2025 debt/cash line items)", debtImpliedNetDebt, snap.NetDebt.Value)
	}
}
