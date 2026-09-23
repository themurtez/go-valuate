package cashforecast

import "time"

// CreditFacility is an optional revolver/line-of-credit a caller wants
// reported alongside the funding gap, purely as available capacity — this
// package never automatically creates a draw against it. See the task's
// "report funding gap, report available facility capacity, do not
// auto-draw" instruction; a caller wanting to model an actual draw adds an
// explicit CashFlowEvent (CategoryLoanProceeds) or uses a scenario
// transform, and DrawnFacilityGap (in liquidity.go) shows the gap that
// would remain after applying available capacity without asserting the
// draw occurred.
type CreditFacility struct {
	FacilityID      string    `json:"facility_id"`
	Name            string    `json:"name,omitempty"`
	AvailableToDraw float64   `json:"available_to_draw"`
	MinimumDraw     float64   `json:"minimum_draw,omitempty"`
	MaximumDraw     float64   `json:"maximum_draw,omitempty"`
	ExpiryDate      time.Time `json:"expiry_date,omitempty"`
}

// totalFacilityCapacity sums AvailableToDraw across facilities, ignoring
// any with a non-positive AvailableToDraw.
func totalFacilityCapacity(facilities []CreditFacility) float64 {
	var total float64
	for _, f := range facilities {
		if f.AvailableToDraw > 0 {
			total += f.AvailableToDraw
		}
	}
	return total
}
