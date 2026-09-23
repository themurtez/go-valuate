// Package benchmarks compares caller-supplied company metrics against
// caller-supplied benchmark datasets — a peer-group or industry-survey
// comparison ("how does our gross margin compare to the industry median,"
// "what percentile is our DSCR in versus our peer set") rather than an
// absolute pass/fail test against a fixed rule (that is analytics/covenants'
// job) or a metric this package computes itself.
//
// This package does not source, fetch, or redistribute benchmark data.
// Every BenchmarkSet — a median, a percentile band table, a quartile
// triple, or a list of explicit peer observations — is supplied entirely
// by the caller, who is responsible for having the appropriate license or
// permission to use whatever third-party survey, association report, or
// peer dataset it derives from. This package only performs the comparison
// arithmetic (percentile estimation, difference, relative difference,
// band/quartile placement) and echoes back the provenance the caller
// attaches to each BenchmarkSet (source name, effective date, population/
// segment, and an optional license/source ID) so a consumer of Result can
// always trace a comparison back to what it was benchmarked against.
//
// Like analytics/debt, analytics/variance, and analytics/covenants, this
// package is deliberately independent of financial.FinancialDataset: a
// company metric to benchmark is typically already computed upstream (via
// financial/metrics, analytics/ratios, analytics/debt, or any other
// caller-side calculation) and a benchmark dataset routinely comes from a
// source with no relationship to this repository's canonical taxonomy at
// all (an industry association survey, a peer-group financial statement
// study, a proprietary database export). This package's only required
// input shape is Comparison{CompanyValue, BenchmarkSet} plus caller
// metadata; it never requires a financial.Code or a financial.Period,
// though a caller may attach a Period for display purposes.
//
// Favorable/unfavorable classification is applied only where the caller
// supplies Direction — this package has no opinion on whether a given
// metric is better higher or lower (a caller's DSO is better lower, a
// caller's gross margin is better higher, and some metrics have no
// inherent direction at all) and never guesses.
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state. Calculate can be called concurrently
// and repeatedly against identical input and always returns byte-for-byte
// identical JSON — see determinism_test.go.
package benchmarks

import "github.com/themurtez/go-valuate/financial"

// FormulaVersion identifies this package's fixed formula set: the
// percentile-band linear interpolation rule, the peer-observation
// rank-based percentile estimate, the quartile-band placement rule, the
// difference/relative-difference formulas, and the favorable/unfavorable
// classification rule. Bump this whenever any of that changes in a way
// that could make a historical Result not reproduce identically under new
// code — see the repository README's versioning-strategy section, which
// this constant follows exactly (financial.TaxonomyVersion, metrics.
// FormulaVersion, debt.FormulaVersion, covenants.FormulaVersion, etc.).
const FormulaVersion = "1.0.0"

// Value represents a single figure that may or may not be available,
// distinguishing "computed/reported to be exactly 0" from "unknown because
// a required input was absent" — the same availability convention
// debt.Value/covenants.Value/variance.VarianceValue/metrics.MetricValue
// all use, duplicated here as its own type per this repository's
// established convention (see cashflow.CashFlowValue's doc comment) rather
// than importing another analytics package into a package that otherwise
// has no dependency on it.
type Value struct {
	// Available is true if Amount is meaningful.
	Available bool `json:"available"`
	// Amount is the figure itself. Meaningful only when Available is true;
	// always 0 when Available is false.
	Amount float64 `json:"amount"`
}

// Unavailable is the canonical zero-information Value.
func Unavailable() Value { return Value{} }

// AvailableValue reports a Value for a known figure (which may
// legitimately be zero or negative).
func AvailableValue(v float64) Value { return Value{Available: true, Amount: v} }

// Direction states whether a higher or lower company value is favorable
// for a given metric, entirely at the caller's discretion — this package
// applies no built-in opinion (a gross margin is better higher, a DSO is
// better lower, and many metrics — an owner-compensation ratio, a
// composition percentage — have no universal "better" direction at all).
type Direction string

