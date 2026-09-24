package vendorspend

import "sort"

// SupplierChangeKind classifies one supplier's status change between two
// chronologically adjacent periods — task section 11.
type SupplierChangeKind string

const (
	ChangeNewSupplierActivity       SupplierChangeKind = "NEW_SUPPLIER_ACTIVITY"
	ChangeSupplierSpendDiscontinued SupplierChangeKind = "SUPPLIER_SPEND_DISCONTINUED"
)

// SupplierChange is one detected new-supplier-activity or
// spend-discontinued event between two adjacent periods.
type SupplierChange struct {
	Kind       SupplierChangeKind `json:"kind"`
	SupplierID string             `json:"supplier_id"`
	FromPeriod string             `json:"from_period,omitempty"`
	ToPeriod   string             `json:"to_period"`
	// Amount is the NetSpend that appeared (NEW_SUPPLIER_ACTIVITY, in
	// ToPeriod) or disappeared (SUPPLIER_SPEND_DISCONTINUED, from
	// FromPeriod).
	Amount float64 `json:"amount"`
	// Material echoes whether Amount cleared the applicable
	// Policy.NewSupplierMaterialAmount/LostSupplierMaterialAmount
	// threshold — carried on the record itself (not only as a Flag) so a
	// caller filtering NewLostSuppliers does not need to cross-reference
	// Flags.
	Material bool `json:"material"`
}

// NewLostSuppliers is the result of new/lost-supplier detection across
// Input.Periods — task section 11. Detection requires at least two
// chronological periods with spend; with only one, this package never
// calls every supplier "new" (task section 11's explicit rule) — see
// Available.
type NewLostSuppliers struct {
	Available bool             `json:"available"`
	Changes   []SupplierChange `json:"changes,omitempty"`
}

// activeSuppliersByPeriod maps each chronological period label to the set
// of SupplierIDs with nonzero-transaction-count activity in that period
// (any included record, regardless of NetSpend sign).
func activeSuppliersByPeriod(orderedPeriods []string, byPeriod map[string][]SpendRecord) map[string]map[string]float64 {
	out := make(map[string]map[string]float64, len(orderedPeriods))
	for _, p := range orderedPeriods {
		spend := map[string]float64{}
		for _, r := range byPeriod[p] {
			spend[r.SupplierID] += netSpendOf(r)
		}
		out[p] = spend
	}
	return out
}

// computeNewLostSuppliers detects new-supplier-activity and
// supplier-spend-discontinued events across every chronologically
// adjacent period pair.
func computeNewLostSuppliers(orderedPeriods []string, byPeriod map[string][]SpendRecord, newMaterial, lostMaterial float64) NewLostSuppliers {
	if len(orderedPeriods) < 2 {
		return NewLostSuppliers{}
	}
	activity := activeSuppliersByPeriod(orderedPeriods, byPeriod)

	var changes []SupplierChange
	for i := 1; i < len(orderedPeriods); i++ {
		from, to := orderedPeriods[i-1], orderedPeriods[i]
		fromSet, toSet := activity[from], activity[to]

		var newIDs, lostIDs []string
		for sid := range toSet {
			if _, hadBefore := fromSet[sid]; !hadBefore {
				newIDs = append(newIDs, sid)
			}
		}
		for sid := range fromSet {
			if _, hasNow := toSet[sid]; !hasNow {
				lostIDs = append(lostIDs, sid)
			}
		}
		sort.Strings(newIDs)
		sort.Strings(lostIDs)

		for _, sid := range newIDs {
			amt := toSet[sid]
			changes = append(changes, SupplierChange{
				Kind: ChangeNewSupplierActivity, SupplierID: sid, FromPeriod: from, ToPeriod: to,
				Amount: amt, Material: newMaterial > 0 && amt >= newMaterial,
			})
		}
		for _, sid := range lostIDs {
			amt := fromSet[sid]
			changes = append(changes, SupplierChange{
				Kind: ChangeSupplierSpendDiscontinued, SupplierID: sid, FromPeriod: from, ToPeriod: to,
				Amount: amt, Material: lostMaterial > 0 && amt >= lostMaterial,
			})
		}
	}

	return NewLostSuppliers{Available: true, Changes: changes}
}

// GrowthDirection is a coarse, deterministic characterization of a
// period-over-period change — mirrors every analytics sibling package's
// identical TrendDirection convention, adapted to a single adjacent-pair
// comparison rather than a first-vs-last series.
type GrowthDirection string

const (
	GrowthIncreasing  GrowthDirection = "increasing"
	GrowthDeclining   GrowthDirection = "declining"
	GrowthStable      GrowthDirection = "stable"
	GrowthUnavailable GrowthDirection = "unavailable"
)

// growthFlatBandPercent is the |change|/|from| band within which a change
// is characterized GrowthStable — fixed, part of FormulaVersion, mirrors
// analytics/concentration.TrendFlatBandPercent's identical ±5% band.
const growthFlatBandPercent = 0.05

// GrowthChange is one adjacent-period-pair change in spend, with explicit
// zero-base handling — task section 12's "zero-base behavior must be
// explicit" rule: PercentChange is Unavailable (never +Inf/NaN) when
// FromAmount is zero, and Direction is still reported from the raw
// amount change alone in that case.
type GrowthChange struct {
	FromPeriod string  `json:"from_period"`
	ToPeriod   string  `json:"to_period"`
	FromAmount float64 `json:"from_amount"`
	ToAmount   float64 `json:"to_amount"`
	// PercentChange is (ToAmount-FromAmount)/|FromAmount|. Unavailable if
	// FromAmount == 0.
	PercentChange Value           `json:"percent_change"`
	Direction     GrowthDirection `json:"direction"`
}

