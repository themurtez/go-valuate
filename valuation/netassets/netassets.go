// Package netassets implements the adjusted net asset value valuation
// method: explicitly supplied fair/adjusted asset and liability line items,
// summed and netted.
//
// This package never reads book values out of a financial.FinancialDataset
// or a metrics.Snapshot itself, and never assumes book value equals fair
// market value. Every AssetItem/LiabilityItem is a caller-supplied fair
// value; if a caller wants to start from book values (e.g.
// metrics.Snapshot.TangibleAssetValue) and apply no further adjustment,
// that is a valid, explicit choice the caller makes when constructing the
// item list — Calculate always documents which basis it received via
// Result.Input, and never silently substitutes one for the other. Every
// function here is pure: no I/O, no mutation of its inputs.
//
// Value type. Calculate always reports valuation.ValueTypeAsset, distinct
// from both Enterprise Value and Equity Value: Adjusted Net Asset Value
// (Adjusted Assets - Adjusted Liabilities) is the same subtraction that
// produces a balance sheet's book equity, but the *adjusted* figure is a
// controlling/liquidation-oriented reference value, not necessarily
// interchangeable with a going-concern Equity Value from an earnings-based
// method (SDE/EBITDA multiple, capitalization of earnings, DCF). This
// package does not attempt to reconcile the two or convert one to the
// other — callers wanting to compare an asset-based value against an
// earnings-based one do so by looking at both results side by side, once
// this repository's later consensus/comparison work exists.
package netassets

import (
	"fmt"
	"math"

	"github.com/themurtez/go-valuate/valuation"
)

// Code is this method's stable identifier. See
// valuation.CodeAdjustedNetAssetValue.
const Code = valuation.CodeAdjustedNetAssetValue

// Version is this method's calculation-logic version. See sde.Version's doc
// comment for what incrementing it means.
const Version = "1.0.0"

// Validation issue codes specific to this method.
const (
	// IssueNoAssets means Input.Assets was empty. A net asset value with no
	// assets at all is not a meaningful calculation — as opposed to assets
	// summing to zero, which is a real, calculable (if warned) outcome; see
	// IssueLiabilitiesExceedAssets.
	IssueNoAssets valuation.IssueCode = "NO_ASSETS"
	// IssueNegativeItemAmount is a warning noting that an AssetItem or
	// LiabilityItem's Amount was negative. Amount is meant to be a
	// non-negative fair value contribution (mirroring
	// financial/adjustments.Adjustment.Amount's magnitude convention); a
	// negative fair value is unusual (e.g. a liability item entered with
	// the wrong sign) but not rejected outright, since a genuinely negative
	// adjustment item (a write-down override) is conceivable.
	IssueNegativeItemAmount valuation.IssueCode = "NEGATIVE_ITEM_AMOUNT"
	// IssueLiabilitiesExceedAssets is a warning noting that total adjusted
	// liabilities exceeded total adjusted assets, producing a negative net
	// asset value — a real, calculable outcome for an insolvent or
	// distressed business, not an error.
	IssueLiabilitiesExceedAssets valuation.IssueCode = "LIABILITIES_EXCEED_ASSETS"
)

// AssetItem is a single caller-supplied fair/adjusted asset value.
type AssetItem struct {
	// Label is a short human-readable description (e.g. "Cash",
	// "Fixed Assets (appraised)", "Accounts Receivable (net of allowance
	// override)").
	Label string `json:"label"`
	// Amount is this item's fair/adjusted value, caller-supplied. This
	// package does not know or assume whether Amount is a raw book value or
	// an adjusted fair value — see the package doc comment.
	Amount float64 `json:"amount"`
	// SourceCode optionally names the canonical financial.Code this item
	// was derived from (e.g. "BS_FIXED_ASSETS"), for traceability, mirroring
	// financial/adjustments.Adjustment.SourceCode. Purely informational.
	SourceCode string `json:"source_code,omitempty"`
	// IsOverride marks this item as an explicit fair-value override that
	// differs from the reported book value (e.g. a fixed asset revalued via
	// appraisal). false means the item is being carried at book value as-is
	// — a legitimate, but distinct, choice this field makes explicit rather
	// than leaving the reader to guess whether "adjustment" happened here.
	IsOverride bool `json:"is_override,omitempty"`
	// Notes is optional freeform commentary (e.g. "per July 2025 appraisal
	// of the Elm St. facility").
	Notes string `json:"notes,omitempty"`
}

// LiabilityItem is a single caller-supplied fair/adjusted liability value.
// Same shape as AssetItem — kept as a distinct type rather than a shared
// alias so a future field that only makes sense for one side (e.g. an
// asset-only "marketability discount applied" flag) can be added without
// disturbing the other.
type LiabilityItem struct {
	Label      string  `json:"label"`
	Amount     float64 `json:"amount"`
	SourceCode string  `json:"source_code,omitempty"`
	IsOverride bool    `json:"is_override,omitempty"`
	Notes      string  `json:"notes,omitempty"`
}

// Input is the adjusted net asset value method's strongly-typed input.
type Input struct {
	// Assets is the itemized list of adjusted asset fair values. Must be
	// non-empty.
	Assets []AssetItem `json:"assets"`
	// Liabilities is the itemized list of adjusted liability fair values.
	// May be empty (a debt-free, liability-free business is possible),
	// unlike Assets.
	Liabilities []LiabilityItem `json:"liabilities"`
}

