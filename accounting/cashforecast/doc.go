// Package cashforecast implements a deterministic rolling 13-week cash
// forecast engine for short-term liquidity management: what cash is
// expected in and out each week, the weekly ending cash balance, when cash
// falls below a caller's minimum, what funding gap exists, and how explicit
// downside/upside timing assumptions change the picture.
//
// # Distinct from analytics/forecast
//
// This package is not a smaller version of, and does not depend on,
// analytics/forecast. analytics/forecast projects financial statements
// (revenue, COGS, opex, EBITDA, ...) from caller growth/margin assumptions
// over months or years — a P&L/valuation-input forecasting engine.
// cashforecast instead buckets caller-supplied cash events (known bills,
// scheduled receipts, payroll, taxes, debt service, ...) into calendar
// weeks and rolls forward an explicit opening cash balance. It answers a
// treasury question ("do we have enough cash the next 13 weeks"), not a
// P&L or valuation question. The two packages share no types and may be
// used together (a long-range forecast informs assumptions a caller feeds
// into this package) but neither imports the other.
//
// # The package never invents future cash flows
//
// Every inflow and outflow in a Result traces back to a caller-supplied
// CashFlowEvent — either directly, or generated deterministically from a
// caller-supplied recurring rule, AR/AP scheduling assumption, or scenario
// transformation. This package performs no statistical prediction, no
// AI/LLM forecasting, and applies no default collection or payment
// timing that the caller did not explicitly request (see
// ARCollectionAssumption and Options.PayAPOnDueDate). An AR receivable's
// due date is never silently assumed to be its cash receipt date — see
// the AR scheduling section of docs/CASH_FORECAST_13_WEEK.md.
//
// # No hidden current-date dependency
//
// Every calculation takes an explicit Input.ForecastStartDate from caller
// input. Nothing in this package calls time.Now() — this is essential for
// reproducibility and for building historical or hypothetical forecasts
// from any date — see determinism_test.go.
//
// # Explicit non-goals
//
// This package contains no persistence, HTTP/API handlers, auth, UI,
// background jobs, bank API integration, QuickBooks/Xero integration,
// payment execution, invoice generation, AI/LLM forecasting, statistical
// prediction, tax calculation, payroll calculation, loan underwriting, or
// treasury execution. It never automatically draws a credit facility to
// cover a funding gap and never automatically schedules AP payments unless
// Options.PayAPOnDueDate is explicitly set — see
// docs/CASH_FORECAST_13_WEEK.md's "Explicit non-goals" section.
//
// # Purity, immutability, and determinism
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (CashFlowEvent, OpeningCash, CashAccount, every recurring rule,
// every AR/AP plan, every Scenario, and every slice/map they appear in are
// never modified in place — see immutability_test.go), no package-global
// mutable state. Calculate can be called concurrently and repeatedly
// against identical input and always returns byte-for-byte identical JSON
// — see determinism_test.go.
package cashforecast
