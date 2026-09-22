package ratios

import (
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// currentRatio computes Current Assets / Current Liabilities, read directly
// from Snapshot (financial/metrics already computes both). Available only
// if both are available and current liabilities is nonzero.
func currentRatio(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioCurrentRatio, "Current Assets / Current Liabilities", s.CurrentAssets, s.CurrentLiabilities, metrics.MetricCurrentAssets, "Current Assets", metrics.MetricCurrentLiabilities, "Current Liabilities", period)
}

// quickRatio computes Quick Assets / Current Liabilities, where Quick
// Assets is Cash + Accounts Receivable + Other Current Assets (the standard
// acid-test exclusion of inventory and prepaid expenses — see
// quickAssetCodes). Available only if at least one quick-asset code is
// present, current liabilities is available, and current liabilities is
// nonzero.
func quickRatio(idx codeIndex, s metrics.Snapshot, period financial.Period) Ratio {
	quick := sumCodes(idx, period, quickAssetCodes)
	r := Ratio{Metric: RatioQuickRatio, Period: period, Formula: "(Cash + Accounts Receivable + Other Current Assets) / Current Liabilities"}
	if !quick.anyPresent || !s.CurrentLiabilities.Available || s.CurrentLiabilities.Value == 0 {
		return r
	}
	r.Value = metrics.AvailableValue(quick.total / s.CurrentLiabilities.Value)
	r.Components = append(append([]Component{}, quick.components...), Component{Code: metrics.MetricCurrentLiabilities, Label: "Current Liabilities", Amount: s.CurrentLiabilities.Value})
	return r
}

// cashRatio computes Cash / Current Liabilities — the most conservative
// liquidity measure, counting only cash itself. Available only if cash is
// present, current liabilities is available, and current liabilities is
// nonzero.
func cashRatio(s metrics.Snapshot, period financial.Period) Ratio {
	return marginRatio(RatioCashRatio, "Cash / Current Liabilities", s.Cash, s.CurrentLiabilities, metrics.MetricCash, "Cash", metrics.MetricCurrentLiabilities, "Current Liabilities", period)
}
