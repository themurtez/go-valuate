package report

import (
	"fmt"

	"github.com/themurtez/go-valuate/financial/adjustments"
	"github.com/themurtez/go-valuate/financial/metrics"
	"github.com/themurtez/go-valuate/valuation"
	"github.com/themurtez/go-valuate/valuation/consensus"
	"github.com/themurtez/go-valuate/valuation/orchestrator"
	"github.com/themurtez/go-valuate/valuation/sensitivity"
)

// BuildInput is everything Build can incorporate into a Report. Every
// field is optional: Build never requires a caller to have run every
// upstream package, and simply leaves the corresponding Report section at
// its zero value when a field is omitted — mirroring every method
// package's "Available" convention, applied here at the section level
// instead of a single figure. This lets a caller build a minimal Report
// (e.g. just Methods) as easily as a complete end-to-end one.
type BuildInput struct {
	// ValuationDate is an optional as-of date, carried straight through to
	// Summary.ValuationDate. See Summary.ValuationDate's doc comment on its
	// expected form; Build does not parse or validate it.
	ValuationDate string

	// Snapshots is the historical financial/metrics.Snapshot series, in
	// chronological order (oldest first), used to build FinancialSummary
	// and the Revenue/EBITDA/SDE/Margin chart series.
	Snapshots []metrics.Snapshot

	// NormalizedEBITDA and NormalizedSDE are the maintainable normalized
	// earnings figures actually used as valuation inputs (e.g.
	// financial/earnings.Result.Value over a financial/adjustments bridge
	// series). Available is false to omit either figure from
	// FinancialSummary.
	NormalizedEBITDA FinancialFigure
	NormalizedSDE    FinancialFigure

	// GrowthMetrics is an already-reshaped list of historical growth/CAGR/
	// volatility figures (e.g. derived from financial/metrics.Trend) —
	// Build does not know how to interpret a metrics.Trend itself, since
	// deciding which of its many fields are report-worthy is a caller/
	// presentation choice; the caller supplies the finished
	// (label, value) pairs.
	GrowthMetrics []Assumption

	// AdjustmentsEBITDA and AdjustmentsSDE are the applied normalization
	// bridges (financial/adjustments.Result.EBITDABridge/SDEBridge) for
	// whichever period(s) the caller wants reflected in
	// AdjustmentSummary — most often the single period whose normalized
	// figures fed NormalizedEBITDA/NormalizedSDE.
	AdjustmentsEBITDA *adjustments.Result
	AdjustmentsSDE    *adjustments.Result

	// Run is the orchestrator.Run whose MethodOutcomes populate Methods and
	// (indirectly, via the caller's own Weight assignment when building
	// ConsensusInput — see BuildConsensusInputs) feed Consensus.
	Run *orchestrator.Run

	// Consensus is the consensus.Result to summarize into Summary. Usually
	// computed by the caller via consensus.Calculate over
	// BuildConsensusInputs(Run, weights) before calling Build.
	Consensus *consensus.Result

	// MultipleSensitivity, EarningsMultipleMatrix, and DCFSensitivity are
	// already-computed valuation/sensitivity outputs to flatten into
	// SensitivityData.
	MultipleSensitivity    *sensitivity.MultipleSensitivityResult
	EarningsMultipleMatrix *sensitivity.Matrix
	DCFSensitivity         *sensitivity.DCFGrid
}

// BuildConsensusInputs is a convenience that converts every successful
// MethodOutcome in run into a consensus.Input slice, applying
// caller-supplied weights by method code (a method absent from weights
// gets Weight 0, which — per valuation/consensus.ValidateWeights — is
// valid as long as at least one included method has a positive weight; an
// all-zero weight set makes WeightedMean unavailable rather than the whole
// consensus). This is a thin convenience, not a requirement: a caller
// building its own []consensus.Input (e.g. to exclude a successful method
// from consensus deliberately) is free to skip this and call
// consensus.Calculate directly.
func BuildConsensusInputs(run orchestrator.Run, weights map[valuation.Code]float64) []consensus.Input {
	successful := run.Successful()
	inputs := make([]consensus.Input, 0, len(successful))
	for _, m := range successful {
		value, valueType, ok := headlineValue(m)
		if !ok {
			continue
		}
		inputs = append(inputs, consensus.Input{
			Method:    m.Method,
			ValueType: valueType,
			Value:     value,
			Weight:    weights[m.Method],
		})
	}
	return inputs
}

