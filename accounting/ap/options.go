package ap

import (
	"math"
	"time"

	"github.com/themurtez/go-valuate/financial"
)

// CreditNettingPolicy controls whether a supplier vendor credit's negative
// OpenAmount offsets that supplier's other open bills within
// PortfolioSummary/SupplierSummary aggregate totals, or is reported
// separately — see the package doc comment's "vendor credits" section. The
// safest default (NoNetting) never silently nets a credit against
// unrelated bills.
type CreditNettingPolicy string

const (
	// CreditNoNetting (the default) reports vendor credit balances
	// separately via VendorCreditSummary and never subtracts them from
	// bucketed bill totals or SupplierSummary.OpenAmount.
	CreditNoNetting CreditNettingPolicy = "NO_NETTING"
	// CreditNetBySupplier nets each supplier's vendor credit balance
	// against that same supplier's open bill balance for
	// SupplierSummary.OpenAmount and PortfolioSummary totals, while
	// VendorCreditSummary still reports the gross credit balance
	// separately for audit purposes.
	CreditNetBySupplier CreditNettingPolicy = "NET_BY_SUPPLIER"
)

// resolvedCreditNettingPolicy returns p if recognized, otherwise
// CreditNoNetting.
func resolvedCreditNettingPolicy(p CreditNettingPolicy) CreditNettingPolicy {
	if p == CreditNetBySupplier {
		return p
	}
	return CreditNoNetting
}

// DPOBasis labels what Options.DPODenominator represents, since DPO's
// denominator semantics differ meaningfully depending on what a caller
// actually has available — see the task's "never imply purchases were
// known when they were not" instruction.
type DPOBasis string

const (
	// DPOBasisPurchases means the denominator is period purchases — the
	// textbook-correct DPO denominator.
	DPOBasisPurchases DPOBasis = "PURCHASES"
	// DPOBasisCOGS means the denominator is cost of goods sold, used as a
	// proxy for purchases because the caller does not separately track
	// purchases. DPO computed from this basis is labeled accordingly on
	// DPOResult.Basis and is a widely used but weaker approximation.
	DPOBasisCOGS DPOBasis = "COGS"
	// DPOBasisOtherExplicit means the denominator is some other
	// caller-defined figure, explicitly labeled as such rather than
	// silently presented as purchases or COGS.
	DPOBasisOtherExplicit DPOBasis = "OTHER_EXPLICIT"
)

// PayablesPeriod is caller-supplied purchases/COGS data for one period,
// used as DPO's denominator. This package never fabricates purchases or
// COGS from bill data — see the task's explicit prohibition.
type PayablesPeriod struct {
	Period financial.Period `json:"period"`
	// DenominatorAmount is the period's purchases (or COGS, or other
	// explicit basis) — see Basis.
	DenominatorAmount float64 `json:"denominator_amount"`
	// Days is the number of days in Period (e.g. 365/366 for a fiscal
	// year, ~91 for a quarter). Required for the DPO formula.
	Days  int      `json:"days"`
	Basis DPOBasis `json:"basis,omitempty"`
	// EndingAP, when supplied, is the caller-known ending AP balance for
	// this specific historical period — required for DPOHistory. This
	// package never derives a historical ending-AP figure from today's
	// open-item list — see the task's explicit prohibition.
	EndingAP *float64 `json:"ending_ap,omitempty"`
}

