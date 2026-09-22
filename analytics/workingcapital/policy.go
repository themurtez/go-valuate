package workingcapital

import "github.com/themurtez/go-valuate/financial"

// InclusionPolicy configures which financial.Code values this package sums
// into OperatingCurrentAssets and OperatingCurrentLiabilities. There is no
// single universal "deal definition" of operating working capital — cash,
// debt, taxes payable, and shareholder/related-party balances are each
// includable or excludable depending on the transaction, industry, and
// negotiated purchase agreement — so this package never hard-codes one.
//
// A financial.Code present in Dataset's balance-sheet items but assigned to
// neither AssetCodes nor LiabilityCodes is simply excluded from NWC and
// reported in Result.ExcludedCodes for auditability; it is not an error.
//
// This repository's taxonomy (see financial.Code) does not carve out
// dedicated codes for "taxes payable" or "shareholder/related-party
// balances" as distinct from CodeBsCurrentLiabilityOther/
// CodeBsCurrentAssetOther — a caller whose source data separately tracks
// those needs to classify them onto a distinct financial.Code upstream (or
// via a caller-defined extension code, since financial.Code is a plain
// string) before this policy can selectively exclude just that account
// rather than the whole "other" bucket.
type InclusionPolicy struct {
	// AssetCodes lists every financial.Code counted as an operating current
	// asset. Duplicates are harmless (deduplicated internally).
	AssetCodes []financial.Code `json:"asset_codes"`
	// LiabilityCodes lists every financial.Code counted as an operating
	// current liability.
	LiabilityCodes []financial.Code `json:"liability_codes"`
}

// DefaultInclusionPolicy returns this package's conservative default: every
// standard current-asset code except cash, and every standard
// current-liability code except short-term debt — the common "operating"
// working-capital convention that excludes financing-related balances
// (cash and interest-bearing debt) since those are typically settled
// separately at close rather than trued up through a working-capital peg.
// Applied whenever a caller passes a zero-value InclusionPolicy to
// Calculate (see resolveInclusionPolicy), mirroring
// review.DefaultPolicy/qoe.DefaultThresholds' identical
// zero-value-means-defaults pattern.
//
// This default deliberately excludes CodeBsCurrentLiabilityOther and
// CodeBsCurrentAssetOther's more exotic contents (taxes payable,
// shareholder loans, etc. — see the InclusionPolicy doc comment on why
// those have no dedicated codes) purely by omission of nothing: both
// "other" codes ARE included by default, since a typical working-capital
// peg does include miscellaneous accrued operating items. A caller with a
// deal-specific reason to strip a subset of "other" activity must supply
// its own InclusionPolicy.
func DefaultInclusionPolicy() InclusionPolicy {
	return InclusionPolicy{
		AssetCodes: []financial.Code{
			financial.CodeBsAccountsReceivable,
			financial.CodeBsInventory,
			financial.CodeBsPrepaid,
			financial.CodeBsCurrentAssetOther,
		},
		LiabilityCodes: []financial.Code{
			financial.CodeBsAccountsPayable,
			financial.CodeBsCurrentLiabilityOther,
		},
	}
}

// resolveInclusionPolicy returns p if either field is non-empty, otherwise
// DefaultInclusionPolicy() — the same zero-value-means-defaults rule
// review.resolvePolicy/qoe.resolveThresholds use. A caller wanting to
// genuinely include zero codes on one side (e.g. an asset-only analysis)
// must supply at least one code on the other side to avoid triggering the
// default substitution; Calculate reports IssueEmptyInclusionPolicy in that
// case rather than silently applying defaults.
func resolveInclusionPolicy(p InclusionPolicy) InclusionPolicy {
	if len(p.AssetCodes) == 0 && len(p.LiabilityCodes) == 0 {
		return DefaultInclusionPolicy()
	}
	return p
}

// codeSet deduplicates a []financial.Code into a lookup set.
func codeSet(codes []financial.Code) map[financial.Code]struct{} {
	set := make(map[financial.Code]struct{}, len(codes))
	for _, c := range codes {
		set[c] = struct{}{}
	}
	return set
}
