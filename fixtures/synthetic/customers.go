package synthetic

import (
	"github.com/themurtez/go-valuate/analytics/concentration"
	"github.com/themurtez/go-valuate/analytics/revenuequality"
	"github.com/themurtez/go-valuate/financial"
)

// customerYears holds Meridian SaaS's customer-level revenue: 9 ongoing
// customers with individual growth trajectories, one that churns after
// 2022 (cust-juliet, exercising lost-customer/CustomerTransitions
// detection), one new customer arriving in 2024 (cust-kilo, exercising
// new-customer detection), and 3 long-tail buckets representing many
// smaller untracked accounts, sized so every year's customer-level total
// reconciles exactly to BuildDataset's REV_RECURRING + REV_OTHER for that
// year (revenuequality.ConcentrationSummary.UnallocatedRevenue == 0
// throughout, though this package's own contract does not require exact
// reconciliation — see CustomerPeriodRevenue.Amount's doc comment).
var customerYears = map[string]map[financial.Period]float64{
	"cust-alpha":   {"2021": 620_000, "2022": 780_000, "2023": 940_000, "2024": 1_120_000, "2025": 1_340_000},
	"cust-bravo":   {"2021": 410_000, "2022": 520_000, "2023": 640_000, "2024": 770_000, "2025": 920_000},
	"cust-charlie": {"2021": 350_000, "2022": 440_000, "2023": 540_000, "2024": 650_000, "2025": 780_000},
	"cust-delta":   {"2021": 300_000, "2022": 380_000, "2023": 470_000, "2024": 570_000, "2025": 680_000},
	"cust-echo":    {"2021": 260_000, "2022": 330_000, "2023": 410_000, "2024": 500_000, "2025": 600_000},
	"cust-foxtrot": {"2021": 220_000, "2022": 280_000, "2023": 350_000, "2024": 430_000, "2025": 520_000},
	"cust-golf":    {"2021": 190_000, "2022": 240_000, "2023": 300_000, "2024": 370_000, "2025": 450_000},
	"cust-hotel":   {"2021": 160_000, "2022": 200_000, "2023": 250_000, "2024": 310_000, "2025": 380_000},
	"cust-india":   {"2021": 130_000, "2022": 165_000, "2023": 205_000, "2024": 255_000, "2025": 310_000},
	"cust-juliet":  {"2021": 180_000, "2022": 210_000}, // churned after 2022
	"cust-kilo":    {"2024": 340_000, "2025": 480_000}, // new customer starting 2024
	"long-tail-a":  {"2021": 136_800, "2022": 223_600, "2023": 446_400, "2024": 604_400, "2025": 905_600},
	"long-tail-b":  {"2021": 119_700, "2022": 195_650, "2023": 390_600, "2024": 528_850, "2025": 792_400},
	"long-tail-c":  {"2021": 85_500, "2022": 139_750, "2023": 279_000, "2024": 377_750, "2025": 566_000},
}

// customerOrder is customerYears' keys in a fixed, deterministic order
// (largest-to-smallest by 2025 revenue), used so both BuildCustomerRevenue
// and BuildConcentrationObservations emit rows in a stable, human-readable
// order rather than Go map iteration order.
var customerOrder = []string{
	"cust-alpha", "cust-bravo", "cust-charlie", "cust-delta", "cust-echo",
	"cust-foxtrot", "cust-golf", "cust-hotel", "cust-india",
	"long-tail-a", "long-tail-b", "long-tail-c",
	"cust-kilo", "cust-juliet",
}

// BuildCustomerRevenue returns Meridian SaaS's customer-level revenue rows
// for analytics/revenuequality.Input.CustomerRevenue, across all 5 years
// (13 customer-year combinations short of the full 12x5=60 grid, since
// cust-juliet and cust-kilo are not present every year — see
// customerYears' doc comment).
func BuildCustomerRevenue() []revenuequality.CustomerPeriodRevenue {
	var rows []revenuequality.CustomerPeriodRevenue
	for _, key := range customerOrder {
		years := customerYears[key]
		for _, p := range Periods {
			amount, ok := years[p]
			if !ok {
				continue
			}
			rows = append(rows, revenuequality.CustomerPeriodRevenue{
				CustomerKey: key,
				Period:      p,
				Amount:      amount,
			})
		}
	}
	return rows
}

// BuildConcentrationObservations returns the same customer-level revenue as
// BuildCustomerRevenue, reshaped into analytics/concentration's
// dataset-independent Observation tuple (EntityKey/Period/Amount) for
// analytics/concentration.Input.Observations.
func BuildConcentrationObservations() []concentration.Observation {
	var obs []concentration.Observation
	for _, key := range customerOrder {
		years := customerYears[key]
		for _, p := range Periods {
			amount, ok := years[p]
			if !ok {
				continue
			}
			obs = append(obs, concentration.Observation{
				EntityKey: key,
				Period:    p,
				Amount:    amount,
			})
		}
	}
	return obs
}