const (
	// DirectionUnspecified means the caller did not state a direction;
	// Calculate never derives Favorable in this case.
	DirectionUnspecified Direction = ""
	// DirectionHigherIsBetter means a company value above the benchmark is
	// favorable.
	DirectionHigherIsBetter Direction = "HIGHER_IS_BETTER"
	// DirectionLowerIsBetter means a company value below the benchmark is
	// favorable.
	DirectionLowerIsBetter Direction = "LOWER_IS_BETTER"
	// DirectionNeutral means the caller explicitly considered direction
	// and determined this metric has no favorable/unfavorable sense (e.g.
	// a composition percentage). Distinct from DirectionUnspecified so a
	// caller can record "I considered this and there is no direction"
	// separately from "I never set this field."
	DirectionNeutral Direction = "NEUTRAL"
)

// BenchmarkForm identifies which shape of benchmark data a BenchmarkSet
// carries. Calculate reads only the fields BenchmarkSet documents as
// belonging to the declared Form; a caller populating a field outside its
// declared Form has that data ignored (see BenchmarkSet's field
// comments), not merged in as if it were additional evidence.
type BenchmarkForm string

const (
	// FormMedian means only BenchmarkSet.Median is populated. No
	// percentile estimate or band/quartile placement is calculable; only
	// the difference/relative-difference against the median.
	FormMedian BenchmarkForm = "MEDIAN"
	// FormPercentileBands means BenchmarkSet.PercentileBands is populated
	// with one or more (percentile, value) points, enabling linear-
	// interpolation percentile estimation and range reporting.
	FormPercentileBands BenchmarkForm = "PERCENTILE_BANDS"
	// FormQuartiles means BenchmarkSet.Quartiles is populated (Q1/Median/
	// Q3), enabling quartile placement (Q1/Q2/Q3/Q4) and an interpolated
	// percentile estimate across the three known points.
	FormQuartiles BenchmarkForm = "QUARTILES"
	// FormPeerObservations means BenchmarkSet.PeerObservations is
	// populated with explicit individual peer values, enabling an
	// empirical rank-based percentile estimate, a computed median, and
	// min/max range — the richest form, since Calculate can derive every
	// other statistic from the raw observations themselves.
	FormPeerObservations BenchmarkForm = "PEER_OBSERVATIONS"
)

// PercentilePoint is one (percentile, value) pair in a percentile-band
// benchmark table (e.g. {25, 0.18} meaning the 25th percentile of the
// population is at value 0.18).
type PercentilePoint struct {
	// Percentile is in [0, 100].
	Percentile float64 `json:"percentile"`
	// Value is the benchmark population's value at Percentile.
	Value float64 `json:"value"`
}

// Quartiles is the common special case of a three-point percentile
// table — Q1 (25th percentile), Median (50th percentile), and Q3 (75th
// percentile) — broken out as its own explicit struct (rather than always
// requiring a caller to build a three-element PercentileBands slice)
// since a quartile table is how most published industry benchmark reports
// actually present their data.
type Quartiles struct {
	Q1     Value `json:"q1"`
	Median Value `json:"median"`
	Q3     Value `json:"q3"`
}

// PeerObservation is one individual peer's value within an explicit peer
// set — the richest BenchmarkForm, since Calculate derives every other
// statistic (median, quartiles, percentile rank, range) directly from the
// raw observations rather than trusting a pre-aggregated summary.
type PeerObservation struct {
	// PeerKey is an opaque, caller-assigned identifier for this peer (an
	// internal ID, a hash, or a display label). This package never
	// requires it to be a real company name and treats it purely as a
	// label — mirroring concentration.Observation.EntityKey's identical
	// privacy stance. Optional; may be empty for an anonymous peer.
	PeerKey string `json:"peer_key,omitempty"`
	// Value is this peer's figure for the metric being benchmarked.
	Value float64 `json:"value"`
}

