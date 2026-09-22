package metrics

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func item(code financial.Code, period string, amount float64) financial.NormalizedItem {
	return financial.NormalizedItem{Code: code, Period: financial.Period(period), Amount: amount}
}

func dataset(items ...financial.NormalizedItem) financial.FinancialDataset {
	return financial.FinancialDataset{Currency: "USD", Items: items}
}

func snapshotFor(t *testing.T, ds financial.FinancialDataset, period string) Snapshot {
	t.Helper()
	result := Calculate(ds, Options{})
	s, ok := result.SnapshotFor(financial.Period(period))
	if !ok {
		t.Fatalf("no snapshot found for period %s", period)
	}
	return s
}

func TestTotalRevenue_SumsAllRevenueCodes(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 100000),
		item(financial.CodeRevService, "2025", 50000),
		item(financial.CodeRevRecurring, "2025", 25000),
		item(financial.CodeRevOther, "2025", 5000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.TotalRevenue.Available {
		t.Fatal("expected TotalRevenue to be available")
	}
	if s.TotalRevenue.Value != 180000 {
		t.Errorf("TotalRevenue = %v, want 180000", s.TotalRevenue.Value)
	}
}

func TestTotalRevenue_UnavailableWhenNoRevenueCodesPresent(t *testing.T) {
	ds := dataset(item(financial.CodeOpexPayroll, "2025", 50000))
	s := snapshotFor(t, ds, "2025")
	if s.TotalRevenue.Available {
		t.Error("expected TotalRevenue to be unavailable when no revenue codes are present")
	}
}

func TestTotalRevenue_ZeroIsDistinctFromUnavailable(t *testing.T) {
	// Explicit $0 product revenue should be Available=true, Value=0 - not
	// confused with "no data."
	ds := dataset(item(financial.CodeRevProduct, "2025", 0))
	s := snapshotFor(t, ds, "2025")
	if !s.TotalRevenue.Available {
		t.Fatal("expected TotalRevenue to be available (a real code with value 0 was present)")
	}
	if s.TotalRevenue.Value != 0 {
		t.Errorf("TotalRevenue = %v, want 0", s.TotalRevenue.Value)
	}
}

func TestTotalCOGS_SumsAllCogsCodes(t *testing.T) {
	ds := dataset(
		item(financial.CodeCogsMaterial, "2025", 40000),
		item(financial.CodeCogsDirectLabor, "2025", 30000),
		item(financial.CodeCogsFreight, "2025", 5000),
		item(financial.CodeCogsOther, "2025", 1000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.TotalCOGS.Available || s.TotalCOGS.Value != 76000 {
		t.Errorf("TotalCOGS = %+v, want Available=true Value=76000", s.TotalCOGS)
	}
}

func TestGrossProfit_RevenueMinusCOGS(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 100000),
		item(financial.CodeCogsMaterial, "2025", 40000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.GrossProfit.Available || s.GrossProfit.Value != 60000 {
		t.Errorf("GrossProfit = %+v, want Available=true Value=60000", s.GrossProfit)
	}
}

func TestGrossProfit_UnavailableWhenCOGSMissing(t *testing.T) {
	ds := dataset(item(financial.CodeRevProduct, "2025", 100000))
	s := snapshotFor(t, ds, "2025")
	if s.GrossProfit.Available {
		t.Error("expected GrossProfit to be unavailable when COGS data is entirely absent")
	}
}

func TestGrossProfit_AvailableWhenCOGSIsExplicitZero(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 100000),
		item(financial.CodeCogsMaterial, "2025", 0),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.GrossProfit.Available {
		t.Fatal("expected GrossProfit to be available when COGS is present but genuinely zero")
	}
	if s.GrossProfit.Value != 100000 {
		t.Errorf("GrossProfit = %v, want 100000", s.GrossProfit.Value)
	}
}

func TestGrossMargin_ComputesRatio(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 200000),
		item(financial.CodeCogsMaterial, "2025", 80000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.GrossMargin.Available {
		t.Fatal("expected GrossMargin to be available")
	}
	if s.GrossMargin.Value != 0.6 {
		t.Errorf("GrossMargin = %v, want 0.6", s.GrossMargin.Value)
	}
}

func TestGrossMargin_UnavailableWhenRevenueIsZero(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 0),
		item(financial.CodeCogsMaterial, "2025", 0),
	)
	s := snapshotFor(t, ds, "2025")
	if s.GrossMargin.Available {
		t.Error("expected GrossMargin to be unavailable when revenue is zero (division by zero guard)")
	}
}

func TestEBIT_GrossProfitMinusOpex(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 300000),
		item(financial.CodeCogsMaterial, "2025", 100000),
		item(financial.CodeOpexPayroll, "2025", 80000),
		item(financial.CodeOpexRent, "2025", 20000),
	)
	s := snapshotFor(t, ds, "2025")
	// gross profit = 200000, opex = 100000, EBIT = 100000
	if !s.EBIT.Available || s.EBIT.Value != 100000 {
		t.Errorf("EBIT = %+v, want Available=true Value=100000", s.EBIT)
	}
}

func TestEBITDA_ExactFormula(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 300000),
		item(financial.CodeCogsMaterial, "2025", 100000),
		item(financial.CodeOpexPayroll, "2025", 100000),
		item(financial.CodeDepreciation, "2025", 15000),
		item(financial.CodeAmortization, "2025", 5000),
	)
	s := snapshotFor(t, ds, "2025")
	// EBIT = 300000-100000-100000 = 100000; EBITDA = 100000+15000+5000 = 120000
	if !s.EBITDA.Available {
		t.Fatal("expected EBITDA to be available")
	}
	if s.EBITDA.Value != 120000 {
		t.Errorf("EBITDA = %v, want 120000 (EBIT + Depreciation + Amortization)", s.EBITDA.Value)
	}
}

