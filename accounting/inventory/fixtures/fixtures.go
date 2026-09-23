// Package fixtures provides synthetic accounting/inventory data for
// tests and examples: a set of small, hand-built Item/InventorySnapshot/
// Movement/PeriodFinancials scenarios covering the situations
// accounting/inventory's own tests and a caller's own tests can both
// reuse. Nothing here is real inventory or supplier data — every figure
// is invented for illustration, mirroring accounting/ar/fixtures,
// accounting/ap/fixtures, and accounting/labor/fixtures' identical
// synthetic-data convention (task section 77: "no proprietary data").
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/inventory"
)

// AsOfDate is the standard as-of date most fixtures below are built
// against.
const AsOfDate = "2025-06-30"

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func datePtr(s string) *time.Time {
	t := date(s)
	return &t
}

func qty(v float64) inventory.Qty   { return inventory.AvailableQty(v, "EA") }
func val(v float64) inventory.Value { return inventory.AvailableValue(v) }

// -----------------------------------------------------------------------
// Healthy / baseline scenarios
// -----------------------------------------------------------------------

// HealthyRetailer returns a small, well-behaved retail inventory: 3
// items, all recently received, moving steadily, no anomalies.
func HealthyRetailer() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.Movement) {
	items := []inventory.Item{
		{ID: "SKU-100", SKU: "SKU-100", Name: "T-Shirt", Category: "Apparel", Class: inventory.ClassMerchandise, Active: true, Currency: "USD", UnitOfMeasure: "EA", Location: "STORE-1"},
		{ID: "SKU-200", SKU: "SKU-200", Name: "Jeans", Category: "Apparel", Class: inventory.ClassMerchandise, Active: true, Currency: "USD", UnitOfMeasure: "EA", Location: "STORE-1"},
		{ID: "SKU-300", SKU: "SKU-300", Name: "Sneakers", Category: "Footwear", Class: inventory.ClassMerchandise, Active: true, Currency: "USD", UnitOfMeasure: "EA", Location: "STORE-1"},
	}
	received := datePtr("2025-06-10")
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-100", ItemID: "SKU-100", AsOfDate: date(AsOfDate), Location: "STORE-1", ReceivedDate: received, QuantityOnHand: qty(200), UnitCost: val(8), Currency: "USD"},
		{ID: "SN-200", ItemID: "SKU-200", AsOfDate: date(AsOfDate), Location: "STORE-1", ReceivedDate: received, QuantityOnHand: qty(150), UnitCost: val(20), Currency: "USD"},
		{ID: "SN-300", ItemID: "SKU-300", AsOfDate: date(AsOfDate), Location: "STORE-1", ReceivedDate: received, QuantityOnHand: qty(80), UnitCost: val(35), Currency: "USD"},
	}
	movements := []inventory.Movement{
		{ID: "M-1", ItemID: "SKU-100", Date: date("2025-06-10"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(250), UnitCost: val(8), Currency: "USD"},
		{ID: "M-2", ItemID: "SKU-100", Date: date("2025-06-20"), Type: inventory.MovementCustomerShipment, Quantity: qty(50), UnitCost: val(8), Currency: "USD"},
		{ID: "M-3", ItemID: "SKU-200", Date: date("2025-06-10"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(180), UnitCost: val(20), Currency: "USD"},
		{ID: "M-4", ItemID: "SKU-200", Date: date("2025-06-22"), Type: inventory.MovementCustomerShipment, Quantity: qty(30), UnitCost: val(20), Currency: "USD"},
		{ID: "M-5", ItemID: "SKU-300", Date: date("2025-06-10"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(100), UnitCost: val(35), Currency: "USD"},
		{ID: "M-6", ItemID: "SKU-300", Date: date("2025-06-25"), Type: inventory.MovementCustomerShipment, Quantity: qty(20), UnitCost: val(35), Currency: "USD"},
	}
	return items, snapshots, movements
}

// FastTurningInventory returns a single item that turns over quickly:
// high COGS relative to average inventory.
func FastTurningInventory() []inventory.PeriodFinancials {
	return []inventory.PeriodFinancials{
		{Period: "2025-06", BeginningInventoryValue: val(20000), EndingInventoryValue: val(22000), COGS: val(180000), Currency: "USD"},
	}
}

// DecliningInventoryWithStrongCOGS returns two periods where inventory
// value falls while COGS remains strong — a positive trend signal (DIO
// improving) that should never trigger an inventory-build flag.
func DecliningInventoryWithStrongCOGS() []inventory.PeriodFinancials {
	return []inventory.PeriodFinancials{
		{Period: "2025-Q1", BeginningInventoryValue: val(500000), EndingInventoryValue: val(400000), COGS: val(1000000), Currency: "USD"},
		{Period: "2025-Q2", BeginningInventoryValue: val(400000), EndingInventoryValue: val(300000), COGS: val(1100000), Currency: "USD"},
	}
}

// -----------------------------------------------------------------------
// Slow/non-moving/aged inventory scenarios
// -----------------------------------------------------------------------

// SlowMovingInventory returns one item with no outbound movement for 120
// days as of AsOfDate.
func SlowMovingInventory() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.Movement) {
	items := []inventory.Item{{ID: "SKU-SLOW", Category: "Discontinued", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-SLOW", ItemID: "SKU-SLOW", AsOfDate: date(AsOfDate), QuantityOnHand: qty(500), UnitCost: val(10), Currency: "USD"},
	}
	movements := []inventory.Movement{
		{ID: "M-SLOW-1", ItemID: "SKU-SLOW", Date: date("2025-01-01"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(500), UnitCost: val(10), Currency: "USD"},
		{ID: "M-SLOW-2", ItemID: "SKU-SLOW", Date: date("2025-03-02"), Type: inventory.MovementCustomerShipment, Quantity: qty(5), UnitCost: val(10), Currency: "USD"}, // last outbound: 120 days before AsOfDate.
	}
	return items, snapshots, movements
}

// NonMovingInventory returns one item with no outbound movement for 250
// days — beyond a typical non-moving threshold.
func NonMovingInventory() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.Movement) {
	items := []inventory.Item{{ID: "SKU-DEAD", Category: "Obsolete", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-DEAD", ItemID: "SKU-DEAD", AsOfDate: date(AsOfDate), QuantityOnHand: qty(300), UnitCost: val(15), Currency: "USD"},
	}
	movements := []inventory.Movement{
		{ID: "M-DEAD-1", ItemID: "SKU-DEAD", Date: date("2024-10-01"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(300), UnitCost: val(15), Currency: "USD"},
		{ID: "M-DEAD-2", ItemID: "SKU-DEAD", Date: date("2024-10-23"), Type: inventory.MovementCustomerShipment, Quantity: qty(2), UnitCost: val(15), Currency: "USD"}, // last outbound: 250 days before AsOfDate.
	}
	return items, snapshots, movements
}

// AgedInventory returns one item received long before AsOfDate, no
// movements at all otherwise.
func AgedInventory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-AGED", Category: "Legacy", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	received := datePtr("2024-01-15")
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-AGED", ItemID: "SKU-AGED", AsOfDate: date(AsOfDate), ReceivedDate: received, QuantityOnHand: qty(50), UnitCost: val(40), Currency: "USD"},
	}
	return items, snapshots
}

// UnknownAgeInventory returns one item with no ReceivedDate, no
// PURCHASE_RECEIPT history, and no movement history at all — age must
// resolve to UNKNOWN.
func UnknownAgeInventory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-UNK", Category: "Imported Opening Balance", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-UNK", ItemID: "SKU-UNK", AsOfDate: date(AsOfDate), QuantityOnHand: qty(75), UnitCost: val(12), Currency: "USD"},
	}
	return items, snapshots
}

