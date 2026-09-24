// Package fixtures provides synthetic accounting/vendorspend data for
// tests and examples: a set of small, hand-built supplier/spend
// scenarios covering vendorspend's tests. Nothing here is real financial
// data — every figure is invented for illustration, mirroring
// accounting/ap/fixtures' identical synthetic-data convention.
package fixtures

import (
	"strconv"
	"time"

	"github.com/themurtez/go-valuate/accounting/vendorspend"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// SixMonthPeriods returns six consecutive monthly Periods spanning
// 2025-01 through 2025-06, the shared calendar most scenarios below use.
func SixMonthPeriods() []vendorspend.Period {
	months := []struct {
		label      string
		start, end string
	}{
		{"2025-01", "2025-01-01", "2025-01-31"},
		{"2025-02", "2025-02-01", "2025-02-28"},
		{"2025-03", "2025-03-01", "2025-03-31"},
		{"2025-04", "2025-04-01", "2025-04-30"},
		{"2025-05", "2025-05-01", "2025-05-31"},
		{"2025-06", "2025-06-01", "2025-06-30"},
	}
	out := make([]vendorspend.Period, 0, len(months))
	for i, m := range months {
		out = append(out, vendorspend.Period{Period: m.label, StartDate: date(m.start), EndDate: date(m.end), SequenceInYear: i + 1})
	}
	return out
}

// TwoQuarterPeriods returns two consecutive quarterly Periods (2025-Q1,
// 2025-Q2) — used by scenarios wanting coarser periods.
func TwoQuarterPeriods() []vendorspend.Period {
	return []vendorspend.Period{
		{Period: "2025-Q1", StartDate: date("2025-01-01"), EndDate: date("2025-03-31"), SequenceInYear: 1},
		{Period: "2025-Q2", StartDate: date("2025-04-01"), EndDate: date("2025-06-30"), SequenceInYear: 2},
	}
}

// OnePeriodOnly returns a single period — used by the insufficient-
// history scenario.
func OnePeriodOnly() []vendorspend.Period {
	return []vendorspend.Period{
		{Period: "2025-01", StartDate: date("2025-01-01"), EndDate: date("2025-01-31"), SequenceInYear: 1},
	}
}

// DiversifiedSuppliers returns five suppliers with no single dominant
// vendor — used by the diversified-spend scenario.
func DiversifiedSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-A", Name: "Acme Supply Co", Category: "Raw Materials", Active: true},
		{SupplierID: "SUP-B", Name: "Beta Services LLC", Category: "Professional Services", Active: true},
		{SupplierID: "SUP-C", Name: "Gamma Logistics Inc", Category: "Logistics", Active: true},
		{SupplierID: "SUP-D", Name: "Delta Software Inc", Category: "Software", Active: true},
		{SupplierID: "SUP-E", Name: "Epsilon Marketing Co", Category: "Marketing", Active: true},
	}
}

// DiversifiedSpend returns six months of roughly even spend across
// DiversifiedSuppliers — no single supplier dominates.
func DiversifiedSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	suppliers := []string{"SUP-A", "SUP-B", "SUP-C", "SUP-D", "SUP-E"}
	amounts := []float64{9000, 8500, 8000, 7500, 7000}
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 1
	for _, p := range periods {
		for i, sid := range suppliers {
			out = append(out, vendorspend.SpendRecord{
				SpendID: spendID(&id), SupplierID: sid, Period: p, Date: periodMid(p),
				Amount: amounts[i], Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
				Category: "General",
			})
		}
	}
	return out
}

// ConcentratedSuppliers returns three suppliers: one dominant, two small.
func ConcentratedSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-BIG", Name: "Big Supplier Inc", Category: "Raw Materials", Active: true},
		{SupplierID: "SUP-SMALL1", Name: "Small One", Category: "Office Supplies", Active: true},
		{SupplierID: "SUP-SMALL2", Name: "Small Two", Category: "Office Supplies", Active: true},
	}
}

// HighlyConcentratedSpend returns six months of spend dominated by
// SUP-BIG (~90% of total spend each period) — used by the
// high-concentration scenario.
func HighlyConcentratedSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 1
	for _, p := range periods {
		out = append(out,
			vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-BIG", Period: p, Date: periodMid(p),
				Amount: 90000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Materials"},
			vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-SMALL1", Period: p, Date: periodMid(p),
				Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
			vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-SMALL2", Period: p, Date: periodMid(p),
				Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
		)
	}
	return out
}

