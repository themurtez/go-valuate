# Analytics Version Inventory

Every meaningful `FormulaVersion`/`ScoreVersion`/`SignalRulesVersion`
constant across the 20 analytics/transactions/portfolio/reporting
packages, verified directly against each package's source (not assumed).
See [`ANALYTICS_MODULES.md`](ANALYTICS_MODULES.md) for what each package
does, and the repository README's versioning-strategy section for the
original semver discipline this inventory follows.

## What a version constant covers

Every package's `FormulaVersion` doc comment states the same rule, in its
own words: **bump `FormulaVersion` whenever a change could make a
historical `Result` not reproduce identically under new code** — a
changed formula, a changed default threshold, a changed deterministic
ordering rule, a changed flag/finding trigger condition. A change that
only affects *how* a result is displayed, or that adds a new optional
field nothing previously populated, does not require a bump.

A package with a second, independent version constant (`ScoreVersion` or
`SignalRulesVersion`) keeps that concept's bump separate specifically so a
caller can tell "the underlying figures changed" apart from "only how
those figures are scored/signaled changed" — see each row's notes below.

## Inventory

| Package | Version constant(s) | Current value(s) | Where echoed on `Result` | Bump guidance present? |
|---|---|---|---|---|
| `analytics/qoe` | `FormulaVersion`, `ScoreVersion` | `"1.0.0"`, `"1.0.0"` | `Result.FormulaVersion` (top-level); `ScoreVersion` only on the optional nested `Result.Score.Version` (`*Score`, omitted entirely if `Options.ComputeScore` was false) | Yes, both — `ScoreVersion` is separate because a caller may want to change how flags/ratios are computed independently of how they're weighted into one composite score |
| `analytics/workingcapital` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/ratios` | `FormulaVersion`, `SignalRulesVersion` | `"1.0.0"`, `"1.0.0"` | Both top-level on `Result` | Yes, both — `SignalRulesVersion` is separate since ratio computation can change independently of which deterministic health signals are derived from the results |
| `analytics/cashflow` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/revenuequality` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/concentration` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/anomalies` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/variance` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/forecast` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/debt` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/covenants` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/benchmarks` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `analytics/valuedrivers` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion`. No `ScoreVersion` — this package produces no composite score. | Yes |
| `transactions/acquisition` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `transactions/dealstructure` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `transactions/salereadiness` | `FormulaVersion`, `ScoreVersion` | `"1.0.0"`, `"1.0.0"` | `Result.FormulaVersion` top-level; `ScoreVersion` only on the optional nested `Result.OverallScore.Version` (`*Score`) | Yes, both — same classification-vs-weighting rationale as `qoe` |
| `analytics/consolidation` | `FormulaVersion` | `"1.0.0"` | `Result.FormulaVersion` | Yes |
| `portfolio/diagnostics` | `FormulaVersion`, `ScoreVersion` | `"1.0.0"`, `"1.0.0"` | **Both directly on `Result`** (`Result.FormulaVersion`, `Result.ScoreVersion`) — the one package where `ScoreVersion` is not nested inside an optional sub-struct | Yes, both |
| `reporting/management` | `FormulaVersion` | `"1.0.0"` | `Report.FormulaVersion` (this package's top-level type is `Report`, not `Result` — see [`ANALYTICS_MODULES.md`](ANALYTICS_MODULES.md)). Also carries `Report.Versions ModuleVersions`, which re-echoes this package's own `FormulaVersion` plus every contributing sibling module's own `FormulaVersion` as a `[]ModuleVersion{Module, Version}` rollup — a version-aggregation pattern unique to this package. | Yes |
| `analytics/diagnostics` | `FormulaVersion`, `ScoreVersion` | `"1.0.0"`, `"1.0.0"` | `Result.FormulaVersion` top-level; `ScoreVersion` only on the optional nested `Result.Score.Version` | Yes, both |

## Findings

1. **Every persistable top-level `Result` (or `Report`, for `reporting/management`) in all 20 packages carries a `FormulaVersion` field.** No gaps.
2. **`ScoreVersion` placement is inconsistent across the 4 packages that have one.** Three of the four (`qoe`, `salereadiness`, `analytics/diagnostics`) echo it only inside a nested, optional `*Score.Version` field — a caller who never requests/populates the optional score sub-object has no way to learn what `ScoreVersion` *would* apply. `portfolio/diagnostics` is the outlier, carrying `ScoreVersion` directly on `Result` regardless of whether any score-shaped field exists on that package's `Finding`s (it doesn't have a separate `Score` sub-struct at all — `PriorityScore` is a plain field on each `Finding`).

   This asymmetry is **documented here, not changed**, for two reasons: fixing it would mean adding a new top-level field to 3 already-frozen-shape `Result` types (a backward-compatible addition, but still a shape change this task's "avoid unnecessary breaking changes" instruction weighs against making without a concrete need), and a caller that cares can already derive "was a score computed under version X" from whether the nested `*Score` is non-nil.
3. **Two packages (`analytics/ratios`, `analytics/qoe`) carry a second, genuinely independent rule-set version** (`SignalRulesVersion`, `ScoreVersion`) rather than just a score-echo — both are top-level on `Result` in `ratios`'s case, nested in `qoe`'s case (see finding 2).
4. **All 20 packages are currently pinned at exactly `"1.0.0"`.** No package has yet been bumped past its initial version, so this inventory has no existing precedent in the codebase for what a `"1.0.1"` or `"1.1.0"` diff actually looks like in practice — semver discipline (what constitutes a patch vs. minor vs. major change) is asserted at the repository README level, not restated per package.
5. **`reporting/management`'s `ModuleVersions` rollup is worth calling out as a reusable pattern** for any future aggregator package: rather than a caller needing to separately track which version of each of 11 contributing sibling packages produced the inputs that went into one `Report`, the `Report` itself carries that provenance.
