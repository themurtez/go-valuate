// Package fixtures provides synthetic accounting/labor data for tests and
// examples: a set of small, hand-built Worker/PayrollRecord/
// ContractorLaborRecord/PeriodInfo scenarios covering the situations
// accounting/labor's tests exercise. Nothing here is real payroll or
// workforce data — every figure is invented for illustration, mirroring
// accounting/ar/fixtures, accounting/ap/fixtures, and
// accounting/cashforecast/fixtures' identical synthetic-data convention.
package fixtures

import (
	"time"

	"github.com/themurtez/go-valuate/accounting/labor"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func datePtr(s string) *time.Time {
	t := date(s)
	return &t
}

// TwoMonthPeriods returns two chronological monthly PeriodInfo entries
// covering 2025-05 and 2025-06 — the standard period pair most fixtures
// below pair with.
func TwoMonthPeriods() []labor.PeriodInfo {
	return []labor.PeriodInfo{
		{Period: "2025-05", StartDate: date("2025-05-01"), EndDate: date("2025-05-31"), Days: 31},
		{Period: "2025-06", StartDate: date("2025-06-01"), EndDate: date("2025-06-30"), Days: 30},
	}
}

// ThreeMonthPeriods returns three chronological monthly periods
// (2025-04, 2025-05, 2025-06) for trend-requiring scenarios.
func ThreeMonthPeriods() []labor.PeriodInfo {
	return []labor.PeriodInfo{
		{Period: "2025-04", StartDate: date("2025-04-01"), EndDate: date("2025-04-30"), Days: 30},
		{Period: "2025-05", StartDate: date("2025-05-01"), EndDate: date("2025-05-31"), Days: 31},
		{Period: "2025-06", StartDate: date("2025-06-01"), EndDate: date("2025-06-30"), Days: 30},
	}
}

// StableServiceBusiness returns a small, stable service business: 3
// employees, flat pay across two periods, no overtime, no contractors —
// the baseline "nothing unusual" scenario.
func StableServiceBusiness() ([]labor.Worker, []labor.PayrollRecord) {
	workers := []labor.Worker{
		{WorkerID: "W1", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2023-01-15"), Department: "Operations", Location: "HQ"},
		{WorkerID: "W2", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2023-03-01"), Department: "Operations", Location: "HQ"},
		{WorkerID: "W3", WorkerType: labor.WorkerTypeOwnerEmployee, Active: true, HireDate: datePtr("2020-01-01"), Department: "Management", Location: "HQ"},
	}
	records := []labor.PayrollRecord{
		{ID: "P-05-W1", WorkerID: "W1", Period: "2025-05", PayDate: date("2025-05-31"), RegularPay: 5000, EmployerTaxes: 400, BenefitsCost: 300, Department: "Operations", Location: "HQ", Currency: "USD"},
		{ID: "P-05-W2", WorkerID: "W2", Period: "2025-05", PayDate: date("2025-05-31"), RegularPay: 4500, EmployerTaxes: 360, BenefitsCost: 300, Department: "Operations", Location: "HQ", Currency: "USD"},
		{ID: "P-05-W3", WorkerID: "W3", Period: "2025-05", PayDate: date("2025-05-31"), RegularPay: 8000, EmployerTaxes: 600, BenefitsCost: 300, Department: "Management", Location: "HQ", Currency: "USD"},
		{ID: "P-06-W1", WorkerID: "W1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, BenefitsCost: 300, Department: "Operations", Location: "HQ", Currency: "USD"},
		{ID: "P-06-W2", WorkerID: "W2", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 4500, EmployerTaxes: 360, BenefitsCost: 300, Department: "Operations", Location: "HQ", Currency: "USD"},
		{ID: "P-06-W3", WorkerID: "W3", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 8000, EmployerTaxes: 600, BenefitsCost: 300, Department: "Management", Location: "HQ", Currency: "USD"},
	}
	return workers, records
}

// LaborIntensiveBusiness returns a business where labor cost dominates:
// many hourly workers with hours reported, moderate overtime, no
// contractors.
func LaborIntensiveBusiness() ([]labor.Worker, []labor.PayrollRecord) {
	var workers []labor.Worker
	var records []labor.PayrollRecord
	for i := 1; i <= 8; i++ {
		id := "LW" + itoa(i)
		workers = append(workers, labor.Worker{WorkerID: id, WorkerType: labor.WorkerTypeEmployee, Active: true,
			HireDate: datePtr("2024-01-01"), Department: "Production", Location: "Plant"})
		records = append(records, labor.PayrollRecord{
			ID: "LP-05-" + id, WorkerID: id, Period: "2025-05", PayDate: date("2025-05-31"),
			RegularPay: 3200, OvertimePay: 400, EmployerTaxes: 280, BenefitsCost: 200,
			HoursRegular: labor.AvailableValue(160), HoursOvertime: labor.AvailableValue(12),
			Department: "Production", Location: "Plant", Currency: "USD",
		})
	}
	return workers, records
}

// GrowingRevenueProportionalLabor returns two periods where both revenue
// and labor cost grow at a similar rate — used for labor-cost-vs-revenue
// trend tests where no gap should be flagged.
func GrowingRevenueProportionalLabor() ([]labor.PayrollRecord, []labor.BusinessMetrics) {
	records := []labor.PayrollRecord{
		{ID: "GR-04-W1", WorkerID: "W1", Period: "2025-04", PayDate: date("2025-04-30"), RegularPay: 10000, EmployerTaxes: 800, BenefitsCost: 500, Currency: "USD"},
		{ID: "GR-05-W1", WorkerID: "W1", Period: "2025-05", PayDate: date("2025-05-31"), RegularPay: 10500, EmployerTaxes: 840, BenefitsCost: 500, Currency: "USD"},
		{ID: "GR-06-W1", WorkerID: "W1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 11000, EmployerTaxes: 880, BenefitsCost: 500, Currency: "USD"},
	}
	metrics := []labor.BusinessMetrics{
		{Period: "2025-04", Revenue: labor.AvailableValue(200000)},
		{Period: "2025-05", Revenue: labor.AvailableValue(210000)},
		{Period: "2025-06", Revenue: labor.AvailableValue(220000)},
	}
	return records, metrics
}

// LaborGrowthOutpacingRevenue returns three periods where labor cost
// grows much faster than revenue — should trigger
// FlagLaborCostGrowthOutpacesRevenue.
func LaborGrowthOutpacingRevenue() ([]labor.PayrollRecord, []labor.BusinessMetrics) {
	records := []labor.PayrollRecord{
		{ID: "LG-04-W1", WorkerID: "W1", Period: "2025-04", PayDate: date("2025-04-30"), RegularPay: 10000, EmployerTaxes: 800, BenefitsCost: 500, Currency: "USD"},
		{ID: "LG-05-W1", WorkerID: "W1", Period: "2025-05", PayDate: date("2025-05-31"), RegularPay: 13000, EmployerTaxes: 1040, BenefitsCost: 500, Currency: "USD"},
		{ID: "LG-06-W1", WorkerID: "W1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 17000, EmployerTaxes: 1360, BenefitsCost: 500, Currency: "USD"},
	}
	metrics := []labor.BusinessMetrics{
		{Period: "2025-04", Revenue: labor.AvailableValue(200000)},
		{Period: "2025-05", Revenue: labor.AvailableValue(204000)},
		{Period: "2025-06", Revenue: labor.AvailableValue(208000)},
	}
	return records, metrics
}

// RevenueDeclineHeadcountGrowth returns a roster/metrics pair where
// headcount grows period-over-period while revenue declines — should
// trigger FlagHeadcountGrowthWithRevenueDecline.
func RevenueDeclineHeadcountGrowth() ([]labor.Worker, []labor.BusinessMetrics) {
	workers := []labor.Worker{
		{WorkerID: "RD-W1", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2024-01-01")},
		{WorkerID: "RD-W2", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2024-06-01")},
		{WorkerID: "RD-W3", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2025-06-05")},
	}
	metrics := []labor.BusinessMetrics{
		{Period: "2025-05", Revenue: labor.AvailableValue(300000)},
		{Period: "2025-06", Revenue: labor.AvailableValue(260000)},
	}
	return workers, metrics
}

// HighOvertime returns payroll records with a high overtime hours share
// (above the 10% default threshold) for one period.
func HighOvertime() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "HO-1", WorkerID: "HW1", Period: "2025-06", PayDate: date("2025-06-30"),
			RegularPay: 3000, OvertimePay: 900, EmployerTaxes: 300, BenefitsCost: 200,
			HoursRegular: labor.AvailableValue(150), HoursOvertime: labor.AvailableValue(30), Currency: "USD"},
		{ID: "HO-2", WorkerID: "HW2", Period: "2025-06", PayDate: date("2025-06-30"),
			RegularPay: 3000, OvertimePay: 850, EmployerTaxes: 300, BenefitsCost: 200,
			HoursRegular: labor.AvailableValue(150), HoursOvertime: labor.AvailableValue(28), Currency: "USD"},
	}
}

// DecliningOvertime returns two periods of payroll records where overtime
// share falls from a high May level to a much lower June level.
func DecliningOvertime() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "DO-05-1", WorkerID: "DW1", Period: "2025-05", PayDate: date("2025-05-31"),
			RegularPay: 3000, OvertimePay: 900, HoursRegular: labor.AvailableValue(150), HoursOvertime: labor.AvailableValue(30), Currency: "USD"},
		{ID: "DO-06-1", WorkerID: "DW1", Period: "2025-06", PayDate: date("2025-06-30"),
			RegularPay: 3000, OvertimePay: 100, HoursRegular: labor.AvailableValue(150), HoursOvertime: labor.AvailableValue(3), Currency: "USD"},
	}
}

// HighContractorShare returns a period with contractor labor well above
// the 30% default threshold of total labor cost.
func HighContractorShare() ([]labor.PayrollRecord, []labor.ContractorLaborRecord) {
	payroll := []labor.PayrollRecord{
		{ID: "HC-1", WorkerID: "HCW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, Currency: "USD"},
	}
	contractors := []labor.ContractorLaborRecord{
		{ID: "HC-C1", ContractorID: "C1", Period: "2025-06", Date: date("2025-06-15"), Amount: 4000, Currency: "USD"},
	}
	return payroll, contractors
}

// ContractorShareIncreasing returns two periods where contractor share of
// total labor cost rises materially.
func ContractorShareIncreasing() ([]labor.PayrollRecord, []labor.ContractorLaborRecord) {
	payroll := []labor.PayrollRecord{
		{ID: "CI-05", WorkerID: "CIW1", Period: "2025-05", PayDate: date("2025-05-31"), RegularPay: 8000, EmployerTaxes: 640, Currency: "USD"},
		{ID: "CI-06", WorkerID: "CIW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 8000, EmployerTaxes: 640, Currency: "USD"},
	}
	contractors := []labor.ContractorLaborRecord{
		{ID: "CI-C-05", ContractorID: "C1", Period: "2025-05", Date: date("2025-05-15"), Amount: 500, Currency: "USD"},
		{ID: "CI-C-06", ContractorID: "C1", Period: "2025-06", Date: date("2025-06-15"), Amount: 4000, Currency: "USD"},
	}
	return payroll, contractors
}

// MultiDepartmentCompany returns payroll records spread across three
// departments for grouping tests.
func MultiDepartmentCompany() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "MD-1", WorkerID: "MW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, Department: "Sales", Currency: "USD"},
		{ID: "MD-2", WorkerID: "MW2", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 6000, EmployerTaxes: 480, Department: "Engineering", Currency: "USD"},
		{ID: "MD-3", WorkerID: "MW3", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 4500, EmployerTaxes: 360, Department: "Support", Currency: "USD"},
		{ID: "MD-4", WorkerID: "MW4", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 6200, EmployerTaxes: 496, Department: "Engineering", Currency: "USD"},
	}
}

// MultiLocationCompany returns payroll records spread across two
// locations for grouping tests.
func MultiLocationCompany() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "ML-1", WorkerID: "MLW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, Location: "New York", Currency: "USD"},
		{ID: "ML-2", WorkerID: "MLW2", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5200, EmployerTaxes: 416, Location: "Austin", Currency: "USD"},
		{ID: "ML-3", WorkerID: "MLW3", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 4800, EmployerTaxes: 384, Location: "Austin", Currency: "USD"},
	}
}

// DirectIndirectMix returns payroll records with a mix of DIRECT,
// INDIRECT, and unclassified labor.
func DirectIndirectMix() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "DI-1", WorkerID: "DIW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 6000, EmployerTaxes: 480, LaborClass: labor.LaborClassDirect, Currency: "USD"},
		{ID: "DI-2", WorkerID: "DIW2", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, LaborClass: labor.LaborClassIndirect, Currency: "USD"},
		{ID: "DI-3", WorkerID: "DIW3", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 3000, EmployerTaxes: 240, Currency: "USD"},
	}
}

