package valuation

import "math"

// knownValueTypes is the closed set ValidateResultEnvelope checks
// ValueType against. Defined once here so it stays in sync with the
// ValueType constants by construction.
var knownValueTypes = map[ValueType]bool{
	ValueTypeEnterprise: true,
	ValueTypeEquity:     true,
	ValueTypeAsset:      true,
}

// knownMethodCodes is the closed set ValidateResultEnvelope checks Method
// against. Defined once here so it stays in sync with the Code constants
// by construction.
var knownMethodCodes = map[Code]bool{
	CodeSDEMultiple:              true,
	CodeEBITDAMultiple:           true,
	CodeCapitalizationOfEarnings: true,
	CodeDCF:                      true,
	CodeAdjustedNetAssetValue:    true,
}

// ValidateResultEnvelope checks the three fields every method Result
// shares (Method, MethodVersion, ValueType) for the invariants a
// successful (Available == true) Result must never violate: an
// unknown/empty method code, an empty version, or an unknown value basis.
// Every method package's Calculate hard-codes correct, constant values for
// all three, so this check exists as a regression guard against a future
// edit accidentally leaving one of them empty or mistyped — not because
// any of the five methods currently produces an invalid envelope.
//
// Returns nil if every field is valid. Every returned Issue carries
// SeverityError — an invalid envelope is never a mere warning, since a
// Result a caller cannot identify or interpret the basis of is not usable
// regardless of whether its headline figure computed cleanly.
func ValidateResultEnvelope(method Code, methodVersion string, valueType ValueType) []Issue {
	var issues []Issue
	if method == "" || !knownMethodCodes[method] {
		issues = append(issues, Issue{
			Code: IssueEmptyMethodCode, Severity: SeverityError,
			Message: "result method code is empty or not a recognized valuation.Code",
		})
	}
	if methodVersion == "" {
		issues = append(issues, Issue{
			Code: IssueEmptyVersion, Severity: SeverityError,
			Message: "result method version is empty",
		})
	}
	if valueType == "" || !knownValueTypes[valueType] {
		issues = append(issues, Issue{
			Code: IssueUnknownValueBasis, Severity: SeverityError,
			Message: "result value type is empty or not a recognized valuation.ValueType",
		})
	}
	return issues
}

// ValidateFiniteSteps checks every Step.Value in steps for NaN/+-Inf,
// catching a derived calculation step that went non-finite even though
// every caller-supplied Input was itself validated as finite (e.g. a
// future formula change that divides by a value not checked for zero).
// Every method's own Input-level validation already rejects non-finite
// inputs (IssueNonFiniteInput); this is the complementary check on
// output, called once after a method's Steps slice is fully built, before
// Calculate returns.
//
// Returns nil if every step is finite. Every returned Issue carries
// SeverityError, mirroring ValidateResultEnvelope: a Result containing a
// non-finite calculation step is not safely usable (it would also fail to
// marshal via encoding/json), regardless of whether the headline value
// itself happens to be finite.
func ValidateFiniteSteps(steps []Step) []Issue {
	var issues []Issue
	for _, s := range steps {
		if math.IsNaN(s.Value) || math.IsInf(s.Value, 0) {
			issues = append(issues, Issue{
				Code: IssueNonFiniteStep, Severity: SeverityError,
				Message: "calculation step " + quoteLabel(s.Label) + " produced a non-finite value",
			})
		}
	}
	return issues
}

// quoteLabel wraps a step label in double quotes for an error message,
// without pulling in fmt.Sprintf for a single string concatenation.
func quoteLabel(label string) string {
	return `"` + label + `"`
}
