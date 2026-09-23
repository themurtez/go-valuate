package inventory

// resolveSnapshotValue determines one snapshot's InventoryValue per task
// section 8/4: if InventoryValue is explicitly supplied, it is preserved
// and used as-is (the more direct fact); otherwise, if both Quantity and
// UnitCost are available, an analytical value is computed as their
// product. If BOTH are supplied and disagree beyond amountTolerance, the
// explicit InventoryValue still wins (never silently overwritten) but an
// IssueValueQuantityCostMismatch is reported so the discrepancy is never
// silent.
func resolveSnapshotValue(s InventorySnapshot) (value Value, issue *Issue) {
	analytical, hasAnalytical := Unavailable(), false
	if s.QuantityOnHand.Available && s.UnitCost.Available {
		analytical = AvailableValue(s.QuantityOnHand.Amount * s.UnitCost.Amount)
		hasAnalytical = true
	}

	if s.InventoryValue.Available {
		if hasAnalytical && absFloat(s.InventoryValue.Amount-analytical.Amount) > amountTolerance {
			return s.InventoryValue, &Issue{
				Code: IssueValueQuantityCostMismatch, Severity: SeverityWarning,
				Message:    "supplied inventory value disagrees with quantity x unit cost beyond tolerance",
				ItemID:     s.ItemID,
				SnapshotID: s.ID,
			}
		}
		return s.InventoryValue, nil
	}
	if hasAnalytical {
		return analytical, nil
	}
	return Unavailable(), nil
}

// resolveMovementValue determines one movement's Amount, mirroring
// resolveSnapshotValue's identical precedence: explicit Amount wins over
// Quantity x UnitCost when both are supplied, with a mismatch reported
// rather than silently resolved.
func resolveMovementValue(m Movement) (value Value, issue *Issue) {
	analytical, hasAnalytical := Unavailable(), false
	if m.Quantity.Available && m.UnitCost.Available {
		analytical = AvailableValue(m.Quantity.Amount * m.UnitCost.Amount)
		hasAnalytical = true
	}

	if m.Amount.Available {
		if hasAnalytical && absFloat(m.Amount.Amount-analytical.Amount) > amountTolerance {
			return m.Amount, &Issue{
				Code: IssueValueQuantityCostMismatch, Severity: SeverityWarning,
				Message:    "supplied movement amount disagrees with quantity x unit cost beyond tolerance",
				ItemID:     m.ItemID,
				MovementID: m.ID,
			}
		}
		return m.Amount, nil
	}
	if hasAnalytical {
		return analytical, nil
	}
	return Unavailable(), nil
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