// IncreasingConcentrationSpend returns six months where SUP-BIG's share
// climbs steadily from ~40% to ~85% of total spend — used by the
// increasing-concentration scenario.
func IncreasingConcentrationSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	bigAmounts := []float64{4000, 5000, 6000, 7000, 8000, 8500}
	id := 1
	for i, p := range periods {
		out = append(out,
			vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-BIG", Period: p, Date: periodMid(p),
				Amount: bigAmounts[i], Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Materials"},
			vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-SMALL1", Period: p, Date: periodMid(p),
				Amount: 3000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
			vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-SMALL2", Period: p, Date: periodMid(p),
				Amount: 3000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
		)
	}
	return out
}

// NewSupplierSuppliers returns two suppliers: one active from the start,
// one that only appears starting in period 4.
func NewSupplierSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-LONGTIME", Name: "Longtime Vendor Co", Active: true},
		{SupplierID: "SUP-NEW", Name: "New Vendor LLC", Active: true},
	}
}

// NewSupplierSpend returns six months of spend where SUP-NEW has zero
// activity in periods 1-3 and material activity starting period 4 — used
// by the new-supplier scenario.
func NewSupplierSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 1
	for i, p := range periods {
		out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-LONGTIME", Period: p, Date: periodMid(p),
			Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual})
		if i >= 3 {
			out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-NEW", Period: p, Date: periodMid(p),
				Amount: 8000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual})
		}
	}
	return out
}

// LostSupplierSuppliers returns two suppliers: one that stops appearing
// after period 3.
func LostSupplierSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-STAYING", Name: "Staying Vendor Co", Active: true},
		{SupplierID: "SUP-LEAVING", Name: "Leaving Vendor LLC", Active: false},
	}
}

// LostSupplierSpend returns six months of spend where SUP-LEAVING has
// material activity in periods 1-3 and zero activity thereafter — used
// by the lost-supplier scenario.
func LostSupplierSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 1
	for i, p := range periods {
		out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-STAYING", Period: p, Date: periodMid(p),
			Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual})
		if i < 3 {
			out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-LEAVING", Period: p, Date: periodMid(p),
				Amount: 9000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual})
		}
	}
	return out
}

// SinglePeriodSuppliers/SinglePeriodSpend supply one period of activity
// for two suppliers — used by the insufficient-history (new/lost
// unavailable) scenario, paired with OnePeriodOnly.
func SinglePeriodSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-ONLY1", Name: "Only Vendor One", Active: true},
		{SupplierID: "SUP-ONLY2", Name: "Only Vendor Two", Active: true},
	}
}

func SinglePeriodSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "SP-1", SupplierID: "SUP-ONLY1", Period: "2025-01", Date: date("2025-01-15"),
			Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "SP-2", SupplierID: "SUP-ONLY2", Period: "2025-01", Date: date("2025-01-20"),
			Amount: 3000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
	}
}

// UnitPriceSuppliers returns one supplier for unit-price scenarios.
func UnitPriceSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-WIDGET", Name: "Widget Supply Co", Active: true},
	}
}

// PriceIncreaseSpend returns two periods of the same product/UOM/quantity
// from SUP-WIDGET with unit price rising from $10 to $12 — used by the
// price-increase scenario (pure price effect, constant volume).
func PriceIncreaseSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "PV-1", SupplierID: "SUP-WIDGET", Period: "2025-01", Date: date("2025-01-15"),
			Amount: 1000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "WIDGET-1", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(10), UnitOfMeasure: "EA"},
		{SpendID: "PV-2", SupplierID: "SUP-WIDGET", Period: "2025-02", Date: date("2025-02-15"),
			Amount: 1200, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "WIDGET-1", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(12), UnitOfMeasure: "EA"},
	}
}

// VolumeIncreaseSpend returns two periods of the same product/UOM/price
// from SUP-WIDGET with quantity rising from 100 to 150 units — used by
// the volume-increase scenario (pure volume effect, constant price).
func VolumeIncreaseSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "PV-3", SupplierID: "SUP-WIDGET", Period: "2025-01", Date: date("2025-01-15"),
			Amount: 1000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "WIDGET-2", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(10), UnitOfMeasure: "EA"},
		{SpendID: "PV-4", SupplierID: "SUP-WIDGET", Period: "2025-02", Date: date("2025-02-15"),
			Amount: 1500, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "WIDGET-2", Quantity: vendorspend.AvailableValue(150), UnitPrice: vendorspend.AvailableValue(10), UnitOfMeasure: "EA"},
	}
}