// BenchmarkSource is provenance metadata a caller attaches to a
// BenchmarkSet, preserved verbatim on every Comparison so a consumer can
// always trace a comparison back to what it was benchmarked against. This
// package never fetches or validates any of these fields; it only stores
// and echoes them.
type BenchmarkSource struct {
	// Name identifies the benchmark's source (e.g. "RMA Annual Statement
	// Studies", "IBISWorld Industry Report", "internal peer set").
	// Required for a well-formed BenchmarkSet; Calculate records
	// IssueMissingSourceName and still evaluates the comparison if empty
	// (advisory only — the arithmetic does not depend on it).
	Name string `json:"name"`
	// EffectiveDate is the date this benchmark data is as-of (e.g.
	// "2025-Q2", "2025-06-30", "FY2024"), in whatever granularity the
	// source itself uses. Free text — this package draws no chronological
	// inference from it.
	EffectiveDate string `json:"effective_date,omitempty"`
	// Population describes the benchmark population or peer segment (e.g.
	// "US manufacturing, $10M-$50M revenue", "SaaS companies, Series B+").
	Population string `json:"population,omitempty"`
	// SampleSize is the number of entities/observations the benchmark
	// summary statistic (median, quartiles, percentile bands) was derived
	// from, if the source discloses it. Zero means not specified. Purely
	// descriptive; Calculate never uses it to weight or validate the
	// comparison.
	SampleSize int `json:"sample_size,omitempty"`
	// SourceID is a caller-supplied identifier for licensing/attribution
	// tracking (e.g. a subscription/report ID). Optional.
	SourceID string `json:"source_id,omitempty"`
}

// BenchmarkSet bundles one metric's benchmark data in whichever Form the
// caller has available. Only the field(s) matching Form are read by
// Calculate — see BenchmarkForm's doc comment.
type BenchmarkSet struct {
	// Form identifies which field(s) below are populated and should be
	// used. Required; Calculate records IssueInvalidForm and leaves the
	// resulting Comparison's benchmark-derived fields unavailable if Form
	// is empty or unrecognized.
	Form BenchmarkForm `json:"form"`

	// Median is this benchmark population's median value. Read when
	// Form == FormMedian; also used (if Available) by every other Form as
	// the definitive "benchmark median" even when a percentile table or
	// peer set could itself imply one, since a caller-disclosed summary
	// statistic takes precedence over a derived approximation. Optional
	// for every Form other than FormMedian.
	Median Value `json:"median,omitempty"`

	// PercentileBands is read when Form == FormPercentileBands: a set of
	// (percentile, value) points, in any order (Calculate sorts by
	// Percentile before interpolating). At least two distinct points are
	// required for interpolation; a single point still supports a
	// difference/relative-difference comparison at that one percentile
	// but no interpolated estimate elsewhere.
	//
	// Value must be non-decreasing as Percentile increases (the higher a
	// percentile, the higher — or equal — its value must be), matching
	// every real percentile table's own definition. A table violating
	// this (e.g. entered in the reverse order a "lower is better" metric
	// might tempt a caller to use) produces IssueNonMonotonicBenchmarkPoints
	// and leaves Comparison.Percentile/Band/BenchmarkRange unavailable
	// rather than computing a value from a self-contradictory table —
	// Direction, not point order, is how this package expresses "lower is
	// better" (see Direction's doc comment).
	PercentileBands []PercentilePoint `json:"percentile_bands,omitempty"`

	// Quartiles is read when Form == FormQuartiles. Q1 <= Median <= Q3 is
	// required among whichever fields are Available, for the same reason
	// PercentileBands requires non-decreasing Value — see
	// PercentileBands's doc comment.
	Quartiles Quartiles `json:"quartiles,omitempty"`

	// PeerObservations is read when Form == FormPeerObservations: the
	// individual peer values Calculate derives every statistic from.
	PeerObservations []PeerObservation `json:"peer_observations,omitempty"`

	// IndustryLabel, SizeLabel, and GeographyLabel are caller-supplied
	// segment metadata, echoed back on Comparison for display/audit only.
	// This package draws no inference from them and does not require them
	// to match the company's own segment in any validated way.
	IndustryLabel  string `json:"industry_label,omitempty"`
	SizeLabel      string `json:"size_label,omitempty"`
	GeographyLabel string `json:"geography_label,omitempty"`

	// Source is this benchmark's provenance metadata (see BenchmarkSource).
	Source BenchmarkSource `json:"source"`
}

