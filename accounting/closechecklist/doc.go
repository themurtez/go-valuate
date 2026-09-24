// Package closechecklist implements a deterministic period-close
// checklist and close-readiness domain engine.
//
// # Distinction from accounting/closequality
//
// [closequality] (accounting/closequality) answers "are there accounting/
// data/control issues that should block or warn on close?" — it mines
// ledger, statement, AR/AP, journal-diagnostics, and reconciliation-status
// facts into a books-quality Result.
//
// This package answers a different question: "what close tasks are
// required, complete, blocked, evidenced, reviewed, signed off, and still
// outstanding before the period can be finalized?" It consumes upstream
// module statuses (including closequality's own Result) as typed, opaque
// gate facts — see [GateFact] and the closequality/reconciliation
// adapters in adapters.go — and never recomputes their diagnostics.
//
// # Distinction from accounting/reconciliation
//
// [reconciliation] (accounting/reconciliation) performs the actual
// book-vs-external matching arithmetic for one account. This package
// never matches transactions itself; a reconciliation.Result is consumed
// through [ReconciliationGateFacts] as a pass/warning/fail gate fact
// attached to whichever checklist task represents "complete that
// reconciliation."
//
// # What this package deliberately does not do
//
// No persistence/database, no HTTP/API, no users/auth, no frontend/UI,
// no notifications (email/Slack), no background jobs/scheduling, no
// actual period locking, no journal posting, no reconciliation matching,
// no bank feeds, no QuickBooks/Xero integrations, no file storage, no
// audit opinion, no tax filing, and no AI/LLM. See
// docs/PERIOD_CLOSE_CHECKLIST.md's "Explicit non-goals" section for the
// complete list and rationale.
//
// # Determinism, immutability, concurrency
//
// Every exported function is pure: no I/O, no mutation of caller-owned
// input (see immutability_test.go), no package-global mutable state, and
// no time.Now() anywhere — every date this package reasons about
// (EvaluationDate, due dates, timestamps) is supplied explicitly by the
// caller. [Calculate] can be called concurrently and repeatedly against
// identical input and always returns byte-for-byte identical JSON — see
// determinism_test.go and concurrency_test.go.
//
// # Two input shapes: template + instance
//
// A [Template] is reusable, versioned, plain domain data (sections +
// task definitions) — see [ServiceBusinessMonthlyClose] and
// [InventoryBusinessMonthlyClose] for ready-made examples. An [Instance]
// pairs one Template (by ID+version) with one [Period]'s caller-supplied
// task states, evidence, sign-offs, exceptions, and gate facts. [Calculate]
// takes an Instance (plus [Policy]) and returns a [Result].
package closechecklist