// Options configures Calculate's behavior. All fields are optional; the
// zero value resolves to documented safe defaults (see each resolved*
// function).
type Options struct {
	// AsOfDate is the explicit analysis date every aging calculation is
	// measured against. Required — Calculate returns Result{Available:
	// false} with an issue if it is the zero time.Time. Never defaulted
	// to time.Now(); see the package doc comment.
	AsOfDate time.Time `json:"as_of_date"`

	// Basis selects due-date vs bill-date aging. Zero value resolves to
	// AgingByDueDate.
	Basis AgingBasis `json:"basis,omitempty"`

	// Buckets defines the aging bucket schema. Zero value (nil/empty)
	// resolves to DefaultBuckets().
	Buckets []BucketDefinition `json:"buckets,omitempty"`
	// AllowBucketGaps permits gaps between bucket ranges in
	// validateBucketDefinitions (still rejects overlaps). Default false.
	AllowBucketGaps bool `json:"allow_bucket_gaps,omitempty"`

	// IncludeStatuses overrides which PayableStatus values are treated as
	// economically outstanding and included in aging. Zero value (nil)
	// resolves to defaultAgingStatuses() (OPEN, PARTIALLY_PAID, DISPUTED).
	IncludeStatuses []PayableStatus `json:"include_statuses,omitempty"`

	// ReportingCurrency, when non-empty, is the single currency this
	// analysis is expected to be in. Payables in a different currency are
	// excluded and flagged with IssueMixedCurrency rather than summed in.
	// If empty and more than one currency is present among included
	// payables, Calculate flags IssueMixedCurrency and still proceeds
	// using the single most common currency (ties broken by currency code
	// ascending) so a caller gets a usable, clearly-labeled result rather
	// than nothing.
	ReportingCurrency string `json:"reporting_currency,omitempty"`

	// CreditNetting controls whether supplier vendor-credit balances
	// offset that supplier's open bills. Zero value resolves to
	// CreditNoNetting.
	CreditNetting CreditNettingPolicy `json:"credit_netting,omitempty"`

	// ControlAccountBalance, when non-nil, is the caller-supplied GL AP
	// control account balance to reconcile the subledger aging total
	// against — see ControlAccountReconciliation.
	ControlAccountBalance *float64 `json:"control_account_balance,omitempty"`
	// ControlAccountTolerance is the absolute-value tolerance for
	// ControlAccountReconciliation.Reconciled. Zero value resolves to
	// defaultControlAccountTolerance (0.01, i.e. one cent, matching
	// accounting/ar's identical reconciliation tolerance).
	ControlAccountTolerance float64 `json:"control_account_tolerance,omitempty"`

	// MaterialityThreshold is an absolute-dollar threshold used to
	// distinguish a minor overdue item from a material one (used by flags
	// and MaterialityPolicy consumers). Zero value resolves to
	// defaultMaterialityThreshold (0, meaning "materiality by percentage
	// only" — see resolvedMateriality).
	MaterialityThreshold float64 `json:"materiality_threshold,omitempty"`
	// MaterialityPercentOfAP is a fraction of total open AP (e.g. 0.05 for
	// 5%) above which an item/supplier balance is considered material.
	// Zero value resolves to defaultMaterialityPercent (0.05).
	MaterialityPercentOfAP float64 `json:"materiality_percent_of_ap,omitempty"`

	// Thresholds configures deterministic flag trigger points — see
	// flags.go.
	Thresholds Thresholds `json:"thresholds,omitempty"`

	// Dimension, when non-empty, selects one Dimension.Key to produce an
	// optional DimensionSummary breakdown for.
	Dimension string `json:"dimension,omitempty"`

	// DueScheduleHorizons overrides the default future contractual
	// obligations schedule buckets — see schedule.go. Zero value (nil)
	// resolves to DefaultDueScheduleHorizons().
	DueScheduleHorizons []DueScheduleHorizon `json:"due_schedule_horizons,omitempty"`

	// PaymentPressure, when non-nil, supplies optional liquidity inputs
	// (CashAvailable/ExpectedNearTermInflows) used to compute
	// Result.PaymentPressure. If nil, PaymentPressure is unavailable —
	// this package never fabricates a liquidity figure.
	PaymentPressure *PaymentPressureInput `json:"payment_pressure,omitempty"`
	// ExcludeDisputedFromPaymentPressure, when true, excludes disputed
	// payables from the AP-due-soon figures PaymentPressure computes —
	// see section 14's "allow policy to exclude disputes from
	// payment-pressure calculations if useful" instruction. Disputed
	// bills are NEVER excluded from aging itself; this flag only affects
	// PaymentPressure.
	ExcludeDisputedFromPaymentPressure bool `json:"exclude_disputed_from_payment_pressure,omitempty"`
}

const defaultControlAccountTolerance = 0.01
const defaultMaterialityPercent = 0.05

func resolvedControlAccountTolerance(t float64) float64 {
	if t > 0 {
		return t
	}
	return defaultControlAccountTolerance
}

func resolvedMaterialityPercent(p float64) float64 {
	if p > 0 {
		return p
	}
	return defaultMaterialityPercent
}

func resolvedIncludeStatuses(s []PayableStatus) []PayableStatus {
	if len(s) > 0 {
		return s
	}
	return defaultAgingStatuses()
}

func isNonFinite(v float64) bool {
	return math.IsNaN(v) || math.IsInf(v, 0)
}

// Input bundles everything Calculate needs.
type Input struct {
	Payables []Payable `json:"payables"`
	// Payments is optional payment-event history — see SupplierPayment.
	Payments []SupplierPayment `json:"payments,omitempty"`
	// PurchasesHistory is optional caller-supplied purchases/COGS data
	// for DPO — see PayablesPeriod.
	PurchasesHistory []PayablesPeriod `json:"purchases_history,omitempty"`
	// Snapshots is optional historical AP aging snapshots for migration
	// and trend analysis — see Snapshot.
	Snapshots []Snapshot `json:"snapshots,omitempty"`
}

// payableAging holds the per-payable computed aging facts used throughout
// the rest of the package — computed once in Calculate and passed down,
// rather than recomputed in every sub-analysis.
type payableAging struct {
	p             Payable
	daysPastDue   int
	bucketCode    string
	isCredit      bool
	includedInAgg bool // false for excluded currency/invalid rows
}

// ageDays returns AsOfDate - basisDate in whole days, clamped to >= 0: a
// not-yet-due or future-dated item is DaysPastDue = 0, never negative.
func ageDays(asOf, basisDate time.Time) int {
	if basisDate.IsZero() || asOf.Before(basisDate) {
		return 0
	}
	days := int(asOf.Sub(basisDate).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// basisDateFor returns the date agingBasis measures DaysPastDue from.
func basisDateFor(p Payable, basis AgingBasis) time.Time {
	if basis == AgingByBillDate {
		return p.BillDate
	}
	return p.DueDate
}

// amountTolerance is the floating-point comparison tolerance used
// throughout payable-amount validation, matching accounting/ar's
// identical money-comparison tolerance.
const amountTolerance = 0.005