// CombinedPriceVolumeSpend returns two periods where BOTH price and
// quantity change from SUP-WIDGET — used by the combined price+volume
// scenario (exercises Interaction being nonzero).
func CombinedPriceVolumeSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "PV-5", SupplierID: "SUP-WIDGET", Period: "2025-01", Date: date("2025-01-15"),
			Amount: 1000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "WIDGET-3", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(10), UnitOfMeasure: "EA"},
		{SpendID: "PV-6", SupplierID: "SUP-WIDGET", Period: "2025-02", Date: date("2025-02-15"),
			Amount: 1980, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "WIDGET-3", Quantity: vendorspend.AvailableValue(150), UnitPrice: vendorspend.AvailableValue(13.2), UnitOfMeasure: "EA"},
	}
}

// MultiSupplierSameProductSuppliers returns two suppliers who both sell
// the same ProductID.
func MultiSupplierSameProductSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-PROD-1", Name: "Product Supplier One", Active: true},
		{SupplierID: "SUP-PROD-2", Name: "Product Supplier Two", Active: true},
	}
}

// MultiSupplierSameProductSpend returns spend for the same ProductID/UOM
// from two different suppliers at different unit prices — used by the
// cross-supplier price comparison scenario.
func MultiSupplierSameProductSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "MP-1", SupplierID: "SUP-PROD-1", Period: "2025-01", Date: date("2025-01-10"),
			Amount: 1000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "COMMON-PART", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(10), UnitOfMeasure: "EA"},
		{SpendID: "MP-2", SupplierID: "SUP-PROD-2", Period: "2025-01", Date: date("2025-01-12"),
			Amount: 1300, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "COMMON-PART", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(13), UnitOfMeasure: "EA"},
	}
}

// ObservedSingleSourceSuppliers returns one supplier for the
// observed-single-source scenario.
func ObservedSingleSourceSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-SOLE", Name: "Sole Source Supplier", Active: true},
	}
}

// ObservedSingleSourceSpend returns spend for one ProductID from exactly
// one supplier across the data — used by the observed-single-source
// scenario.
func ObservedSingleSourceSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "SS-1", SupplierID: "SUP-SOLE", Period: "2025-01", Date: date("2025-01-10"),
			Amount: 2000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "SOLE-PART"},
		{SpendID: "SS-2", SupplierID: "SUP-SOLE", Period: "2025-02", Date: date("2025-02-10"),
			Amount: 2000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "SOLE-PART"},
	}
}

// DeclaredRecurringSuppliers returns one supplier for the
// declared-recurring-spend scenario.
func DeclaredRecurringSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-SAAS", Name: "SaaS Vendor Inc", Active: true},
	}
}

// DeclaredRecurringSpend returns six months of identical monthly spend
// explicitly declared RecurrenceRecurring — used by the
// declared-recurring-spend scenario (caller declaration, never
// overwritten).
func DeclaredRecurringSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 1
	for _, p := range periods {
		out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-SAAS", Period: p, Date: periodMid(p),
			Amount: 1500, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			SpendType: vendorspend.SpendTypeSoftware, Recurrence: vendorspend.RecurrenceRecurring, Category: "Software"})
	}
	return out
}

// ObservedRepeatedSpend returns six months of comparable (within
// tolerance) monthly spend from one supplier with NO Recurrence declared
// — used by the observed-repeated-spend scenario (pattern detection
// only, never caller-declared).
func ObservedRepeatedSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	amounts := []float64{2000, 2050, 1980, 2100, 1950, 2020}
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 100
	for i, p := range periods {
		out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-SAAS", Period: p, Date: periodMid(p),
			Amount: amounts[i], Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Software"})
	}
	return out
}

// OneTimeSpend returns a single one-time purchase explicitly declared
// RecurrenceOneTime — used by the one-time-spend scenario.
func OneTimeSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "OT-1", SupplierID: "SUP-SAAS", Period: "2025-03", Date: date("2025-03-15"),
			Amount: 50000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			SpendType: vendorspend.SpendTypeCapex, Recurrence: vendorspend.RecurrenceOneTime, Category: "Equipment"},
	}
}

// CommitmentMixSuppliers returns two suppliers for the committed/
// discretionary mix scenario.
func CommitmentMixSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-LANDLORD", Name: "Landlord LLC", Active: true},
		{SupplierID: "SUP-EVENTS", Name: "Events Co", Active: true},
	}
}