// -----------------------------------------------------------------------
// Purchases / build signals
// -----------------------------------------------------------------------

// InventoryBuild returns two periods where ending inventory rises
// materially while COGS is flat — task section 33's neutral finding.
func InventoryBuild() []inventory.PeriodFinancials {
	return []inventory.PeriodFinancials{
		{Period: "2025-Q1", BeginningInventoryValue: val(100000), EndingInventoryValue: val(105000), COGS: val(300000), Currency: "USD"},
		{Period: "2025-Q2", BeginningInventoryValue: val(105000), EndingInventoryValue: val(180000), COGS: val(295000), Currency: "USD"},
	}
}

// PurchasesOutpacingUsage returns one period where purchase receipts far
// exceed outbound usage value.
func PurchasesOutpacingUsage() []inventory.Movement {
	return []inventory.Movement{
		{ID: "M-PU-1", ItemID: "SKU-PU", Date: date("2025-06-05"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(1000), UnitCost: val(10), Currency: "USD"},
		{ID: "M-PU-2", ItemID: "SKU-PU", Date: date("2025-06-20"), Type: inventory.MovementCustomerShipment, Quantity: qty(100), UnitCost: val(10), Currency: "USD"},
	}
}

// -----------------------------------------------------------------------
// Negative inventory
// -----------------------------------------------------------------------

// NegativeInventory returns one item with a negative on-hand quantity —
// a timing/system artifact, not auto-corrected.
func NegativeInventory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-NEG", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-NEG", ItemID: "SKU-NEG", AsOfDate: date(AsOfDate), QuantityOnHand: qty(-15), UnitCost: val(5), Currency: "USD"},
	}
	return items, snapshots
}

