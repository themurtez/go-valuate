package inventory

import "time"

// AdjustmentSummary summarizes adjustment/write-off activity for one
// period/item/category/location grouping — task section 34. Language
// throughout stays neutral ("inventory adjustment," "write-off," "review
// recommended") — never shrinkage/fraud/theft (task sections 6/35/66); see
// safety_test.go.
type AdjustmentSummary struct {
	Available bool `json:"available"`

	AdjustmentIncrease Value `json:"adjustment_increase"`
	AdjustmentDecrease Value `json:"adjustment_decrease"`
	WriteOffValue      Value `json:"write_off_value"`
	// NetAdjustment is AdjustmentIncrease + AdjustmentDecrease (decrease
	// is already a negative-signed magnitude here — see
	// buildAdjustmentSummary) + WriteOffValue's magnitude effect
	// (write-offs always reduce inventory).
	NetAdjustment   Value `json:"net_adjustment"`
	AdjustmentCount int   `json:"adjustment_count"`

	// AbsoluteAdjustmentValue is |increase| + |decrease| + |write-off| —
	// the numerator for AdjustmentRate.
	AbsoluteAdjustmentValue Value `json:"absolute_adjustment_value"`
	// AdjustmentRate is AbsoluteAdjustmentValue / AverageInventory — task
	// section 34. Unavailable if AverageInventory is unavailable or zero.
	AdjustmentRate Value `json:"adjustment_rate"`

	// WriteOffMateriality assesses WriteOffValue against the caller's
	// resolved materiality policy (task section 56). Unavailable when
	// WriteOffValue itself is unavailable.
	WriteOffMateriality MaterialityAssessment `json:"write_off_materiality"`

	// PeriodEndAdjustmentCount is the number of adjustment/write-off
	// movements dated within Policy.PeriodEndAdjustmentWindowDays of this
	// period's own EndDate — task section 35's "period-end inventory
	// adjustment" neutral timing observation. This is a genuine
	// date-proximity check against EndDate, not merely "any adjustment
	// activity occurred somewhere in the period" (a real bug caught and
	// fixed during this package's own code review — see
	// TestAdjustments_PeriodEndAdjustment and
	// TestAdjustments_NotFlaggedWhenAdjustmentIsMidPeriod).
	PeriodEndAdjustmentCount int `json:"period_end_adjustment_count,omitempty"`
}

func buildAdjustmentSummary(movements []Movement, start, end time.Time, averageInventory Value, materialityAbs, materialityPercent float64, periodEndWindowDays int) AdjustmentSummary {
	var increase, decrease, writeOff valueAccumulator
	var count, periodEndCount int
	for _, m := range movements {
		if m.Date.Before(start) || m.Date.After(end) || !isAdjustmentType(m.Type) {
			continue
		}
		val, _ := resolveMovementValue(m)
		if !val.Available {
			continue
		}
		count++
		if periodEndWindowDays > 0 && daysBetween(m.Date, end) <= float64(periodEndWindowDays) {
			periodEndCount++
		}
		magnitude := val.Amount * adjustmentSign(m.Type)
		switch m.Type {
		case MovementAdjustmentIncrease:
			increase.add(AvailableValue(magnitude))
		case MovementAdjustmentDecrease:
			decrease.add(AvailableValue(magnitude))
		case MovementWriteOff:
			writeOff.add(AvailableValue(val.Amount)) // reported as a positive magnitude — see WriteOffValue's doc comment.
		}
	}

	s := AdjustmentSummary{
		Available:                increase.ok || decrease.ok || writeOff.ok,
		AdjustmentIncrease:       increase.result(),
		AdjustmentDecrease:       decrease.result(),
		WriteOffValue:            writeOff.result(),
		AdjustmentCount:          count,
		PeriodEndAdjustmentCount: periodEndCount,
	}
	if !s.Available {
		return s
	}

	var net, absTotal float64
	if increase.ok {
		net += increase.sum
		absTotal += absFloat(increase.sum)
	}
	if decrease.ok {
		net += decrease.sum
		absTotal += absFloat(decrease.sum)
	}
	if writeOff.ok {
		net -= writeOff.sum // write-off always reduces inventory.
		absTotal += absFloat(writeOff.sum)
	}
	s.NetAdjustment = AvailableValue(net)
	s.AbsoluteAdjustmentValue = AvailableValue(absTotal)
	if averageInventory.Available && averageInventory.Amount != 0 {
		s.AdjustmentRate = AvailableValue(absTotal / averageInventory.Amount)
	}
	if writeOff.ok {
		base := 0.0
		if averageInventory.Available {
			base = averageInventory.Amount
		}
		s.WriteOffMateriality = assessMateriality(writeOff.sum, materialityAbs, materialityPercent, base)
	}
	return s
}

// ItemAdjustmentCount is one item's adjustment/write-off movement count
// within the analyzed window — used for FlagRepeatedItemAdjustments.
type ItemAdjustmentCount struct {
	ItemID string `json:"item_id"`
	Count  int    `json:"count"`
}

func countAdjustmentsByItem(movements []Movement) map[string]int {
	counts := map[string]int{}
	for _, m := range movements {
		if isAdjustmentType(m.Type) {
			counts[m.ItemID]++
		}
	}
	return counts
}