// computeGrowthChange builds one GrowthChange from two period amounts.
func computeGrowthChange(fromPeriod, toPeriod string, fromAmt, toAmt float64) GrowthChange {
	gc := GrowthChange{FromPeriod: fromPeriod, ToPeriod: toPeriod, FromAmount: fromAmt, ToAmount: toAmt}
	if fromAmt == 0 {
		gc.Direction = GrowthUnavailable
		if toAmt > 0 {
			gc.Direction = GrowthIncreasing
		} else if toAmt < 0 {
			gc.Direction = GrowthDeclining
		}
		return gc
	}
	pct := (toAmt - fromAmt) / absFloat(fromAmt)
	gc.PercentChange = AvailableValue(pct)
	switch {
	case pct > growthFlatBandPercent:
		gc.Direction = GrowthIncreasing
	case pct < -growthFlatBandPercent:
		gc.Direction = GrowthDeclining
	default:
		gc.Direction = GrowthStable
	}
	return gc
}

// ShareChange is one supplier's spend-share change between two adjacent
// periods, in percentage points (raw decimal, e.g. 0.05 means 5 points) —
// task section 12's "use percentage points for share changes" rule.
type ShareChange struct {
	SupplierID   string `json:"supplier_id"`
	FromPeriod   string `json:"from_period"`
	ToPeriod     string `json:"to_period"`
	FromShare    Value  `json:"from_share"`
	ToShare      Value  `json:"to_share"`
	ChangePoints Value  `json:"change_points"`
}

// SupplierGrowth is one supplier's GrowthChange for one adjacent period
// pair.
type SupplierGrowth struct {
	SupplierID string       `json:"supplier_id"`
	Growth     GrowthChange `json:"growth"`
}

// CategoryGrowth is one category's GrowthChange for one adjacent period
// pair.
type CategoryGrowth struct {
	Category string       `json:"category"`
	Growth   GrowthChange `json:"growth"`
}

// SpendTrends bundles every growth/share-trend output — task section 12.
type SpendTrends struct {
	Available bool `json:"available"`
	// TotalSpendGrowth is one GrowthChange per adjacent period pair,
	// computed against each period's NetSpend total.
	TotalSpendGrowth []GrowthChange `json:"total_spend_growth,omitempty"`
	// SupplierSpendGrowth is one SupplierGrowth per supplier per adjacent
	// period pair the supplier had activity in either period, sorted by
	// ToPeriod then SupplierID.
	SupplierSpendGrowth []SupplierGrowth `json:"supplier_spend_growth,omitempty"`
	// CategorySpendGrowth mirrors SupplierSpendGrowth, keyed by Category.
	CategorySpendGrowth []CategoryGrowth `json:"category_spend_growth,omitempty"`
	// SupplierShareChanges is one ShareChange per supplier per adjacent
	// period pair.
	SupplierShareChanges []ShareChange `json:"supplier_share_changes,omitempty"`
}

// computeSpendTrends computes every growth/share-trend output across
// orderedPeriods.
func computeSpendTrends(orderedPeriods []string, byPeriod map[string][]SpendRecord) SpendTrends {
	if len(orderedPeriods) < 2 {
		return SpendTrends{}
	}

	periodBridge := map[string]SpendBridge{}
	periodSupplierNet := map[string]map[string]float64{}
	periodCategoryNet := map[string]map[string]float64{}
	for _, p := range orderedPeriods {
		periodBridge[p] = computeBridge(byPeriod[p])
		supNet := map[string]float64{}
		catNet := map[string]float64{}
		for _, r := range byPeriod[p] {
			supNet[r.SupplierID] += netSpendOf(r)
			if r.Category != "" {
				catNet[r.Category] += netSpendOf(r)
			}
		}
		periodSupplierNet[p] = supNet
		periodCategoryNet[p] = catNet
	}

	st := SpendTrends{Available: true}

	for i := 1; i < len(orderedPeriods); i++ {
		from, to := orderedPeriods[i-1], orderedPeriods[i]
		st.TotalSpendGrowth = append(st.TotalSpendGrowth,
			computeGrowthChange(from, to, periodBridge[from].NetSpend, periodBridge[to].NetSpend))

		supIDs := unionKeys(periodSupplierNet[from], periodSupplierNet[to])
		for _, sid := range supIDs {
			growth := computeGrowthChange(from, to, periodSupplierNet[from][sid], periodSupplierNet[to][sid])
			st.SupplierSpendGrowth = append(st.SupplierSpendGrowth, SupplierGrowth{SupplierID: sid, Growth: growth})

			var fromShare, toShare Value
			if periodBridge[from].NetSpend != 0 {
				fromShare = AvailableValue(periodSupplierNet[from][sid] / periodBridge[from].NetSpend)
			}
			if periodBridge[to].NetSpend != 0 {
				toShare = AvailableValue(periodSupplierNet[to][sid] / periodBridge[to].NetSpend)
			}
			sc := ShareChange{SupplierID: sid, FromPeriod: from, ToPeriod: to, FromShare: fromShare, ToShare: toShare}
			if fromShare.Available && toShare.Available {
				sc.ChangePoints = AvailableValue(toShare.Value - fromShare.Value)
			}
			st.SupplierShareChanges = append(st.SupplierShareChanges, sc)
		}

		catKeys := unionKeys(periodCategoryNet[from], periodCategoryNet[to])
		for _, cat := range catKeys {
			growth := computeGrowthChange(from, to, periodCategoryNet[from][cat], periodCategoryNet[to][cat])
			st.CategorySpendGrowth = append(st.CategorySpendGrowth, CategoryGrowth{Category: cat, Growth: growth})
		}
	}

	return st
}

// unionKeys returns the sorted union of a's and b's keys.
func unionKeys(a, b map[string]float64) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