// -----------------------------------------------------------------------
// Multi-category / multi-location / mixed UOM / composition
// -----------------------------------------------------------------------

// MultiCategory returns items spread across three categories with
// distinct concentration.
func MultiCategory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{
		{ID: "SKU-A", Category: "Electronics", Active: true, Currency: "USD", UnitOfMeasure: "EA"},
		{ID: "SKU-B", Category: "Furniture", Active: true, Currency: "USD", UnitOfMeasure: "EA"},
		{ID: "SKU-C", Category: "Office Supplies", Active: true, Currency: "USD", UnitOfMeasure: "EA"},
	}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-A", ItemID: "SKU-A", AsOfDate: date(AsOfDate), QuantityOnHand: qty(50), UnitCost: val(200), Currency: "USD"},
		{ID: "SN-B", ItemID: "SKU-B", AsOfDate: date(AsOfDate), QuantityOnHand: qty(20), UnitCost: val(150), Currency: "USD"},
		{ID: "SN-C", ItemID: "SKU-C", AsOfDate: date(AsOfDate), QuantityOnHand: qty(500), UnitCost: val(2), Currency: "USD"},
	}
	return items, snapshots
}

// MultiLocation returns one item split across two warehouses.
func MultiLocation() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-ML", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-ML-1", ItemID: "SKU-ML", AsOfDate: date(AsOfDate), Location: "WH-EAST", QuantityOnHand: qty(300), UnitCost: val(5), Currency: "USD"},
		{ID: "SN-ML-2", ItemID: "SKU-ML", AsOfDate: date(AsOfDate), Location: "WH-WEST", QuantityOnHand: qty(100), UnitCost: val(5), Currency: "USD"},
	}
	return items, snapshots
}

// MixedUOM returns one item with snapshots in two incompatible units of
// measure and no UOMConversion supplied — quantity aggregation is
// expected to be unavailable, value still aggregates.
func MixedUOM() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-MIXED", Category: "Chemicals", Active: true, Currency: "USD", UnitOfMeasure: "KG"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-MIXED-1", ItemID: "SKU-MIXED", AsOfDate: date(AsOfDate), Location: "TANK-1", QuantityOnHand: inventory.AvailableQty(500, "KG"), UnitCost: val(3), Currency: "USD"},
		{ID: "SN-MIXED-2", ItemID: "SKU-MIXED", AsOfDate: date(AsOfDate), Location: "TANK-2", QuantityOnHand: inventory.AvailableQty(200, "L"), UnitCost: val(4), Currency: "USD"},
	}
	return items, snapshots
}

