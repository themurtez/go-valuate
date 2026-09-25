package advisory

// Stable ActionItem.ActionCode values — task section 33's "stable action
// codes required" rule. This is the CLOSED set of GENERATED action codes
// this package ever produces; every one has exactly one actionTemplate
// entry below (see actionTemplateByCode). A caller wanting a different
// action supplies its own via Input's caller-action mechanism
// (ActionOriginCallerSupplied) rather than this package inventing a new
// code — task section 38.
const (
	ActionReviewMinimumCash                = "REVIEW_MINIMUM_CASH"
	ActionReviewFundingGap                 = "REVIEW_FUNDING_GAP"
	ActionReviewFacilityCapacity           = "REVIEW_FACILITY_CAPACITY"
	ActionReviewNearTermCashOutflows       = "REVIEW_NEAR_TERM_CASH_OUTFLOWS"
	ActionReviewOverdueAR                  = "REVIEW_OVERDUE_AR"
	ActionReviewARConcentration            = "REVIEW_AR_CONCENTRATION"
	ActionReviewARControlReconciliation    = "REVIEW_AR_CONTROL_RECONCILIATION"
	ActionReviewNearTermAP                 = "REVIEW_NEAR_TERM_AP"
	ActionReviewOverdueAP                  = "REVIEW_OVERDUE_AP"
	ActionReviewAPControlReconciliation    = "REVIEW_AP_CONTROL_RECONCILIATION"
	ActionReviewSlowMovingInventory        = "REVIEW_SLOW_MOVING_INVENTORY"
	ActionReviewInventoryControlDifference = "REVIEW_INVENTORY_CONTROL_DIFFERENCE"
	ActionReviewStockPolicyException       = "REVIEW_STOCK_POLICY_EXCEPTION"
	ActionReviewOvertimeTrend              = "REVIEW_OVERTIME_TREND"
	ActionReviewLaborCostChange            = "REVIEW_LABOR_COST_CHANGE"
	ActionReviewPayrollControlDifference   = "REVIEW_PAYROLL_CONTROL_DIFFERENCE"
	ActionReviewNegativeContributionEntity = "REVIEW_NEGATIVE_CONTRIBUTION_ENTITY"
	ActionReviewMarginChange               = "REVIEW_MARGIN_CHANGE"
	ActionReviewUnallocatedSharedCost      = "REVIEW_UNALLOCATED_SHARED_COST"
	ActionReviewSupplierConcentration      = "REVIEW_SUPPLIER_CONCENTRATION"
	ActionReviewUnitPriceChange            = "REVIEW_UNIT_PRICE_CHANGE"
	ActionReviewNonPreferredSupplierSpend  = "REVIEW_NON_PREFERRED_SUPPLIER_SPEND"
	ActionReviewUncategorizedVendorSpend   = "REVIEW_UNCATEGORIZED_VENDOR_SPEND"
	ActionReviewCovenantHeadroom           = "REVIEW_COVENANT_HEADROOM"
	ActionReviewDebtServiceRequirement     = "REVIEW_DEBT_SERVICE_REQUIREMENT"
	ActionReviewCovenantFailure            = "REVIEW_COVENANT_FAILURE"
	ActionResolveReconciliationBlocker     = "RESOLVE_RECONCILIATION_BLOCKER"
	ActionResolveCloseQualityBlocker       = "RESOLVE_CLOSE_QUALITY_BLOCKER"
	ActionCompleteRequiredCloseTask        = "COMPLETE_REQUIRED_CLOSE_TASK"
	ActionObtainRequiredSignoff            = "OBTAIN_REQUIRED_SIGNOFF"
	ActionAttachRequiredEvidence           = "ATTACH_REQUIRED_EVIDENCE"
	ActionReviewValueDriverChange          = "REVIEW_VALUE_DRIVER_CHANGE"
	ActionReviewValuationSensitivity       = "REVIEW_VALUATION_SENSITIVITY"
	ActionReviewJournalEntryFinding        = "REVIEW_JOURNAL_ENTRY_FINDING"
	ActionReviewSourceConflict             = "REVIEW_SOURCE_CONFLICT"
	ActionReviewCustomerRetentionChange    = "REVIEW_CUSTOMER_RETENTION_CHANGE"
)

