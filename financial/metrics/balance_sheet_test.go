package metrics

import (
	"testing"

	"github.com/themurtez/go-valuate/financial"
)

func TestCurrentAssets_SumsCurrentAssetCodes(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsCash, "2025", 50000),
		item(financial.CodeBsAccountsReceivable, "2025", 30000),
		item(financial.CodeBsInventory, "2025", 20000),
		item(financial.CodeBsPrepaid, "2025", 5000),
		item(financial.CodeBsCurrentAssetOther, "2025", 1000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.CurrentAssets.Available || s.CurrentAssets.Value != 106000 {
		t.Errorf("CurrentAssets = %+v, want Available=true Value=106000", s.CurrentAssets)
	}
}

func TestCurrentAssets_UnavailableWhenNoCodesPresent(t *testing.T) {
	ds := dataset(item(financial.CodeRevProduct, "2025", 1000))
	s := snapshotFor(t, ds, "2025")
	if s.CurrentAssets.Available {
		t.Error("expected CurrentAssets to be unavailable when no current-asset codes are present")
	}
}

func TestCurrentLiabilities_IncludesShortTermDebt(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsAccountsPayable, "2025", 20000),
		item(financial.CodeBsCurrentLiabilityOther, "2025", 5000),
		item(financial.CodeBsShortTermDebt, "2025", 10000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.CurrentLiabilities.Available || s.CurrentLiabilities.Value != 35000 {
		t.Errorf("CurrentLiabilities = %+v, want Available=true Value=35000", s.CurrentLiabilities)
	}
}

func TestWorkingCapital_CurrentAssetsMinusCurrentLiabilities(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsCash, "2025", 50000),
		item(financial.CodeBsAccountsReceivable, "2025", 30000),
		item(financial.CodeBsAccountsPayable, "2025", 20000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.WorkingCapital.Available || s.WorkingCapital.Value != 60000 {
		t.Errorf("WorkingCapital = %+v, want Available=true Value=60000", s.WorkingCapital)
	}
}

func TestWorkingCapital_NegativeWhenLiabilitiesExceedAssets(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsCash, "2025", 10000),
		item(financial.CodeBsAccountsPayable, "2025", 40000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.WorkingCapital.Available || s.WorkingCapital.Value != -30000 {
		t.Errorf("WorkingCapital = %+v, want Available=true Value=-30000", s.WorkingCapital)
	}
}

func TestWorkingCapital_UnavailableWhenBalanceSheetMissing(t *testing.T) {
	ds := dataset(item(financial.CodeRevProduct, "2025", 1000))
	s := snapshotFor(t, ds, "2025")
	if s.WorkingCapital.Available {
		t.Error("expected WorkingCapital to be unavailable with no balance sheet data")
	}
}

func TestDebt_TotalAndShortLongBreakdown(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsShortTermDebt, "2025", 15000),
		item(financial.CodeBsLongTermDebt, "2025", 85000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.ShortTermDebt.Available || s.ShortTermDebt.Value != 15000 {
		t.Errorf("ShortTermDebt = %+v, want Available=true Value=15000", s.ShortTermDebt)
	}
	if !s.LongTermDebt.Available || s.LongTermDebt.Value != 85000 {
		t.Errorf("LongTermDebt = %+v, want Available=true Value=85000", s.LongTermDebt)
	}
	if !s.TotalDebt.Available || s.TotalDebt.Value != 100000 {
		t.Errorf("TotalDebt = %+v, want Available=true Value=100000", s.TotalDebt)
	}
}

func TestDebt_TotalUnavailableWhenNoDebtCodesPresent(t *testing.T) {
	ds := dataset(item(financial.CodeBsCash, "2025", 10000))
	s := snapshotFor(t, ds, "2025")
	if s.TotalDebt.Available {
		t.Error("expected TotalDebt to be unavailable when neither debt code is present")
	}
}

func TestNetDebt_TotalDebtMinusCash(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsShortTermDebt, "2025", 20000),
		item(financial.CodeBsLongTermDebt, "2025", 80000),
		item(financial.CodeBsCash, "2025", 30000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.NetDebt.Available || s.NetDebt.Value != 70000 {
		t.Errorf("NetDebt = %+v, want Available=true Value=70000", s.NetDebt)
	}
}

func TestNetDebt_NegativeWhenCashExceedsDebt(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsShortTermDebt, "2025", 5000),
		item(financial.CodeBsCash, "2025", 50000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.NetDebt.Available || s.NetDebt.Value != -45000 {
		t.Errorf("NetDebt = %+v, want Available=true Value=-45000 (net cash position)", s.NetDebt)
	}
}

func TestNetDebt_UnavailableWhenCashMissing(t *testing.T) {
	// TotalDebt is available but cash is entirely absent from the dataset;
	// netting against an unknown cash position should not happen.
	ds := dataset(item(financial.CodeBsShortTermDebt, "2025", 5000))
	s := snapshotFor(t, ds, "2025")
	if s.NetDebt.Available {
		t.Error("expected NetDebt to be unavailable when cash is not present in the dataset")
	}
}

func TestNetDebt_UnavailableWhenTotalDebtUnavailable(t *testing.T) {
	ds := dataset(item(financial.CodeBsCash, "2025", 30000))
	s := snapshotFor(t, ds, "2025")
	if s.NetDebt.Available {
		t.Error("expected NetDebt to be unavailable when no debt codes are present at all")
	}
}

func TestTangibleAssetValue_FixedAssetsMinusAccumDep(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsFixedAssets, "2025", 500000),
		item(financial.CodeBsAccumDepreciation, "2025", 150000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.TangibleAssetValue.Available || s.TangibleAssetValue.Value != 350000 {
		t.Errorf("TangibleAssetValue = %+v, want Available=true Value=350000", s.TangibleAssetValue)
	}
}

func TestTangibleAssetValue_ExcludesIntangiblesAndGoodwill(t *testing.T) {
	ds := dataset(
		item(financial.CodeBsFixedAssets, "2025", 500000),
		item(financial.CodeBsIntangibleAssets, "2025", 1000000),
		item(financial.CodeBsGoodwill, "2025", 2000000),
	)
	s := snapshotFor(t, ds, "2025")
	if !s.TangibleAssetValue.Available || s.TangibleAssetValue.Value != 500000 {
		t.Errorf("TangibleAssetValue = %+v, want Available=true Value=500000 (intangibles/goodwill excluded)", s.TangibleAssetValue)
	}
}

func TestTangibleAssetValue_UnavailableWhenNoFixedAssets(t *testing.T) {
	ds := dataset(item(financial.CodeBsCash, "2025", 10000))
	s := snapshotFor(t, ds, "2025")
	if s.TangibleAssetValue.Available {
		t.Error("expected TangibleAssetValue to be unavailable with no fixed assets")
	}
}
