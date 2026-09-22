package settings

// Resolution is the outcome of resolving four scoped Settings layers into a
// single effective set of values, along with which scope each resolved
// value came from.
//
// Values holds resolved numeric fields keyed by field name (e.g.
// "sde_multiple") and resolved method-enable flags keyed by
// "method_enabled.<method>" (e.g. "method_enabled.sde"). A key is present
// in Values only if some scope explicitly set it; a field left unset at
// every scope is simply absent from Values (and from Sources), not present
// with a zero value, so callers can distinguish "resolved to zero" from
// "never set anywhere."
//
// Sources holds, for each key present in Values, which Scope supplied the
// winning value.
//
// Both maps use plain Go types (float64, bool) rather than pointers, since
// by the time a key is resolved into Values its unset/set ambiguity has
// already been settled — a key's mere presence in the map is what signals
// "was set."
type Resolution struct {
	Values  map[string]any   `json:"values"`
	Sources map[string]Scope `json:"sources"`
}

// FieldKey returns the Values/Sources key for a numeric settings field, for
// callers that want to look up a specific field without hardcoding the
// string (e.g. resolution.Values[settings.FieldKeySDEMultiple]).
const (
	FieldKeySDEMultiple           = "sde_multiple"
	FieldKeyEBITDAMultiple        = "ebitda_multiple"
	FieldKeyDiscountRate          = "discount_rate"
	FieldKeyTerminalGrowthRate    = "terminal_growth_rate"
	FieldKeyTaxRate               = "tax_rate"
	FieldKeyMarketabilityDiscount = "marketability_discount"
	FieldKeyControlPremium        = "control_premium"
)

// MethodFieldKey returns the Values/Sources key for a method-enable flag,
// e.g. MethodFieldKey(MethodSDE) == "method_enabled.sde".
func MethodFieldKey(m Method) string {
	return "method_enabled." + string(m)
}

// Resolve combines system, account, client, and valuation settings into a
// single Resolution, applying precedence valuation > client > account >
// system: for each field, the highest-precedence scope that has explicitly
// set the field wins; scopes that leave a field unset (nil) are skipped for
// that field and resolution falls through to the next lower-precedence
// scope.
//
// The Scope field on each argument is informational only (and is
// overwritten internally to the position it was passed in, so callers
// cannot cause incorrect precedence by mislabeling it); Resolve always
// treats system as lowest precedence and valuation as highest, in the order
// the arguments are named.
//
// Resolve does not validate business-logic ranges (e.g. that a discount
// rate is between 0 and 1) — it only combines what it is given. It performs
// no I/O and retains no state between calls.
func Resolve(system, account, client, valuation Settings) Resolution {
	system.Scope = ScopeSystem
	account.Scope = ScopeAccount
	client.Scope = ScopeClient
	valuation.Scope = ScopeValuation
	layers := []Settings{system, account, client, valuation}

	result := Resolution{
		Values:  make(map[string]any),
		Sources: make(map[string]Scope),
	}

	for _, field := range numericFields {
		for i := len(layers) - 1; i >= 0; i-- {
			if v := field.get(&layers[i]); v != nil {
				result.Values[field.key] = *v
				result.Sources[field.key] = scopeOrder[i]
				break
			}
		}
	}

	for _, method := range allMethods {
		key := MethodFieldKey(method)
		for i := len(layers) - 1; i >= 0; i-- {
			if v, ok := layers[i].MethodEnabled[method]; ok && v != nil {
				result.Values[key] = *v
				result.Sources[key] = scopeOrder[i]
				break
			}
		}
	}

	return result
}
