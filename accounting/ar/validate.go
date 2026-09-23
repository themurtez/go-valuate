package ar

import (
	"sort"
	"time"
)

// validateReceivables checks each Receivable for the structural rules
// section 9 requires. Returns issues plus the set of receivable IDs
// excluded from every downstream computation (non-finite amounts and
// missing required fields). Duplicate IDs are flagged here
// (IssueDuplicateReceivable) but NOT added to the excluded set — the
// caller (Calculate) separately keeps only the first occurrence of each ID
// and skips every later duplicate before consulting excluded, so adding an
// ID to excluded here would incorrectly drop that legitimate first
// occurrence too. A row that is merely flagged (e.g.
// IssueOpenExceedsOriginal, IssuePaidWithOpenBalance) but not structurally
// broken is NOT excluded — this package reports problems rather than
// silently repairing or dropping financial records, per the task's
// explicit instruction.
func validateReceivables(receivables []Receivable, asOfDate time.Time) ([]Issue, map[string]bool) {
	var issues []Issue
	excluded := map[string]bool{}
	seenID := map[string]bool{}

	for _, r := range receivables {
		if r.ID == "" {
			issues = append(issues, Issue{Code: IssueInvalidStatus, Severity: SeverityError, Message: "receivable missing ID"})
			continue
		}
		if seenID[r.ID] {
			// Do not add r.ID to excluded here: excluded is keyed by ID
			// only, and Calculate's own first-occurrence-wins dedup (see
			// its "seen" map) already keeps the FIRST row with this ID and
			// skips every later one before ever consulting excluded.
			// Marking the ID excluded here would incorrectly exclude that
			// legitimate first occurrence too.
			issues = append(issues, Issue{Code: IssueDuplicateReceivable, Severity: SeverityError, Message: "duplicate receivable ID: " + r.ID, ReceivableID: r.ID})
			continue
		}
		seenID[r.ID] = true

		if r.CustomerID == "" {
			issues = append(issues, Issue{Code: IssueMissingCustomer, Severity: SeverityError, Message: "receivable missing customer ID", ReceivableID: r.ID})
			excluded[r.ID] = true
		}

		if !isRecognizedStatus(r.Status) {
			issues = append(issues, Issue{Code: IssueInvalidStatus, Severity: SeverityError, Message: "unrecognized status", ReceivableID: r.ID})
			excluded[r.ID] = true
		}

		if isNonFinite(r.OriginalAmount) || isNonFinite(r.OpenAmount) {
			issues = append(issues, Issue{Code: IssueNonFiniteAmount, Severity: SeverityError, Message: "non-finite monetary amount", ReceivableID: r.ID})
			excluded[r.ID] = true
			continue // remaining amount checks are meaningless on non-finite values.
		}

		if r.Currency == "" {
			issues = append(issues, Issue{Code: IssueMixedCurrency, Severity: SeverityError, Message: "receivable missing currency", ReceivableID: r.ID})
			excluded[r.ID] = true
		}

		if r.InvoiceDate.IsZero() || r.DueDate.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityError, Message: "missing invoice or due date", ReceivableID: r.ID})
			excluded[r.ID] = true
		} else {
			if r.DueDate.Before(r.InvoiceDate) {
				issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityWarning, Message: "due date precedes invoice date", ReceivableID: r.ID})
			}
			if !asOfDate.IsZero() && r.InvoiceDate.After(asOfDate) {
				issues = append(issues, Issue{Code: IssueFutureInvoice, Severity: SeverityWarning, Message: "invoice date is after AsOfDate", ReceivableID: r.ID})
			}
		}

		docType := resolvedDocumentType(r.DocumentType)
		switch docType {
		case DocumentTypeCreditMemo:
			if r.OriginalAmount > 0 {
				issues = append(issues, Issue{Code: IssueInvalidAmount, Severity: SeverityWarning, Message: "credit memo has positive original amount", ReceivableID: r.ID})
			}
			if -r.OpenAmount > -r.OriginalAmount+amountTolerance && r.OriginalAmount <= 0 {
				issues = append(issues, Issue{Code: IssueOpenExceedsOriginal, Severity: SeverityWarning, Message: "credit memo open amount exceeds original amount in magnitude", ReceivableID: r.ID})
			}
		default:
			if r.OriginalAmount < 0 {
				issues = append(issues, Issue{Code: IssueInvalidAmount, Severity: SeverityWarning, Message: "invoice has negative original amount without credit-memo document type", ReceivableID: r.ID})
			}
			if r.OpenAmount > r.OriginalAmount+amountTolerance {
				issues = append(issues, Issue{Code: IssueOpenExceedsOriginal, Severity: SeverityWarning, Message: "open amount exceeds original amount", ReceivableID: r.ID})
			}
		}

		if r.Status == StatusPaid && (r.OpenAmount > amountTolerance || r.OpenAmount < -amountTolerance) {
			issues = append(issues, Issue{Code: IssuePaidWithOpenBalance, Severity: SeverityWarning, Message: "status PAID but open amount is nonzero", ReceivableID: r.ID})
		}
	}

	return issues, excluded
}

// amountTolerance is the floating-point comparison tolerance used
// throughout receivable-amount validation, matching this repository's
// other money-comparison tolerances (e.g. financial's reconciliation
// tolerance).
const amountTolerance = 0.005

// validatePayments checks each Payment for section 17's implied rules and
// returns issues plus the set of Payment IDs excluded from downstream
// computation.
func validatePayments(payments []Payment, receivableIDs map[string]bool) ([]Issue, map[string]bool) {
	var issues []Issue
	excluded := map[string]bool{}

	for _, p := range payments {
		if p.ID == "" || isNonFinite(p.Amount) || p.Amount <= 0 || p.Date.IsZero() {
			issues = append(issues, Issue{Code: IssueInvalidPayment, Severity: SeverityError, Message: "invalid payment record", PaymentID: p.ID, ReceivableID: p.ReceivableID})
			if p.ID != "" {
				excluded[p.ID] = true
			}
			continue
		}
		if p.ReceivableID != "" && !receivableIDs[p.ReceivableID] {
			issues = append(issues, Issue{Code: IssueUnknownReceivablePayment, Severity: SeverityWarning, Message: "payment references unknown receivable ID", PaymentID: p.ID, ReceivableID: p.ReceivableID})
		}
	}

	return issues, excluded
}

// resolveReportingCurrency determines the single currency an analysis
// proceeds under, per Options.ReportingCurrency's doc comment: explicit
// caller choice if supplied, otherwise the most common currency among
// included receivables (ties broken by currency code ascending). Returns
// ("", nil) only if there are no included receivables with a currency at
// all.
func resolveReportingCurrency(receivables []Receivable, excluded map[string]bool, explicit string) (string, []Issue) {
	if explicit != "" {
		return explicit, checkMixedCurrency(receivables, excluded, explicit)
	}

	counts := map[string]int{}
	for _, r := range receivables {
		if excluded[r.ID] || r.Currency == "" {
			continue
		}
		counts[r.Currency]++
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
	return best, checkMixedCurrency(receivables, excluded, best)
}

func checkMixedCurrency(receivables []Receivable, excluded map[string]bool, reporting string) []Issue {
	for _, r := range receivables {
		if excluded[r.ID] || r.Currency == "" {
			continue
		}
		if r.Currency != reporting {
			return []Issue{{Code: IssueMixedCurrency, Severity: SeverityWarning, Message: "receivables use more than one currency; only " + reporting + " included in aggregate totals"}}
		}
	}
	return nil
}