// ReconciledPayrollAndGL returns a period's payroll records plus a
// GLPayrollControl whose totals match exactly.
func ReconciledPayrollAndGL() ([]labor.PayrollRecord, labor.GLPayrollControl) {
	records := []labor.PayrollRecord{
		{ID: "RG-1", WorkerID: "RGW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 10000, EmployerTaxes: 800, BenefitsCost: 500, Currency: "USD"},
	}
	gl := labor.GLPayrollControl{
		Period:         "2025-06",
		GrossWages:     labor.AvailableValue(10000),
		EmployerTaxes:  labor.AvailableValue(800),
		Benefits:       labor.AvailableValue(500),
		TotalLaborCost: labor.AvailableValue(11300),
	}
	return records, gl
}

// PayrollGLMismatch returns a period's payroll records plus a
// GLPayrollControl with a material mismatch on gross wages.
func PayrollGLMismatch() ([]labor.PayrollRecord, labor.GLPayrollControl) {
	records := []labor.PayrollRecord{
		{ID: "PM-1", WorkerID: "PMW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 10000, EmployerTaxes: 800, BenefitsCost: 500, Currency: "USD"},
	}
	gl := labor.GLPayrollControl{
		Period:         "2025-06",
		GrossWages:     labor.AvailableValue(15000), // mismatch
		EmployerTaxes:  labor.AvailableValue(800),
		Benefits:       labor.AvailableValue(500),
		TotalLaborCost: labor.AvailableValue(16300),
	}
	return records, gl
}

// WorkerPaidAfterTerminationWithinGrace returns a worker terminated
// 2025-06-10 with a final payroll record paid 2025-06-17 (7 days after,
// within the default 14-day grace period).
func WorkerPaidAfterTerminationWithinGrace() ([]labor.Worker, []labor.PayrollRecord) {
	workers := []labor.Worker{
		{WorkerID: "TG-W1", WorkerType: labor.WorkerTypeEmployee, Active: false,
			HireDate: datePtr("2023-01-01"), TerminationDate: datePtr("2025-06-10")},
	}
	records := []labor.PayrollRecord{
		{ID: "TG-P1", WorkerID: "TG-W1", Period: "2025-06", PayDate: date("2025-06-17"), RegularPay: 2000, EmployerTaxes: 160, Currency: "USD"},
	}
	return workers, records
}

// WorkerPaidMateriallyAfterTermination returns a worker terminated
// 2025-04-01 with a payroll record paid 2025-06-30 — far beyond any
// reasonable grace period.
func WorkerPaidMateriallyAfterTermination() ([]labor.Worker, []labor.PayrollRecord) {
	workers := []labor.Worker{
		{WorkerID: "TM-W1", WorkerType: labor.WorkerTypeEmployee, Active: false,
			HireDate: datePtr("2022-01-01"), TerminationDate: datePtr("2025-04-01")},
	}
	records := []labor.PayrollRecord{
		{ID: "TM-P1", WorkerID: "TM-W1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, Currency: "USD"},
	}
	return workers, records
}

// MissingHours returns payroll records with no hours data at all — used
// to exercise "overtime/FTE unavailable" paths.
func MissingHours() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "MH-1", WorkerID: "MHW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 4000, EmployerTaxes: 320, Currency: "USD"},
	}
}

// ZeroRevenueBusiness returns a period's payroll records paired with a
// BusinessMetrics entry reporting zero (but available) revenue — used to
// exercise productivity-ratio "unavailable, not Inf" handling.
func ZeroRevenueBusiness() ([]labor.PayrollRecord, []labor.BusinessMetrics) {
	records := []labor.PayrollRecord{
		{ID: "ZR-1", WorkerID: "ZRW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 3000, EmployerTaxes: 240, Currency: "USD"},
	}
	metrics := []labor.BusinessMetrics{
		{Period: "2025-06", Revenue: labor.AvailableValue(0)},
	}
	return records, metrics
}

// ZeroWorkforce returns an empty worker roster and empty payroll set —
// the degenerate "nothing supplied" case.
func ZeroWorkforce() ([]labor.Worker, []labor.PayrollRecord) {
	return nil, nil
}

// MixedCurrencyInvalid returns payroll records spanning USD and EUR with
// no reporting-currency resolution possible cleanly — exercises
// IssueMixedCurrency.
func MixedCurrencyInvalid() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "MC-1", WorkerID: "MCW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 4000, EmployerTaxes: 320, Currency: "USD"},
		{ID: "MC-2", WorkerID: "MCW2", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 3500, EmployerTaxes: 280, Currency: "EUR"},
	}
}