// actionTemplates is the closed set of GENERATED action templates — task
// section 33/107. Every Title/Description uses only
// review/resolve/validate/investigate/confirm/complete-framed language;
// see TestNoPrescriptiveLanguage (safety_test.go) for the permanent
// regression guard.
var actionTemplates = map[string]actionTemplate{
	ActionReviewMinimumCash: {
		Code: ActionReviewMinimumCash, Category: string(SectionLiquidity),
		Title:       "Review minimum projected cash balance",
		Description: "The 13-week cash forecast's minimum projected balance is at or below the configured threshold. Review the underlying assumptions and timing.",
	},
	ActionReviewFundingGap: {
		Code: ActionReviewFundingGap, Category: string(SectionLiquidity),
		Title:       "Review projected funding gap",
		Description: "The 13-week cash forecast projects a funding gap. Review the required-funding calculation and the weeks it applies to.",
	},
	ActionReviewFacilityCapacity: {
		Code: ActionReviewFacilityCapacity, Category: string(SectionLiquidity),
		Title:       "Review available credit facility capacity",
		Description: "Review available credit facility capacity against the projected funding gap.",
	},
	ActionReviewNearTermCashOutflows: {
		Code: ActionReviewNearTermCashOutflows, Category: string(SectionLiquidity),
		Title:       "Review near-term cash outflows",
		Description: "Review the near-term cash outflow schedule contributing to the projected cash position.",
	},
	ActionReviewOverdueAR: {
		Code: ActionReviewOverdueAR, Category: string(SectionWorkingCapital),
		Title:       "Review AR balances over 90 days",
		Description: "Review accounts receivable balances aged over 90 days for collectability and status.",
	},
	ActionReviewARConcentration: {
		Code: ActionReviewARConcentration, Category: string(SectionWorkingCapital),
		Title:       "Review AR customer concentration",
		Description: "Review the concentration of accounts receivable balances among the largest customers.",
	},
	ActionReviewARControlReconciliation: {
		Code: ActionReviewARControlReconciliation, Category: string(SectionWorkingCapital),
		Title:       "Review AR control account reconciliation",
		Description: "Review the difference between the accounts-receivable subledger and control account balance.",
		Blocking:    true,
	},
	ActionReviewNearTermAP: {
		Code: ActionReviewNearTermAP, Category: string(SectionWorkingCapital),
		Title:       "Review near-term AP due amounts",
		Description: "Review accounts payable amounts due in the near-term payment-pressure window.",
	},
	ActionReviewOverdueAP: {
		Code: ActionReviewOverdueAP, Category: string(SectionWorkingCapital),
		Title:       "Review overdue AP balances",
		Description: "Review accounts payable balances past due for status and terms.",
	},
	ActionReviewAPControlReconciliation: {
		Code: ActionReviewAPControlReconciliation, Category: string(SectionWorkingCapital),
		Title:       "Review AP control account reconciliation",
		Description: "Review the difference between the accounts-payable subledger and control account balance.",
		Blocking:    true,
	},
	ActionReviewSlowMovingInventory: {
		Code: ActionReviewSlowMovingInventory, Category: string(SectionInventory),
		Title:       "Review slow-moving inventory",
		Description: "Review inventory items classified as slow-moving or non-moving.",
	},
	ActionReviewInventoryControlDifference: {
		Code: ActionReviewInventoryControlDifference, Category: string(SectionInventory),
		Title:       "Review inventory control account difference",
		Description: "Review the difference between the inventory subledger and general-ledger control balance.",
		Blocking:    true,
	},
	ActionReviewStockPolicyException: {
		Code: ActionReviewStockPolicyException, Category: string(SectionInventory),
		Title:       "Review stock-policy exception",
		Description: "Review inventory items outside their configured minimum/maximum/reorder-point policy.",
	},
	ActionReviewOvertimeTrend: {
		Code: ActionReviewOvertimeTrend, Category: string(SectionLabor),
		Title:       "Review overtime trend",
		Description: "Review the trend in overtime hours or overtime share of labor cost.",
	},
	ActionReviewLaborCostChange: {
		Code: ActionReviewLaborCostChange, Category: string(SectionLabor),
		Title:       "Review labor cost change",
		Description: "Review the change in labor cost relative to revenue.",
	},
	ActionReviewPayrollControlDifference: {
		Code: ActionReviewPayrollControlDifference, Category: string(SectionLabor),
		Title:       "Review payroll control account difference",
		Description: "Review the difference between the payroll register and general-ledger control balance.",
		Blocking:    true,
	},
	ActionReviewNegativeContributionEntity: {
		Code: ActionReviewNegativeContributionEntity, Category: string(SectionProfitability),
		Title:       "Review negative-contribution result",
		Description: "Review the customer, job, or product result showing a negative contribution margin.",
	},
	ActionReviewMarginChange: {
		Code: ActionReviewMarginChange, Category: string(SectionProfitability),
		Title:       "Review margin change",
		Description: "Review the change in gross or contribution margin.",
	},
	ActionReviewUnallocatedSharedCost: {
		Code: ActionReviewUnallocatedSharedCost, Category: string(SectionProfitability),
		Title:       "Review unallocated shared cost",
		Description: "Review the portion of shared costs not yet allocated across customers, jobs, or products.",
	},
	ActionReviewSupplierConcentration: {
		Code: ActionReviewSupplierConcentration, Category: string(SectionVendorSpend),
		Title:       "Review supplier concentration",
		Description: "Review the concentration of spend among the largest suppliers.",
	},
	ActionReviewUnitPriceChange: {
		Code: ActionReviewUnitPriceChange, Category: string(SectionVendorSpend),
		Title:       "Review supplier unit-price change",
		Description: "Review the change in unit price for a supplier or product.",
	},
	ActionReviewNonPreferredSupplierSpend: {
		Code: ActionReviewNonPreferredSupplierSpend, Category: string(SectionVendorSpend),
		Title:       "Review non-preferred supplier spend",
		Description: "Review spend with suppliers outside the configured preferred-supplier policy.",
	},
	ActionReviewUncategorizedVendorSpend: {
		Code: ActionReviewUncategorizedVendorSpend, Category: string(SectionVendorSpend),
		Title:       "Review uncategorized vendor spend",
		Description: "Review vendor spend not yet assigned a category or product.",
	},
	ActionReviewCovenantHeadroom: {
		Code: ActionReviewCovenantHeadroom, Category: string(SectionDebtAndCovenants),
		Title:       "Review covenant headroom",
		Description: "Review the remaining headroom on a financial covenant approaching its threshold.",
	},
	ActionReviewDebtServiceRequirement: {
		Code: ActionReviewDebtServiceRequirement, Category: string(SectionDebtAndCovenants),
		Title:       "Review debt service requirement",
		Description: "Review the projected annual debt service requirement against coverage.",
	},
	ActionReviewCovenantFailure: {
		Code: ActionReviewCovenantFailure, Category: string(SectionDebtAndCovenants),
		Title:       "Review covenant status",
		Description: "Review the covenant test reported as failed.",
		Blocking:    true,
	},
	ActionResolveReconciliationBlocker: {
		Code: ActionResolveReconciliationBlocker, Category: string(SectionAccountingAndClose),
		Title:       "Resolve reconciliation blocker",
		Description: "Resolve the unreconciled or unmatched item preventing this account's reconciliation from completing.",
		Blocking:    true,
	},
	ActionResolveCloseQualityBlocker: {
		Code: ActionResolveCloseQualityBlocker, Category: string(SectionAccountingAndClose),
		Title:       "Resolve close-quality blocker",
		Description: "Resolve the condition preventing this period's close from being assessed as ready.",
		Blocking:    true,
	},
	ActionCompleteRequiredCloseTask: {
		Code: ActionCompleteRequiredCloseTask, Category: string(SectionAccountingAndClose),
		Title:       "Complete required close task",
		Description: "Complete the required close-checklist task that remains outstanding.",
		Blocking:    true,
	},
	ActionObtainRequiredSignoff: {
		Code: ActionObtainRequiredSignoff, Category: string(SectionAccountingAndClose),
		Title:       "Obtain required sign-off",
		Description: "Obtain the required reviewer sign-off for this close task.",
		Blocking:    true,
	},
	ActionAttachRequiredEvidence: {
		Code: ActionAttachRequiredEvidence, Category: string(SectionAccountingAndClose),
		Title:       "Attach required evidence",
		Description: "Attach the required supporting evidence for this close task.",
		Blocking:    true,
	},
	ActionReviewValueDriverChange: {
		Code: ActionReviewValueDriverChange, Category: string(SectionValuation),
		Title:       "Review value-driver change",
		Description: "Review the change in a key value driver against the baseline valuation.",
	},
	ActionReviewValuationSensitivity: {
		Code: ActionReviewValuationSensitivity, Category: string(SectionValuation),
		Title:       "Review valuation sensitivity",
		Description: "Review the sensitivity range around the consensus valuation.",
	},
	ActionReviewJournalEntryFinding: {
		Code: ActionReviewJournalEntryFinding, Category: string(SectionAccountingAndClose),
		Title:       "Review journal entry finding",
		Description: "Review the journal entry flagged for manual, timing, amount, account, duplicate, reversal, or clustering review.",
	},
	ActionReviewSourceConflict: {
		Code: ActionReviewSourceConflict, Category: string(SectionAppendix),
		Title:       "Review conflicting source values",
		Description: "Review the two supplied source values for this metric, which disagree beyond the configured tolerance.",
	},
	ActionReviewCustomerRetentionChange: {
		Code: ActionReviewCustomerRetentionChange, Category: string(SectionRevenue),
		Title:       "Review customer retention change",
		Description: "Review the change in customer retention, expansion, or contraction revenue.",
	},
}

// actionTemplateByCode returns the fixed actionTemplate for code and true,
// or the zero actionTemplate and false if code is not one of this
// package's defined templates — the guard every section_*.go builder uses
// before calling newGeneratedAction, so an unrecognized code (a
// programming error, since every code above is this package's own
// constant) never silently produces a blank-titled action.
func actionTemplateByCode(code string) (actionTemplate, bool) {
	t, ok := actionTemplates[code]
	return t, ok
}
