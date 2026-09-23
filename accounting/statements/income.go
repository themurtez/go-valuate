package statements

import (
	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/metrics"
)

// incomeSectionOrder is the fixed, deterministic section ordering for an
// income statement — task section 30's example template, adapted to the
// financial.CodeCategory groupings the taxonomy actually supports (see
// financial/taxonomy.go: revenue, cogs, opex, other_income_statement).
// This package does not create a section for a category the taxonomy
// cannot support (e.g. no separate "Tax" section beyond the single
// CodeIncomeTax code within Other Income/Expense), per the task's
// explicit "do not create rows for categories the taxonomy cannot
// support" instruction.
var incomeSectionOrder = []string{
	"Revenue",
	"Cost of Goods Sold",
	"Operating Expenses",
	"Other Income / Expense",
}

// categoryToIncomeSection maps a financial.CodeCategory to its
// incomeSectionOrder entry.
var categoryToIncomeSection = map[financial.CodeCategory]string{
	financial.CategoryRevenue:              "Revenue",
	financial.CategoryCogs:                 "Cost of Goods Sold",
	financial.CategoryOpex:                 "Operating Expenses",
	financial.CategoryOtherIncomeStatement: "Other Income / Expense",
}

// buildIncomeStatement builds the income statement Statement model. Rows
// are grouped by financial.CodeCategory into fixed sections
// (incomeSectionOrder), plus this package's own calculated structural
// rows (Gross Profit, Operating Income, Net Income), computed via
// financial/metrics.Calculate rather than reinvented — see the task's
// explicit "use existing metrics formulas rather than inventing another
// definition" instruction (task section 26).
func buildIncomeStatement(
	chart ledger.ChartOfAccounts,
	balances []resolvedBalance,
	mappingResults []AccountMappingResult,
	periods []financial.Period,
	opts Options,
) (*Statement, DatasetAvailability) {
	figures := aggregateMappedAccounts(chart, balances, mappingResults, financial.StatementIncomeStatement)
	if len(figures) == 0 {
		return nil, AvailabilityUnavailable
	}

	sectionRows := make(map[string][]Row)
	for _, f := range figures {
		meta, ok := financial.LookupCode(f.code)
		if !ok {
			continue
		}
		sectionName, ok := categoryToIncomeSection[meta.Category]
		if !ok {
			continue
		}
		sectionRows[sectionName] = append(sectionRows[sectionName], mappedFiguresToRow(meta, f))
	}

	dataset := datasetFromFigures(figures, "") // currency irrelevant for metrics.Calculate's internal use
	metricsResult := metrics.Calculate(dataset, metrics.Options{})
	snapshotByPeriod := make(map[financial.Period]metrics.Snapshot, len(metricsResult.Snapshots))
	for _, s := range metricsResult.Snapshots {
		snapshotByPeriod[s.Period] = s
	}

	sections := make([]Section, 0, len(incomeSectionOrder)+3)
	for _, name := range incomeSectionOrder {
		rows := sectionRows[name]
		if len(rows) == 0 {
			continue
		}
		sections = append(sections, Section{Name: name, Rows: rows})

		switch name {
		case "Revenue":
			sections = append(sections, structuralSubtotalSection("Total Revenue", periods, snapshotByPeriod, func(s metrics.Snapshot) metrics.MetricValue { return s.TotalRevenue }))
		case "Cost of Goods Sold":
			sections = append(sections, structuralTotalSection("Gross Profit", periods, snapshotByPeriod, func(s metrics.Snapshot) metrics.MetricValue { return s.GrossProfit }))
		case "Operating Expenses":
			sections = append(sections, structuralTotalSection("Operating Income", periods, snapshotByPeriod, func(s metrics.Snapshot) metrics.MetricValue { return s.EBIT }))
		}
	}

	sections = append(sections, structuralTotalSection("Net Income", periods, snapshotByPeriod, func(s metrics.Snapshot) metrics.MetricValue { return s.NetIncome }))

	stmt := &Statement{
		StatementType: financial.StatementIncomeStatement,
		Periods:       periods,
		Sections:      sections,
	}

	return stmt, AvailabilityBuilt
}

// structuralSubtotalSection builds a single-row, unnamed Section holding
// one financial.RowKindSubtotal Row for a calculated figure (e.g. "Total
// Revenue," which subtotals the preceding section but is not itself the
// bottom line of the whole statement).
func structuralSubtotalSection(label string, periods []financial.Period, snapshots map[financial.Period]metrics.Snapshot, get func(metrics.Snapshot) metrics.MetricValue) Section {
	return Section{Rows: []Row{structuralRow(label, financial.RowKindSubtotal, periods, snapshots, get)}}
}

// structuralTotalSection builds a single-row, unnamed Section holding one
// financial.RowKindTotal Row for a calculated bottom-line figure (Gross
// Profit, Operating Income, Net Income) — task section 15's structural-row
// requirement.
func structuralTotalSection(label string, periods []financial.Period, snapshots map[financial.Period]metrics.Snapshot, get func(metrics.Snapshot) metrics.MetricValue) Section {
	return Section{Rows: []Row{structuralRow(label, financial.RowKindTotal, periods, snapshots, get)}}
}

// structuralRow builds one Row for a calculated (not directly mapped)
// figure, populating Values only for periods where the underlying
// metrics.MetricValue is Available — an unavailable period is simply
// absent from Values, consistent with Row.Values' documented zero-vs-
// absent distinction.
func structuralRow(label string, kind financial.RowKind, periods []financial.Period, snapshots map[financial.Period]metrics.Snapshot, get func(metrics.Snapshot) metrics.MetricValue) Row {
	values := make(map[financial.Period]float64)
	for _, p := range periods {
		snap, ok := snapshots[p]
		if !ok {
			continue
		}
		mv := get(snap)
		if mv.Available {
			values[p] = mv.Value
		}
	}
	return Row{Label: label, Kind: kind, Values: values}
}

// mappedFiguresToRow converts one mappedAccountFigures into a normal
// (Kind == financial.RowKindNormal) Row, using the canonical code's own
// registered display label.
func mappedFiguresToRow(meta financial.CodeMeta, f mappedAccountFigures) Row {
	values := make(map[financial.Period]float64, len(f.valuesByPeriod))
	for p, v := range f.valuesByPeriod {
		values[p] = v
	}
	return Row{
		Label:         meta.Label,
		FinancialCode: f.code,
		Kind:          financial.RowKindNormal,
		Values:        values,
		Contributors:  f.contributors,
	}
}
