package sensitivity

import "github.com/themurtez/go-valuate/valuation/dcf"

// DCFCell is a single (discount rate, terminal growth rate) combination's
// outcome within a DCFGrid.
type DCFCell struct {
	// DiscountRate is this cell's discount rate.
	DiscountRate float64 `json:"discount_rate"`
	// TerminalGrowthRate is this cell's terminal growth rate.
	TerminalGrowthRate float64 `json:"terminal_growth_rate"`
	// Valid is false if this combination fails valuation/dcf.Calculate's
	// own validation (most notably DiscountRate <= TerminalGrowthRate, or
	// DiscountRate <= 0) — see the package doc comment: invalid
	// combinations are marked, never computed under a substituted or
	// clamped rate.
	Valid bool `json:"valid"`
	// EnterpriseValue is this cell's dcf.Result.EnterpriseValue. Meaningful
	// only when Valid is true.
	EnterpriseValue float64 `json:"enterprise_value"`
	// Result is the full dcf.Result for this cell (every projected period,
	// terminal value detail, steps, bridge if requested) — returned
	// alongside the convenience EnterpriseValue field so a caller wanting
	// the complete calculation trace for a specific cell never has to
	// re-run dcf.Calculate itself. Populated whether or not Valid, since
	// dcf.Result itself already carries Available/Errors for the invalid
	// case; a caller inspecting a specific cell can read either signal.
	Result dcf.Result `json:"result"`
}

// DCFGrid is a 2D discount-rate x terminal-growth-rate sensitivity grid
// over one caller-supplied forecast.
type DCFGrid struct {
	// ForecastPeriods echoes the forecast every cell was computed against.
	ForecastPeriods []dcf.ForecastPeriod `json:"forecast_periods"`
	// DiscountRates echoes the input discount rates, in row order.
	DiscountRates []float64 `json:"discount_rates"`
	// TerminalGrowthRates echoes the input terminal growth rates, in
	// column order.
	TerminalGrowthRates []float64 `json:"terminal_growth_rates"`
	// Rows holds one row per DiscountRates entry, each containing one
	// DCFCell per TerminalGrowthRates entry — Rows[i][j] is always
	// DiscountRates[i] combined with TerminalGrowthRates[j].
	Rows [][]DCFCell `json:"rows"`
}

// DCFSensitivity computes a full discount-rate x terminal-growth-rate
// grid over a single caller-supplied forecast (forecastPeriods) and
// optional equity bridge (equityBridge, applied identically to every
// cell): for every combination, it calls dcf.Calculate with that cell's
// rates, and reports the full dcf.Result. A combination where
// discountRate <= terminalGrowthRate (or any other dcf.Calculate
// validation failure, e.g. a non-positive discount rate) is marked
// Valid=false rather than computed — see dcf.Calculate's own validation
// for the exact rules, which this package does not duplicate or relax.
func DCFSensitivity(forecastPeriods []dcf.ForecastPeriod, discountRates, terminalGrowthRates []float64, equityBridge dcf.EquityBridgeInput) DCFGrid {
	rows := make([][]DCFCell, 0, len(discountRates))
	for _, dr := range discountRates {
		row := make([]DCFCell, 0, len(terminalGrowthRates))
		for _, tg := range terminalGrowthRates {
			res := dcf.Calculate(dcf.Input{
				ForecastPeriods:    forecastPeriods,
				DiscountRate:       dr,
				TerminalGrowthRate: tg,
				EquityBridge:       equityBridge,
			})
			row = append(row, DCFCell{
				DiscountRate:       dr,
				TerminalGrowthRate: tg,
				Valid:              res.Available,
				EnterpriseValue:    res.EnterpriseValue,
				Result:             res,
			})
		}
		rows = append(rows, row)
	}
	return DCFGrid{
		ForecastPeriods:     forecastPeriods,
		DiscountRates:       discountRates,
		TerminalGrowthRates: terminalGrowthRates,
		Rows:                rows,
	}
}
