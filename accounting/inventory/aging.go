package inventory

import (
	"sort"
	"time"
)

// ItemLastMovement is one item's last-movement facts as of AsOfDate —
// task section 23. DaysSinceOutbound/DaysSinceAnyMovement are Unavailable
// when the corresponding date is unknown (never inferred, never zero).
type ItemLastMovement struct {
	ItemID string `json:"item_id"`

	LastReceiptDate     *time.Time `json:"last_receipt_date,omitempty"`
	LastOutboundDate    *time.Time `json:"last_outbound_date,omitempty"`
	LastAnyMovementDate *time.Time `json:"last_any_movement_date,omitempty"`

	DaysSinceOutbound    Value `json:"days_since_outbound"`
	DaysSinceAnyMovement Value `json:"days_since_any_movement"`
}

func buildItemLastMovement(itemID string, st *itemState, asOf time.Time) ItemLastMovement {
	lm := ItemLastMovement{
		ItemID:              itemID,
		LastReceiptDate:     st.lastReceiptDate,
		LastOutboundDate:    st.lastOutboundDate,
		LastAnyMovementDate: st.lastAnyMovementDate,
	}
	if st.lastOutboundDate != nil {
		lm.DaysSinceOutbound = AvailableValue(daysBetween(*st.lastOutboundDate, asOf))
	}
	if st.lastAnyMovementDate != nil {
		lm.DaysSinceAnyMovement = AvailableValue(daysBetween(*st.lastAnyMovementDate, asOf))
	}
	return lm
}

func daysBetween(from, to time.Time) float64 {
	return to.Sub(from).Hours() / 24
}

// AgeEvidence is the basis this package used to determine one item/lot's
// age — task section 17: "preferred lot-level aging when available," and
// never inferring exact age from current snapshot + aggregate movement
// history when attribution is ambiguous.
type AgeEvidence string

const (
	AgeEvidenceLotReceivedDate   AgeEvidence = "LOT_RECEIVED_DATE"
	AgeEvidenceLastReceiptDate   AgeEvidence = "LAST_RECEIPT_DATE"
	AgeEvidenceLastMovementDate  AgeEvidence = "LAST_MOVEMENT_DATE"
	AgeEvidenceCallerSuppliedAge AgeEvidence = "CALLER_SUPPLIED_AGE"
	AgeEvidenceUnknown           AgeEvidence = "UNKNOWN"
)

// ItemAging is one item's (or one lot's, when lot-level snapshots exist)
// value-aging result.
type ItemAging struct {
	ItemID   string `json:"item_id"`
	LotID    string `json:"lot_id,omitempty"`
	Location string `json:"location,omitempty"`

	AgeDays  Value       `json:"age_days"`
	Evidence AgeEvidence `json:"evidence"`

	// BucketCode is UnknownAgeBucketCode when Evidence is
	// AgeEvidenceUnknown, or when AgeDays falls into a gap the caller's
	// custom bucket schema does not cover.
	BucketCode string `json:"bucket_code"`
	Quantity   Qty    `json:"quantity"`
	Value      Value  `json:"value"`
}

// AgingBucketTotal is one bucket's aggregated totals across every aged
// row — task section 18: "return aging by quantity where compatible,
// value, item count."
type AgingBucketTotal struct {
	BucketCode string `json:"bucket_code"`
	Label      string `json:"label,omitempty"`

	Value float64 `json:"value"`
	// Quantity is unavailable if any contributing row's UOM is
	// incompatible with the bucket's dominant UOM — see aggregateAgingQty.
	Quantity  Qty `json:"quantity"`
	ItemCount int `json:"item_count"`
}

// AgingSummary is the full value-aging result — task sections 17-21, 50.
type AgingSummary struct {
	Available bool `json:"available"`

	Buckets []BucketDefinition `json:"buckets"`

	Rows         []ItemAging        `json:"rows,omitempty"`
	BucketTotals []AgingBucketTotal `json:"bucket_totals,omitempty"`

	TotalValue        float64 `json:"total_value"`
	UnknownAgeValue   float64 `json:"unknown_age_value"`
	UnknownAgePercent Value   `json:"unknown_age_percent"`

	SlowMovingValue   Value `json:"slow_moving_value"`
	NonMovingValue    Value `json:"non_moving_value"`
	SlowMovingPercent Value `json:"slow_moving_percent"`
	NonMovingPercent  Value `json:"non_moving_percent"`

	// SlowMovingMateriality assesses SlowMovingValue against the caller's
	// resolved materiality policy (task section 56) — review-attention
	// materiality, never audit materiality. Unavailable when
	// SlowMovingValue itself is unavailable.
	SlowMovingMateriality MaterialityAssessment `json:"slow_moving_materiality"`
}

