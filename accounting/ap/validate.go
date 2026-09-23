package ap

import (
	"sort"
	"time"
)

// validatePayables checks each Payable for the structural rules this
// package requires. Returns issues plus the set of payable IDs excluded
// from every downstream computation (non-finite amounts and missing
// required fields). Duplicate IDs are flagged here (IssueDuplicatePayable)
// but NOT added to the excluded set — the caller (Calculate) separately
// keeps only the first occurrence of each ID and skips every later
// duplicate before consulting excluded, so adding an ID to excluded here
// would incorrectly drop that legitimate first occurrence too. A row that
// is merely flagged (e.g. IssueOpenExceedsOriginal,
// IssuePaidWithOpenBalance) but not structurally broken is NOT excluded —
// this package reports problems rather than silently repairing or
// dropping financial records, per the task's explicit "never silently
// repair input" instruction.
func validatePayables(payables []Payable, asOfDate time.Time) ([]Issue, map[string]bool) {
	var issues []Issue
	excluded := map[string]bool{}
	seenID := map[string]bool{}

	for _, p := range payables {
		if p.ID == "" {
			issues = append(issues, Issue{Code: IssueInvalidStatus, Severity: SeverityError, Message: "payable missing ID"})
			continue
		}
		if seenID[p.ID] {
			// Do not add p.ID to excluded here: excluded is keyed by ID
			// only, and Calculate's own first-occurrence-wins dedup (see
			// its "seen" map) already keeps the FIRST row with this ID and
			// skips every later one before ever consulting excluded.
			// Marking the ID excluded here would incorrectly exclude that
			// legitimate first occurrence too.
			issues = append(issues, Issue{Code: IssueDuplicatePayable, Severity: SeverityError, Message: "duplicate payable ID: " + p.ID, PayableID: p.ID})
			continue
		}
		seenID[p.ID] = true

		if p.SupplierID == "" {
			issues = append(issues, Issue{Code: IssueMissingSupplier, Severity: SeverityError, Message: "payable missing supplier ID", PayableID: p.ID})
			excluded[p.ID] = true
		}

		if !isRecognizedStatus(p.Status) {
			issues = append(issues, Issue{Code: IssueInvalidStatus, Severity: SeverityError, Message: "unrecognized status", PayableID: p.ID})
			excluded[p.ID] = true
		}

		if isNonFinite(p.OriginalAmount) || isNonFinite(p.OpenAmount) {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError, Message: "non-finite monetary amount", PayableID: p.ID})
			excluded[p.ID] = true
			continue // remaining amount checks are meaningless on non-finite values.
		}

		if p.Currency == "" {
			issues = append(issues, Issue{Code: IssueMixedCurrency, Severity: SeverityError, Message: "payable missing currency", PayableID: p.ID})
			excluded[p.ID] = true
		}

		if p.BillDate.IsZero() || p.DueDate.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityError, Message: "missing bill or due date", PayableID: p.ID})
			excluded[p.ID] = true
		} else {
			if p.DueDate.Before(p.BillDate) {
				issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityWarning, Message: "due date precedes bill date", PayableID: p.ID})
			}
			if !asOfDate.IsZero() && p.BillDate.After(asOfDate) {
				issues = append(issues, Issue{Code: IssueFutureBill, Severity: SeverityWarning, Message: "bill date is after AsOfDate", PayableID: p.ID})
			}
		}

		docType := resolvedDocumentType(p.DocumentType)
		switch docType {
		case DocumentTypeVendorCredit:
			if p.OriginalAmount > 0 {
				issues = append(issues, Issue{Code: IssueInvalidAmount, Severity: SeverityWarning, Message: "vendor credit has positive original amount", PayableID: p.ID})
			}
			if -p.OpenAmount > -p.OriginalAmount+amountTolerance && p.OriginalAmount <= 0 {
				issues = append(issues, Issue{Code: IssueOpenExceedsOriginal, Severity: SeverityWarning, Message: "vendor credit open amount exceeds original amount in magnitude", PayableID: p.ID})
			}
		default:
			if p.OriginalAmount < 0 {
				issues = append(issues, Issue{Code: IssueInvalidAmount, Severity: SeverityWarning, Message: "bill has negative original amount without vendor-credit document type", PayableID: p.ID})
			}
			if p.OpenAmount > p.OriginalAmount+amountTolerance {
				issues = append(issues, Issue{Code: IssueOpenExceedsOriginal, Severity: SeverityWarning, Message: "open amount exceeds original amount", PayableID: p.ID})
			}
		}

		if p.Status == StatusPaid && (p.OpenAmount > amountTolerance || p.OpenAmount < -amountTolerance) {
			issues = append(issues, Issue{Code: IssuePaidWithOpenBalance, Severity: SeverityWarning, Message: "status PAID but open amount is nonzero", PayableID: p.ID})
		}
	}

	return issues, excluded
}

// validatePayments checks each SupplierPayment for basic structural rules
// and returns issues plus the set of Payment IDs excluded from downstream
// computation.
func validatePayments(payments []SupplierPayment, payableIDs map[string]bool) ([]Issue, map[string]bool) {
	var issues []Issue
	excluded := map[string]bool{}

	for _, sp := range payments {
		if sp.ID == "" || isNonFinite(sp.Amount) || sp.Amount <= 0 || sp.Date.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidPayment, Severity: SeverityError, Message: "invalid payment record", PaymentID: sp.ID, PayableID: sp.PayableID})
			if sp.ID != "" {
				excluded[sp.ID] = true
			}
			continue
		}
		if sp.PayableID != "" && !payableIDs[sp.PayableID] {
			issues = append(issues, Issue{Code: IssueUnknownPayablePayment, Severity: SeverityWarning, Message: "payment references unknown payable ID", PaymentID: sp.ID, PayableID: sp.PayableID})
		}
	}

	return issues, excluded
}

// resolveReportingCurrency determines the single currency an analysis
// proceeds under: explicit caller choice if supplied, otherwise the most
// common currency among included payables (ties broken by currency code
// ascending). Returns ("", nil) only if there are no included payables
// with a currency at all.
func resolveReportingCurrency(payables []Payable, excluded map[string]bool, explicit string) (string, []Issue) {
	if explicit != "" {
		return explicit, checkMixedCurrency(payables, excluded, explicit)
	}

	counts := map[string]int{}
	for _, p := range payables {
		if excluded[p.ID] || p.Currency == "" {
			continue
		}
		counts[p.Currency]++
	}
	if len(counts) == 0 {
		return "", nil
	}

	codes := make([]string, 0, len(counts))
	for c := range counts {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	best := codes[0]
	for _, c := range codes[1:] {
		if counts[c] > counts[best] {
			best = c
		}
	}
	return best, checkMixedCurrency(payables, excluded, best)
}

func checkMixedCurrency(payables []Payable, excluded map[string]bool, reporting string) []Issue {
	for _, p := range payables {
		if excluded[p.ID] || p.Currency == "" {
			continue
		}
		if p.Currency != reporting {
			return []Issue{{Code: IssueMixedCurrency, Severity: SeverityWarning, Message: "payables use more than one currency; only " + reporting + " included in aggregate totals"}}
		}
	}
	return nil
}

func includedStatus(s PayableStatus, allowed []PayableStatus) bool {
	for _, a := range allowed {
		if s == a {
			return true
		}
	}
	return false
}
