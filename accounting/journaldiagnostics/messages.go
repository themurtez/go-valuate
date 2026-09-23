package journaldiagnostics

// Fixed message templates, one per FindingCode. Every string here uses
// neutral review language only ("anomaly," "unusual pattern," "unusual
// activity," "review recommended," "control-review indicator") — never
// "fraud," "theft," "embezzlement," "manipulation," or "misconduct." See
// the package doc comment's non-fraud boundary and
// safety_no_fraud_language_test.go, which scans every message this package
// can produce (including these templates and every dynamically-built
// message in duplicates.go/reversals.go/clustering.go) for prohibited
// language.
const (
	msgMaterialManualEntry             = "material manual journal entry — review recommended"
	msgMaterialPeriodEndEntry          = "material entry posted near period end — review recommended"
	msgPostCloseEntry                  = "entry posted after the stated close date but affecting the closed period — review recommended"
	msgWeekendEntry                    = "entry posted on a non-working day"
	msgOutsideBusinessHours            = "entry posted outside configured business hours"
	msgRoundDollarEntry                = "entry amount is an unusually round figure for its size — review recommended"
	msgLargeEntry                      = "entry amount exceeds the configured absolute large-entry threshold"
	msgAccountRelativeLargeEntry       = "entry amount is unusual relative to this account's historical activity — review recommended"
	msgRareAccountActivity             = "material entry posted to an account with little prior history"
	msgNewAccountActivity              = "material entry posted to an account with no prior posting history"
	msgOppositeNormalBalanceMovement   = "entry moves this account against its normal balance side — review recommended"
	msgManualRevenueEntry              = "manual entry directly affects a revenue account — review recommended"
	msgManualEquityEntry               = "manual entry directly affects an equity account — review recommended"
	msgSensitiveAccountEntry           = "entry posted to a caller-designated sensitive account"
	msgExactDuplicateEntry             = "two or more entries share identical normalized economic content and date — review recommended"
	msgPossibleDuplicateEntry          = "two or more entries share matching normalized economic content within a short date window — review recommended"
	msgRepeatedIdenticalAmount         = "the same amount recurs across multiple distinct entries — review recommended"
	msgRapidReversal                   = "entry was reversed shortly after posting — review recommended"
	msgCrossPeriodReversal             = "entry's reversal falls in a different period than the original — review recommended"
	msgPeriodEndEntryWithEarlyReversal = "material entry posted near period end and reversed shortly after period start — review recommended"
	msgThresholdCluster                = "multiple entries cluster just below the configured approval threshold — review recommended"
	msgSplitEntryCluster               = "multiple related entries, each below the approval threshold, combine to meet or exceed it — review recommended"
	msgBlankDescription                = "entry has a blank description"
	msgGenericDescription              = "entry description matches a caller-configured generic/uninformative term"
	msgMissingReference                = "material entry is missing an expected reference"
	msgSamePreparerApprover            = "entry's preparer and approver are the same identifier — control-review indicator"
	msgMissingApprover                 = "entry has a preparer but no approver — control-review indicator"
	msgHighVolumeByPreparer            = "preparer has a high volume of entries in this analysis period"
	msgRareAccountCombination          = "entry uses an account combination with little or no prior history"
)