// buildAgingSummary computes value-aging across every item's latest
// (location, lot) snapshots. Lot-level ReceivedDate is preferred evidence;
// falling back to LastReceiptDate, then LastAnyMovementDate, then
// AgeEvidenceUnknown (task section 17's preference order). Slow/non-moving
// classification (task sections 19-20) is based on outbound-movement
// recency, not on age bucket, and is reported at the whole-item level
// (SlowMovingValue/NonMovingValue) since "no outbound movement" is an
// item-wide fact, not a per-lot one.
func buildAgingSummary(order []string, states map[string]*itemState, buckets []BucketDefinition, asOf time.Time, policy Policy) AgingSummary {
	sortedBuckets := sortedBucketsByMin(buckets)

	var rows []ItemAging
	var totalValue float64

	for _, id := range order {
		st := states[id]
		keys := make([][2]string, 0, len(st.latestByLocationLot))
		for k := range st.latestByLocationLot {
			keys = append(keys, k)
		}
		sortLocationLotKeys(keys)

		for _, k := range keys {
			snap := st.latestByLocationLot[k]
			val, _ := resolveSnapshotValue(snap)
			ageDays, evidence := resolveAgeEvidence(snap, st, asOf)

			row := ItemAging{
				ItemID:   id,
				LotID:    snap.LotID,
				Location: snap.Location,
				Evidence: evidence,
				Quantity: snap.QuantityOnHand,
				Value:    val,
			}
			if evidence == AgeEvidenceUnknown {
				row.BucketCode = UnknownAgeBucketCode
			} else {
				row.AgeDays = AvailableValue(ageDays)
				code, ok := bucketForDays(sortedBuckets, int(ageDays))
				if !ok {
					code = UnknownAgeBucketCode
				}
				row.BucketCode = code
			}
			rows = append(rows, row)
			if val.Available {
				totalValue += val.Amount
			}
		}
	}

	sortItemAgingRows(rows)

	bucketTotals := buildAgingBucketTotals(rows, sortedBuckets)

	summary := AgingSummary{
		Available:    len(rows) > 0,
		Buckets:      sortedBuckets,
		Rows:         rows,
		BucketTotals: bucketTotals,
		TotalValue:   totalValue,
	}

	var unknownValue float64
	for _, r := range rows {
		if r.BucketCode == UnknownAgeBucketCode && r.Value.Available {
			unknownValue += r.Value.Amount
		}
	}
	summary.UnknownAgeValue = unknownValue
	if totalValue != 0 {
		summary.UnknownAgePercent = AvailableValue(unknownValue / totalValue)
	}

	slow, nonMoving := buildSlowNonMovingValues(order, states, asOf, policy)
	summary.SlowMovingValue = slow
	summary.NonMovingValue = nonMoving
	if slow.Available && totalValue != 0 {
		summary.SlowMovingPercent = AvailableValue(slow.Amount / totalValue)
	}
	if nonMoving.Available && totalValue != 0 {
		summary.NonMovingPercent = AvailableValue(nonMoving.Amount / totalValue)
	}
	if slow.Available {
		summary.SlowMovingMateriality = assessMateriality(slow.Amount, policy.Materiality, policy.MaterialityPercent, totalValue)
	}

	return summary
}

// resolveAgeEvidence determines one lot/item's age in days as of asOf,
// per task section 17's preference order: lot ReceivedDate, then the
// item's LastReceiptDate, then LastAnyMovementDate, else Unknown. This
// package never falls back further to "current snapshot plus aggregate
// movement history" reconstruction — that attribution is ambiguous across
// multiple lots and this package does not guess which unit is oldest.
func resolveAgeEvidence(snap InventorySnapshot, st *itemState, asOf time.Time) (float64, AgeEvidence) {
	if snap.ReceivedDate != nil && !snap.ReceivedDate.After(asOf) {
		return daysBetween(*snap.ReceivedDate, asOf), AgeEvidenceLotReceivedDate
	}
	if st.lastReceiptDate != nil && !st.lastReceiptDate.After(asOf) {
		return daysBetween(*st.lastReceiptDate, asOf), AgeEvidenceLastReceiptDate
	}
	if st.lastAnyMovementDate != nil && !st.lastAnyMovementDate.After(asOf) {
		return daysBetween(*st.lastAnyMovementDate, asOf), AgeEvidenceLastMovementDate
	}
	return 0, AgeEvidenceUnknown
}

