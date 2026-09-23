package cashforecast

import "time"

// PayrollEvent is one portable payroll cash obligation. This package never
// calculates payroll (no employment/tax rules) — the caller supplies the
// cash-out figures directly; see the task's section 18.
type PayrollEvent struct {
	ID                           string    `json:"id"`
	Date                         time.Time `json:"date"`
	GrossPayroll                 float64   `json:"gross_payroll,omitempty"`
	EmployeeNetCash              float64   `json:"employee_net_cash"`
	EmployerTaxes                float64   `json:"employer_taxes,omitempty"`
	EmployeeWithholdingsRemitted float64   `json:"employee_withholdings_remitted,omitempty"`
	Benefits                     float64   `json:"benefits,omitempty"`
	OtherCash                    float64   `json:"other_cash,omitempty"`
	Certainty                    Certainty `json:"certainty,omitempty"`
	Basis                        CashBasis `json:"basis,omitempty"`
}

// totalCashOut sums every cash-out component of a PayrollEvent into the
// single outflow amount a CashFlowEvent needs. EmployeeNetCash is the only
// component always present; the others are optional additional cash-outs
// (employer-side taxes, remitted withholdings, benefits, other) a caller
// may itemize or fold into EmployeeNetCash — this package sums whatever
// was supplied rather than assuming any single field represents the
// total.
func (p PayrollEvent) totalCashOut() float64 {
	return p.EmployeeNetCash + p.EmployerTaxes + p.EmployeeWithholdingsRemitted + p.Benefits + p.OtherCash
}

func buildPayrollEvents(events []PayrollEvent) []CashFlowEvent {
	out := make([]CashFlowEvent, 0, len(events))
	for _, p := range events {
		out = append(out, CashFlowEvent{
			ID:         "payroll#" + p.ID,
			Date:       p.Date,
			Amount:     p.totalCashOut(),
			Direction:  DirectionOutflow,
			Category:   CategoryPayroll,
			SourceType: SourcePayrollSchedule,
			SourceID:   p.ID,
			Basis:      resolvedAPBasis(p.Basis),
			Certainty:  p.Certainty,
		})
	}
	return out
}

// TaxCategory distinguishes the kind of tax/remittance a TaxEvent
// represents — used only to select TaxEvent's generated CashCategory (see
// the task's section 19).
type TaxCategory string

const (
	TaxSales        TaxCategory = "SALES"
	TaxPayrollRemit TaxCategory = "PAYROLL_REMITTANCE"
	TaxIncome       TaxCategory = "INCOME"
	TaxOther        TaxCategory = "OTHER"
)

func taxCashCategory(t TaxCategory) CashCategory {
	switch t {
	case TaxSales:
		return CategorySalesTax
	case TaxPayrollRemit:
		return CategoryPayrollTax
	case TaxIncome:
		return CategoryIncomeTax
	default:
		return CategoryOtherOperatingOutflow
	}
}

// TaxEvent is one portable explicit tax/remittance cash obligation. This
// package never calculates tax liability — see the task's section 19.
type TaxEvent struct {
	ID        string      `json:"id"`
	Date      time.Time   `json:"date"`
	Amount    float64     `json:"amount"`
	Category  TaxCategory `json:"category"`
	Certainty Certainty   `json:"certainty,omitempty"`
	Basis     CashBasis   `json:"basis,omitempty"`
}

func buildTaxEvents(events []TaxEvent) []CashFlowEvent {
	out := make([]CashFlowEvent, 0, len(events))
	for _, e := range events {
		out = append(out, CashFlowEvent{
			ID:         "tax#" + e.ID,
			Date:       e.Date,
			Amount:     e.Amount,
			Direction:  DirectionOutflow,
			Category:   taxCashCategory(e.Category),
			SourceType: SourceTaxSchedule,
			SourceID:   e.ID,
			Basis:      resolvedAPBasis(e.Basis),
			Certainty:  e.Certainty,
		})
	}
	return out
}

// DebtServiceEvent is one caller-supplied explicit debt-service payment
// with a real date — the only way debt service enters this package (see
// debtadapter.go for why no analytics/debt adapter exists). This package
// never assumes an evenly-spread schedule from an annual total.
type DebtServiceEvent struct {
	ID        string    `json:"id"`
	Date      time.Time `json:"date"`
	Amount    float64   `json:"amount"`
	LoanLabel string    `json:"loan_label,omitempty"`
	Certainty Certainty `json:"certainty,omitempty"`
	Basis     CashBasis `json:"basis,omitempty"`
}

