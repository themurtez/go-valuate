package labor

// WorkerDateFinding is one review finding about a PayrollRecord's PayDate
// relative to the referenced Worker's HireDate/TerminationDate — the
// task's section 37. Never a conclusion about improper payment; a
// finding within Policy.PostTerminationPayGraceDays of TerminationDate is
// Informational (a legitimate final pay is common), a finding materially
// beyond the grace period is a Warning.
type WorkerDateFinding struct {
	RecordID string                  `json:"record_id"`
	WorkerID string                  `json:"worker_id"`
	Reason   WorkerDateFindingReason `json:"reason"`
	// DaysAfterTermination is populated only for
	// ReasonPayAfterTermination, the number of days PayDate falls after
	// TerminationDate.
	DaysAfterTermination int          `json:"days_after_termination,omitempty"`
	Severity             FlagSeverity `json:"severity"`
	Message              string       `json:"message"`
}

// WorkerDateFindingReason is a closed taxonomy for WorkerDateFinding.
type WorkerDateFindingReason string

const (
	ReasonPayBeforeHire       WorkerDateFindingReason = "PAY_BEFORE_HIRE"
	ReasonPayAfterTermination WorkerDateFindingReason = "PAY_AFTER_TERMINATION"
)

// buildWorkerDateFindings checks every payroll record whose worker is
// known (present in workers) for PayDate falling before HireDate or
// materially after TerminationDate.
func buildWorkerDateFindings(payroll []PayrollRecord, workers map[string]Worker, graceDays int) []WorkerDateFinding {
	var findings []WorkerDateFinding
	for _, r := range payroll {
		w, ok := workers[r.WorkerID]
		if !ok {
			continue
		}
		if w.HireDate != nil && r.PayDate.Before(*w.HireDate) {
			findings = append(findings, WorkerDateFinding{
				RecordID: r.ID, WorkerID: r.WorkerID, Reason: ReasonPayBeforeHire,
				Severity: FlagSeverityWarning,
				Message:  "payroll record pay date is before the worker's recorded hire date",
			})
		}
		if w.TerminationDate != nil && r.PayDate.After(*w.TerminationDate) {
			daysAfter := int(r.PayDate.Sub(*w.TerminationDate).Hours() / 24)
			severity := FlagSeverityInfo
			message := "payroll record pay date is after the worker's recorded termination date, within the configured grace period for a final pay"
			if daysAfter > graceDays {
				severity = FlagSeverityWarning
				message = "payroll record pay date is materially after the worker's recorded termination date, beyond the configured grace period"
			}
			findings = append(findings, WorkerDateFinding{
				RecordID: r.ID, WorkerID: r.WorkerID, Reason: ReasonPayAfterTermination,
				DaysAfterTermination: daysAfter, Severity: severity, Message: message,
			})
		}
	}
	sortWorkerDateFindings(findings)
	return findings
}

// KeyWorkerConcentration is the task's section 34 minimal key-worker
// labor-cost-share calculation, computed only when at least one Worker is
// marked KeyWorker. This package never infers operational dependency or a
// "key employee risk" score from compensation alone — it reports a plain
// cost share.
type KeyWorkerConcentration struct {
	Available bool `json:"available"`
	// KeyWorkerLaborCost is the sum of gross+burden cost for payroll
	// records belonging to caller-marked key workers.
	KeyWorkerLaborCost float64 `json:"key_worker_labor_cost"`
	// ShareOfTotalEmployeeCost is KeyWorkerLaborCost /
	// LaborCostBridge.TotalEmployeeCost, available when that denominator
	// is nonzero.
	ShareOfTotalEmployeeCost Value `json:"share_of_total_employee_cost"`
}

func buildKeyWorkerConcentration(payroll []PayrollRecord, workers map[string]Worker, totalEmployeeCost float64) KeyWorkerConcentration {
	var haveKeyWorker bool
	for _, w := range workers {
		if w.KeyWorker {
			haveKeyWorker = true
			break
		}
	}
	if !haveKeyWorker {
		return KeyWorkerConcentration{}
	}
	var sum float64
	for _, r := range payroll {
		w, ok := workers[r.WorkerID]
		if !ok || !w.KeyWorker {
			continue
		}
		sum += r.RegularPay + r.OvertimePay + r.BonusPay + r.CommissionPay + r.OtherPay +
			r.EmployerTaxes + r.BenefitsCost + r.OtherEmployerCost
	}
	k := KeyWorkerConcentration{Available: true, KeyWorkerLaborCost: sum}
	if totalEmployeeCost != 0 {
		k.ShareOfTotalEmployeeCost = AvailableValue(sum / totalEmployeeCost)
	}
	return k
}