// RawMaterialWIPFinishedGoods returns one item per manufacturing stage —
// task section 46's composition breakdown.
func RawMaterialWIPFinishedGoods() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{
		{ID: "SKU-RM", Category: "Steel", Class: inventory.ClassRawMaterial, Active: true, Currency: "USD", UnitOfMeasure: "KG"},
		{ID: "SKU-WIP", Category: "Partial Assembly", Class: inventory.ClassWIP, Active: true, Currency: "USD", UnitOfMeasure: "EA"},
		{ID: "SKU-FG", Category: "Finished Widgets", Class: inventory.ClassFinishedGood, Active: true, Currency: "USD", UnitOfMeasure: "EA"},
	}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-RM", ItemID: "SKU-RM", AsOfDate: date(AsOfDate), QuantityOnHand: inventory.AvailableQty(2000, "KG"), UnitCost: val(3), Currency: "USD"},
		{ID: "SN-WIP", ItemID: "SKU-WIP", AsOfDate: date(AsOfDate), QuantityOnHand: qty(150), UnitCost: val(20), Currency: "USD"},
		{ID: "SN-FG", ItemID: "SKU-FG", AsOfDate: date(AsOfDate), QuantityOnHand: qty(400), UnitCost: val(45), Currency: "USD"},
	}
	return items, snapshots
}

// -----------------------------------------------------------------------
// Stock-policy scenarios
// -----------------------------------------------------------------------

