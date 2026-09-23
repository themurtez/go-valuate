package labor

// PayrollCashEvent is a minimal, portable shape this package's
// CashForecastPayrollEvents helper produces — deliberately field-
// compatible with accounting/cashforecast.CashFlowEvent's payroll-relevant
// fields (Date, Amount, Direction, Category, SourceType, SourceID, Basis)
// without importing accounting/cashforecast, per the task's section 31/62
// "define your OWN minimal portable source types" architectural rule. A
// caller integrating with accounting/cashforecast converts each
// PayrollCashEvent into a cashforecast.CashFlowEvent with
// Category: cashforecast.CategoryPayroll, Direction:
// cashforecast.DirectionOutflow — see cashforecastadapter_test.go for a
// worked example against the real cashforecast package.
type PayrollCashEvent struct {
	// Date is the actual cash pay date — copied directly from
	// PayrollRecord.PayDate.
	Date string `json:"date"`
	// Amount is the known cash obligation for this pay event: the sum of
	// every payroll component this package treats as employer-paid-out
	// cash (RegularPay + OvertimePay + BonusPay + CommissionPay +
	// OtherPay + EmployerTaxes + BenefitsCost + OtherEmployerCost). This
	// package has no gross-to-net calculation, so Amount is the payroll
	// register's full employer cost, not net employee take-home pay — see
	// the doc comment on CashForecastPayrollEvents for the explicit
	// boundary this implies.
	Amount float64 `json:"amount"`
	// SourceID is the originating PayrollRecord.ID, for provenance.
	SourceID string `json:"source_id"`
	// WorkerID is the originating PayrollRecord.WorkerID, for provenance.
	WorkerID string `json:"worker_id"`
}

// CashForecastPayrollEvents converts every validated, currency-matched
// PayrollRecord with a known PayDate into one PayrollCashEvent — the
// task's section 31/62 cashforecast integration boundary. This is
// intentionally narrow: it only ever reports the payroll register's full
// employer cost (gross pay + employer burden) on the record's own
// PayDate, since that is the only "known caller-supplied cash/pay-date
// amount" this package's input model actually carries.
//
// # Explicit boundary: this is not net pay
//
// A real payroll cash disbursement is usually net pay (after tax
// withholding) to employees, plus separate remittances for withheld/
// employer taxes and benefits, on their own separate dates. This package
// has no withholding calculation and no remittance-date model (see the
// package doc comment's non-goals) — RegularPay/OvertimePay/etc. are
// gross figures. A caller that needs net-pay-accurate cash events must
// supply its own true cash amounts; this helper is only appropriate when
// a caller treats "full employer payroll cost, disbursed on PayDate" as
// an acceptable approximation for cash forecasting, or when the caller's
// own PayrollRecord amounts already represent net cash paid (e.g. an
// outsourced payroll provider's single net-cash draft). Callers needing
// the split should build their own events from RegularPay+OvertimePay+...
// (employee net) and EmployerTaxes+BenefitsCost+OtherEmployerCost
// (employer remittance) separately rather than using this helper as-is.
func CashForecastPayrollEvents(records []PayrollRecord) []PayrollCashEvent {
	var out []PayrollCashEvent
	for _, r := range records {
		if r.PayDate.IsZero() {
			continue
		}
		if resolvedRecordType(r.RecordType) != RecordTypeNormal {
			continue
		}
		amount := r.RegularPay + r.OvertimePay + r.BonusPay + r.CommissionPay + r.OtherPay +
			r.EmployerTaxes + r.BenefitsCost + r.OtherEmployerCost
		out = append(out, PayrollCashEvent{
			Date:     r.PayDate.Format("2006-01-02"),
			Amount:   amount,
			SourceID: r.ID,
			WorkerID: r.WorkerID,
		})
	}
	return out
}
