package classification

import "github.com/themurtez/go-valuate/financial"

// Alias maps one account label to a canonical code. Label is matched against
// a row's normalized label (see NormalizeLabel), so callers do not need to
// pre-normalize it themselves, though it is normalized again defensively
// when the layer is built.
type Alias struct {
	// Label is the account label this alias matches, in its natural,
	// human-readable form (e.g. "Advertising & Promotion"). It is
	// normalized internally the same way row labels are.
	Label string
	// Code is the canonical taxonomy code this label maps to.
	Code financial.Code
}

// AliasLayer is a named, ordered set of aliases at a single precedence
// level. This package has no built-in concept of "account" or "client" —
// Name is an opaque label supplied by the caller purely for
// Result.MatchedRule / debugging purposes. Precedence between layers is
// determined entirely by their position in Config.AliasLayers: later layers
// override earlier ones, matching the convention used by
// settings.Resolve (lowest precedence first, highest precedence last).
type AliasLayer struct {
	// Name identifies this layer for provenance (e.g. "global", "account",
	// "client", "valuation", or any other name the caller chooses). Purely
	// descriptive; classification logic never branches on it.
	Name string
	// Aliases is the set of label-to-code mappings at this layer.
	Aliases []Alias
}

// resolvedAlias is one alias after normalization and layer-precedence
// resolution, ready for exact-match lookup by comparable label.
type resolvedAlias struct {
	code      financial.Code
	layerName string
	label     string // original (non-normalized) label, for the Reason message
}

// buildAliasIndex flattens a caller-supplied, precedence-ordered list of
// AliasLayer values into a single lookup keyed by normalized label. Layers
// later in the slice take precedence over earlier ones: for a given
// normalized label, the last layer that defines an alias for it wins. This
// mirrors settings.Resolve's "later argument wins" convention, generalized
// to an arbitrary number of layers since alias precedence chains
// (global/account/client/valuation, or any caller-defined set) can vary by
// application.
//
// Within a single layer, later aliases for the same normalized label
// override earlier ones in that same layer, so a layer's own list is also
// deterministic top-to-bottom.
func buildAliasIndex(layers []AliasLayer) map[string]resolvedAlias {
	index := make(map[string]resolvedAlias)
	for _, layer := range layers {
		for _, alias := range layer.Aliases {
			key := NormalizeLabel(alias.Label).Comparable
			if key == "" {
				continue
			}
			index[key] = resolvedAlias{
				code:      alias.Code,
				layerName: layer.Name,
				label:     alias.Label,
			}
		}
	}
	return index
}