// CommitmentMixSpend returns a mix of COMMITTED (rent) and DISCRETIONARY
// (events/marketing) spend — used by the committed/discretionary mix
// scenario.
func CommitmentMixSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "CM-1", SupplierID: "SUP-LANDLORD", Period: "2025-01", Date: date("2025-01-01"),
			Amount: 10000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			SpendType: vendorspend.SpendTypeRent, Commitment: vendorspend.CommitmentCommitted},
		{SpendID: "CM-2", SupplierID: "SUP-EVENTS", Period: "2025-01", Date: date("2025-01-10"),
			Amount: 4000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			SpendType: vendorspend.SpendTypeMarketing, Commitment: vendorspend.CommitmentDiscretionary},
	}
}

// CategoryTrendSuppliers returns two suppliers for the category-trend
// scenario.
func CategoryTrendSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-CLOUD1", Name: "Cloud Vendor One", Active: true},
		{SupplierID: "SUP-CLOUD2", Name: "Cloud Vendor Two", Active: true},
	}
}

// CategoryTrendSpend returns six months of "Cloud Hosting" category
// spend growing steadily — used by the category-trend scenario.
func CategoryTrendSpend() []vendorspend.SpendRecord {
	var out []vendorspend.SpendRecord
	amounts := []float64{2000, 2500, 3200, 4000, 5000, 6200}
	periods := []string{"2025-01", "2025-02", "2025-03", "2025-04", "2025-05", "2025-06"}
	id := 200
	for i, p := range periods {
		out = append(out, vendorspend.SpendRecord{SpendID: spendID(&id), SupplierID: "SUP-CLOUD1", Period: p, Date: periodMid(p),
			Amount: amounts[i], Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Cloud Hosting"})
	}
	return out
}

// UncategorizedSpendSuppliers returns one supplier for the
// uncategorized-spend scenario.
func UncategorizedSpendSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-MISC", Name: "Misc Vendor", Active: true},
	}
}

// UncategorizedSpend returns a mix of categorized and uncategorized
// records — used by the uncategorized-spend coverage scenario.
func UncategorizedSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "UC-1", SupplierID: "SUP-MISC", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 3000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
		{SpendID: "UC-2", SupplierID: "SUP-MISC", Period: "2025-01", Date: date("2025-01-08"),
			Amount: 7000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
	}
}

// NonPreferredSuppliers returns one preferred and one non-preferred
// supplier — used by the non-preferred-supplier scenario.
func NonPreferredSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-PREFERRED", Name: "Preferred Vendor Co", Active: true, PreferredSupplier: true},
		{SupplierID: "SUP-OTHER", Name: "Other Vendor Co", Active: true},
	}
}

// NonPreferredSpend returns spend split between a preferred and a
// non-preferred supplier.
func NonPreferredSpendRecords() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "NP-1", SupplierID: "SUP-PREFERRED", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 8000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "NP-2", SupplierID: "SUP-OTHER", Period: "2025-01", Date: date("2025-01-06"),
			Amount: 4000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
	}
}

// TailSpendSuppliers returns one large supplier and several small ones —
// used by the tail-spend scenario.
func TailSpendSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-HEAD", Name: "Head Supplier", Active: true},
		{SupplierID: "SUP-TAIL1", Name: "Tail One", Active: true},
		{SupplierID: "SUP-TAIL2", Name: "Tail Two", Active: true},
		{SupplierID: "SUP-TAIL3", Name: "Tail Three", Active: true},
	}
}

// TailSpendRecords returns spend dominated by SUP-HEAD with three small
// tail suppliers below a $1,000 threshold.
func TailSpendRecords() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "TS-1", SupplierID: "SUP-HEAD", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 50000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "TS-2", SupplierID: "SUP-TAIL1", Period: "2025-01", Date: date("2025-01-06"),
			Amount: 500, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "TS-3", SupplierID: "SUP-TAIL2", Period: "2025-01", Date: date("2025-01-07"),
			Amount: 300, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "TS-4", SupplierID: "SUP-TAIL3", Period: "2025-01", Date: date("2025-01-08"),
			Amount: 200, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
	}
}

// DuplicateLikeSuppliers returns one supplier for the duplicate-like
// scenario.
func DuplicateLikeSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-DUP", Name: "Duplicate Test Vendor", Active: true},
	}
}

// DuplicateLikeSpend returns two spend records with the same supplier,
// amount, category, and dates one day apart — used by the
// duplicate-like scenario (an exact same-day/near-day match).
func DuplicateLikeSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "DL-1", SupplierID: "SUP-DUP", Period: "2025-01", Date: date("2025-01-10"),
			Amount: 4500, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
		{SpendID: "DL-2", SupplierID: "SUP-DUP", Period: "2025-01", Date: date("2025-01-11"),
			Amount: 4500, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
	}
}

