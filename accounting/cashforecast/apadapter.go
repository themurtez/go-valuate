package cashforecast

import "time"

// APPayableSource is one open payable's portable identity, open amount,
// and due date, sufficient to validate AP scheduling against and to power
// the opt-in due-date adapter without a compile-time dependency on
// accounting/ap — see ARReceivableSource's identical rationale.
type APPayableSource struct {
	PayableID  string    `json:"payable_id"`
	SupplierID string    `json:"supplier_id,omitempty"`
	OpenAmount float64   `json:"open_amount"`
	DueDate    time.Time `json:"due_date"`
}

// APPaymentPlan is one caller-supplied explicit planned payment against a
// specific open payable — see the task's section 17.
type APPaymentPlan struct {
	ID          string    `json:"id"`
	PayableID   string    `json:"payable_id"`
	PaymentDate time.Time `json:"payment_date"`
	Amount      float64   `json:"amount"`
	Certainty   Certainty `json:"certainty,omitempty"`
	Basis       CashBasis `json:"basis,omitempty"`
}

func resolvedAPBasis(b CashBasis) CashBasis {
	if isRecognizedBasis(b) {
		return b
	}
	return BasisScheduled
}

// buildAPEvents converts explicit plans into CashFlowEvents, validating
// each plan's PayableID against sources (when supplied) and the running
// per-payable scheduled total against that payable's OpenAmount — see the
// task's section 17. Mirrors buildAREvents' structure exactly but is a
// separate function since AP's default-safe-behavior and event Category
// differ (AP_PAYMENT vs AR_COLLECTION, OUTFLOW vs INFLOW).
func buildAPEvents(plans []APPaymentPlan, sources []APPayableSource) ([]CashFlowEvent, []Issue) {
	if len(plans) == 0 {
		return nil, nil
	}
	openByID := make(map[string]float64, len(sources))
	haveSources := len(sources) > 0
	for _, s := range sources {
		openByID[s.PayableID] = s.OpenAmount
	}

	var issues []Issue
	scheduled := make(map[string]float64)
	events := make([]CashFlowEvent, 0, len(plans))
	for _, p := range plans {
		if haveSources {
			if _, ok := openByID[p.PayableID]; !ok {
				issues = append(issues, Issue{Code: IssueUnknownPayable, Severity: SeverityWarning,
					Message:  "AP payment plan references a payable not present in the supplied AP source data",
					SourceID: p.PayableID})
			}
		}
		scheduled[p.PayableID] += p.Amount
		if haveSources {
			if open, ok := openByID[p.PayableID]; ok && scheduled[p.PayableID] > open+amountTolerance {
				issues = append(issues, Issue{Code: IssueAPScheduleExceedsOpen, Severity: SeverityError,
					Message:  "sum of planned AP payments against this payable exceeds its open amount",
					SourceID: p.PayableID})
				continue
			}
		}
		events = append(events, CashFlowEvent{
			ID:         "ap-payment#" + p.ID,
			Date:       p.PaymentDate,
			Amount:     p.Amount,
			Direction:  DirectionOutflow,
			Category:   CategoryAPPayment,
			SourceType: SourceAPPayable,
			SourceID:   p.PayableID,
			Basis:      resolvedAPBasis(p.Basis),
			Certainty:  p.Certainty,
		})
	}
	return events, issues
}

// buildAPDueDateEvents implements the task's section 16/54 opt-in
// due-date adapter: when payAPOnDueDate is true, every source payable NOT
// already covered by an explicit plan generates one outflow event on its
// own DueDate for its remaining (unscheduled) open amount. This is never
// the default — a caller must explicitly set Options.PayAPOnDueDate,
// since businesses often pay early/late/selectively (see the task's
// explicit "default safest behavior: no automatic AP payment schedule"
// instruction). A payable already fully covered by explicit plans
// generates no due-date event; a partially-covered payable generates one
// for the remainder only, never double-counting the explicitly planned
// portion.
func buildAPDueDateEvents(sources []APPayableSource, plans []APPaymentPlan) []CashFlowEvent {
	if len(sources) == 0 {
		return nil
	}
	scheduled := make(map[string]float64, len(sources))
	for _, p := range plans {
		scheduled[p.PayableID] += p.Amount
	}
	events := make([]CashFlowEvent, 0, len(sources))
	for _, s := range sources {
		remaining := s.OpenAmount - scheduled[s.PayableID]
		if remaining <= amountTolerance {
			continue
		}
		events = append(events, CashFlowEvent{
			ID:         "ap-duedate#" + s.PayableID,
			Date:       s.DueDate,
			Amount:     remaining,
			Direction:  DirectionOutflow,
			Category:   CategoryAPPayment,
			SourceType: SourceAPPayable,
			SourceID:   s.PayableID,
			Basis:      BasisKnown,
		})
	}
	return events
}

// unscheduledAPAmount mirrors unscheduledARAmount for AP — see the task's
// section 40 and section 53's integration invariant. When
// payAPOnDueDate is true, every source payable is considered scheduled by
// the due-date adapter (buildAPDueDateEvents covers 100% of remaining
// open amount by construction), so UnscheduledAP is always empty/zero in
// that mode.
func unscheduledAPAmount(sources []APPayableSource, plans []APPaymentPlan, payAPOnDueDate bool) UnscheduledSummary {
	if len(sources) == 0 {
		return UnscheduledSummary{}
	}
	if payAPOnDueDate {
		return UnscheduledSummary{Available: true}
	}
	scheduled := make(map[string]float64, len(sources))
	for _, p := range plans {
		scheduled[p.PayableID] += p.Amount
	}
	var amount float64
	var ids []string
	var count int
	for _, s := range sources {
		remaining := s.OpenAmount - scheduled[s.PayableID]
		if remaining > amountTolerance {
			amount += remaining
			ids = append(ids, s.PayableID)
			count++
		}
	}
	return UnscheduledSummary{Available: true, Count: count, Amount: amount, IDs: ids}
}