// headlineValue extracts a MethodOutcome's headline figure and value type
// from whichever method Result is populated.
func headlineValue(m orchestrator.MethodOutcome) (value float64, valueType valuation.ValueType, ok bool) {
	switch {
	case m.SDE != nil:
		return m.SDE.EquityValue, m.SDE.ValueType, true
	case m.EBITDA != nil:
		return m.EBITDA.EnterpriseValue, m.EBITDA.ValueType, true
	case m.Capitalization != nil:
		return m.Capitalization.EquityValue, m.Capitalization.ValueType, true
	case m.DCF != nil:
		return m.DCF.EnterpriseValue, m.DCF.ValueType, true
	case m.NetAssets != nil:
		return m.NetAssets.AdjustedNetAssetValue, m.NetAssets.ValueType, true
	default:
		return 0, "", false
	}
}

// Build assembles a complete Report from in. Every section is independent:
// a nil/zero field on in simply leaves the corresponding Report section at
// its zero value rather than causing Build to fail or panic — see
// BuildInput's doc comment.
func Build(in BuildInput) Report {
	return Report{
		Summary:     buildSummary(in),
		Financial:   buildFinancialSummary(in),
		Methods:     buildMethods(in),
		Adjustments: buildAdjustmentSummary(in),
		Sensitivity: buildSensitivityData(in),
		Series:      buildSeries(in),
	}
}

func buildSummary(in BuildInput) Summary {
	s := Summary{ValuationDate: in.ValuationDate}
	if in.Consensus == nil || !in.Consensus.Available {
		return s
	}
	c := in.Consensus
	s.ConsensusAvailable = true
	s.SimpleConsensus = c.Statistics.SimpleMean
	s.WeightsValid = c.WeightsValid
	if c.WeightsValid {
		s.WeightedConsensus = c.Statistics.WeightedMean
	}
	s.Median = c.Statistics.Median
	s.MethodRange = c.Range
	s.ConsensusLevel = c.Dispersion.Level
	s.ConsensusScore = c.Dispersion.Score
	s.IncludedMethodCount = c.Statistics.Count
	return s
}

func buildFinancialSummary(in BuildInput) FinancialSummary {
	fs := FinancialSummary{
		NormalizedEBITDA: in.NormalizedEBITDA,
		NormalizedSDE:    in.NormalizedSDE,
		GrowthMetrics:    in.GrowthMetrics,
	}
	fs.Periods = make([]FinancialPeriod, 0, len(in.Snapshots))
	for _, snap := range in.Snapshots {
		fs.Periods = append(fs.Periods, FinancialPeriod{
			Period:       string(snap.Period),
			Revenue:      figureFrom(snap.TotalRevenue),
			EBITDA:       figureFrom(snap.EBITDA),
			EBITDAMargin: figureFrom(snap.EBITDAMargin),
			SDE:          figureFrom(snap.SDE),
			GrossMargin:  figureFrom(snap.GrossMargin),
		})
	}
	return fs
}

func figureFrom(mv metrics.MetricValue) FinancialFigure {
	return FinancialFigure{Available: mv.Available, Value: mv.Value}
}

// methodOrder is the fixed display order for the method comparison table,
// matching valuation/orchestrator.Execute's own fixed evaluation order.
var methodOrder = []valuation.Code{
	valuation.CodeSDEMultiple,
	valuation.CodeEBITDAMultiple,
	valuation.CodeCapitalizationOfEarnings,
	valuation.CodeDCF,
	valuation.CodeAdjustedNetAssetValue,
}

func buildMethods(in BuildInput) []MethodComparisonRow {
	if in.Run == nil {
		return nil
	}

	weightByMethod := normalizedWeightsByMethod(in.Consensus)

	rows := make([]MethodComparisonRow, 0, len(methodOrder))
	for _, code := range methodOrder {
		outcome, ok := findOutcome(*in.Run, code)
		if !ok {
			continue
		}
		row := MethodComparisonRow{
			Method:  code,
			Outcome: string(outcome.Outcome),
		}
		if outcome.Applicability != nil {
			row.Applicability = outcome.Applicability
		}
		switch outcome.Outcome {
		case orchestrator.OutcomeExcluded:
			row.ExclusionReason = string(outcome.ExclusionReason)
			row.Warnings = []string{outcome.Detail}
		case orchestrator.OutcomeSuccess, orchestrator.OutcomeUnavailable:
			populateMethodRow(&row, outcome)
			row.Included = outcome.Outcome == orchestrator.OutcomeSuccess
			if w, ok := weightByMethod[code]; ok {
				row.Weight = w
			}
		}
		rows = append(rows, row)
	}
	return rows
}

