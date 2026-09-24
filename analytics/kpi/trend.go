package kpi

// TrendDirection is the caller-configurable-tolerance direction a KPI's
// period series moved — task section 24. This package labels only
// direction (numeric increase/decrease/stability), never
// "improving"/"deteriorating" (task section 24's explicit "do not label
// improving/deteriorating unless caller supplies direction semantics"
// instruction — no such semantics field exists in V1, so this package
// never emits an improving/deteriorating label at all).
type TrendDirection string

const (
	TrendIncreasing  TrendDirection = "INCREASING"
	TrendDecreasing  TrendDirection = "DECREASING"
	TrendStable      TrendDirection = "STABLE"
	TrendUnavailable TrendDirection = "UNAVAILABLE"
)

// Change is one KPI's current-vs-prior comparison — task sections 21/24.
// AbsoluteChange/RelativeChangePercent/PercentagePointChange are computed
// independently and are each individually Available/unavailable; a caller
// reads whichever is meaningful for this KPI's Unit (task section 13:
// PercentagePointChange is meaningful only when the KPI's own Unit is
// UnitPercent — see computeChange).
type Change struct {
	Current Value `json:"current"`
	Prior   Value `json:"prior"`
	// AbsoluteChange is Current - Prior, in the KPI's own Unit.
	AbsoluteChange Value `json:"absolute_change"`
	// RelativeChangePercent is (Current - Prior) / Prior * 100 — a
	// *relative* percent change, unavailable when Prior is exactly 0.
	// Meaningful for any Unit, but see PercentagePointChange for why a
	// UnitPercent KPI usually wants that field instead — task section
	// 13's "do not mix them" instruction: a 30% -> 35% KPI's
	// RelativeChangePercent is +16.7%, its PercentagePointChange is +5;
	// both are always computed, a caller picks the one it means.
	RelativeChangePercent Value `json:"relative_change_percent"`
	// PercentagePointChange is Current - Prior, computed ONLY when the
	// KPI's own Unit is UnitPercent (otherwise left unavailable — it
	// would be identical to AbsoluteChange for any other Unit and so
	// carries no separate meaning). Task section 13's worked example: 30%
	// -> 35% is +5 percentage points here.
	PercentagePointChange Value `json:"percentage_point_change"`
}

// computeChange builds a Change from current/prior Values and unit —
// current/prior must already be resolved (unavailable Values propagate
// naturally: every downstream field stays unavailable).
func computeChange(current, prior Value, unit Unit) Change {
	c := Change{Current: current, Prior: prior}
	if !current.Available || !prior.Available {
		return c
	}
	c.AbsoluteChange = availableValue(current.Amount - prior.Amount)
	if prior.Amount != 0 {
		c.RelativeChangePercent = availableValue((current.Amount - prior.Amount) / prior.Amount * 100)
	} else {
		c.RelativeChangePercent = unavailableValue(AvailabilityDivideByZero)
	}
	if unit.Kind == UnitPercent {
		c.PercentagePointChange = availableValue(current.Amount - prior.Amount)
	}
	return c
}

// TrendPoint is one period's value within a Trend series.
type TrendPoint struct {
	Period string `json:"period"`
	Value  Value  `json:"value"`
}

// Trend is a KPI's full period series with adjacent/first-vs-last
// comparisons and an overall direction — task section 24. Only populated
// when Options.IncludeTrend is set (avoids unnecessary payload by
// default, mirroring Options.IncludeTrace's identical opt-in convention —
// task section 25).
type Trend struct {
	Points []TrendPoint `json:"points,omitempty"`
	// AdjacentAbsoluteChange/AdjacentPercentChange are one Change per
	// adjacent pair of Points, in chronological order (length =
	// len(Points)-1, or 0 if fewer than 2 Points).
	Adjacent []Change `json:"adjacent,omitempty"`
	// FirstVsLast compares Points[0] to Points[len-1] directly (skipping
	// every point in between) — task section 24's explicit
	// "first-vs-last" output.
	FirstVsLast Change `json:"first_vs_last"`
	// Direction summarizes FirstVsLast.AbsoluteChange against
	// Options.TrendStabilityTolerance — see resolveTrendDirection.
	Direction TrendDirection `json:"direction"`
}

// buildTrend evaluates code for dim across every period known to this
// Calculate call (ctx.periodOrder — every valid Input.Periods entry,
// chronologically ordered, NOT the possibly-narrower Options.Periods
// evaluation subset — a Trend for one requested period should still show
// that period's full available history) up to and including
// targetPeriod, building the full Points/Adjacent/FirstVsLast/Direction
// series — task section 24. Only periods at or before targetPeriod's
// chronological position are included, so a KPI's Trend for an early
// period never looks ahead at later periods' values.
func buildTrend(ctx *evalContext, code string, dim DimensionKey, targetPeriod string, tolerance float64) *Trend {
	var upTo []string
	for _, p := range ctx.periodOrder {
		upTo = append(upTo, p.Code)
		if p.Code == targetPeriod {
			break
		}
	}
	if len(upTo) == 0 {
		return &Trend{Direction: TrendUnavailable}
	}

	t := &Trend{}
	def := ctx.graph.byCode[code]
	for _, p := range upTo {
		res := ctx.evaluateKPI(code, evalRequest{period: p, dim: dim}, map[string]bool{})
		t.Points = append(t.Points, TrendPoint{Period: p, Value: res.value})
	}
	for i := 1; i < len(t.Points); i++ {
		t.Adjacent = append(t.Adjacent, computeChange(t.Points[i].Value, t.Points[i-1].Value, def.Unit))
	}
	if len(t.Points) >= 2 {
		t.FirstVsLast = computeChange(t.Points[len(t.Points)-1].Value, t.Points[0].Value, def.Unit)
		t.Direction = resolveTrendDirection(t.FirstVsLast.AbsoluteChange, tolerance)
	} else {
		t.Direction = TrendUnavailable
	}
	return t
}

// resolveTrendDirection classifies change against tolerance (an absolute
// value in the KPI's own Unit; a caller wanting a relative/percentage
// tolerance instead computes it from RelativeChangePercent itself and
// compares that in its own code — this package's Direction field only
// ever reflects the one tolerance basis it was configured with, never
// silently switches basis).
func resolveTrendDirection(change Value, tolerance float64) TrendDirection {
	if !change.Available {
		return TrendUnavailable
	}
	if tolerance < 0 {
		tolerance = 0
	}
	switch {
	case change.Amount > tolerance:
		return TrendIncreasing
	case change.Amount < -tolerance:
		return TrendDecreasing
	default:
		return TrendStable
	}
}