// FTECalculationCase returns payroll records with hours plus a standard
// full-time-hours policy value sufficient to compute hours-based FTE.
func FTECalculationCase() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "FC-1", WorkerID: "FCW1", Period: "2025-06", PayDate: date("2025-06-30"),
			RegularPay: 4000, HoursRegular: labor.AvailableValue(173.33), Currency: "USD"},
		{ID: "FC-2", WorkerID: "FCW2", Period: "2025-06", PayDate: date("2025-06-30"),
			RegularPay: 2000, HoursRegular: labor.AvailableValue(86.67), Currency: "USD"},
	}
}

// OwnerOperatedBusiness returns a single OWNER_EMPLOYEE worker with
// payroll records — the smallest realistic business shape.
func OwnerOperatedBusiness() ([]labor.Worker, []labor.PayrollRecord) {
	workers := []labor.Worker{
		{WorkerID: "OO-W1", WorkerType: labor.WorkerTypeOwnerEmployee, Active: true, HireDate: datePtr("2019-01-01")},
	}
	records := []labor.PayrollRecord{
		{ID: "OO-P1", WorkerID: "OO-W1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 9000, EmployerTaxes: 700, BenefitsCost: 400, Currency: "USD"},
	}
	return workers, records
}

// BonusHeavyPeriod returns a period where bonus pay is a large share of
// gross pay — used for variable-compensation tests.
func BonusHeavyPeriod() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "BH-1", WorkerID: "BHW1", Period: "2025-06", PayDate: date("2025-06-30"),
			RegularPay: 4000, BonusPay: 6000, EmployerTaxes: 800, Currency: "USD"},
	}
}

