# Payroll / labor analytics (`accounting/labor`)

`accounting/labor` provides deterministic payroll / labor analytics for
accountants, controllers, CFOs, and business-advisory workflows: total labor
cost and its bridge, cash-vs-expense basis, headcount and FTE, overtime and
contractor mix, direct-vs-indirect labor, department/location/cost-center
summaries, productivity metrics (revenue/gross-profit/EBITDA per employee or
FTE), labor-cost-vs-revenue trend signals, workforce movement, worker-date
consistency review, and payroll-register-to-GL reconciliation.

Like every other package in this repository, it contains **no** persistence,
HTTP/API, auth, UI, background jobs, QuickBooks/Xero integration, payroll-
provider API integration, AI/LLM, or tax logic — see [What this project
intentionally does not
contain](../README.md#what-this-project-intentionally-does-not-contain) in
the main README. Every exported function is pure (no I/O, no mutation of
caller-owned input, no package-global mutable state, no wall-clock reads) and
deterministic — see
[`determinism_test.go`](../accounting/labor/determinism_test.go) and
[`immutability_test.go`](../accounting/labor/immutability_test.go).

## Purpose and non-payroll-processing boundary

**This is not payroll processing.** It never calculates statutory payroll
deductions, gross-to-net pay, tax withholding, employer tax liability,
remittances, benefits administration, or T4/W-2 preparation. It has no
time-clock system, no HR workflow, no recruiting logic, and produces no
individual-employee performance score, productivity ranking, or termination
recommendation. Every payroll dollar amount is a fact the caller supplies;
this package never derives or estimates one component from another.

This package is not `accounting/ar` or `accounting/ap` — a payroll register
and a contractor labor ledger are not open-item receivables/payables, and
this package never reuses their shapes. It is not `analytics/*` — it performs
no valuation, forecasting, or DCF math; its only cross-reference to a
business's broader financial picture is the caller-supplied, intentionally
narrow `BusinessMetrics` (Revenue, GrossProfit, EBITDA) used for productivity
ratios.

Every generated message uses neutral, factual, non-legal, non-worker-decision
language — see [No payroll-law conclusions / Neutral
language](#no-payroll-law-conclusions--neutral-language).

## Package composition

```
caller-supplied payroll register / workforce roster / contractor spend
                          |
                accounting/labor
                          |
     period labor analytics, workforce/productivity metrics,
     cost mix, department/location summaries, payroll-vs-GL
     reconciliation, flags/trends
```

This package has **no** compile-time dependency on `accounting/ledger`,
`accounting/statements`, `accounting/ar`, `accounting/ap`,
`accounting/cashforecast`, or `analytics/*`. Integration with those packages
is demonstrated only through small, portable typed adapters and tests — see
[Financial-metric integration](#financial-metric-integration),
[Cashforecast boundary](#cashforecast-boundary), and
[`integration_test.go`](../accounting/labor/integration_test.go).

## Entry point

```go
func Calculate(in Input, policy Policy) Result
```

## Required vs optional inputs

`Input.Periods` is effectively required for any `PeriodSummary` output.
Every other field is optional and `Calculate` computes whatever subset of
the analysis the supplied input supports:

```go
type Input struct {
    Periods           []PeriodInfo
    Workers           []Worker
    PayrollRecords    []PayrollRecord
    ContractorRecords []ContractorLaborRecord
    WorkerPeriodFTEs  []WorkerPeriodFTE
    BusinessMetrics   []BusinessMetrics
    GLControls        []GLPayrollControl
}
```

### Two supported input levels

A caller can supply either (a) detailed worker/payroll-register data
(worker-level `PayrollRecord` rows), or (b) already-aggregated period-summary
data constructed as one synthetic `PayrollRecord` per period/category. Both
paths converge on the same internal period-level representation
(`PeriodSummary`) — this package never forces an integration to provide
true employee-level data to get period-level analytics.

## Period identity

```go
type PeriodInfo struct {
    Period    string
    StartDate time.Time
    EndDate   time.Time
    Days      int
}
```

This package never infers a fiscal calendar and never calls `time.Now()`.
An invalid period (empty label, missing dates, `EndDate` before `StartDate`)
produces `INVALID_PERIOD` and is excluded; a duplicate `Period` label
produces `DUPLICATE_PERIOD` and only the first occurrence is used. Periods
are always processed in chronological order regardless of input order.

## Result model

```go
type Result struct {
    Periods                []PeriodSummary
    DepartmentSummaries    []GroupSummary
    LocationSummaries      []GroupSummary
    CostCenterSummaries    []GroupSummary
    OvertimeByDepartment   []OvertimeByDepartment
    Trend                  TrendResult
    ReconciliationSummary  ReconciliationSummary
    PaySchedule            PayScheduleSummary
    WorkerDateFindings     []WorkerDateFinding
    DuplicateGroups        []DuplicateGroup
    Flags                  []Flag
    Issues                 []Issue
    Coverage               Coverage
    SchemaVersion          string
    FormulaVersion         string
}
```

Each `PeriodSummary` carries: `LaborCostBridge`, `PayrollBasis`,
`Headcount`, `WorkforceMovement`, `FTE`, `Overtime`, `ContractorMix`,
`DirectIndirectSplit`, `AverageLaborCost`, `VariableCompensation`,
`Productivity`, `Reconciliation`, `KeyWorkerConcentration`, and
`Provenance`.

## Labor cost bridge

```
RegularPay + OvertimePay + BonusPay + CommissionPay + OtherPay = GrossEmployeePay
GrossEmployeePay + EmployerTaxes + Benefits + OtherEmployerCosts = TotalEmployeeCost
TotalEmployeeCost + ContractorLabor = TotalLaborCost
```

Every component is a caller-supplied fact — this package never derives one
component from another (it never estimates `EmployerTaxes` from
`RegularPay`, for example). Employee deductions are never included as
employer labor cost: `PayrollRecord` has no employee-deduction field at all,
so `TotalEmployeeCost` is always exactly gross pay plus the three employer-
cost fields. `CostMix` reports the percentage-of-`TotalLaborCost` breakdown,
available only when `TotalLaborCost` is nonzero. `BurdenRate` is
`EmployerBurden / GrossEmployeePay`, an observed accounting ratio — not a
statutory payroll burden calculation.

## Cash vs expense basis

`PayrollBasis.ExpenseBasis` sums each `PayrollRecord`'s employee+employer
cost by its accrual `Period` field; `PayrollBasis.CashBasis` sums the same
total by the period whose `[StartDate, EndDate]` window contains `PayDate`.
This package never assumes labor expense equals cash paid in the same
period, and never invents an accrual adjustment to reconcile the two bases.
If a record's `PayDate` falls outside every supplied `PeriodInfo` window,
`CashBasis` is simply unavailable for that period while `ExpenseBasis`
remains available — see
[`fixtures.PayrollExpenseOnlyCashUnavailable`](../accounting/labor/fixtures/fixtures.go).

## Headcount

```go
type Headcount struct {
    Available             bool
    Approximated          bool
    BeginningHeadcount    int
    EndingHeadcount       int
    AverageHeadcount      float64
    ActiveEmployeeCount   int
    ActiveContractorCount int
}
```

Computed only when `Input.Workers` is supplied: `BeginningHeadcount` counts
workers active as of the period's `StartDate`, `EndingHeadcount` as of
`EndDate`, `AverageHeadcount` is the simple average of the two. Without a
roster, `Headcount` is unavailable **unless** the caller explicitly opts
into `Policy.PayrollActiveWorkerApproximation`, which approximates
`EndingHeadcount`/`ActiveEmployeeCount` from distinct `WorkerID`s appearing
in that period's payroll records (`Approximated: true`) — this package never
does this silently.

### Workforce movement and turnover

`WorkforceMovement` reports `Hires`/`Departures` within the period's date
window (from roster `HireDate`/`TerminationDate`), `NetHeadcountChange`, and
`TurnoverRate = Departures / AverageHeadcount` when roster date history is
sufficient. This package never infers a departure from a missing payroll
row, and never infers resignation vs. termination (no reason metadata
exists in `Worker`).

## FTE

```go
type FTESummary struct {
    Available bool
    Method    FTEMethod // HOURS_BASED | CALLER_SUPPLIED
    FTE       float64
}
```

Two supported paths, in precedence order:

1. **Caller-supplied FTE** (`Input.WorkerPeriodFTEs`) — summed across
   workers for the period when present; takes precedence when supplied.
2. **Hours-based FTE** — `FTE = total eligible hours / Policy.
   StandardFullTimeHoursPerPeriod`, computed only when
   `StandardFullTimeHoursPerPeriod` is positive and at least one payroll
   record in the period supplies hours.

This package never assumes 40 hours/week universally; without a caller-
supplied standard and without caller-supplied FTE values, `FTESummary` is
unavailable.

## Overtime

```go
type OvertimeSummary struct {
    Available             bool
    RegularHours          float64
    OvertimeHours         float64
    TotalHours            float64
    OvertimeHoursPercent  Value
    OvertimePayPercent    Value
}
```

Available only when at least one payroll record in the period supplies
hours. `OvertimeByDepartment` reports each department's share of total
overtime hours, for concentration review. This package never determines
whether overtime was legally required or correctly paid — see [No
payroll-law conclusions](#no-payroll-law-conclusions--neutral-language).

## Contractor labor

```go
type ContractorMix struct {
    ContractorLabor            float64
    EmployeeLabor              float64
    ContractorShareOfLaborCost Value
}
```

A caller must explicitly identify contractor labor via
`ContractorLaborRecord` — this package never automatically classifies
vendor/AP spend as contractor labor. It never makes a worker-classification
or legal conclusion about whether contractor labor is properly classified.

## Direct vs indirect labor

```go
type DirectIndirectSplit struct {
    DirectLaborCost       float64
    IndirectLaborCost     float64
    UnclassifiedLaborCost float64
    DirectLaborPercent    Value
}
```

Classified strictly from caller-supplied `PayrollRecord.LaborClass`
(`DIRECT`/`INDIRECT`/`UNCLASSIFIED`) — **never** inferred from department
name. A record with no explicit classification is `Unclassified` even if its
department name "sounds direct."

## Department / location / cost-center summaries

```go
type GroupSummary struct {
    GroupKey               string
    Headcount              int
    FTE                    Value
    GrossPay               float64
    EmployerBurden         float64
    ContractorLabor        float64
    TotalLaborCost         float64
    OvertimeHours          Value
    ShareOfTotalLaborCost  Value
}
```

A small, fixed set of grouping dimensions (department, location, cost
center) — not a generic OLAP cube. Sorted by normalized (case-folded) group
name, never Go map order.

## Productivity metrics

```go
type Productivity struct {
    RevenuePerEmployee            Value
    RevenuePerFTE                 Value
    GrossProfitPerEmployee        Value
    GrossProfitPerFTE             Value
    EBITDAPerEmployee             Value
    EBITDAPerFTE                  Value
    LaborCostPercentRevenue       Value
    EmployeeCostPercentRevenue    Value
    ContractorCostPercentRevenue  Value
    DirectLaborPercentRevenue     Value
}
```

Every ratio requires both its numerator (`BusinessMetrics`) and denominator
(headcount/FTE/revenue) available and nonzero; a zero or unavailable
denominator makes the ratio unavailable, never `Inf`/`NaN`.

## Trends

```go
type TrendResult struct {
    TotalLaborCost, GrossPay, EmployerBurden, ContractorLabor,
    Headcount, FTE, OvertimeShare, LaborCostPercentRevenue,
    RevenuePerFTE, GrossProfitPerFTE, EBITDAPerFTE TrendSeries

    RevenueGrowth, LaborCostGrowth, HeadcountGrowth, FTEGrowth,
    RevenuePerFTEChange, LaborCostPercentRevenueChange Value
}
```

Each series requires at least `Policy.MinimumTrendPeriods` (default 2)
periods with a value before being reported available. Growth/change figures
are simple adjacent-period (last-vs-second-to-last) comparisons — no future
prediction, no smoothing, no statistical model.

### Flags

Deterministic, threshold-driven review signals (never a composite workforce
score):

| Code | Meaning |
|---|---|
| `LABOR_COST_GROWTH_OUTPACES_REVENUE` | Labor cost growth exceeds revenue growth by more than `Policy.GrowthGapThreshold`. |
| `HEADCOUNT_GROWTH_WITH_REVENUE_DECLINE` | Headcount grew while revenue declined — a review indicator, never a staffing-quality conclusion. Disableable via `Policy.DisableHeadcountVsRevenueSignal`. |
| `HIGH_OVERTIME_SHARE` | Overtime hours or pay share exceeds its threshold for a period. |
| `OVERTIME_INCREASING` / `OVERTIME_SHARE_DECLINING` | Adjacent-period overtime share change exceeds the trend threshold, in either direction. |
| `HIGH_CONTRACTOR_SHARE` | Contractor share of total labor cost exceeds threshold. |
| `CONTRACTOR_SHARE_INCREASING` | Adjacent-period contractor share increased materially. |
| `LABOR_COST_PERCENT_REVENUE_INCREASING` / `..._IMPROVING` | Adjacent-period labor-cost-as-percent-of-revenue moved materially, either direction. |
| `REVENUE_PER_FTE_DECLINING` / `..._IMPROVING` | Adjacent-period revenue-per-FTE moved materially, either direction. |
| `PAYROLL_GL_MISMATCH` / `PAYROLL_GL_RECONCILED` | Payroll-register-vs-GL reconciliation status. |
| `WORKER_DATE_INCONSISTENCY` | A material worker-date finding (see below). |
| `POSSIBLE_DUPLICATE_PAYROLL_RECORD` | Two or more payroll records share worker, pay date, and every amount. |

## Payroll burden

`EmployerBurden = EmployerTaxes + Benefits + OtherEmployerCosts`.
`BurdenRate = EmployerBurden / GrossEmployeePay`. This is an observed
accounting ratio computed from caller-supplied figures, never a statutory
payroll burden calculation.

## GL reconciliation

```go
type ReconciliationSummary struct {
    Status     ReconciliationStatus // RECONCILED | RECONCILED_WITH_DIFFERENCES | UNRECONCILED | UNAVAILABLE
    Components []ComponentReconciliation
}
type ComponentReconciliation struct {
    Component  string
    Register   Value
    GL         Value
    Difference Value
    Tolerance  float64
    Reconciled bool
}
```

Six components are compared independently whenever both a register total
and a `GLPayrollControl` value are available for a period: `gross_wages`,
`employer_taxes`, `benefits`, `contractor_labor`, `other_labor`,
`total_labor_cost`. Component-level differences are **never** hidden behind
a matching total — a mismatched component still reports `Reconciled: false`
even if the overall total happens to match. Tolerance is caller-configured
(`Policy.ReconciliationTolerance`, falling back to
`Policy.MaterialPayrollDifference`) — this package never invents an
accounting materiality threshold of its own. This package never posts
adjustments.

## Financial-metric integration

`BusinessMetrics` (`Revenue`, `GrossProfit`, `EBITDA`, each a `Value`) is
this package's only cross-reference to a business's broader financial
picture, used solely for [Productivity metrics](#productivity-metrics) and
the labor-cost-vs-revenue trend signal. A caller with existing
financial/statement analytics converts its own output into this shape with
a simple one-to-one field mapping — see
`TestIntegration_FinancialMetricsAdapter` in
[`integration_test.go`](../accounting/labor/integration_test.go). This
package never requires a `financial.FinancialDataset`.

An optional `GLControlFromLedgerBalances` helper
([`ledgeradapter.go`](../accounting/labor/ledgeradapter.go)) aggregates
caller-identified `accounting/ledger`-style account balances into a
`GLPayrollControl`, using only a caller-supplied account-to-category
mapping — payroll accounts are never inferred from account names.

## Cashforecast boundary

`CashForecastPayrollEvents` ([`cashforecastadapter.go`](../accounting/labor/cashforecastadapter.go))
converts every payroll record with a known `PayDate` into a minimal,
portable `PayrollCashEvent` (field-compatible with
`accounting/cashforecast.CashFlowEvent`'s payroll-relevant fields), which a
caller then maps into a real `cashforecast.CashFlowEvent` with
`Category: cashforecast.CategoryPayroll`. This package never imports
`accounting/cashforecast`.

**Explicit boundary: this is not net pay.** `PayrollCashEvent.Amount` is the
payroll register's full employer cost (gross pay + employer burden) on the
record's own `PayDate` — the only "known cash obligation" this package's
input model carries. A real payroll disbursement is usually net pay to
employees plus separate tax/benefit remittances on their own dates; this
package has no withholding calculation and no remittance-date model, so a
caller needing net-pay-accurate cash events must supply its own true cash
amounts rather than relying on this helper as-is.

## Worker-level output boundary

`Worker` deliberately carries only opaque identity fields (`WorkerID`,
`WorkerType`, `Active`, dates, department/location/cost-center,
`KeyWorker`) — no race, religion, disability, health, political affiliation,
sexual orientation, or other protected/sensitive attribute. The primary
product is business-level and grouped analytics; this package produces no
best-employee/worst-employee ranking, no productivity ranking, no
performance score, and no termination recommendation. Individual `Worker`/
`PayrollRecord`/`ContractorLaborRecord` identity is preserved only for
provenance/reconciliation (`PeriodSummary.Provenance`), never for
individual output.

`KeyWorkerConcentration` is the one narrow exception: only computed when a
caller explicitly marks a `Worker.KeyWorker = true`, and it reports only a
plain labor-cost share — never an inferred "key employee risk" score or
operational-dependency conclusion.

## No payroll-law conclusions / Neutral language

Generated messages **never** claim: overtime violation, tax underpayment,
incorrect withholding, misclassification, wage-law breach, or benefits
noncompliance — this package has no jurisdiction-specific legal rules. It
can say factual, threshold-relative things like *"overtime pay represents
18% of gross pay, above the caller-configured 12% review threshold."*
Generated output never recommends firing, terminating, laying off,
disciplining, promoting, or demoting a worker, and never ranks individual
workers. Enforced by a permanent regression test,
[`safety_test.go`](../accounting/labor/safety_test.go), which scans every
generated `Flag`/`Issue`/`WorkerDateFinding` message for a fixed list of
prohibited legal-conclusion and worker-decision terms.

### Worker-date consistency

```go
type WorkerDateFinding struct {
    RecordID              string
    WorkerID               string
    Reason                 WorkerDateFindingReason // PAY_BEFORE_HIRE | PAY_AFTER_TERMINATION
    DaysAfterTermination   int
    Severity                FlagSeverity
    Message                 string
}
```

A payroll record dated before a worker's `HireDate`, or after
`TerminationDate`, produces a finding — never a conclusion about improper
payment. A final pay within `Policy.PostTerminationPayGraceDays` (default
14) of termination is `Info` severity (a legitimate final pay is common); one
materially beyond the grace period is `Warning` severity and also produces
a `WORKER_DATE_INCONSISTENCY` flag and an `INVALID_WORKER_DATES` issue.

### Duplicate detection

Exact duplicate `PayrollRecord.ID`s produce `DUPLICATE_PAYROLL_RECORD` (an
`Issue`; only the first occurrence is used downstream). Separately, two
*distinct* IDs sharing the same worker, pay date, and every component
amount produce a conservative `POSSIBLE_DUPLICATE_PAYROLL_RECORD` finding —
labeled "possible," never asserted as a duplicate payment.

## Coverage model

```go
type Coverage struct {
    PayrollRecordsSupplied      bool
    WorkerRosterSupplied        bool
    ContractorDataSupplied      bool
    HoursCoveragePercent        Value
    DepartmentCoveragePercent   Value
    LocationCoveragePercent     Value
    FTEAvailable                bool
    FinancialMetricsAvailable   bool
    GLReconciliationAvailable   bool
}
```

Every figure here is a factual count or percentage — never an opaque
completeness score. "Zero contractor spend" (an explicit zero-amount
`ContractorLaborRecord`) is distinguished from "contractor data not
supplied" (`ContractorDataSupplied: false`) throughout this package.

## Issues vs Flags

- **`Issue`** — a problem with the `Input`/`Policy` this package was given
  (invalid period, duplicate worker, non-finite amount, mixed currency,
  invalid policy threshold, ...). Never a business-result signal.
- **`Flag`** — a deterministic labor-analytics review signal (high
  overtime share, labor cost growth outpacing revenue, payroll-vs-GL
  mismatch, ...). Never an input/validation problem.

`HasErrors(issues []Issue) bool` is this package's own local copy of the
repo-wide convention — intentionally duplicated rather than shared, per
`financial/adjustments.HasErrors`'s documented rationale.

## JSON and determinism

Every exported type uses explicit snake_case JSON tags. `Calculate` never
produces NaN/Inf — non-finite policy thresholds and record amounts are
rejected/excluded as structured `Issue`s, never propagated. Ordering is
fully deterministic: periods chronologically, departments/locations/cost
centers by normalized name, flags by `FlagCode` declaration order then
period, issues by `IssueCode` declaration order then record/worker/period —
never Go map order. See
[`determinism_test.go`](../accounting/labor/determinism_test.go),
[`roundtrip_test.go`](../accounting/labor/roundtrip_test.go), and
[`immutability_test.go`](../accounting/labor/immutability_test.go).

## Versioning

- `SchemaVersion` — the shape of `Worker`, `PayrollRecord`,
  `ContractorLaborRecord`, `PeriodInfo`, `BusinessMetrics`,
  `GLPayrollControl`, `Policy`, `PeriodSummary`, `GroupSummary`,
  `TrendResult`, `ReconciliationSummary`, `Flag`, `Issue`, `Coverage`, and
  `Result`.
- `FormulaVersion` — the labor-cost bridge, cash/expense basis, headcount/
  FTE, overtime, contractor mix, grouping, productivity, trend, workforce
  movement, worker-date consistency, duplicate detection, and GL
  reconciliation calculation semantics, and every flag-trigger rule.

Both are echoed on `Result.SchemaVersion`/`Result.FormulaVersion`.

## Limitations

- No hours-of-work legal-compliance analysis (overtime eligibility rules,
  meal/rest break rules) — analytics only.
- No worker-classification legal test (employee vs. contractor) —
  descriptive mix/share only.
- No net-pay/withholding model — `CashForecastPayrollEvents` reports gross
  employer cost, not net cash to employees, unless the caller's own
  `PayrollRecord` amounts already represent net cash.
- Pay-schedule frequency detection ([`payschedule.go`](../accounting/labor/payschedule.go))
  is a descriptive summary of observed pay-date spacing, never an assertion
  of employer policy.
- No composite workforce/health score anywhere in this package's output.

## Explicit non-goals

This package does not implement: payroll calculations, gross-to-net, tax
withholding, employer tax rules, payroll remittances, payroll filing,
T4/W-2 preparation, benefits administration, time-clock calculations,
employment-law compliance, worker-classification legal tests, recruiting,
individual performance analytics, termination recommendations, payroll
payment execution, HRIS synchronization, or AI/LLM workforce
recommendations.
