package advisory

// Metric.Code constants used by more than one file (an adapter that
// produces the figure, and buildSnapshot/executive.go that look it up by
// fixed code) — centralized here so the producer and consumer can never
// drift apart via a typo. A Metric.Code used by only one adapter file is
// declared locally in that file instead; see each section_*.go file's own
// const block.
const (
	metricCodeRevenue        = "revenue"
	metricCodeEBITDA         = "ebitda"
	metricCodeGrossMargin    = "gross_margin"
	metricCodeEndingCash     = "ending_cash_13_week"
	metricCodeMinimumCash    = "minimum_cash_13_week"
	metricCodeAR             = "ar_open_balance"
	metricCodeAP             = "ap_open_balance"
	metricCodeInventoryValue = "inventory_value"
	metricCodeDSO            = "dso"
	metricCodeDPO            = "dpo"
	metricCodeDIO            = "dio"
	metricCodeTotalDebt      = "total_debt_balance"
	metricCodeCloseReadiness = "close_readiness_status"
	metricCodeIndicatedValue = "indicated_value"
)