// NearDuplicateSpend returns two records within the default 3-day window
// but NOT on the same day — used to exercise the adjacent-bucket match
// path (see accounting/vendorspend duplicates.go's bucket+1 lookup).
func NearDuplicateSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "ND-1", SupplierID: "SUP-DUP", Period: "2025-02", Date: date("2025-02-10"),
			Amount: 2200, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
		{SpendID: "ND-2", SupplierID: "SUP-DUP", Period: "2025-02", Date: date("2025-02-12"),
			Amount: 2200, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual, Category: "Supplies"},
	}
}

// VendorCreditSuppliers returns one supplier for the credit/refund
// scenario.
func VendorCreditSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-CREDIT", Name: "Credit Test Vendor", Active: true},
	}
}

// VendorCreditSpend returns one normal purchase and one credit against
// it — used by the vendor-credit/refund scenario (gross/net bridge).
func VendorCreditSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "VC-1", SupplierID: "SUP-CREDIT", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 10000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "VC-2", SupplierID: "SUP-CREDIT", Period: "2025-01", Date: date("2025-01-15"),
			Amount: 1500, Currency: "USD", Effect: vendorspend.EffectCredit, Basis: vendorspend.BasisAccrual},
	}
}

// MixedCurrencySuppliers returns one supplier for the mixed-currency
// scenario.
func MixedCurrencySuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-INTL", Name: "International Vendor", Active: true},
	}
}

// MixedCurrencySpend returns two records in different currencies with no
// Policy.ReportingCurrency resolution supplied by the caller — used by
// the mixed-currency-invalid scenario.
func MixedCurrencySpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "MC-1", SupplierID: "SUP-INTL", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
		{SpendID: "MC-2", SupplierID: "SUP-INTL", Period: "2025-01", Date: date("2025-01-06"),
			Amount: 4000, Currency: "EUR", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
	}
}

// MixedUOMSuppliers returns one supplier for the mixed-UOM scenario.
func MixedUOMSuppliers() []vendorspend.Supplier {
	return []vendorspend.Supplier{
		{SupplierID: "SUP-UOM", Name: "Mixed UOM Vendor", Active: true},
	}
}

// MixedUOMSpend returns two records for the same ProductID but different
// UnitOfMeasure values (EA vs. CASE) — used to verify this package never
// aggregates/compares across incompatible units.
func MixedUOMSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "UOM-1", SupplierID: "SUP-UOM", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 1000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "MULTI-UOM-PART", Quantity: vendorspend.AvailableValue(100), UnitPrice: vendorspend.AvailableValue(10), UnitOfMeasure: "EA"},
		{SpendID: "UOM-2", SupplierID: "SUP-UOM", Period: "2025-01", Date: date("2025-01-06"),
			Amount: 500, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual,
			ProductID: "MULTI-UOM-PART", Quantity: vendorspend.AvailableValue(5), UnitPrice: vendorspend.AvailableValue(100), UnitOfMeasure: "CASE"},
	}
}

// ZeroSpendPeriodPeriods returns two periods where the second has no
// spend activity at all — used by the zero-spend-period scenario.
func ZeroSpendPeriodPeriods() []vendorspend.Period {
	return []vendorspend.Period{
		{Period: "2025-01", StartDate: date("2025-01-01"), EndDate: date("2025-01-31"), SequenceInYear: 1},
		{Period: "2025-02", StartDate: date("2025-02-01"), EndDate: date("2025-02-28"), SequenceInYear: 2},
	}
}

// ZeroSpendPeriodSpend returns spend only in the first of
// ZeroSpendPeriodPeriods' two periods.
func ZeroSpendPeriodSpend() []vendorspend.SpendRecord {
	return []vendorspend.SpendRecord{
		{SpendID: "ZS-1", SupplierID: "SUP-A", Period: "2025-01", Date: date("2025-01-05"),
			Amount: 5000, Currency: "USD", Effect: vendorspend.EffectNormal, Basis: vendorspend.BasisAccrual},
	}
}

// spendID formats *id as "SP-<n>" and increments it — a small counter
// helper for scenario functions generating many records.
func spendID(id *int) string {
	n := *id
	*id++
	return "SP-" + strconv.Itoa(n)
}

// periodMid returns the 15th of the month for a "YYYY-MM" period label —
// a convenience for scenario functions that just need "some date inside
// this period."
func periodMid(period string) time.Time {
	return date(period + "-15")
}
