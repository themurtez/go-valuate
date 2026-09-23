package inventory

// AverageBasis labels how AverageInventory was computed — task section 13:
// "label average basis."
type AverageBasis string

const (
	// AverageBasisBeginningEnding is (Beginning + Ending) / 2 — the
	// default basis.
	AverageBasisBeginningEnding AverageBasis = "BEGINNING_ENDING"
	// AverageBasisCallerSupplied is a caller-supplied higher-frequency
	// average (e.g. a true daily/weekly average), passed through as-is.
	AverageBasisCallerSupplied AverageBasis = "CALLER_SUPPLIED"
	// AverageBasisEndingOnly is used only when no beginning balance is
	// available at all — a single point-in-time balance, clearly labeled
	// as such rather than silently presented as a two-point average.
	AverageBasisEndingOnly AverageBasis = "ENDING_ONLY"
)

// TurnoverResult is inventory turnover for one period — task section 13.
type TurnoverResult struct {
	Available        bool         `json:"available"`
	Value            float64      `json:"value"`
	Period           string       `json:"period,omitempty"`
	AverageInventory Value        `json:"average_inventory"`
	AverageBasis     AverageBasis `json:"average_basis,omitempty"`
	COGS             Value        `json:"cogs"`
}

// DIOResult is Days Inventory Outstanding for one period — task section
// 14.
type DIOResult struct {
	Available        bool         `json:"available"`
	Value            float64      `json:"value"`
	Period           string       `json:"period,omitempty"`
	AverageInventory Value        `json:"average_inventory"`
	AverageBasis     AverageBasis `json:"average_basis,omitempty"`
	COGS             Value        `json:"cogs"`
	Days             int          `json:"days,omitempty"`
}

// periodInventoryBasis bundles the beginning/ending/average/COGS facts
// one period needs for turnover/DIO, resolved once from whichever input
// path (detailed snapshots or PeriodFinancials) supplied it — see
// resolvePeriodInventoryBasis.
type periodInventoryBasis struct {
	beginning Value
	ending    Value
	average   Value
	basis     AverageBasis
	cogs      Value
	days      int
}

// resolveAverageInventory computes AverageInventory from beginning/ending
// per task section 13's default formula, falling back to ending-only when
// beginning is unavailable, and preserving an explicit caller-supplied
// average when one is given directly (average != Unavailable takes
// precedence over recomputing from beginning/ending).
func resolveAverageInventory(beginning, ending, callerAverage Value) (Value, AverageBasis) {
	if callerAverage.Available {
		return callerAverage, AverageBasisCallerSupplied
	}
	if beginning.Available && ending.Available {
		return AvailableValue((beginning.Amount + ending.Amount) / 2), AverageBasisBeginningEnding
	}
	if ending.Available {
		return ending, AverageBasisEndingOnly
	}
	return Unavailable(), ""
}

// calculateTurnover computes COGS / AverageInventory — task section 13.
// Unavailable if either input is unavailable or AverageInventory is zero
// (never substitutes revenue for COGS — task section 13's explicit rule).
func calculateTurnover(period string, basis periodInventoryBasis) TurnoverResult {
	r := TurnoverResult{Period: period, AverageInventory: basis.average, AverageBasis: basis.basis, COGS: basis.cogs}
	if !basis.cogs.Available || !basis.average.Available || basis.average.Amount == 0 {
		return r
	}
	r.Available = true
	r.Value = basis.cogs.Amount / basis.average.Amount
	return r
}

// calculateDIO computes (AverageInventory / COGS) * Days — task section
// 14. Unavailable (not Inf) if COGS is zero/unavailable or Days <= 0.
func calculateDIO(period string, basis periodInventoryBasis) DIOResult {
	r := DIOResult{Period: period, AverageInventory: basis.average, AverageBasis: basis.basis, COGS: basis.cogs, Days: basis.days}
	if !basis.cogs.Available || basis.cogs.Amount == 0 || !basis.average.Available || basis.days <= 0 {
		return r
	}
	r.Available = true
	r.Value = (basis.average.Amount / basis.cogs.Amount) * float64(basis.days)
	return r
}

