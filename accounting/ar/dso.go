package ar

import "github.com/themurtez/go-valuate/financial"

// SalesBasis labels what SalesPeriod.SalesAmount represents, since DSO's
// denominator semantics differ meaningfully between the two — see the
// task's "do not claim exact DSO when denominator semantics are weaker"
// instruction.
type SalesBasis string

const (
	// SalesBasisCreditSales means SalesAmount is credit (on-account) sales
	// only — the textbook-correct DSO denominator.
	SalesBasisCreditSales SalesBasis = "CREDIT_SALES"
	// SalesBasisTotalSales means SalesAmount is total sales (cash + credit)
	// because the caller does not separately track credit sales. DSO
	// computed from this basis is labeled accordingly on DSOResult.Basis
	// and is a weaker approximation.
	SalesBasisTotalSales SalesBasis = "TOTAL_SALES"
)

// SalesPeriod is caller-supplied sales data for one period, used as DSO's
// denominator. This package never fabricates sales from invoice data
// unless InvoiceBasedSalesApproximation is explicitly requested (see
// Options-level DSO input below) — see the task's explicit prohibition.
type SalesPeriod struct {
	Period      financial.Period `json:"period"`
	SalesAmount float64          `json:"sales_amount"`
	// Days is the number of days in Period (e.g. 365/366 for a fiscal
	// year, ~91 for a quarter). Required for the DSO formula.
	Days  int        `json:"days"`
	Basis SalesBasis `json:"basis,omitempty"`
	// EndingAR, when supplied, is the caller-known ending AR balance for
	// this specific historical period — required for DSOHistory (section
	// 14). This package never derives a historical ending-AR figure from
	// today's open-item list — see the task's explicit prohibition.
	EndingAR *float64 `json:"ending_ar,omitempty"`
}

// DSOResult is a single DSO figure with its basis/quality labeled — the
// task's "label basis accordingly... do not claim exact DSO" instruction.
type DSOResult struct {
	Available bool             `json:"available"`
	Value     float64          `json:"value"`
	Basis     SalesBasis       `json:"basis,omitempty"`
	Period    financial.Period `json:"period,omitempty"`
}

// DSOHistoryPoint is one period's DSO in a historical series.
type DSOHistoryPoint struct {
	Period financial.Period `json:"period"`
	DSO    DSOResult        `json:"dso"`
}

// DSOHistory is the section-14 historical DSO series.
type DSOHistory struct {
	Available bool              `json:"available"`
	Points    []DSOHistoryPoint `json:"points,omitempty"`
	// Trend characterizes the first-vs-last direction. "improving" means
	// DSO decreased (faster collections); "deteriorating" means it
	// increased; "flat" means no material change; "" (empty) if
	// unavailable (fewer than two points).
	Trend string `json:"trend,omitempty"`
	// FirstVsLastChange is Points[last].DSO - Points[0].DSO.
	FirstVsLastChange AmountValue `json:"first_vs_last_change"`
	// AdjacentChanges is one entry per adjacent pair of Points,
	// Points[i+1].DSO - Points[i].DSO.
	AdjacentChanges []AmountValue `json:"adjacent_changes,omitempty"`
}

// calculateCurrentDSO computes simple DSO for the most recent SalesPeriod
// supplied (by chronological input order — this package does not infer
// period chronology from the financial.Period string; a caller supplying
// SalesHistory should supply it in chronological order), using
// PortfolioSummary.TotalOpenReceivables as the ending-AR numerator, per the
// task's:
//
//	Ending AR / Period Credit Sales x Days in Period
func calculateCurrentDSO(totalOpenAR float64, sales []SalesPeriod) DSOResult {
	if len(sales) == 0 {
		return DSOResult{}
	}
	latest := sales[len(sales)-1]
	if latest.SalesAmount == 0 || latest.Days <= 0 {
		return DSOResult{}
	}
	basis := latest.Basis
	if basis == "" {
		basis = SalesBasisCreditSales
	}
	dso := (totalOpenAR / latest.SalesAmount) * float64(latest.Days)
	return DSOResult{Available: true, Value: dso, Basis: basis, Period: latest.Period}
}

// calculateDSOHistory computes one DSO point per SalesPeriod that supplies
// EndingAR, in caller-supplied (input) order, per the task's "DSO history"
// and "do not derive historical AR from today's open-item list"
// instructions.
func calculateDSOHistory(sales []SalesPeriod) DSOHistory {
	var points []DSOHistoryPoint
	for _, sp := range sales {
		if sp.EndingAR == nil || sp.SalesAmount == 0 || sp.Days <= 0 {
			continue
		}
		basis := sp.Basis
		if basis == "" {
			basis = SalesBasisCreditSales
		}
		dso := (*sp.EndingAR / sp.SalesAmount) * float64(sp.Days)
		points = append(points, DSOHistoryPoint{Period: sp.Period, DSO: DSOResult{Available: true, Value: dso, Basis: basis, Period: sp.Period}})
	}
	if len(points) == 0 {
		return DSOHistory{}
	}

	h := DSOHistory{Available: true, Points: points}
	if len(points) >= 2 {
		first, last := points[0].DSO.Value, points[len(points)-1].DSO.Value
		change := last - first
		h.FirstVsLastChange = AvailableAmount(change)
		switch {
		case change > dsoFlatTolerance:
			h.Trend = "deteriorating"
		case change < -dsoFlatTolerance:
			h.Trend = "improving"
		default:
			h.Trend = "flat"
		}
		for i := 1; i < len(points); i++ {
			h.AdjacentChanges = append(h.AdjacentChanges, AvailableAmount(points[i].DSO.Value-points[i-1].DSO.Value))
		}
	}
	return h
}

// dsoFlatTolerance is the minimum |change in days| for DSOHistory.Trend to
// report "improving"/"deteriorating" rather than "flat".
const dsoFlatTolerance = 0.5
