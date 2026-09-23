package journaldiagnostics

// RuleState reports whether a diagnostic rule was actually able to run —
// see the package doc's "Rule availability" section. A rule with no
// findings and RuleState == RuleUnavailable means "could not be tested,"
// never "tested clean"; a caller must check this before treating an empty
// Findings list as reassurance.
type RuleState string

const (
	// RuleAvailable means the rule ran with sufficient data/configuration.
	RuleAvailable RuleState = "AVAILABLE"
	// RuleUnavailable means required data or configuration was absent (e.g.
	// no timestamps for weekend detection, no ApprovalThreshold for
	// threshold clustering, insufficient historical baseline for every
	// account).
	RuleUnavailable RuleState = "UNAVAILABLE"
	// RuleDisabled means the rule was deliberately not opted into (e.g.
	// Policy.RequiredReferenceFields left empty) as opposed to lacking data.
	RuleDisabled RuleState = "DISABLED"
)

// RuleAvailability reports, per diagnostic rule family, whether it actually
// ran this analysis — see RuleState. Field names match the corresponding
// FindingCode(s) each rule produces.
type RuleAvailability struct {
	MaterialManualEntry           RuleState `json:"material_manual_entry"`
	PeriodEnd                     RuleState `json:"period_end"`
	PostClose                     RuleState `json:"post_close"`
	Weekend                       RuleState `json:"weekend"`
	OutsideBusinessHours          RuleState `json:"outside_business_hours"`
	RoundDollar                   RuleState `json:"round_dollar"`
	LargeEntryAbsolute            RuleState `json:"large_entry_absolute"`
	AccountRelativeLargeEntry     RuleState `json:"account_relative_large_entry"`
	RareAccountActivity           RuleState `json:"rare_account_activity"`
	NewAccountActivity            RuleState `json:"new_account_activity"`
	OppositeNormalBalanceMovement RuleState `json:"opposite_normal_balance_movement"`
	ManualRevenueEquity           RuleState `json:"manual_revenue_equity"`
	SensitiveAccountEntry         RuleState `json:"sensitive_account_entry"`
	ExactDuplicateEntry           RuleState `json:"exact_duplicate_entry"`
	PossibleDuplicateEntry        RuleState `json:"possible_duplicate_entry"`
	RepeatedIdenticalAmount       RuleState `json:"repeated_identical_amount"`
	Reversals                     RuleState `json:"reversals"`
	PeriodEndEarlyReversal        RuleState `json:"period_end_early_reversal"`
	ThresholdCluster              RuleState `json:"threshold_cluster"`
	SplitEntryCluster             RuleState `json:"split_entry_cluster"`
	BlankDescription              RuleState `json:"blank_description"`
	GenericDescription            RuleState `json:"generic_description"`
	MissingReference              RuleState `json:"missing_reference"`
	PreparerApproverDiagnostics   RuleState `json:"preparer_approver_diagnostics"`
	RareAccountCombination        RuleState `json:"rare_account_combination"`
	CrossPeriodComparison         RuleState `json:"cross_period_comparison"`
}
