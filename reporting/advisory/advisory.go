// Package advisory assembles a deterministic Controller/CFO Advisory Pack
// from whichever of this repository's already-computed accounting/
// analytics/transactions/reporting/valuation results a caller has on
// hand. It is the final module in the accounting-operations/advisory
// roadmap: a composition/reporting layer over every sibling package's own
// deterministic output, never a new accounting or valuation calculation
// engine — see [Build]'s doc comment.
//
// # Composition, not recalculation
//
// Every figure in a [Result] is read directly from an already-computed
// sibling Result (accounting/ar.Result, accounting/cashforecast.Result,
// analytics/debt.Result, valuation/consensus.Result, and so on — see
// [Input]'s field-by-field doc comments) via one of this package's
// section_*.go adapters. This package computes only generic composition
// math: counts, current-vs-prior [Change], ranking/prioritization under
// an explicit caller [Policy], and a small, closed set of cross-module
// [synthesis] rules (synthesis.go) — never a new DSO/DPO/DIO/margin/
// covenant/valuation formula of its own. If a fact is missing, the
// corresponding [Section]/[Metric] reports [StatusUnavailable] or
// [StatusNotSupplied]; it is never reconstructed from unrelated fields.
//
// # No presentation, no narrative
//
// [Build] produces structured data only. It never renders PDF/PowerPoint/
// HTML, and it never generates narrative text via AI/LLM — every
// Insight.Statement/ActionItem.Description here is either a caller-
// supplied string passed through unchanged (ActionOriginCallerSupplied)
// or a short, fixed-form template built entirely from already-computed
// typed fields (statements.go, actiontemplates.go, question.go) — the
// same discipline reporting/management already established for this
// repository's presentation-neutral composition packages. A caller
// wanting a rendered report takes this package's [Result] as its data
// source and builds that presentation layer itself.
//
// # Neutral, non-prescriptive language
//
// Every generated ActionItem/ManagementQuestion uses only review/resolve/
// validate/investigate/confirm/complete-framed language — never a
// prescriptive business decision (fire, terminate, drop a customer,
// replace a supplier, raise prices, borrow money, sell the company,
// change accounting treatment, write off inventory). See
// TestNoPrescriptiveLanguage (safety_test.go) for the permanent
// regression guard scanning every default string this package generates.
// Caller-supplied text (ActionOriginCallerSupplied) is exempt but always
// remains marked as caller-supplied — never presented as though this
// package generated it.
//
// # Determinism
//
// Every function here is pure: no I/O, no mutation of caller-owned input,
// no package-global mutable state, no wall-clock/randomness, no
// time.Now(). Build can be called concurrently and repeatedly against
// identical input and always returns byte-for-byte identical JSON — see
// determinism_test.go. Section/Metric/Insight/ActionItem ordering never
// depends on Go map iteration order — see section.go/insight.go/
// action.go's sort-key helpers.
package advisory

import "sort"

// FormulaVersion identifies this package's own composition rule set — see
// the FormulaVersion constant's doc comment in versions.go for exactly
// what it covers and how it differs from SchemaVersion/
// AdvisoryContractVersion.
//
// Build derives a full [Result] from in and policy. It never mutates in,
// policy, or any nested sibling Result; it performs no I/O and calls no
// sibling package's own Calculate/Build — see the package doc comment.
func Build(in Input, policy Policy) Result {
	policy = resolvePolicy(policy)
	order := resolveCategoryOrder(policy)

	result := Result{
		SchemaVersion: SchemaVersion, FormulaVersion: FormulaVersion, AdvisoryContractVersion: AdvisoryContractVersion,
		Company: in.Company,
	}

	if issues := validateInput(in, policy); len(issues) > 0 {
		result.Warnings, result.Errors = splitIssues(issues)
		if HasErrors(result.Errors) && !hasAnySupplied(in) {
			result.Status = BuildInvalid
			return result
		}
	}

	sections, buildIssues := buildAllSections(in, policy)
	warnIssues, errIssues := splitIssues(buildIssues)
	result.Warnings = append(result.Warnings, warnIssues...)
	result.Errors = append(result.Errors, errIssues...)

	// Cross-module synthesis (task section 82) runs after every section is
	// built, appending its output onto the owning section's own Findings
	// so executive selection/coverage see it uniformly — never a separate,
	// untracked list.
	synthesized := synthesizeInsights(sections, policy)
	for _, in := range synthesized {
		appendSynthesizedInsight(sections, in, policy)
	}

	sections = sortSections(sections, order)

	result.Sections = sections
	result.ExecutiveSummary = selectExecutiveSummary(sections, order, policy)
	result.Snapshot = buildSnapshot(sections)
	result.Questions = buildManagementQuestions(sections, policy)

	suppliedModules := suppliedModuleNames(in)
	usedModules := usedModuleNames(sections)
	result.Coverage = buildCoverage(sections, order, suppliedModules, usedModules, append(result.Warnings, result.Errors...))
	result.Status = resolveBuildStatus(result.Coverage, append(result.Warnings, result.Errors...))

	result.SourceVersions = buildSourceVersions(in)

	if in.Prior != nil {
		result.PriorComparison = comparePrior(*in.Prior, result, in)
	}

	return result
}

