package salereadiness

import "fmt"

// blockingDimensions is the fixed set of DimensionCode values whose
// StatusConcerning classification produces a Blocker rather than only a
// Risk — deliberately narrower than "every StatusConcerning dimension,"
// since some dimensions (e.g. DataCompleteness) reflect analysis coverage
// rather than a transaction-blocking business fact. A caller-visible,
// documented list rather than an inferred one, per the package's
// structured-classification discipline.
var blockingDimensions = map[DimensionCode]bool{
	DimensionFinancialRecordQuality: true,
	DimensionEarningsStability:      true,
	DimensionCustomerConcentration:  true,
	DimensionOwnerDependence:        true,
	DimensionDebtLeverage:           true,
}

// neededInputFor names which Input field(s) would let an unassessed
// dimension be assessed — used to build MissingInformation entries.
var neededInputFor = map[DimensionCode]string{
	DimensionFinancialRecordQuality:   "DataQuality or Dataset",
	DimensionEarningsStability:        "QoE or Metrics (with Trend)",
	DimensionNormalizationBurden:      "QoE",
	DimensionCustomerConcentration:    "Concentration",
	DimensionRecurringRevenue:         "RevenueQuality or Profile.RecurringRevenuePercent",
	DimensionOwnerDependence:          "Profile.OwnerOperated",
	DimensionMarginTrend:              "QoE or Metrics (with Trend)",
	DimensionWorkingCapitalStability:  "WorkingCapital",
	DimensionDebtLeverage:             "Metrics (with NetDebt/EBITDA)",
	DimensionDataCompleteness:         "",
	DimensionValuationMethodConsensus: "Consensus",
}

// opportunityFor maps a DimensionCode to the single fixed, factual
// improvement action associated with it — see Opportunity's doc comment on
// why every action is phrased as "do X," never an evaluative judgment.
var opportunityFor = map[DimensionCode]struct {
	Code    OpportunityCode
	Message string
}{
	DimensionFinancialRecordQuality:  {OpportunityCommissionReviewedFinancials, "Commission a review or audit of the financial statements and reconcile them to filed tax returns."},
	DimensionNormalizationBurden:     {OpportunityDocumentAdjustments, "Assemble formal supporting documentation for each proposed earnings add-back."},
	DimensionCustomerConcentration:   {OpportunityDiversifyCustomerBase, "Pursue new customer relationships to reduce reliance on the largest existing customer(s)."},
	DimensionRecurringRevenue:        {OpportunityIncreaseRecurringRevenueShare, "Convert more revenue to contractual/recurring arrangements."},
	DimensionOwnerDependence:         {OpportunityReduceOwnerDependence, "Document key processes and delegate owner-held responsibilities to reduce transition risk."},
	DimensionMarginTrend:             {OpportunityStabilizeMargins, "Identify and address the drivers of margin erosion before beginning a sale process."},
	DimensionWorkingCapitalStability: {OpportunityStabilizeWorkingCapital, "Tighten working-capital management (receivables, payables, inventory) to reduce period-to-period swings."},
	DimensionDebtLeverage:            {OpportunityReduceLeverage, "Pay down debt or grow EBITDA to reduce the net-debt-to-EBITDA multiple."},
}

// buildFindings derives Blockers, Risks, Strengths, MissingInformation, and
// Opportunities from an already-built dims slice, each in dimensionOrder.
func buildFindings(dims []Dimension) (blockers []Blocker, risks []Risk, strengths []Strength, missing []MissingInformation, opportunities []Opportunity) {
	for _, d := range dims {
		switch d.Status {
		case StatusConcerning:
			if blockingDimensions[d.Code] {
				blockers = append(blockers, Blocker{
					Dimension: d.Code,
					Severity:  SeverityCritical,
					Message:   d.Explanation,
				})
			} else {
				risks = append(risks, Risk{
					Dimension: d.Code,
					Severity:  SeverityCritical,
					Message:   d.Explanation,
				})
			}
			if opp, ok := opportunityFor[d.Code]; ok {
				opportunities = append(opportunities, Opportunity{Code: opp.Code, Dimension: d.Code, Message: opp.Message})
			}
		case StatusWeak:
			risks = append(risks, Risk{
				Dimension: d.Code,
				Severity:  SeverityWarning,
				Message:   d.Explanation,
			})
			if opp, ok := opportunityFor[d.Code]; ok {
				opportunities = append(opportunities, Opportunity{Code: opp.Code, Dimension: d.Code, Message: opp.Message})
			}
		case StatusStrong:
			strengths = append(strengths, Strength{Dimension: d.Code, Message: d.Explanation})
		case StatusUnassessed:
			needed := neededInputFor[d.Code]
			msg := fmt.Sprintf("%s could not be assessed: %s", d.Code, d.Explanation)
			missing = append(missing, MissingInformation{Dimension: d.Code, NeededInput: needed, Message: msg})
			opportunities = append(opportunities, Opportunity{
				Code:      OpportunitySupplyMissingModuleInput,
				Dimension: d.Code,
				Message:   fmt.Sprintf("Supply %s to enable a %s assessment.", needed, d.Code),
			})
		}
	}
	return blockers, risks, strengths, missing, opportunities
}

// negativeEarningsBlocker returns a Blocker when the most recent available
// EBITDA/SDE figure (QoE's normalized figure preferred, else the most
// recent Metrics snapshot's reported EBITDA) is negative — a fixed
// structural check independent of any single Dimension's classification,
// since negative earnings is a blocking fact about the business, not a
// threshold-graded quality signal. ok is false when no earnings figure is
// available to check at all.
func negativeEarningsBlocker(in Input) (Blocker, bool) {
	if in.QoE.Available && len(in.QoE.History) > 0 {
		latest := in.QoE.History[len(in.QoE.History)-1]
		if latest.NormalizedEBITDA.Available {
			if latest.NormalizedEBITDA.Value < 0 {
				return Blocker{
					Dimension: DimensionEarningsStability,
					Severity:  SeverityCritical,
					Message:   fmt.Sprintf("most recent normalized EBITDA is negative (%.2f)", latest.NormalizedEBITDA.Value),
				}, true
			}
			return Blocker{}, false
		}
	}
	if snap, ok := mostRecentSnapshot(in.Metrics.Snapshots, in.PeriodMeta); ok && snap.EBITDA.Available {
		if snap.EBITDA.Value < 0 {
			return Blocker{
				Dimension: DimensionEarningsStability,
				Severity:  SeverityCritical,
				Message:   fmt.Sprintf("most recent reported EBITDA is negative (%.2f)", snap.EBITDA.Value),
			}, true
		}
	}
	return Blocker{}, false
}
