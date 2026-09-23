package closequality

// Coverage reports how much of the expected input surface this
// assessment actually had available, split from DimensionCoverage
// (which reports the same idea at dimension granularity — see
// DimensionCoverage). Percents are 0-1 decimal fractions, matching
// analytics/diagnostics.Coverage (the closest existing "how many
// optional sibling Results did I get" precedent) rather than
// statements.MappingCoverage's 0-100 scale.
type Coverage struct {
	LedgerAvailable             bool `json:"ledger_available"`
	StatementsAvailable         bool `json:"statements_available"`
	ARAvailable                 bool `json:"ar_available"`
	APAvailable                 bool `json:"ap_available"`
	JournalDiagnosticsAvailable bool `json:"journal_diagnostics_available"`
	ReconciliationsAvailable    bool `json:"reconciliations_available"`
	CloseTasksAvailable         bool `json:"close_tasks_available"`
	PriorPeriodAvailable        bool `json:"prior_period_available"`

	// TotalModules/AvailableModules count only modules that are either
	// Required by Applicability or actually supplied — a module that is
	// both not required and not supplied is NotApplicable, not counted
	// against coverage (see DimensionCoverage.CoveragePercent's
	// "never counts non-applicable dimensions in the denominator" rule,
	// which this mirrors at the module level).
	TotalModules         int      `json:"total_modules"`
	AvailableModules     int      `json:"available_modules"`
	CoveragePercent      float64  `json:"coverage_percent"`
	MissingModules       []string `json:"missing_modules,omitempty"`
	NotApplicableModules []string `json:"not_applicable_modules,omitempty"`
}

// moduleAvailabilityOrder fixes the module enumeration order for
// Coverage.MissingModules/NotApplicableModules — never Go map order.
var moduleAvailabilityOrder = []string{
	"ledger", "statements", "ar", "ap", "journal_diagnostics",
	"reconciliations", "close_tasks",
}

func buildCoverage(in Input, policy Policy, priorAvailable bool) Coverage {
	avail := map[string]bool{
		"ledger":              in.LedgerProvided,
		"statements":          in.statementsAvailable(),
		"ar":                  in.arAvailable(),
		"ap":                  in.apAvailable(),
		"journal_diagnostics": in.journalDiagnosticsAvailable(),
		"reconciliations":     len(in.Reconciliations) > 0,
		"close_tasks":         len(in.CloseTasks) > 0,
	}
	required := map[string]bool{
		// ledger has no Applicability toggle (Applicability's fields per
		// Prompt 42 section 26 cover AR/AP/statements/journal
		// diagnostics/reconciliations/close tasks only) — a caller who
		// wants ledger checks simply supplies a ledger; an absent ledger
		// is NotApplicable, never MISSING_REQUIRED_INPUT.
		"ledger":              false,
		"statements":          policy.Applicability.StatementsRequired,
		"ar":                  policy.Applicability.ARRequired,
		"ap":                  policy.Applicability.APRequired,
		"journal_diagnostics": policy.Applicability.JournalDiagnosticsRequired,
		"reconciliations":     policy.Applicability.ReconciliationsRequired,
		"close_tasks":         policy.Applicability.CloseTasksRequired,
	}

	c := Coverage{
		LedgerAvailable:             avail["ledger"],
		StatementsAvailable:         avail["statements"],
		ARAvailable:                 avail["ar"],
		APAvailable:                 avail["ap"],
		JournalDiagnosticsAvailable: avail["journal_diagnostics"],
		ReconciliationsAvailable:    avail["reconciliations"],
		CloseTasksAvailable:         avail["close_tasks"],
		PriorPeriodAvailable:        priorAvailable,
	}

	var missing, notApplicable []string
	for _, m := range moduleAvailabilityOrder {
		if avail[m] {
			c.TotalModules++
			c.AvailableModules++
			continue
		}
		if required[m] {
			c.TotalModules++
			missing = append(missing, m)
			continue
		}
		notApplicable = append(notApplicable, m)
	}
	if c.TotalModules > 0 {
		c.CoveragePercent = float64(c.AvailableModules) / float64(c.TotalModules)
	}
	c.MissingModules = missing
	c.NotApplicableModules = notApplicable
	return c
}

// DimensionCoverage summarizes readiness across Dimensions themselves
// (as opposed to Coverage, which summarizes upstream module presence).
// RequiredDimensions/AssessedDimensions never count a dimension that is
// legitimately not applicable to this business (a dimension is either
// assessable given the supplied/required inputs, or it is not — this
// package has no per-dimension "not applicable" declaration beyond
// what Applicability already implies for the module it depends on).
type DimensionCoverage struct {
	RequiredDimensions   []Dimension `json:"required_dimensions"`
	AssessedDimensions   []Dimension `json:"assessed_dimensions"`
	PassedDimensions     []Dimension `json:"passed_dimensions"`
	WarningDimensions    []Dimension `json:"warning_dimensions"`
	BlockingDimensions   []Dimension `json:"blocking_dimensions"`
	UnassessedDimensions []Dimension `json:"unassessed_dimensions"`
	CoveragePercent      float64     `json:"coverage_percent"`
}

func buildDimensionCoverage(dims []DimensionResult) DimensionCoverage {
	dc := DimensionCoverage{}
	for _, d := range dims {
		if d.Assessed {
			dc.AssessedDimensions = append(dc.AssessedDimensions, d.Dimension)
		} else {
			dc.UnassessedDimensions = append(dc.UnassessedDimensions, d.Dimension)
		}
		switch d.Status {
		case DimensionPass:
			dc.PassedDimensions = append(dc.PassedDimensions, d.Dimension)
		case DimensionWarning:
			dc.WarningDimensions = append(dc.WarningDimensions, d.Dimension)
		case DimensionBlocking:
			dc.BlockingDimensions = append(dc.BlockingDimensions, d.Dimension)
		}
		dc.RequiredDimensions = append(dc.RequiredDimensions, d.Dimension)
	}
	if len(dims) > 0 {
		dc.CoveragePercent = float64(len(dc.AssessedDimensions)) / float64(len(dims))
	}
	return dc
}
