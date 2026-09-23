package cashforecast

import "time"

// ARReceivableSource is one open receivable's portable identity and open
// amount, sufficient to validate AR scheduling against without a
// compile-time dependency on accounting/ar. A caller with an accounting/ar
// Result maps its own Input.Receivables (or Result.CustomerSummaries, if
// only customer-level totals are available) into this shape; this package
// never imports accounting/ar directly, matching the architectural
// instruction to prefer portable typed inputs over a runtime dependency
// when the semantics fit cleanly — see the task's section 13.
type ARReceivableSource struct {
	ReceivableID string  `json:"receivable_id"`
	CustomerID   string  `json:"customer_id,omitempty"`
	OpenAmount   float64 `json:"open_amount"`
}

// ARCollectionAssumption is one caller-supplied expected-receipt
// assumption against a specific open receivable. This is never generated
// automatically — an invoice's due date is not the same as its expected
// cash receipt date, and this package never assumes otherwise unless the
// caller explicitly opts into ARDueDatePolicy (see aradapter.go's
// due-date opt-in below, mirroring accounting/ap's PayAPOnDueDate
// precedent but for AR explicitly requiring the caller to ask for it
// too) — see the task's section 15's "do not create default collection
// behavior" instruction.
type ARCollectionAssumption struct {
	ID                  string    `json:"id"`
	ReceivableID        string    `json:"receivable_id"`
	ExpectedReceiptDate time.Time `json:"expected_receipt_date"`
	ExpectedAmount      float64   `json:"expected_amount"`
	Certainty           Certainty `json:"certainty,omitempty"`
	Basis               CashBasis `json:"basis,omitempty"`
}

// resolvedARBasis returns a.Basis if recognized, otherwise BasisAssumed —
// the safe default for an AR collection assumption, which by definition
// is the caller's own estimate unless explicitly marked otherwise.
func resolvedARBasis(b CashBasis) CashBasis {
	if isRecognizedBasis(b) {
		return b
	}
	return BasisAssumed
}

// buildAREvents converts assumptions into CashFlowEvents, and validates
// each assumption's ReceivableID against sources (when supplied) and the
// running per-receivable scheduled total against that receivable's
// OpenAmount — see the task's section 14. Returns the generated events and
// every Issue found; assumptions with no matching source are still
// converted to events (sources is optional context, not a hard
// dependency) but flagged with IssueUnknownReceivable.
func buildAREvents(assumptions []ARCollectionAssumption, sources []ARReceivableSource) ([]CashFlowEvent, []Issue) {
	if len(assumptions) == 0 {
		return nil, nil
	}
	openByID := make(map[string]float64, len(sources))
	haveSources := len(sources) > 0
	for _, s := range sources {
		openByID[s.ReceivableID] = s.OpenAmount
	}

	var issues []Issue
	scheduled := make(map[string]float64)
	events := make([]CashFlowEvent, 0, len(assumptions))
	for _, a := range assumptions {
		if haveSources {
			if _, ok := openByID[a.ReceivableID]; !ok {
				issues = append(issues, Issue{Code: IssueUnknownReceivable, Severity: SeverityWarning,
					Message:  "AR collection assumption references a receivable not present in the supplied AR source data",
					SourceID: a.ReceivableID})
			}
		}
		scheduled[a.ReceivableID] += a.ExpectedAmount
		if haveSources {
			if open, ok := openByID[a.ReceivableID]; ok && scheduled[a.ReceivableID] > open+amountTolerance {
				issues = append(issues, Issue{Code: IssueARScheduleExceedsOpen, Severity: SeverityError,
					Message:  "sum of scheduled AR collections against this receivable exceeds its open amount",
					SourceID: a.ReceivableID})
				continue
			}
		}
		events = append(events, CashFlowEvent{
			ID:         "ar-collection#" + a.ID,
			Date:       a.ExpectedReceiptDate,
			Amount:     a.ExpectedAmount,
			Direction:  DirectionInflow,
			Category:   CategoryARCollection,
			SourceType: SourceARReceivable,
			SourceID:   a.ReceivableID,
			Basis:      resolvedARBasis(a.Basis),
			Certainty:  a.Certainty,
		})
	}
	return events, issues
}

// unscheduledARAmount returns UnscheduledSummary computing, for every
// source in sources, the portion of OpenAmount not covered by any
// scheduled assumption in assumptions — see the task's section 40 and
// section 52's integration invariant.
func unscheduledARAmount(sources []ARReceivableSource, assumptions []ARCollectionAssumption) UnscheduledSummary {
	if len(sources) == 0 {
		return UnscheduledSummary{}
	}
	scheduled := make(map[string]float64, len(sources))
	for _, a := range assumptions {
		scheduled[a.ReceivableID] += a.ExpectedAmount
	}
	var amount float64
	var ids []string
	var count int
	for _, s := range sources {
		remaining := s.OpenAmount - scheduled[s.ReceivableID]
		if remaining > amountTolerance {
			amount += remaining
			ids = append(ids, s.ReceivableID)
			count++
		}
	}
	return UnscheduledSummary{Available: true, Count: count, Amount: amount, IDs: ids}
}

// amountTolerance is the floating-point comparison tolerance used
// throughout this package's amount validation, matching
// accounting/ar/accounting/ap's identical money-comparison tolerance.
const amountTolerance = 0.005