// PayrollCashScheduleAvailable returns payroll records with distinct,
// evenly-spaced (biweekly) pay dates spanning several pay runs — suitable
// for pay-schedule analytics and the cashforecast adapter.
func PayrollCashScheduleAvailable() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "PS-1", WorkerID: "PSW1", Period: "2025-05", PayDate: date("2025-05-02"), RegularPay: 2500, EmployerTaxes: 200, Currency: "USD"},
		{ID: "PS-2", WorkerID: "PSW1", Period: "2025-05", PayDate: date("2025-05-16"), RegularPay: 2500, EmployerTaxes: 200, Currency: "USD"},
		{ID: "PS-3", WorkerID: "PSW1", Period: "2025-05", PayDate: date("2025-05-30"), RegularPay: 2500, EmployerTaxes: 200, Currency: "USD"},
		{ID: "PS-4", WorkerID: "PSW1", Period: "2025-06", PayDate: date("2025-06-13"), RegularPay: 2500, EmployerTaxes: 200, Currency: "USD"},
	}
}

// PayrollExpenseOnlyCashUnavailable returns payroll records whose PayDate
// values fall outside the supplied PeriodInfo windows entirely (so
// CashBasis cannot resolve a period label for them), demonstrating the
// "expense basis available, cash basis unavailable" split.
func PayrollExpenseOnlyCashUnavailable() ([]labor.PeriodInfo, []labor.PayrollRecord) {
	periods := []labor.PeriodInfo{
		{Period: "2025-06", StartDate: date("2025-06-01"), EndDate: date("2025-06-30"), Days: 30},
	}
	records := []labor.PayrollRecord{
		// Accrued to June but not actually paid until a much later date
		// outside any supplied period window.
		{ID: "EO-1", WorkerID: "EOW1", Period: "2025-06", PayDate: date("2025-08-15"), RegularPay: 4000, EmployerTaxes: 320, Currency: "USD"},
	}
	return periods, records
}