// normalizedWeightsByMethod re-derives the normalized weight actually used
// in WeightedMean for each included method, by re-running
// consensus.ValidateWeights over c.Included — the same normalization
// consensus.Calculate itself applied. Returns an empty map if c is nil or
// its weights were invalid (WeightsValid false), in which case
// MethodComparisonRow.Weight is left at its zero value rather than
// displaying a meaningless raw weight.
func normalizedWeightsByMethod(c *consensus.Result) map[valuation.Code]float64 {
	out := make(map[valuation.Code]float64)
	if c == nil || !c.WeightsValid {
		return out
	}
	normalized, ok, _ := consensus.ValidateWeights(c.Included)
	if !ok {
		return out
	}
	for i, in := range c.Included {
		out[in.Method] = normalized[i]
	}
	return out
}

func findOutcome(run orchestrator.Run, code valuation.Code) (orchestrator.MethodOutcome, bool) {
	for _, m := range run.Methods {
		if m.Method == code {
			return m, true
		}
	}
	return orchestrator.MethodOutcome{}, false
}

// populateMethodRow fills in Value/ValueType/MethodVersion/Assumptions/
// Steps/Warnings from whichever of outcome's method Results is populated.
func populateMethodRow(row *MethodComparisonRow, outcome orchestrator.MethodOutcome) {
	switch {
	case outcome.SDE != nil:
		r := outcome.SDE
		row.MethodVersion = r.MethodVersion
		row.ValueType = r.ValueType
		row.Value = r.EquityValue
		row.Steps = r.Steps
		row.Assumptions = []Assumption{
			{Label: "Maintainable SDE", Value: formatMoney(r.Input.MaintainableSDE)},
			{Label: "Multiple", Value: fmt.Sprintf("%.2fx", r.Input.Multiple)},
		}
		row.Warnings = issueMessages(r.Warnings, r.Errors)
	case outcome.EBITDA != nil:
		r := outcome.EBITDA
		row.MethodVersion = r.MethodVersion
		row.ValueType = r.ValueType
		row.Value = r.EnterpriseValue
		row.Steps = r.Steps
		row.Assumptions = []Assumption{
			{Label: "Maintainable EBITDA", Value: formatMoney(r.Input.MaintainableEBITDA)},
			{Label: "Multiple", Value: fmt.Sprintf("%.2fx", r.Input.Multiple)},
		}
		row.Warnings = issueMessages(r.Warnings, r.Errors)
	case outcome.Capitalization != nil:
		r := outcome.Capitalization
		row.MethodVersion = r.MethodVersion
		row.ValueType = r.ValueType
		row.Value = r.EquityValue
		row.Steps = r.Steps
		row.Assumptions = []Assumption{
			{Label: "Maintainable Earnings", Value: formatMoney(r.Input.MaintainableEarnings)},
			{Label: "Capitalization Rate", Value: fmt.Sprintf("%.1f%%", r.Input.CapitalizationRate*100)},
		}
		row.Warnings = issueMessages(r.Warnings, r.Errors)
	case outcome.DCF != nil:
		r := outcome.DCF
		row.MethodVersion = r.MethodVersion
		row.ValueType = r.ValueType
		row.Value = r.EnterpriseValue
		row.Steps = r.Steps
		row.Assumptions = []Assumption{
			{Label: "Forecast Periods", Value: fmt.Sprintf("%d", len(r.Input.ForecastPeriods))},
			{Label: "Discount Rate", Value: fmt.Sprintf("%.1f%%", r.Input.DiscountRate*100)},
			{Label: "Terminal Growth Rate", Value: fmt.Sprintf("%.1f%%", r.Input.TerminalGrowthRate*100)},
		}
		row.Warnings = issueMessages(r.Warnings, r.Errors)
	case outcome.NetAssets != nil:
		r := outcome.NetAssets
		row.MethodVersion = r.MethodVersion
		row.ValueType = r.ValueType
		row.Value = r.AdjustedNetAssetValue
		row.Steps = r.Steps
		row.Assumptions = []Assumption{
			{Label: "Total Adjusted Assets", Value: formatMoney(r.TotalAdjustedAssets)},
			{Label: "Total Adjusted Liabilities", Value: formatMoney(r.TotalAdjustedLiabilities)},
		}
		row.Warnings = issueMessages(r.Warnings, r.Errors)
	}
}