// StockPolicyBelowMinimum returns one item below its caller-supplied
// minimum.
func StockPolicyBelowMinimum() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.StockPolicy) {
	items := []inventory.Item{{ID: "SKU-LOW", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-LOW", ItemID: "SKU-LOW", AsOfDate: date(AsOfDate), QuantityOnHand: qty(5), UnitCost: val(10), Currency: "USD"},
	}
	policies := []inventory.StockPolicy{{ItemID: "SKU-LOW", MinimumQuantitySet: true, MinimumQuantity: 25}}
	return items, snapshots, policies
}

// StockPolicyAboveMaximum returns one item above its caller-supplied
// maximum.
func StockPolicyAboveMaximum() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.StockPolicy) {
	items := []inventory.Item{{ID: "SKU-HIGH", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-HIGH", ItemID: "SKU-HIGH", AsOfDate: date(AsOfDate), QuantityOnHand: qty(500), UnitCost: val(10), Currency: "USD"},
	}
	policies := []inventory.StockPolicy{{ItemID: "SKU-HIGH", MaximumQuantitySet: true, MaximumQuantity: 200}}
	return items, snapshots, policies
}

// -----------------------------------------------------------------------
// Adjustments / write-offs
// -----------------------------------------------------------------------

// LargeAdjustment returns a single, large-magnitude adjustment decrease.
func LargeAdjustment() []inventory.Movement {
	return []inventory.Movement{
		{ID: "M-ADJ-LARGE", ItemID: "SKU-ADJ", Date: date("2025-06-15"), Type: inventory.MovementAdjustmentDecrease, Quantity: qty(1000), UnitCost: val(25), Currency: "USD"},
	}
}

// RepeatedAdjustments returns several smaller adjustments against the
// same item across a period.
func RepeatedAdjustments() []inventory.Movement {
	return []inventory.Movement{
		{ID: "M-REP-1", ItemID: "SKU-REP", Date: date("2025-06-05"), Type: inventory.MovementAdjustmentDecrease, Quantity: qty(5), UnitCost: val(10), Currency: "USD"},
		{ID: "M-REP-2", ItemID: "SKU-REP", Date: date("2025-06-12"), Type: inventory.MovementAdjustmentDecrease, Quantity: qty(3), UnitCost: val(10), Currency: "USD"},
		{ID: "M-REP-3", ItemID: "SKU-REP", Date: date("2025-06-19"), Type: inventory.MovementAdjustmentIncrease, Quantity: qty(2), UnitCost: val(10), Currency: "USD"},
		{ID: "M-REP-4", ItemID: "SKU-REP", Date: date("2025-06-26"), Type: inventory.MovementAdjustmentDecrease, Quantity: qty(4), UnitCost: val(10), Currency: "USD"},
	}
}

// WriteOff returns one write-off movement.
func WriteOff() []inventory.Movement {
	return []inventory.Movement{
		{ID: "M-WO-1", ItemID: "SKU-WO", Date: date("2025-06-18"), Type: inventory.MovementWriteOff, Quantity: qty(200), UnitCost: val(8), Currency: "USD"},
	}
}

// PeriodEndAdjustment returns an adjustment dated within the last few
// days of a standard June period.
func PeriodEndAdjustment() []inventory.Movement {
	return []inventory.Movement{
		{ID: "M-PE-1", ItemID: "SKU-PE", Date: date("2025-06-29"), Type: inventory.MovementAdjustmentDecrease, Quantity: qty(50), UnitCost: val(6), Currency: "USD"},
	}
}

// -----------------------------------------------------------------------
// Reconciliation scenarios
// -----------------------------------------------------------------------

// GLReconciliationExact returns items/snapshots plus a GLControl that
// exactly matches the resulting subledger total.
func GLReconciliationExact() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.GLControl) {
	items := []inventory.Item{{ID: "SKU-GL", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-GL", ItemID: "SKU-GL", AsOfDate: date(AsOfDate), QuantityOnHand: qty(100), UnitCost: val(10), Currency: "USD"},
	}
	controls := []inventory.GLControl{{AsOfDate: AsOfDate, Balance: 1000}}
	return items, snapshots, controls
}

// GLReconciliationMismatch is GLReconciliationExact with a GL balance
// that does not tie.
func GLReconciliationMismatch() ([]inventory.Item, []inventory.InventorySnapshot, []inventory.GLControl) {
	items, snapshots, _ := GLReconciliationExact()
	controls := []inventory.GLControl{{AsOfDate: AsOfDate, Balance: 1500}}
	return items, snapshots, controls
}

// QuantityRollforwardExact returns beginning/ending snapshots and
// movements that tie exactly.
func QuantityRollforwardExact() (beginning, ending inventory.InventorySnapshot, movements []inventory.Movement) {
	beginning = inventory.InventorySnapshot{ID: "SN-RF-BEG", ItemID: "SKU-RF", AsOfDate: date("2025-05-31"), QuantityOnHand: qty(100), UnitCost: val(5), Currency: "USD"}
	ending = inventory.InventorySnapshot{ID: "SN-RF-END", ItemID: "SKU-RF", AsOfDate: date("2025-06-30"), QuantityOnHand: qty(130), UnitCost: val(5), Currency: "USD"}
	movements = []inventory.Movement{
		{ID: "M-RF-1", ItemID: "SKU-RF", Date: date("2025-06-10"), Type: inventory.MovementPurchaseReceipt, Quantity: qty(50), UnitCost: val(5), Currency: "USD"},
		{ID: "M-RF-2", ItemID: "SKU-RF", Date: date("2025-06-20"), Type: inventory.MovementCustomerShipment, Quantity: qty(20), UnitCost: val(5), Currency: "USD"},
	}
	return
}

// QuantityRollforwardMismatch is QuantityRollforwardExact with an ending
// balance that does not tie to the same movements.
func QuantityRollforwardMismatch() (beginning, ending inventory.InventorySnapshot, movements []inventory.Movement) {
	beginning, ending, movements = QuantityRollforwardExact()
	ending.QuantityOnHand = qty(999) // no longer ties.
	return
}

// ValueRollforwardExact mirrors QuantityRollforwardExact for value.
func ValueRollforwardExact() (beginning, ending inventory.InventorySnapshot, movements []inventory.Movement) {
	return QuantityRollforwardExact() // uniform $5 unit cost -> value ties whenever quantity ties.
}

// ValueRollforwardMismatch returns an ending snapshot whose cost basis
// shifted, breaking the value tie even though quantity still ties.
func ValueRollforwardMismatch() (beginning, ending inventory.InventorySnapshot, movements []inventory.Movement) {
	beginning, ending, movements = QuantityRollforwardExact()
	ending.QuantityOnHand = qty(130) // quantity still ties...
	ending.UnitCost = val(8)         // ...but cost basis shifted, so value does not.
	return
}

// -----------------------------------------------------------------------
// Expiry
// -----------------------------------------------------------------------

// ExpiringInventory returns one lot expiring within a typical warning
// window.
func ExpiringInventory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-EXP", Category: "Perishables", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	expiry := datePtr("2025-07-10")
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-EXP", ItemID: "SKU-EXP", AsOfDate: date(AsOfDate), ExpiryDate: expiry, QuantityOnHand: qty(60), UnitCost: val(4), Currency: "USD"},
	}
	return items, snapshots
}