// DuplicatePayrollRecords returns two distinct PayrollRecord IDs sharing
// the same worker, pay date, and component amounts — exercises
// POSSIBLE_DUPLICATE_PAYROLL_RECORD.
func DuplicatePayrollRecords() []labor.PayrollRecord {
	return []labor.PayrollRecord{
		{ID: "DUP-1", WorkerID: "DUPW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 3000, EmployerTaxes: 240, Currency: "USD"},
		{ID: "DUP-2", WorkerID: "DUPW1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 3000, EmployerTaxes: 240, Currency: "USD"},
	}
}

// KeyWorkerScenario returns a roster with one worker marked KeyWorker
// plus payroll records, for KeyWorkerConcentration tests.
func KeyWorkerScenario() ([]labor.Worker, []labor.PayrollRecord) {
	workers := []labor.Worker{
		{WorkerID: "KW-1", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2020-01-01"), KeyWorker: true},
		{WorkerID: "KW-2", WorkerType: labor.WorkerTypeEmployee, Active: true, HireDate: datePtr("2022-01-01")},
	}
	records := []labor.PayrollRecord{
		{ID: "KW-P1", WorkerID: "KW-1", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 12000, EmployerTaxes: 960, Currency: "USD"},
		{ID: "KW-P2", WorkerID: "KW-2", Period: "2025-06", PayDate: date("2025-06-30"), RegularPay: 5000, EmployerTaxes: 400, Currency: "USD"},
	}
	return workers, records
}

// LargePopulation generates a synthetic large-scale dataset for
// benchmarking: numWorkers workers, one payroll record per worker per
// period across numPeriods months, spread across numDepartments
// departments.
func LargePopulation(numWorkers, numPeriods, numDepartments int) ([]labor.PeriodInfo, []labor.Worker, []labor.PayrollRecord) {
	var periods []labor.PeriodInfo
	start := date("2020-01-01")
	for i := 0; i < numPeriods; i++ {
		s := start.AddDate(0, i, 0)
		e := s.AddDate(0, 1, -1)
		periods = append(periods, labor.PeriodInfo{
			Period: s.Format("2006-01"), StartDate: s, EndDate: e, Days: int(e.Sub(s).Hours()/24) + 1,
		})
	}

	var workers []labor.Worker
	for i := 0; i < numWorkers; i++ {
		id := "GW" + itoa(i)
		dept := "Dept" + itoa(i%numDepartments)
		workers = append(workers, labor.Worker{
			WorkerID: id, WorkerType: labor.WorkerTypeEmployee, Active: true,
			HireDate: datePtr("2019-01-01"), Department: dept, Location: "HQ", CostCenter: dept,
		})
	}

	var records []labor.PayrollRecord
	for _, p := range periods {
		for i := 0; i < numWorkers; i++ {
			id := "GW" + itoa(i)
			dept := "Dept" + itoa(i%numDepartments)
			records = append(records, labor.PayrollRecord{
				ID:            p.Period + "-" + id,
				WorkerID:      id,
				Period:        p.Period,
				PayDate:       p.EndDate,
				RegularPay:    4000,
				OvertimePay:   200,
				EmployerTaxes: 320,
				BenefitsCost:  150,
				HoursRegular:  labor.AvailableValue(160),
				HoursOvertime: labor.AvailableValue(8),
				Department:    dept,
				Location:      "HQ",
				CostCenter:    dept,
				Currency:      "USD",
			})
		}
	}

	return periods, workers, records
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
