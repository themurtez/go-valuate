package inventory

import "time"

// ExpiryStatus classifies one lot against AsOfDate and the caller's
// warning window — task section 48.
type ExpiryStatus string

const (
	ExpiryStatusNone     ExpiryStatus = "NONE"     // no ExpiryDate supplied.
	ExpiryStatusOK       ExpiryStatus = "OK"       // ExpiryDate known, not within the warning window.
	ExpiryStatusExpiring ExpiryStatus = "EXPIRING" // within Policy.ExpiryWarningDays of AsOfDate, not yet expired.
	ExpiryStatusExpired  ExpiryStatus = "EXPIRED"  // ExpiryDate is on or before AsOfDate.
)

// LotExpiry is one lot's expiry-review row.
type LotExpiry struct {
	ItemID   string `json:"item_id"`
	LotID    string `json:"lot_id,omitempty"`
	Location string `json:"location,omitempty"`

	ExpiryDate      *time.Time   `json:"expiry_date,omitempty"`
	Status          ExpiryStatus `json:"status"`
	DaysUntilExpiry Value        `json:"days_until_expiry"`

	Quantity Qty   `json:"quantity"`
	Value    Value `json:"value"`
}

// ExpirySummary is the full expiry-review result — task section 48.
type ExpirySummary struct {
	Available bool `json:"available"`

	Rows []LotExpiry `json:"rows,omitempty"`

	ExpiredValue  Value `json:"expired_value"`
	ExpiringValue Value `json:"expiring_value"`
}

func buildExpirySummary(order []string, states map[string]*itemState, asOf time.Time, warningDays int) ExpirySummary {
	var rows []LotExpiry
	var expired, expiring valueAccumulator

	for _, id := range order {
		st := states[id]
		keys := make([][2]string, 0, len(st.latestByLocationLot))
		for k := range st.latestByLocationLot {
			keys = append(keys, k)
		}
		sortLocationLotKeys(keys)

		for _, k := range keys {
			snap := st.latestByLocationLot[k]
			if snap.ExpiryDate == nil {
				continue
			}
			val, _ := resolveSnapshotValue(snap)
			row := LotExpiry{ItemID: id, LotID: snap.LotID, Location: snap.Location, ExpiryDate: snap.ExpiryDate, Quantity: snap.QuantityOnHand, Value: val}

			daysUntil := daysBetween(asOf, *snap.ExpiryDate)
			row.DaysUntilExpiry = AvailableValue(daysUntil)

			switch {
			case !snap.ExpiryDate.After(asOf):
				row.Status = ExpiryStatusExpired
				expired.add(val)
			case warningDays > 0 && daysUntil <= float64(warningDays):
				row.Status = ExpiryStatusExpiring
				expiring.add(val)
			default:
				row.Status = ExpiryStatusOK
			}
			rows = append(rows, row)
		}
	}

	return ExpirySummary{
		Available:     len(rows) > 0,
		Rows:          rows,
		ExpiredValue:  expired.result(),
		ExpiringValue: expiring.result(),
	}
}