// ExpiredInventory returns one lot already past its expiry date as of
// AsOfDate.
func ExpiredInventory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-DEAD-EXP", Category: "Perishables", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	expiry := datePtr("2025-05-01")
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-DEAD-EXP", ItemID: "SKU-DEAD-EXP", AsOfDate: date(AsOfDate), ExpiryDate: expiry, QuantityOnHand: qty(40), UnitCost: val(6), Currency: "USD"},
	}
	return items, snapshots
}

// -----------------------------------------------------------------------
// Currency / zero-value scenarios
// -----------------------------------------------------------------------

// MixedCurrencyInvalid returns two items in different currencies with no
// explicit ReportingCurrency resolution supplied.
func MixedCurrencyInvalid() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{
		{ID: "SKU-USD", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"},
		{ID: "SKU-EUR", Category: "Widgets", Active: true, Currency: "EUR", UnitOfMeasure: "EA"},
	}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-USD", ItemID: "SKU-USD", AsOfDate: date(AsOfDate), QuantityOnHand: qty(10), UnitCost: val(5), Currency: "USD"},
		{ID: "SN-EUR", ItemID: "SKU-EUR", AsOfDate: date(AsOfDate), QuantityOnHand: qty(10), UnitCost: val(5), Currency: "EUR"},
	}
	return items, snapshots
}

// ZeroInventory returns one item with exactly zero on-hand quantity/value
// — a known fact, not "unavailable."
func ZeroInventory() ([]inventory.Item, []inventory.InventorySnapshot) {
	items := []inventory.Item{{ID: "SKU-ZERO", Category: "Widgets", Active: true, Currency: "USD", UnitOfMeasure: "EA"}}
	snapshots := []inventory.InventorySnapshot{
		{ID: "SN-ZERO", ItemID: "SKU-ZERO", AsOfDate: date(AsOfDate), QuantityOnHand: qty(0), UnitCost: val(10), Currency: "USD"},
	}
	return items, snapshots
}

// ZeroCOGS returns one period with zero COGS — DIO must be Unavailable,
// never Inf.
func ZeroCOGS() []inventory.PeriodFinancials {
	return []inventory.PeriodFinancials{
		{Period: "2025-06", BeginningInventoryValue: val(50000), EndingInventoryValue: val(50000), COGS: val(0), Currency: "USD"},
	}
}

// -----------------------------------------------------------------------
// Historical trend
// -----------------------------------------------------------------------

// HistoricalMultiPeriodTrend returns four chronological quarterly periods
// with gradually improving turnover.
func HistoricalMultiPeriodTrend() []inventory.PeriodFinancials {
	return []inventory.PeriodFinancials{
		{Period: "2024-Q3", BeginningInventoryValue: val(300000), EndingInventoryValue: val(300000), COGS: val(600000), Currency: "USD"},
		{Period: "2024-Q4", BeginningInventoryValue: val(300000), EndingInventoryValue: val(280000), COGS: val(650000), Currency: "USD"},
		{Period: "2025-Q1", BeginningInventoryValue: val(280000), EndingInventoryValue: val(250000), COGS: val(700000), Currency: "USD"},
		{Period: "2025-Q2", BeginningInventoryValue: val(250000), EndingInventoryValue: val(220000), COGS: val(750000), Currency: "USD"},
	}
}

// FourQuarterPeriods returns the PeriodInfo series matching
// HistoricalMultiPeriodTrend, with explicit equal Days so the trend is
// driven purely by the underlying figures, not calendar-length noise.
func FourQuarterPeriods() []inventory.PeriodInfo {
	return []inventory.PeriodInfo{
		{Period: "2024-Q3", StartDate: date("2024-07-01"), EndDate: date("2024-09-30"), Days: 90},
		{Period: "2024-Q4", StartDate: date("2024-10-01"), EndDate: date("2024-12-31"), Days: 90},
		{Period: "2025-Q1", StartDate: date("2025-01-01"), EndDate: date("2025-03-31"), Days: 90},
		{Period: "2025-Q2", StartDate: date("2025-04-01"), EndDate: date("2025-06-30"), Days: 90},
	}
}