// TurnoverHistoryPoint is one period's turnover/DIO in a historical
// series — task section 15.
type TurnoverHistoryPoint struct {
	Period   string         `json:"period"`
	Turnover TurnoverResult `json:"turnover"`
	DIO      DIOResult      `json:"dio"`
}

// TurnoverHistory is the chronological turnover/DIO series across every
// supplied period — task section 15: adjacent-period change, first-vs-
// last, trend. No future prediction.
type TurnoverHistory struct {
	Available bool                   `json:"available"`
	Points    []TurnoverHistoryPoint `json:"points,omitempty"`

	// TurnoverFirstVsLastChange/DIOFirstVsLastChange are Points[last] -
	// Points[0], unavailable if fewer than 2 available points exist.
	TurnoverFirstVsLastChange Value `json:"turnover_first_vs_last_change"`
	DIOFirstVsLastChange      Value `json:"dio_first_vs_last_change"`
	// DIOAdjacentChanges is one entry per adjacent pair of available DIO
	// points.
	DIOAdjacentChanges []Value `json:"dio_adjacent_changes,omitempty"`

	// DIOTrend: "improving" means DIO decreased (turning faster);
	// "deteriorating" means DIO increased; "flat" means no material
	// change (below turnoverTrendTolerance); "" if unavailable. This
	// label describes direction only, mirroring ap.DPOHistory.Trend's
	// identical "direction only, not a value judgment" framing — a
	// declining DIO is usually favorable but this package does not
	// assert that universally (e.g. an aggressive just-in-time cut that
	// causes stockouts is not obviously "better").
	DIOTrend string `json:"dio_trend,omitempty"`
}

// turnoverTrendTolerance is the minimum |change in DIO days| for
// TurnoverHistory.DIOTrend to report "improving"/"deteriorating" rather
// than "flat."
const turnoverTrendTolerance = 0.5

// buildTurnoverHistory computes one TurnoverHistoryPoint per period with
// a resolvable basis, in caller-supplied chronological order (periods is
// expected pre-sorted by the caller — see sortedPeriods).
func buildTurnoverHistory(periods []PeriodInfo, basisByPeriod map[string]periodInventoryBasis) TurnoverHistory {
	var points []TurnoverHistoryPoint
	for _, p := range periods {
		basis, ok := basisByPeriod[p.Period]
		if !ok {
			continue
		}
		points = append(points, TurnoverHistoryPoint{
			Period:   p.Period,
			Turnover: calculateTurnover(p.Period, basis),
			DIO:      calculateDIO(p.Period, basis),
		})
	}
	if len(points) == 0 {
		return TurnoverHistory{}
	}
	h := TurnoverHistory{Available: true, Points: points}

	var availableTurnover, availableDIO []TurnoverHistoryPoint
	for _, pt := range points {
		if pt.Turnover.Available {
			availableTurnover = append(availableTurnover, pt)
		}
		if pt.DIO.Available {
			availableDIO = append(availableDIO, pt)
		}
	}
	if len(availableTurnover) >= 2 {
		first, last := availableTurnover[0].Turnover.Value, availableTurnover[len(availableTurnover)-1].Turnover.Value
		h.TurnoverFirstVsLastChange = AvailableValue(last - first)
	}
	if len(availableDIO) >= 2 {
		first, last := availableDIO[0].DIO.Value, availableDIO[len(availableDIO)-1].DIO.Value
		change := last - first
		h.DIOFirstVsLastChange = AvailableValue(change)
		switch {
		case change > turnoverTrendTolerance:
			h.DIOTrend = "deteriorating"
		case change < -turnoverTrendTolerance:
			h.DIOTrend = "improving"
		default:
			h.DIOTrend = "flat"
		}
		for i := 1; i < len(availableDIO); i++ {
			h.DIOAdjacentChanges = append(h.DIOAdjacentChanges, AvailableValue(availableDIO[i].DIO.Value-availableDIO[i-1].DIO.Value))
		}
	}
	return h
}

// ItemTurnover is item/category-level turnover — task section 16.
// Available only if the caller supplied item/category-level COGS/usage
// cost directly; this package never allocates company-level COGS to
// items using guessed percentages.
type ItemTurnover struct {
	Key      string         `json:"key"` // ItemID or category name, per grouping
	Turnover TurnoverResult `json:"turnover"`
	DIO      DIOResult      `json:"dio"`
}