// MetricRequest bundles one company metric together with the
// BenchmarkSet to compare it against — the portable input tuple this
// package requires for every computation (see the package doc comment),
// mirroring covenants.CovenantTest's identical "rule plus observation in
// one caller-assembled row" shape.
type MetricRequest struct {
	// MetricID is a caller-assigned stable identifier for this metric
	// (e.g. "GROSS_MARGIN", "DSCR", or an internal metric code). Required;
	// Calculate records IssueMissingMetricID and leaves the resulting
	// Comparison unavailable if empty.
	MetricID string `json:"metric_id"`
	// Label is a short human-readable name for this metric (e.g. "Gross
	// Margin %"), for display only. Optional; falls back to MetricID when
	// empty.
	Label string `json:"label,omitempty"`

	// CompanyValue is the company's already-calculated figure for this
	// metric, computed upstream by the caller (e.g. via financial/metrics
	// or analytics/ratios). Unavailable means the figure could not be
	// computed (e.g. a required input was missing upstream) — this
	// package never treats an unavailable CompanyValue as zero.
	CompanyValue Value `json:"company_value"`

	// Period is the reporting period CompanyValue applies to, for display
	// only. financial.Period is a plain string with no guaranteed sort
	// order; this package draws no chronological inference from it.
	// Optional.
	Period financial.Period `json:"period,omitempty"`

	// Direction states whether higher or lower is favorable for this
	// metric (see Direction). Optional; DirectionUnspecified (the
	// default) means Calculate never derives Favorable for this request.
	Direction Direction `json:"direction,omitempty"`

	// Benchmark is the benchmark dataset to compare CompanyValue against.
	Benchmark BenchmarkSet `json:"benchmark"`
}

// Input bundles everything Calculate needs.
type Input struct {
	// Metrics is the full set of company-metric-vs-benchmark comparisons
	// to evaluate, in the order Calculate reports them in
	// Result.Comparisons. Required; Calculate returns a Result with
	// Available == false and an Errors entry if empty.
	Metrics []MetricRequest `json:"metrics"`
}

// IssueSeverity distinguishes an input problem Calculate could not
// proceed past for a given metric (SeverityError) from one that is
// advisory only (SeverityWarning) — the same two-severity model every
// analytics sibling package uses.
type IssueSeverity string

const (
	SeverityError   IssueSeverity = "error"
	SeverityWarning IssueSeverity = "warning"
)

// IssueCode is a stable identifier for one kind of Calculate-time input
// problem. This package defines its own separate taxonomy, consistent
// with every other package in this repository, rather than reusing one of
// theirs.
type IssueCode string