// buildAgingBucketTotals aggregates rows into per-bucket totals in fixed
// bucket order (sortedBuckets, then UnknownAgeBucketCode last) — task
// section 70's "aging buckets defined order."
func buildAgingBucketTotals(rows []ItemAging, sortedBuckets []BucketDefinition) []AgingBucketTotal {
	type acc struct {
		value     float64
		itemCount int
		qty       float64
		qtyOK     bool
		qtyUOM    string
		qtySeen   bool
	}
	byCode := map[string]*acc{}
	order := []string{}
	for _, b := range sortedBuckets {
		byCode[b.Code] = &acc{qtyOK: true}
		order = append(order, b.Code)
	}
	byCode[UnknownAgeBucketCode] = &acc{qtyOK: true}
	order = append(order, UnknownAgeBucketCode)

	for _, r := range rows {
		a, ok := byCode[r.BucketCode]
		if !ok {
			continue // defensive; bucketForDays/UnknownAgeBucketCode covers every case.
		}
		a.itemCount++
		if r.Value.Available {
			a.value += r.Value.Amount
		}
		if !r.Quantity.Available {
			a.qtyOK = false
			continue
		}
		if !a.qtySeen {
			a.qtyUOM = r.Quantity.UnitOfMeasure
			a.qtySeen = true
		} else if a.qtyUOM != r.Quantity.UnitOfMeasure {
			a.qtyOK = false
			continue
		}
		a.qty += r.Quantity.Amount
	}

	var out []AgingBucketTotal
	for _, code := range order {
		a := byCode[code]
		if a.itemCount == 0 {
			continue
		}
		label := ""
		if def := bucketDefByCode(sortedBuckets, code); def != nil {
			label = def.Label
		}
		bt := AgingBucketTotal{BucketCode: code, Label: label, Value: a.value, ItemCount: a.itemCount}
		if a.qtyOK && a.qtySeen {
			bt.Quantity = AvailableQty(a.qty, a.qtyUOM)
		}
		out = append(out, bt)
	}
	return out
}

// buildSlowNonMovingValues sums InventoryValue for items classified
// slow-moving/non-moving per policy — task sections 19-20. A caller
// without SlowMovingDays configured gets Unavailable for both (this
// package never invents a default obsolescence period); NonMovingValue is
// separately Unavailable when only NonMovingDays is left unconfigured.
func buildSlowNonMovingValues(order []string, states map[string]*itemState, asOf time.Time, policy Policy) (slow, nonMoving Value) {
	if policy.SlowMovingDays <= 0 {
		return Unavailable(), Unavailable()
	}
	var slowTotal, nonMovingTotal float64
	var any bool
	for _, id := range order {
		st := states[id]
		if !st.value.Available {
			continue
		}
		days, ok := slowMovingReferenceDays(st, asOf)
		if !ok {
			continue
		}
		any = true
		if days >= float64(policy.SlowMovingDays) {
			slowTotal += st.value.Amount
		}
		if policy.NonMovingDays > 0 && days >= float64(policy.NonMovingDays) {
			nonMovingTotal += st.value.Amount
		}
	}
	if !any {
		return Unavailable(), Unavailable()
	}
	slowVal := AvailableValue(slowTotal)
	nonMovingVal := Unavailable()
	if policy.NonMovingDays > 0 {
		nonMovingVal = AvailableValue(nonMovingTotal)
	}
	return slowVal, nonMovingVal
}

// slowMovingReferenceDays returns the day count used for slow/non-moving
// classification as of asOf: days since last outbound movement, or — if
// no outbound evidence exists at all — days since any movement (task
// section 19's "LastOutboundMovementDate + caller SlowMovingDays,"
// falling back per this package's general "use the best available
// evidence, never nothing" rule). Returns (0, false) if neither date is
// known.
func slowMovingReferenceDays(st *itemState, asOf time.Time) (float64, bool) {
	if st.lastOutboundDate != nil {
		return daysBetween(*st.lastOutboundDate, asOf), true
	}
	if st.lastAnyMovementDate != nil {
		return daysBetween(*st.lastAnyMovementDate, asOf), true
	}
	return 0, false
}

func sortItemAgingRows(rows []ItemAging) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].ItemID != rows[j].ItemID {
			return rows[i].ItemID < rows[j].ItemID
		}
		if rows[i].Location != rows[j].Location {
			return rows[i].Location < rows[j].Location
		}
		return rows[i].LotID < rows[j].LotID
	})
}
