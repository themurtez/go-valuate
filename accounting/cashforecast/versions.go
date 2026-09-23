package cashforecast

// SchemaVersion identifies this package's public result/schema contract:
// the shape of CashFlowEvent, OpeningCash, CashAccount, every recurring
// rule and AR/AP plan type, CreditFacility, Scenario, WeeklyForecast,
// ScenarioResult, Coverage, and Result. Bump whenever any of that shape
// changes in a way that could make a historical persisted Result not
// reproduce identically under new code — see the repository README's
// versioning-strategy section, which every sibling package's SchemaVersion
// (ar.SchemaVersion, ap.SchemaVersion, ledger.SchemaVersion, ...) already
// follows.
const SchemaVersion = "1.0.0"

// FormulaVersion identifies this package's fixed calculation semantics:
// week-boundary assignment (weeks.go), the weekly rollforward and
// category-breakdown aggregation (calculate.go), AR/AP scheduling
// adapters (aradapter.go/apadapter.go), recurring-event generation
// (recurring.go), scenario transformation and delta-vs-base calculation
// (scenario.go), liquidity/funding-gap/runway/required-funding formulas
// (liquidity.go), and coverage/completeness (coverage.go). Bump this
// whenever any of that changes in a way that could make a historical
// Result not reproduce identically under new code.
const FormulaVersion = "1.0.0"