const (
	// IssueNoMetrics means Input.Metrics was empty; Calculate returns
	// Available == false.
	IssueNoMetrics IssueCode = "NO_METRICS"
	// IssueMissingMetricID means a MetricRequest had an empty MetricID.
	// That request's Comparison is still produced (so a caller sees every
	// input row reflected in output) but is left entirely unavailable.
	IssueMissingMetricID IssueCode = "MISSING_METRIC_ID"
	// IssueDuplicateMetricID means two or more requests share the same
	// non-empty MetricID and Period. Every such request is still evaluated
	// independently and included in Result.Comparisons; this is advisory
	// only since a caller may legitimately re-benchmark the same metric ID
	// against a different BenchmarkSet, but is surfaced since it usually
	// indicates duplicated input.
	IssueDuplicateMetricID IssueCode = "DUPLICATE_METRIC_ID"
	// IssueCompanyValueUnavailable means a MetricRequest's
	// CompanyValue.Available was false. Every benchmark-derived field is
	// still computed and reported (the benchmark side of the comparison
	// does not depend on the company side), but Difference,
	// RelativeDifference, Percentile, Band, and Favorable are all left
	// unavailable. Advisory only.
	IssueCompanyValueUnavailable IssueCode = "COMPANY_VALUE_UNAVAILABLE"
	// IssueInvalidCompanyValue means a MetricRequest's CompanyValue was
	// Available but CompanyValue.Amount was NaN or +/-Inf — a genuinely
	// invalid figure, distinct from IssueCompanyValueUnavailable's "no
	// figure was supplied" (see the package doc comment on
	// available-but-invalid vs. unavailable). Treated the same as
	// unavailable for every downstream comparison field (Difference,
	// RelativeDifference, Percentile, Band, Favorable all left
	// unavailable) rather than letting a non-finite value propagate into
	// arithmetic, mirroring analytics/concentration.IssueInvalidObservation's
	// identical NaN/Inf guard.
	IssueInvalidCompanyValue IssueCode = "INVALID_COMPANY_VALUE"
	// IssueInvalidBenchmarkMedian means the resolved benchmark median
	// (from BenchmarkSet.Median, Quartiles.Median, or an interpolated
	// percentile-band value — whichever Form supplied it) was NaN or
	// +/-Inf. Treated the same as an unavailable median: BenchmarkRange,
	// Percentile, Band, Difference, RelativeDifference, and Favorable are
	// all computed as if no median were known, rather than letting a
	// non-finite value propagate into Difference/RelativeDifference's
	// arithmetic. Mirrors IssueInvalidCompanyValue's identical guard on
	// the company side of the comparison.
	IssueInvalidBenchmarkMedian IssueCode = "INVALID_BENCHMARK_MEDIAN"
	// IssueRelativeDifferenceOverflow means CompanyValue and
	// BenchmarkMedian were each individually finite, but Difference
	// (CompanyValue - BenchmarkMedian) or RelativeDifference's division
	// (Difference / |BenchmarkMedian|) overflowed float64's range — an
	// extreme-magnitude edge case (e.g. a near-float64-max company value
	// against a near-float64-min one, or against a
	// near-float64-zero-but-nonzero median), distinct from
	// IssueBenchmarkMedianZero's exactly-zero-denominator case. The
	// affected field (Difference and/or RelativeDifference) is left
	// unavailable rather than reporting +/-Inf.
	IssueRelativeDifferenceOverflow IssueCode = "RELATIVE_DIFFERENCE_OVERFLOW"
	// IssueInvalidForm means a BenchmarkSet's Form was empty or not one of
	// the recognized BenchmarkForm constants. No benchmark-derived field
	// can be computed.
	IssueInvalidForm IssueCode = "INVALID_FORM"
	// IssueBenchmarkDataMissing means Form named a field (e.g.
	// FormMedian) but that field was left at its zero value (e.g.
	// Median.Available == false, or PercentileBands/PeerObservations
	// empty). No benchmark-derived field can be computed.
	IssueBenchmarkDataMissing IssueCode = "BENCHMARK_DATA_MISSING"
	// IssueInsufficientPercentileBands means Form == FormPercentileBands
	// but fewer than two distinct-percentile points were supplied, so no
	// interpolated percentile estimate is calculable (a difference against
	// the nearest/only point may still be reported — see
	// Comparison.Difference).
	IssueInsufficientPercentileBands IssueCode = "INSUFFICIENT_PERCENTILE_BANDS"
	// IssueMissingSourceName means BenchmarkSet.Source.Name was empty.
	// Advisory only; the comparison still proceeds.
	IssueMissingSourceName IssueCode = "MISSING_SOURCE_NAME"
	// IssueBenchmarkMedianZero means BenchmarkMedian was available but
	// exactly 0, so RelativeDifference (a fraction of the median) has no
	// meaningful answer and is left unavailable. Difference itself is
	// unaffected. Advisory only.
	IssueBenchmarkMedianZero IssueCode = "BENCHMARK_MEDIAN_ZERO"
	// IssueNonMonotonicBenchmarkPoints means a BenchmarkSet's percentile
	// points (from PercentileBands, Quartiles, or PeerObservations) do not
	// have Value strictly non-decreasing as Percentile increases —
	// Calculate's interpolation, range, and band logic all assume a
	// higher percentile implies a value at or above every lower
	// percentile's value (true by construction for FormPeerObservations,
	// but not guaranteed for a caller-supplied FormPercentileBands/
	// FormQuartiles table describing a "lower is better" metric published
	// in descending order, or simple data-entry error). When this Issue
	// fires, Percentile, Band, and BenchmarkRange are all left
	// unavailable for that metric rather than reporting a value computed
	// from a self-contradictory points table; BenchmarkMedian and
	// Difference/RelativeDifference/Favorable (which depend only on
	// Median, not on the points table's ordering) are unaffected.
	IssueNonMonotonicBenchmarkPoints IssueCode = "NON_MONOTONIC_BENCHMARK_POINTS"
)

