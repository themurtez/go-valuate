package ar

import (
	"math"
	"time"
)

// CreditNettingPolicy controls whether a customer credit memo's negative
// OpenAmount offsets that customer's other open invoices within
// PortfolioSummary/CustomerSummary aggregate totals, or is reported
// separately — see the package doc comment's "credit memos" section. The
// safest default (NoNetting) never silently nets a credit against
// unrelated invoices.
type CreditNettingPolicy string

const (
	// CreditNoNetting (the default) reports credit memo balances separately
	// via CreditSummary and never subtracts them from bucketed invoice
	// totals or CustomerSummary.OpenAmount.
	CreditNoNetting CreditNettingPolicy = "NO_NETTING"
	// CreditNetByCustomer nets each customer's credit memo balance against
	// that same customer's open invoice balance for CustomerSummary.OpenAmount
	// and PortfolioSummary totals, while CreditSummary still reports the
	// gross credit balance separately for audit purposes.
	CreditNetByCustomer CreditNettingPolicy = "NET_BY_CUSTOMER"
)

// resolvedCreditNettingPolicy returns p if recognized, otherwise
// CreditNoNetting.
func resolvedCreditNettingPolicy(p CreditNettingPolicy) CreditNettingPolicy {
	if p == CreditNetByCustomer {
		return p
	}
	return CreditNoNetting
}

// Options configures Calculate's behavior. All fields are optional; the
// zero value resolves to documented safe defaults (see each resolved*
// function).
type Options struct {
	// AsOfDate is the explicit analysis date every aging calculation is
	// measured against. Required — Calculate returns Result{Available:
	// false} with an issue if it is the zero time.Time. Never defaulted to
	// time.Now(); see the package doc comment.
	AsOfDate time.Time `json:"as_of_date"`

	// Basis selects due-date vs invoice-date aging. Zero value resolves to
	// AgingByDueDate.
	Basis AgingBasis `json:"basis,omitempty"`

	// Buckets defines the aging bucket schema. Zero value (nil/empty)
	// resolves to DefaultBuckets().
	Buckets []BucketDefinition `json:"buckets,omitempty"`
	// AllowBucketGaps permits gaps between bucket ranges in
	// validateBucketDefinitions (still rejects overlaps). Default false.
	AllowBucketGaps bool `json:"allow_bucket_gaps,omitempty"`

	// IncludeStatuses overrides which ReceivableStatus values are treated
	// as economically outstanding and included in aging. Zero value (nil)
	// resolves to defaultAgingStatuses() (OPEN, PARTIALLY_PAID, DISPUTED).
	IncludeStatuses []ReceivableStatus `json:"include_statuses,omitempty"`

	// ReportingCurrency, when non-empty, is the single currency this
	// analysis is expected to be in. Receivables in a different currency
	// are excluded and flagged with IssueMixedCurrency rather than summed
	// in. If empty and more than one currency is present among included
	// receivables, Calculate flags IssueMixedCurrency and still proceeds
	// using the single most common currency (ties broken by currency code
	// ascending) so a caller gets a usable, clearly-labeled result rather
	// than nothing.
	ReportingCurrency string `json:"reporting_currency,omitempty"`

	// CreditNetting controls whether customer credit balances offset that
	// customer's open invoices. Zero value resolves to CreditNoNetting.
	CreditNetting CreditNettingPolicy `json:"credit_netting,omitempty"`

	// ControlAccountBalance, when non-nil, is the caller-supplied GL AR
	// control account balance to reconcile the subledger aging total
	// against — see ControlAccountReconciliation.
	ControlAccountBalance *float64 `json:"control_account_balance,omitempty"`
	// ControlAccountTolerance is the absolute-value tolerance for
	// ControlAccountReconciliation.Reconciled. Zero value resolves to
	// defaultControlAccountTolerance (0.01, i.e. one cent, matching this
	// repository's other reconciliation tolerances such as
	// statements.reconcileTolerance).
	ControlAccountTolerance float64 `json:"control_account_tolerance,omitempty"`

	// MaterialityThreshold is an absolute-dollar threshold used to
	// distinguish a minor overdue item from a material one (used by flags
	// and MaterialityPolicy consumers). Zero value resolves to
	// defaultMaterialityThreshold (0, meaning "materiality by percentage
	// only" — see resolvedMateriality).
	MaterialityThreshold float64 `json:"materiality_threshold,omitempty"`
	// MaterialityPercentOfAR is a fraction of total open AR (e.g. 0.05 for
	// 5%) above which an item/customer balance is considered material.
	// Zero value resolves to defaultMaterialityPercent (0.05).
	MaterialityPercentOfAR float64 `json:"materiality_percent_of_ar,omitempty"`

	// Thresholds configures deterministic flag trigger points — see
	// flags.go.
	Thresholds Thresholds `json:"thresholds,omitempty"`

	// Dimension, when non-empty, selects one Dimension.Key to produce an
	// optional DimensionSummary breakdown for.
	Dimension string `json:"dimension,omitempty"`

	// PriorityWeights overrides CollectionPriority's default formula
	// weights — see PriorityWeights. Zero value resolves to
	// DefaultPriorityWeights().
	PriorityWeights PriorityWeights `json:"priority_weights,omitempty"`
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

func resolvedIncludeStatuses(s []ReceivableStatus) []ReceivableStatus {
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
	Receivables []Receivable `json:"receivables"`
	// Payments is optional payment-event history — see Payment.
	Payments []Payment `json:"payments,omitempty"`
	// SalesHistory is optional caller-supplied sales data for DSO — see
	// SalesPeriod.
	SalesHistory []SalesPeriod `json:"sales_history,omitempty"`
	// Snapshots is optional historical AR aging snapshots for migration
	// and trend analysis — see Snapshot.
	Snapshots []Snapshot `json:"snapshots,omitempty"`
	// WriteOffs is optional historical write-off records — see WriteOff.
	WriteOffs []WriteOff `json:"write_offs,omitempty"`
}

// receivableAging holds the per-receivable computed aging facts used
// throughout the rest of the package — computed once in Calculate and
// passed down, rather than recomputed in every sub-analysis.
type receivableAging struct {
	r             Receivable
	daysPastDue   int
	bucketCode    string
	isCredit      bool
	includedInAgg bool // false for excluded currency/invalid rows
}

// ageDays returns AsOfDate - basisDate in whole days, clamped to >= 0 per
// the task's future-dated-item rule (section 33): a not-yet-due or
// future-dated item is DaysPastDue = 0, never negative.
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
func basisDateFor(r Receivable, basis AgingBasis) time.Time {
	if basis == AgingByInvoiceDate {
		return r.InvoiceDate
	}
	return r.DueDate
}
