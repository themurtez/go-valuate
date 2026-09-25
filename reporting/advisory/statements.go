package advisory

import "fmt"

// StatementCode values this package actually generates — task section 79.
// Every one has a corresponding statement* function below assembling it
// from typed values via a fixed Go format string, never free text — task
// section 78's "no generative text" rule.
const (
	StatementMinimumCashBelowThreshold       StatementCode = "MINIMUM_CASH_BELOW_THRESHOLD"
	StatementAROver90Increased               StatementCode = "AR_OVER_90_INCREASED"
	StatementAPOver90Increased               StatementCode = "AP_OVER_90_INCREASED"
	StatementCovenantFailed                  StatementCode = "COVENANT_FAILED"
	StatementCovenantNearBreach              StatementCode = "COVENANT_NEAR_BREACH"
	StatementCloseNotReady                   StatementCode = "CLOSE_NOT_READY"
	StatementSupplierConcentrationIncreased  StatementCode = "SUPPLIER_CONCENTRATION_INCREASED"
	StatementContributionMarginDeclined      StatementCode = "CONTRIBUTION_MARGIN_DECLINED"
	StatementRevenueChanged                  StatementCode = "REVENUE_CHANGED"
	StatementEBITDAChanged                   StatementCode = "EBITDA_CHANGED"
	StatementCustomerConcentrationIncreased  StatementCode = "CUSTOMER_CONCENTRATION_INCREASED"
	StatementInventoryBuildOutpacingUsage    StatementCode = "INVENTORY_BUILD_OUTPACING_USAGE"
	StatementLaborCostPercentIncreased       StatementCode = "LABOR_COST_PERCENT_INCREASED"
	StatementReconciliationBlocker           StatementCode = "RECONCILIATION_BLOCKER"
	StatementLiquidityAndCollectionsPressure StatementCode = "LIQUIDITY_AND_COLLECTIONS_PRESSURE"
	StatementCloseBlockedByReconciliation    StatementCode = "CLOSE_BLOCKED_BY_RECONCILIATION"
	StatementMarginPressureFromLabor         StatementCode = "MARGIN_PRESSURE_FROM_LABOR"
)

// statementMinimumCash renders task section 17's minimum-cash fact —
// example allowed statement from task section 8.
func statementMinimumCash(amount float64, week int, negative bool) string {
	if negative {
		return fmt.Sprintf("The 13-week cash forecast reaches a negative minimum cash balance of %.2f in week %d.", amount, week)
	}
	return fmt.Sprintf("The 13-week cash forecast reaches a minimum cash balance of %.2f in week %d.", amount, week)
}

// statementPercentIncreased renders the canonical "X over 90 days
// increased from A% to B%" template — task section 78's worked example.
func statementPercentIncreased(subject string, fromPercent, toPercent float64) string {
	return fmt.Sprintf("%s increased from %.1f%% to %.1f%%.", subject, fromPercent*100, toPercent*100)
}

// statementPercentDecreased mirrors statementPercentIncreased for a
// decline.
func statementPercentDecreased(subject string, fromPercent, toPercent float64) string {
	return fmt.Sprintf("%s decreased from %.1f%% to %.1f%%.", subject, fromPercent*100, toPercent*100)
}

// statementValueChanged renders a generic current-vs-prior currency
// change.
func statementValueChanged(subject string, from, to float64) string {
	return fmt.Sprintf("%s changed from %.2f to %.2f.", subject, from, to)
}

// statementCovenantFailed renders a covenant-failure fact, preserving the
// source's own exact terminology (task section 25/74's "no legal
// conclusion, use source status exactly" rule) rather than restating it.
func statementCovenantFailed(covenantID string, explanation string) string {
	if explanation != "" {
		return explanation
	}
	return fmt.Sprintf("Covenant %s is reported as failed.", covenantID)
}

// statementCloseNotReady renders a close-readiness fact from the source's
// own status label.
func statementCloseNotReady(status string) string {
	return fmt.Sprintf("The period close is reported as %s.", status)
}

// statementReconciliationBlocker renders a reconciliation finding using
// the source's own message verbatim when available.
func statementReconciliationBlocker(accountID, message string) string {
	if message != "" {
		return message
	}
	return fmt.Sprintf("Account %s has an unresolved reconciliation condition.", accountID)
}