// buildAllSections calls every section_*.go builder in sectionOrder,
// returning the built sections in that same fixed order (never
// caller-reordered at this stage — sortSections applies Policy's own
// order afterward, so every downstream consumer of "sections in
// sectionOrder" — e.g. synthesis's cross-references — sees a stable,
// predictable slice regardless of Policy.CategoryOrder).
func buildAllSections(in Input, policy Policy) ([]Section, []Issue) {
	var issues []Issue
	sections := make([]Section, 0, len(sectionOrder))

	appendSection := func(code SectionCode, s Section) {
		s.Code = code
		sections = append(sections, s)
	}

	appendSection(SectionLiquidity, buildLiquiditySection(in, policy))
	appendSection(SectionFinancialPerf, buildFinancialPerformanceSection(in, policy))
	wcSection, wcIssues := buildWorkingCapitalSection(in, policy)
	appendSection(SectionWorkingCapital, wcSection)
	issues = append(issues, wcIssues...)
	appendSection(SectionRevenue, buildRevenueSection(in, policy))
	appendSection(SectionProfitability, buildProfitabilitySection(in, policy))
	appendSection(SectionLabor, buildLaborSection(in, policy))
	appendSection(SectionInventory, buildInventorySection(in, policy))
	appendSection(SectionVendorSpend, buildVendorSpendSection(in, policy))
	appendSection(SectionDebtAndCovenants, buildDebtSection(in, policy))
	appendSection(SectionAccountingAndClose, buildAccountingCloseSection(in, policy))
	appendSection(SectionForecastAndOutlook, buildForecastSection(in, policy))
	appendSection(SectionValuation, buildValuationSection(in, policy))
	appendSection(SectionTransactionReady, buildTransactionReadinessSection(in, policy))
	appendSection(SectionKPI, buildKPISection(in, policy))

	appendSection(SectionActionRegister, buildActionRegisterSection(sections, in.CallerActions, policy)) // aggregates every prior section's own Actions plus caller-supplied ones
	appendSection(SectionAppendix, buildAppendixSection(issues, policy))

	return sections, issues
}

// sortSections reorders sections into order (Policy's resolved
// CategoryOrder), preserving every section's own content — this never
// changes which sections exist, only their position in
// Result.Sections.
func sortSections(sections []Section, order []SectionCode) []Section {
	out := make([]Section, len(sections))
	copy(out, sections)
	sort.SliceStable(out, func(i, j int) bool {
		return sectionRank(out[i].Code, order) < sectionRank(out[j].Code, order)
	})
	return out
}

// appendSynthesizedInsight appends in onto the Section named by
// in.Category's own Findings, ranked/capped like every other Finding —
// mutates sections' matching entry in place (sections is a local slice
// built fresh by buildAllSections/Build in this same call, never
// caller-owned, so this is not a violation of Build's "never mutates
// caller-owned input" guarantee).
func appendSynthesizedInsight(sections []Section, in Insight, policy Policy) {
	for i := range sections {
		if string(sections[i].Code) == in.Category {
			sections[i].Findings = capInsightsForSection(append(sections[i].Findings, in), policy)
			return
		}
	}
}

func splitIssues(issues []Issue) (warnings, errors []Issue) {
	for _, iss := range issues {
		if iss.Severity == IssueSeverityError {
			errors = append(errors, iss)
		} else {
			warnings = append(warnings, iss)
		}
	}
	return warnings, errors
}