// Issue is a single Calculate-time input finding.
type Issue struct {
	Code     IssueCode     `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	// MetricID identifies which MetricRequest this Issue relates to, by
	// echoing MetricRequest.MetricID (or, when MetricID itself is empty/
	// the problem, an index reference like "metrics[2]"). Empty when the
	// Issue is not metric-specific.
	MetricID string `json:"metric_id,omitempty"`
}

// HasErrors reports whether any Issue in issues has SeverityError.
//
// Intentionally duplicated from covenants.HasErrors/debt.HasErrors/this
// repository's other sibling HasErrors functions rather than shared — see
// adjustments.HasErrors's doc comment for the full rationale (each
// package's Issue is a distinct Go type with no common interface worth
// introducing for one boolean function).
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Band classifies a company value's placement within a benchmark
// population, at whatever granularity the BenchmarkForm supports.
type Band string

const (
	// BandUnavailable means no band could be determined (see the Comparison
	// Issues for why).
	BandUnavailable Band = ""
	// BandBelowMin means the company value is below every known benchmark
	// point (below the lowest percentile point, below Q1's implied range
	// floor with no lower point available, or below the minimum peer
	// observation).
	BandBelowMin Band = "BELOW_MIN"
	// BandQ1 means the company value falls at or below the benchmark's
	// 25th percentile.
	BandQ1 Band = "Q1"
	// BandQ2 means the company value falls between the 25th and 50th
	// percentile.
	BandQ2 Band = "Q2"
	// BandQ3 means the company value falls between the 50th and 75th
	// percentile.
	BandQ3 Band = "Q3"
	// BandQ4 means the company value falls above the benchmark's 75th
	// percentile.
	BandQ4 Band = "Q4"
	// BandAboveMax means the company value is above every known benchmark
	// point (above the highest percentile point or above the maximum peer
	// observation).
	BandAboveMax Band = "ABOVE_MAX"
)

// Favorable classifies whether the company's value is favorable relative
// to the benchmark median, per MetricRequest.Direction — never guessed
// when Direction is DirectionUnspecified (see Direction's doc comment).
type Favorable string

const (
	// FavorableNotApplicable means Direction was DirectionUnspecified,
	// Direction was DirectionNeutral, or the comparison could not be made
	// (unavailable CompanyValue or benchmark median).
	FavorableNotApplicable Favorable = "NOT_APPLICABLE"
	// FavorableYes means the company value is favorable relative to the
	// benchmark median per Direction.
	FavorableYes Favorable = "FAVORABLE"
	// FavorableNo means the company value is unfavorable relative to the
	// benchmark median per Direction.
	FavorableNo Favorable = "UNFAVORABLE"
	// FavorableEqual means the company value exactly equals the benchmark
	// median — neither favorable nor unfavorable under either direction.
	FavorableEqual Favorable = "EQUAL"
)

// Range is the benchmark population's known [min, max] extent, when
// calculable from the supplied BenchmarkForm (the endpoints of
// PercentileBands, an implied range from Quartiles — Q1/Q3 only, not a
// true min/max — or the actual min/max of PeerObservations). Guaranteed
// Min.Amount <= Max.Amount whenever both are Available — Calculate leaves
// Range entirely unavailable rather than reporting an inverted range when
// the source points fail the non-decreasing-value precondition (see
// IssueNonMonotonicBenchmarkPoints).
type Range struct {
	Min Value `json:"min"`
	Max Value `json:"max"`
}

// Comparison is one MetricRequest's full evaluation.
type Comparison struct {
	// MetricID/Label/Period/Direction echo the source MetricRequest.
	MetricID  string           `json:"metric_id"`
	Label     string           `json:"label,omitempty"`
	Period    financial.Period `json:"period,omitempty"`
	Direction Direction        `json:"direction,omitempty"`

	// CompanyValue echoes the source MetricRequest.CompanyValue.
	CompanyValue Value `json:"company_value"`

	// BenchmarkMedian is the benchmark population's median, when
	// available: BenchmarkSet.Median directly if supplied, else derived
	// from PercentileBands (interpolated at the 50th percentile),
	// Quartiles.Median, or the computed median of PeerObservations.
	BenchmarkMedian Value `json:"benchmark_median"`
	// BenchmarkRange is the benchmark population's known extent, when
	// calculable (see Range's doc comment).
	BenchmarkRange Range `json:"benchmark_range"`

	// Percentile is an estimate of what percentile of the benchmark
	// population CompanyValue falls at, in [0, 100], when calculable:
	// linear interpolation across PercentileBands/Quartiles, or an
	// empirical rank-based estimate across PeerObservations. Unavailable
	// for FormMedian (a single point supports no percentile estimate) or
	// when fewer than two distinct benchmark points are available.
	Percentile Value `json:"percentile"`
	// Band classifies CompanyValue's placement at whatever granularity is
	// calculable (see Band).
	Band Band `json:"band"`

	// Difference is CompanyValue.Amount - BenchmarkMedian.Amount.
	// Available only when both are available.
	Difference Value `json:"difference"`
	// RelativeDifference is Difference / |BenchmarkMedian.Amount|,
	// expressed as a fraction (0.10 = 10% above/below the median).
	// Unavailable when Difference is unavailable or BenchmarkMedian.Amount
	// is exactly 0 (division by zero has no meaningful fractional
	// answer — see IssueBenchmarkMedianZero).
	RelativeDifference Value `json:"relative_difference"`

	// Favorable classifies CompanyValue relative to BenchmarkMedian per
	// Direction (see Favorable). Always FavorableNotApplicable when
	// Direction is DirectionUnspecified or DirectionNeutral.
	Favorable Favorable `json:"favorable"`

	// SourceIndustryLabel/SourceSizeLabel/SourceGeographyLabel echo the
	// source BenchmarkSet's segment labels.
	SourceIndustryLabel  string `json:"source_industry_label,omitempty"`
	SourceSizeLabel      string `json:"source_size_label,omitempty"`
	SourceGeographyLabel string `json:"source_geography_label,omitempty"`
	// Source echoes the source BenchmarkSet.Source verbatim — see the
	// package doc comment's provenance guarantee.
	Source BenchmarkSource `json:"source"`

	// Form echoes the source BenchmarkSet.Form.
	Form BenchmarkForm `json:"form"`

	// Available is false only if this Comparison could not be evaluated
	// at all (missing MetricID) — every benchmark-derived field is then
	// zero-value. A missing/invalid benchmark or an unavailable
	// CompanyValue still produces Available == true with the affected
	// fields individually left unavailable (see the Issue recorded for
	// the specific reason) — this package reports partial results rather
	// than an all-or-nothing failure per comparison.
	Available bool `json:"available"`
}

// Summary aggregates every Comparison in Result.Comparisons.
type Summary struct {
	// MetricCount is len(Result.Comparisons).
	MetricCount int `json:"metric_count"`
	// FavorableCount is the number of Comparison entries with
	// Favorable == FavorableYes.
	FavorableCount int `json:"favorable_count"`
	// UnfavorableCount is the number of Comparison entries with
	// Favorable == FavorableNo.
	UnfavorableCount int `json:"unfavorable_count"`
	// UnavailableCount is the number of Comparison entries with
	// Available == false.
	UnavailableCount int `json:"unavailable_count"`

	// UnfavorableMetricIDs is MetricID, in Result.Comparisons order, for
	// every Comparison with Favorable == FavorableNo — for direct display
	// without requiring a caller to filter Result.Comparisons itself.
	UnfavorableMetricIDs []string `json:"unfavorable_metric_ids,omitempty"`
}

// Result is the output of Calculate: every metric's comparison plus a
// summary and formula version.
type Result struct {
	// FormulaVersion identifies which version of this package's fixed
	// formula set produced this Result.
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueNoMetrics) — every other field is then zero-value.
	Available bool `json:"available"`

	// Comparisons is one Comparison per Input.Metrics entry, in the same
	// order.
	Comparisons []Comparison `json:"comparisons,omitempty"`

	// Summary aggregates Comparisons.
	Summary Summary `json:"summary"`

	// Warnings carries every Issue with SeverityWarning.
	Warnings []Issue `json:"warnings,omitempty"`
	// Errors carries every Issue with SeverityError.
	Errors []Issue `json:"errors,omitempty"`
}