func formatMoney(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func issueMessages(warnings, errs []valuation.Issue) []string {
	out := make([]string, 0, len(warnings)+len(errs))
	for _, w := range warnings {
		out = append(out, w.Message)
	}
	for _, e := range errs {
		out = append(out, e.Message)
	}
	return out
}

func buildAdjustmentSummary(in BuildInput) AdjustmentSummary {
	var summary AdjustmentSummary
	if in.AdjustmentsEBITDA != nil {
		summary.Applied = append(summary.Applied, flattenApplied(*in.AdjustmentsEBITDA, "ebitda")...)
		summary.EBITDABridge = flattenBridge(in.AdjustmentsEBITDA.EBITDABridge, "EBITDA")
	}
	if in.AdjustmentsSDE != nil {
		summary.Applied = append(summary.Applied, flattenApplied(*in.AdjustmentsSDE, "sde")...)
		summary.SDEBridge = flattenBridge(in.AdjustmentsSDE.SDEBridge, "SDE")
	}
	return summary
}

func flattenApplied(res adjustments.Result, target string) []AppliedAdjustment {
	var bridge adjustments.Bridge
	switch target {
	case "ebitda":
		bridge = res.EBITDABridge
	case "sde":
		bridge = res.SDEBridge
	}
	out := make([]AppliedAdjustment, 0, len(bridge.Applied))
	for _, line := range bridge.Applied {
		out = append(out, AppliedAdjustment{
			Period:       res.Period,
			Type:         string(line.Adjustment.Type),
			Reason:       line.Adjustment.Reason,
			SignedAmount: line.SignedAmount,
			Target:       target,
		})
	}
	return out
}

func flattenBridge(b adjustments.Bridge, label string) []BridgeLine {
	if !b.BaseAvailable {
		return nil
	}
	lines := make([]BridgeLine, 0, len(b.Applied)+2)
	lines = append(lines, BridgeLine{Label: "Reported " + label, Amount: b.BaseValue, IsTotal: true})
	for _, applied := range b.Applied {
		lines = append(lines, BridgeLine{
			Label:  adjustmentLineLabel(applied),
			Amount: applied.SignedAmount,
		})
	}
	lines = append(lines, BridgeLine{Label: "Normalized " + label, Amount: b.NormalizedValue, IsTotal: true})
	return lines
}

func adjustmentLineLabel(line adjustments.AppliedLine) string {
	sign := "+"
	if line.SignedAmount < 0 {
		sign = "-"
	}
	reason := line.Adjustment.Reason
	if reason == "" {
		reason = string(line.Adjustment.Type)
	}
	return sign + " " + reason
}

func buildSensitivityData(in BuildInput) SensitivityData {
	var data SensitivityData
	if in.MultipleSensitivity != nil {
		for _, p := range in.MultipleSensitivity.Points {
			data.MultipleSensitivity = append(data.MultipleSensitivity, SensitivityRow{
				Multiple: p.Multiple, Value: p.Value, Valid: p.Valid, Reason: p.Reason,
			})
		}
	}
	if in.EarningsMultipleMatrix != nil {
		for _, row := range in.EarningsMultipleMatrix.Rows {
			for _, cell := range row {
				data.EarningsMultipleMatrix = append(data.EarningsMultipleMatrix, MatrixRow{
					EarningsLabel: cell.EarningsLabel, Earnings: cell.Earnings,
					Multiple: cell.Multiple, Value: cell.Value, Valid: cell.Valid, Reason: cell.Reason,
				})
			}
		}
	}
	if in.DCFSensitivity != nil {
		for _, row := range in.DCFSensitivity.Rows {
			for _, cell := range row {
				data.DCFSensitivity = append(data.DCFSensitivity, DCFSensitivityRow{
					DiscountRate: cell.DiscountRate, TerminalGrowthRate: cell.TerminalGrowthRate,
					Valid: cell.Valid, EnterpriseValue: cell.EnterpriseValue,
				})
			}
		}
	}
	return data
}

func buildSeries(in BuildInput) ChartSeries {
	var series ChartSeries

	if in.Run != nil {
		for _, code := range methodOrder {
			outcome, ok := findOutcome(*in.Run, code)
			if !ok || outcome.Outcome != orchestrator.OutcomeSuccess {
				continue
			}
			value, _, ok := headlineValue(outcome)
			if !ok {
				continue
			}
			series.ValuationByMethod = append(series.ValuationByMethod, SeriesPoint{Label: string(code), Value: value})
		}
	}

	for _, snap := range in.Snapshots {
		if snap.TotalRevenue.Available {
			series.RevenueHistory = append(series.RevenueHistory, SeriesPoint{Label: string(snap.Period), Value: snap.TotalRevenue.Value})
		}
		if snap.EBITDA.Available {
			series.EBITDAHistory = append(series.EBITDAHistory, SeriesPoint{Label: string(snap.Period), Value: snap.EBITDA.Value})
		}
		if snap.SDE.Available {
			series.SDEHistory = append(series.SDEHistory, SeriesPoint{Label: string(snap.Period), Value: snap.SDE.Value})
		}
		if snap.EBITDAMargin.Available {
			series.MarginHistory = append(series.MarginHistory, SeriesPoint{Label: string(snap.Period), Value: snap.EBITDAMargin.Value})
		}
	}

	return series
}