func TestEBITDA_TreatsMissingDAAsZeroNotUnavailable(t *testing.T) {
	// Many statements bury D&A inside COGS/OPEX without a separate line.
	// EBITDA should still be calculable (equal to EBIT) rather than
	// becoming Unavailable just because D&A wasn't broken out.
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 100000),
		item(financial.CodeCogsMaterial, "2025", 40000),
		item(financial.CodeOpexPayroll, "2025", 20000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.EBITDA.Available {
		t.Fatal("expected EBITDA to be available even without explicit D&A")
	}
	if s.EBITDA.Value != s.EBIT.Value {
		t.Errorf("EBITDA = %v, want equal to EBIT %v when D&A is absent", s.EBITDA.Value, s.EBIT.Value)
	}
}

func TestEBITDA_UnavailableWhenEBITUnavailable(t *testing.T) {
	ds := dataset(item(financial.CodeDepreciation, "2025", 5000))
	s := snapshotFor(t, ds, "2025")
	if s.EBITDA.Available {
		t.Error("expected EBITDA to be unavailable when EBIT cannot be reconstructed")
	}
}

func TestEBITDAMargin_ComputesRatio(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 500000),
		item(financial.CodeCogsMaterial, "2025", 200000),
		item(financial.CodeOpexPayroll, "2025", 100000),
	)
	s := snapshotFor(t, ds, "2025")
	// EBIT = 200000, EBITDA = 200000 (no D&A), margin = 200000/500000 = 0.4
	if !s.EBITDAMargin.Available || s.EBITDAMargin.Value != 0.4 {
		t.Errorf("EBITDAMargin = %+v, want Available=true Value=0.4", s.EBITDAMargin)
	}
}

func TestSDE_ExactFormula(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 500000),
		item(financial.CodeCogsMaterial, "2025", 200000),
		item(financial.CodeOpexPayroll, "2025", 50000),
		item(financial.CodeOpexOwnerComp, "2025", 90000),
	)
	s := snapshotFor(t, ds, "2025")
	// gross profit=300000, opex=140000 (payroll+ownerComp), EBIT=160000,
	// EBITDA=160000 (no D&A), SDE = EBITDA + ownerComp = 160000+90000=250000
	if !s.SDE.Available {
		t.Fatal("expected SDE to be available")
	}
	if s.SDE.Value != 250000 {
		t.Errorf("SDE = %v, want 250000 (EBITDA + Owner Compensation)", s.SDE.Value)
	}
}

func TestSDE_EqualsEBITDAWhenNoOwnerCompensation(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 500000),
		item(financial.CodeCogsMaterial, "2025", 200000),
		item(financial.CodeOpexPayroll, "2025", 50000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.SDE.Available {
		t.Fatal("expected SDE to be available")
	}
	if s.SDE.Value != s.EBITDA.Value {
		t.Errorf("SDE = %v, want equal to EBITDA %v when there is no owner compensation", s.SDE.Value, s.EBITDA.Value)
	}
}

func TestSDE_UnavailableWhenEBITDAUnavailable(t *testing.T) {
	ds := dataset(item(financial.CodeOpexOwnerComp, "2025", 90000))
	s := snapshotFor(t, ds, "2025")
	if s.SDE.Available {
		t.Error("expected SDE to be unavailable when EBITDA cannot be reconstructed")
	}
}

func TestNetIncome_FullBridge(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 500000),
		item(financial.CodeCogsMaterial, "2025", 200000),
		item(financial.CodeOpexPayroll, "2025", 100000),
		item(financial.CodeOtherIncome, "2025", 10000),
		item(financial.CodeInterestIncome, "2025", 2000),
		item(financial.CodeInterestExpense, "2025", 15000),
		item(financial.CodeOtherExpense, "2025", 3000),
		item(financial.CodeIncomeTax, "2025", 40000),
	)
	s := snapshotFor(t, ds, "2025")
	// EBIT = 500000-200000-100000 = 200000
	// NetIncome = 200000+10000+2000-15000-3000-40000 = 154000
	if !s.NetIncome.Available {
		t.Fatal("expected NetIncome to be available")
	}
	if s.NetIncome.Value != 154000 {
		t.Errorf("NetIncome = %v, want 154000", s.NetIncome.Value)
	}
}

func TestNetIncome_MissingBelowTheLineItemsTreatedAsZero(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 500000),
		item(financial.CodeCogsMaterial, "2025", 200000),
		item(financial.CodeOpexPayroll, "2025", 100000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.NetIncome.Available {
		t.Fatal("expected NetIncome to be available (below-the-line items absent should contribute 0, not make it unavailable)")
	}
	if s.NetIncome.Value != s.EBIT.Value {
		t.Errorf("NetIncome = %v, want equal to EBIT %v", s.NetIncome.Value, s.EBIT.Value)
	}
}

func TestNetIncome_NegativeEarnings(t *testing.T) {
	ds := dataset(
		item(financial.CodeRevProduct, "2025", 100000),
		item(financial.CodeCogsMaterial, "2025", 60000),
		item(financial.CodeOpexPayroll, "2025", 90000),
	)
	s := snapshotFor(t, ds, "2025")
	// gross profit = 40000, EBIT = 40000-90000 = -50000
	if !s.EBIT.Available {
		t.Fatal("expected EBIT to be available")
	}
	if s.EBIT.Value != -50000 {
		t.Errorf("EBIT = %v, want -50000 (a real loss)", s.EBIT.Value)
	}
	if !s.NetIncome.Available || s.NetIncome.Value != -50000 {
		t.Errorf("NetIncome = %+v, want Available=true Value=-50000", s.NetIncome)
	}
}
