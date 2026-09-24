package kpi

import (
	"sort"
	"strings"
)

// DimensionKey is an opaque, exact-match set of dimension tags (e.g.
// department=service, location=toronto) — task section 5. This package
// never infers hierarchies or fuzzy matches between dimension values; two
// DimensionKeys either match exactly (Equal) or they don't.
//
// The zero DimensionKey (nil/empty map) represents the business-level
// (no-dimension) scope. See Broadcast/BroadcastAllowed below for the
// strict rule around mixing business-level and dimensioned MetricValues.
type DimensionKey map[string]string

// canonicalize returns a deterministic copy of d for equality/hashing/
// storage: keys sorted, empty-string values dropped (an empty tag value is
// treated as "tag absent," not as a meaningful empty string, avoiding a
// silent distinction between DimensionKey{"x": ""} and DimensionKey{}).
// The original is never mutated (task section 43's immutability rule).
func (d DimensionKey) canonicalize() DimensionKey {
	if len(d) == 0 {
		return nil
	}
	out := make(DimensionKey, len(d))
	for k, v := range d {
		if k == "" || v == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Canonical returns d's canonical form — see canonicalize. Exported so a
// caller building its own indexes over DimensionKey values can rely on the
// same normalization this package uses internally.
func (d DimensionKey) Canonical() DimensionKey {
	return d.canonicalize()
}

// hashKey returns a stable string encoding of d's canonical form, used as
// this package's internal map key wherever a DimensionKey needs one.
// Deterministic: canonical key order, "=" between key/value, "&" between
// pairs — never derived from Go map iteration order.
func (d DimensionKey) hashKey() string {
	c := d.canonicalize()
	if len(c) == 0 {
		return ""
	}
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(c[k])
	}
	return b.String()
}

// Equal reports whether d and o denote the exact same dimension set after
// canonicalization.
func (d DimensionKey) Equal(o DimensionKey) bool {
	return d.hashKey() == o.hashKey()
}

// IsBusinessLevel reports whether d is the zero (no-dimension) scope.
func (d DimensionKey) IsBusinessLevel() bool {
	return len(d.canonicalize()) == 0
}

// sortedDimensionKeys returns keys sorted by their canonical hashKey —
// task section 42's "dimension keys canonical" determinism rule.
func sortedDimensionKeys(keys []DimensionKey) []DimensionKey {
	out := make([]DimensionKey, len(keys))
	copy(out, keys)
	sort.Slice(out, func(i, j int) bool { return out[i].hashKey() < out[j].hashKey() })
	return out
}
