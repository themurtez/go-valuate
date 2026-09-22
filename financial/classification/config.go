package classification

import "github.com/themurtez/go-valuate/financial"

// Explicit maps a specific source row (by its financial.RawLineItem.ID) to
// an exact canonical code, bypassing all other classification stages. This
// is the escape hatch for a human reviewer who has already made a decision
// about one specific row and wants it to stick regardless of what any alias
// or rule would otherwise propose.
type Explicit struct {
	// RowID is the financial.RawLineItem.ID this explicit mapping applies
	// to.
	RowID string
	// Code is the canonical taxonomy code to assign.
	Code financial.Code
}

// Config bundles everything Classify/ClassifyBatch need to turn raw rows
// into classification Results. A Config is plain data: this package places
// no requirement on how a caller builds one (hardcoded, loaded from a file,
// assembled from database rows elsewhere in a larger application, etc.).
//
// The zero Config is valid and will classify every row as SourceUnknown
// except for structural (subtotal/total) detection, which always runs.
type Config struct {
	// Explicits are exact, row-ID-scoped overrides. Checked first, before
	// any alias or rule. See Explicit.
	Explicits []Explicit
	// AliasLayers are caller-supplied alias sets in increasing precedence
	// order (first = lowest precedence, last = highest). See AliasLayer for
	// the generic layering model; a typical application-level ordering
	// might be global, account, client, valuation, but this package assigns
	// no meaning to layer names beyond provenance/debugging.
	AliasLayers []AliasLayer
	// Rules are context-aware and phrase/token rules, evaluated in order.
	// Pass DefaultRules() to use the built-in set, your own rules, or a
	// combination (e.g. append(myRules, DefaultRules()...) to give your
	// rules higher precedence). A nil/empty Rules means no rule stage runs.
	Rules []Rule
	// ReviewThreshold is the minimum Confidence at or above which a result
	// is considered trustworthy enough to skip human review. Results with
	// Confidence below this threshold get ReviewRequired = true. Defaults
	// to DefaultReviewThreshold when zero.
	ReviewThreshold Confidence
	// MaxAlternatives caps how many entries Result.Alternatives may contain.
	// Defaults to 3 when zero. Set to a negative value to disable
	// alternatives entirely.
	MaxAlternatives int
}

func (c Config) reviewThreshold() Confidence {
	if c.ReviewThreshold == 0 {
		return DefaultReviewThreshold
	}
	return c.ReviewThreshold
}

func (c Config) maxAlternatives() int {
	if c.MaxAlternatives == 0 {
		return 3
	}
	if c.MaxAlternatives < 0 {
		return 0
	}
	return c.MaxAlternatives
}