func buildDebtServiceEvents(events []DebtServiceEvent) []CashFlowEvent {
	out := make([]CashFlowEvent, 0, len(events))
	for _, e := range events {
		out = append(out, CashFlowEvent{
			ID:          "debt#" + e.ID,
			Date:        e.Date,
			Amount:      e.Amount,
			Direction:   DirectionOutflow,
			Category:    CategoryDebtService,
			Description: e.LoanLabel,
			SourceType:  SourceDebtSchedule,
			SourceID:    e.ID,
			Basis:       resolvedAPBasis(e.Basis),
			Certainty:   e.Certainty,
		})
	}
	return out
}

// CapexEvent is one caller-supplied explicit capital expenditure. Kept
// separate from ordinary operating outflows via CategoryCapex; no
// depreciation calculation — see the task's section 22.
type CapexEvent struct {
	ID          string     `json:"id"`
	Date        time.Time  `json:"date"`
	Amount      float64    `json:"amount"`
	Description string     `json:"description,omitempty"`
	Certainty   Certainty  `json:"certainty,omitempty"`
	Commitment  Commitment `json:"commitment,omitempty"`
	Basis       CashBasis  `json:"basis,omitempty"`
}

func buildCapexEvents(events []CapexEvent) []CashFlowEvent {
	out := make([]CashFlowEvent, 0, len(events))
	for _, e := range events {
		out = append(out, CashFlowEvent{
			ID:          "capex#" + e.ID,
			Date:        e.Date,
			Amount:      e.Amount,
			Direction:   DirectionOutflow,
			Category:    CategoryCapex,
			Description: e.Description,
			SourceType:  SourceCapexPlan,
			SourceID:    e.ID,
			Basis:       resolvedAPBasis(e.Basis),
			Certainty:   e.Certainty,
			Commitment:  e.Commitment,
		})
	}
	return out
}

// FinancingEventType distinguishes the kind of financing cash flow a
// FinancingEvent represents — see the task's sections 23-24.
type FinancingEventType string

const (
	FinancingLoanProceeds       FinancingEventType = "LOAN_PROCEEDS"
	FinancingLineOfCreditDraw   FinancingEventType = "LINE_OF_CREDIT_DRAW"
	FinancingLoanRepayment      FinancingEventType = "LOAN_REPAYMENT"
	FinancingEquityContribution FinancingEventType = "EQUITY_CONTRIBUTION"
	FinancingOwnerContribution  FinancingEventType = "OWNER_CONTRIBUTION"
	FinancingOwnerDistribution  FinancingEventType = "OWNER_DISTRIBUTION"
)

func financingEventCategory(t FinancingEventType) (CashCategory, bool) {
	switch t {
	case FinancingLoanProceeds, FinancingLineOfCreditDraw:
		return CategoryLoanProceeds, true // inflow
	case FinancingEquityContribution, FinancingOwnerContribution:
		return CategoryOwnerContribution, true // inflow
	case FinancingLoanRepayment:
		return CategoryDebtService, false // outflow
	case FinancingOwnerDistribution:
		return CategoryOwnerDistribution, false // outflow
	default:
		return "", true
	}
}

// FinancingEvent is one caller-supplied explicit financing cash flow —
// loan proceeds, a line-of-credit draw, a loan repayment, an equity
// contribution, an owner contribution, or an owner distribution. This
// package never automatically creates a financing event to cover a
// funding gap — see the task's section 24's "do not automatically borrow"
// instruction; every FinancingEvent here is caller-initiated.
type FinancingEvent struct {
	ID          string             `json:"id"`
	Date        time.Time          `json:"date"`
	Amount      float64            `json:"amount"`
	Type        FinancingEventType `json:"type"`
	Description string             `json:"description,omitempty"`
	Certainty   Certainty          `json:"certainty,omitempty"`
	Basis       CashBasis          `json:"basis,omitempty"`
}

func buildFinancingEvents(events []FinancingEvent) ([]CashFlowEvent, []Issue) {
	var issues []Issue
	out := make([]CashFlowEvent, 0, len(events))
	for _, e := range events {
		cat, isInflow := financingEventCategory(e.Type)
		if cat == "" {
			issues = append(issues, Issue{Code: IssueInvalidCategory, Severity: SeverityWarning,
				Message: "financing event has an unrecognized type", SourceID: e.ID})
			continue
		}
		direction := DirectionOutflow
		if isInflow {
			direction = DirectionInflow
		}
		out = append(out, CashFlowEvent{
			ID:          "financing#" + e.ID,
			Date:        e.Date,
			Amount:      e.Amount,
			Direction:   direction,
			Category:    cat,
			Description: e.Description,
			SourceType:  SourceFinancingPlan,
			SourceID:    e.ID,
			Basis:       resolvedAPBasis(e.Basis),
			Certainty:   e.Certainty,
		})
	}
	return out, issues
}
