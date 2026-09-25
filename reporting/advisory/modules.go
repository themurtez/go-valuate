package advisory

// moduleEntry pairs one Input-embedded sibling module's canonical name
// (matching SourceVersions' field naming) with whether it was supplied at
// all. "Supplied" here means the caller populated Input's corresponding
// field beyond its zero value — never whether the underlying sibling
// Result itself reports Available (that finer distinction is what
// usedModuleNames/Coverage.SourceModulesUsed exists to report
// separately) — task section 63's SourcesSupplied vs SourcesUsed
// distinction.
type moduleEntry struct {
	name     string
	supplied bool
}

// suppliedModuleNames returns, in Input's own field declaration order,
// the canonical name of every sibling module the caller supplied
// (non-zero-value) — task section 63/127's Coverage.SourceModulesSupplied.
func suppliedModuleNames(in Input) []string {
	entries := []moduleEntry{
		{"metrics", len(in.Financial.Metrics.Snapshots) > 0},
		{"ratios", in.Financial.Ratios.Available},
		{"working_capital", in.Financial.WorkingCapital.Available},
		{"revenue_quality", in.Financial.RevenueQuality.Available},
		{"concentration", in.Financial.Concentration.Available},
		{"debt", in.Financial.Debt.Available},
		{"covenants", in.Financial.Covenants.Available},
		{"value_drivers", in.Financial.ValueDrivers.Available},
		{"consolidation", in.Financial.Consolidation.Available},
		{"diagnostics", len(in.Financial.Diagnostics.Findings) > 0},
		{"forecast", in.Financial.Forecast.Available},

		{"ar", in.Operating.AR.Available},
		{"ap", in.Operating.AP.Available},
		{"inventory", in.Operating.Inventory.Available},
		{"labor", len(in.Operating.Labor.Periods) > 0},
		{"profitability", len(in.Operating.Profitability.BusinessTotals.Periods) > 0},
		{"vendor_spend", in.Operating.VendorSpend.Available},
		{"cash_forecast", in.Operating.CashForecast.Available},
		{"portfolio_diagnostics", len(in.Operating.Portfolio.Findings) > 0},

		{"reconciliation", in.Close.Reconciliation.AccountID != "" || in.Close.Reconciliation.AsOfDate != ""},
		{"close_quality", in.Close.CloseQuality.Status != ""},
		{"close_checklist", in.Close.CloseChecklist.Readiness != ""},

		{"sale_readiness", in.Transaction.SaleReadiness.Available},
		{"acquisition", in.Transaction.Acquisition.Available},
		{"deal_structure", in.Transaction.DealStructure.Available},

		{"consensus", in.Valuation.Consensus.Available},

		{"kpi", len(in.KPIValues) > 0},
	}

	var out []string
	for _, e := range entries {
		if e.supplied {
			out = append(out, e.name)
		}
	}
	return out
}

// usedModuleNames returns, as a set, every SourceRef.Module actually
// present across every built Section's Sources — task section 63's
// Coverage.SourceModulesUsed (the subset of SourcesSupplied that
// contributed at least one composed fact).
func usedModuleNames(sections []Section) map[string]bool {
	used := make(map[string]bool)
	for _, s := range sections {
		for _, ref := range s.Sources {
			if ref.Module != "" {
				used[ref.Module] = true
			}
		}
	}
	return used
}

// buildSourceVersions echoes every supplied sibling module's own version
// constant — task section 96. A module with no SchemaVersion of its own
// (analytics/debt, analytics/covenants — confirmed by this package's own
// adapter research) echoes its FormulaVersion here instead, since that is
// the only version string those packages expose; SourceVersions makes no
// schema/formula distinction of its own (see SourceVersions' doc
// comment).
func buildSourceVersions(in Input) SourceVersions {
	v := SourceVersions{}
	if len(in.Financial.Metrics.Snapshots) > 0 {
		v.Metrics = in.Financial.Metrics.FormulaVersion
	}
	if in.Financial.Ratios.Available {
		v.Ratios = in.Financial.Ratios.FormulaVersion
	}
	if in.Financial.WorkingCapital.Available {
		v.WorkingCapital = in.Financial.WorkingCapital.FormulaVersion
	}
	if in.Financial.RevenueQuality.Available {
		v.RevenueQuality = in.Financial.RevenueQuality.FormulaVersion
	}
	if in.Financial.Concentration.Available {
		v.Concentration = in.Financial.Concentration.FormulaVersion
	}
	if in.Financial.Debt.Available {
		v.Debt = in.Financial.Debt.FormulaVersion
	}
	if in.Financial.Covenants.Available {
		v.Covenants = in.Financial.Covenants.FormulaVersion
	}
	if in.Financial.ValueDrivers.Available {
		v.ValueDrivers = in.Financial.ValueDrivers.FormulaVersion
	}
	if in.Financial.Consolidation.Available {
		v.Consolidation = in.Financial.Consolidation.FormulaVersion
	}
	if len(in.Financial.Diagnostics.Findings) > 0 {
		v.Diagnostics = in.Financial.Diagnostics.FormulaVersion
	}
	if in.Financial.Forecast.Available {
		v.Forecast = in.Financial.Forecast.FormulaVersion
	}

	if in.Operating.AR.Available {
		v.AR = in.Operating.AR.FormulaVersion
	}
	if in.Operating.AP.Available {
		v.AP = in.Operating.AP.FormulaVersion
	}
	if in.Operating.Inventory.Available {
		v.Inventory = in.Operating.Inventory.FormulaVersion
	}
	if len(in.Operating.Labor.Periods) > 0 {
		v.Labor = in.Operating.Labor.FormulaVersion
	}
	if len(in.Operating.Profitability.BusinessTotals.Periods) > 0 {
		v.Profitability = in.Operating.Profitability.FormulaVersion
	}
	if in.Operating.VendorSpend.Available {
		v.VendorSpend = in.Operating.VendorSpend.FormulaVersion
	}
	if in.Operating.CashForecast.Available {
		v.CashForecast = in.Operating.CashForecast.FormulaVersion
	}
	if len(in.Operating.Portfolio.Findings) > 0 {
		v.PortfolioDiagnostics = in.Operating.Portfolio.FormulaVersion
	}

	if in.Close.Reconciliation.AccountID != "" || in.Close.Reconciliation.AsOfDate != "" {
		v.Reconciliation = in.Close.Reconciliation.Versions.FormulaVersion
	}
	if in.Close.CloseQuality.Status != "" {
		v.CloseQuality = in.Close.CloseQuality.Versions.FormulaVersion
	}
	if in.Close.CloseChecklist.Readiness != "" {
		v.CloseChecklist = in.Close.CloseChecklist.Versions.FormulaVersion
	}

	if in.Transaction.SaleReadiness.Available {
		v.SaleReadiness = in.Transaction.SaleReadiness.FormulaVersion
	}
	if in.Transaction.Acquisition.Available {
		v.Acquisition = in.Transaction.Acquisition.FormulaVersion
	}
	if in.Transaction.DealStructure.Available {
		v.DealStructure = in.Transaction.DealStructure.FormulaVersion
	}

	if in.Valuation.Consensus.Available {
		v.Consensus = in.Valuation.Consensus.FormulaVersion
	}

	if len(in.KPIValues) > 0 {
		// KPIResult carries no version of its own; the pack-level
		// kpi.Result.FormulaVersion is not part of Input (only
		// []kpi.KPIResult is), so no version is echoed here — this is a
		// documented gap: a caller wanting exact KPI-engine version
		// traceability supplies it via a future Input extension, not
		// invented here.
		_ = v.KPI
	}

	return v
}