// Result is the adjusted net asset value method's output.
type Result struct {
	// Method is this method's stable Code.
	Method valuation.Code `json:"method"`
	// MethodVersion is the Version Calculate ran under.
	MethodVersion string `json:"method_version"`
	// ValueType is always valuation.ValueTypeAsset for this method — see
	// the package doc comment.
	ValueType valuation.ValueType `json:"value_type"`
	// Input echoes the exact Input Calculate was given.
	Input Input `json:"input"`
	// Available is false only if AdjustedNetAssetValue could not be
	// computed at all (a blocking validation error: no assets, or a
	// non-finite item amount). Available is true even when Liabilities
	// exceed Assets — see Calculate's doc comment.
	Available bool `json:"available"`
	// TotalAdjustedAssets is the sum of every Input.Assets[i].Amount.
	TotalAdjustedAssets float64 `json:"total_adjusted_assets"`
	// TotalAdjustedLiabilities is the sum of every
	// Input.Liabilities[i].Amount.
	TotalAdjustedLiabilities float64 `json:"total_adjusted_liabilities"`
	// AdjustedNetAssetValue is TotalAdjustedAssets -
	// TotalAdjustedLiabilities. Meaningful only when Available is true.
	AdjustedNetAssetValue float64 `json:"adjusted_net_asset_value"`
	// AssetComponents itemizes every Assets entry that contributed to
	// TotalAdjustedAssets, in input order.
	AssetComponents []valuation.Component `json:"asset_components,omitempty"`
	// LiabilityComponents itemizes every Liabilities entry that
	// contributed to TotalAdjustedLiabilities, in input order.
	LiabilityComponents []valuation.Component `json:"liability_components,omitempty"`
	// Steps is the full calculation trace.
	Steps []valuation.Step `json:"steps,omitempty"`
	// Warnings carries non-blocking Issues.
	Warnings []valuation.Issue `json:"warnings,omitempty"`
	// Errors carries blocking Issues that made Available false.
	Errors []valuation.Issue `json:"errors,omitempty"`
}

// Calculate computes the adjusted net asset value:
//
//	Adjusted Net Asset Value = Adjusted Assets - Adjusted Liabilities
//
// Calculate never panics on bad financial input. A blocking SeverityError
// (Result.Available == false, every numeric result field zero) occurs only
// when Input.Assets is empty or any item's Amount is non-finite — see
// IssueNoAssets' doc comment for why zero assets specifically (as opposed
// to assets summing to zero) is treated as invalid input rather than a
// calculable outcome. Liabilities exceeding Assets is NOT a blocking
// error: it is a real, calculable outcome (a negative net asset value for
// an insolvent or heavily-levered business), reported as a
// SeverityWarning, and Calculate does not clamp the result up to zero.
func Calculate(input Input) Result {
	result := Result{
		Method:        Code,
		MethodVersion: Version,
		ValueType:     valuation.ValueTypeAsset,
		Input:         input,
	}

	var issues []valuation.Issue

	if len(input.Assets) == 0 {
		issues = append(issues, valuation.Issue{
			Code: IssueNoAssets, Severity: valuation.SeverityError,
			Message: "at least one asset item is required",
		})
	}
	for i, a := range input.Assets {
		if !isFinite(a.Amount) {
			issues = append(issues, valuation.Issue{
				Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
				Message: fmt.Sprintf("asset item %d (%q) amount is not a finite number", i, a.Label),
			})
		}
	}
	for i, l := range input.Liabilities {
		if !isFinite(l.Amount) {
			issues = append(issues, valuation.Issue{
				Code: valuation.IssueNonFiniteInput, Severity: valuation.SeverityError,
				Message: fmt.Sprintf("liability item %d (%q) amount is not a finite number", i, l.Label),
			})
		}
	}

	if valuation.HasErrors(issues) {
		result.Errors = valuation.Errors(issues)
		result.Warnings = valuation.Warnings(issues)
		return result
	}

	assetComponents := make([]valuation.Component, 0, len(input.Assets))
	totalAssets := 0.0
	for i, a := range input.Assets {
		if a.Amount < 0 {
			issues = append(issues, valuation.Issue{
				Code: IssueNegativeItemAmount, Severity: valuation.SeverityWarning,
				Message: fmt.Sprintf("asset item %d (%q) has a negative amount", i, a.Label),
			})
		}
		assetComponents = append(assetComponents, valuation.Component{Label: a.Label, Amount: a.Amount})
		totalAssets += a.Amount
	}

	liabilityComponents := make([]valuation.Component, 0, len(input.Liabilities))
	totalLiabilities := 0.0
	for i, l := range input.Liabilities {
		if l.Amount < 0 {
			issues = append(issues, valuation.Issue{
				Code: IssueNegativeItemAmount, Severity: valuation.SeverityWarning,
				Message: fmt.Sprintf("liability item %d (%q) has a negative amount", i, l.Label),
			})
		}
		liabilityComponents = append(liabilityComponents, valuation.Component{Label: l.Label, Amount: l.Amount})
		totalLiabilities += l.Amount
	}

	netAssetValue := totalAssets - totalLiabilities
	if totalLiabilities > totalAssets {
		issues = append(issues, valuation.Issue{
			Code: IssueLiabilitiesExceedAssets, Severity: valuation.SeverityWarning,
			Message: "adjusted liabilities exceed adjusted assets; the resulting net asset value is negative",
		})
	}

	result.Available = true
	result.TotalAdjustedAssets = totalAssets
	result.TotalAdjustedLiabilities = totalLiabilities
	result.AdjustedNetAssetValue = netAssetValue
	result.AssetComponents = assetComponents
	result.LiabilityComponents = liabilityComponents
	result.Steps = []valuation.Step{
		{Label: "Total Adjusted Assets", Value: totalAssets},
		{Label: "Total Adjusted Liabilities", Value: totalLiabilities},
		{Label: "Adjusted Net Asset Value = Adjusted Assets - Adjusted Liabilities", Value: netAssetValue},
	}
	result.Errors = valuation.Errors(issues)
	result.Warnings = valuation.Warnings(issues)
	return result
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
