// This file intentionally contains no adapter from analytics/debt or
// transactions/dealstructure.
//
// analytics/debt's public API (AmortizationSchedule) exposes only
// FirstYearAnnualDebtService and SteadyStateAnnualDebtService — annual
// aggregate figures with no calendar anchor and no per-payment date list
// at all (LoanTerms has no origination/start date, and Amortize never
// enumerates individual payment dates, even internally the
// per-period interest loop in analytics/debt's own calculation only
// ever produces one annual aggregate, never a returned schedule). There
// is therefore no way to derive which week within a 13-week horizon any
// specific debt payment falls in from that package's output alone, and
// dividing an annual figure by 52 would silently fabricate weekly timing
// this package has no basis for — exactly what the task explicitly
// prohibits ("Do not divide annual debt service by 52").
//
// transactions/dealstructure likewise models transaction-level debt
// structuring, not a payment-date schedule.
//
// A caller who has exact debt-service payment dates and amounts (from a
// loan servicer, an amortization table with real dates, or its own
// origination-date-aware wrapper around analytics/debt.AmortizationSchedule)
// supplies them directly as ordinary CashFlowEvents with
// Category: CategoryDebtService, SourceType: SourceDebtSchedule — no
// adapter is needed for that path, since CashFlowEvent is already the
// portable, dated contract every other source funnels through.
//
// If a future prompt adds date-anchored loan schedules to analytics/debt
// (e.g. an optional OriginationDate on LoanTerms plus a returned
// per-payment schedule), an adapter here becomes possible and should
// generate one CashFlowEvent per scheduled payment date, never an evenly-
// spread approximation.
package cashforecast
