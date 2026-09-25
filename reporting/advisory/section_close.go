package advisory

import (
	"github.com/themurtez/go-valuate/accounting/closechecklist"
	"github.com/themurtez/go-valuate/accounting/closequality"
	"github.com/themurtez/go-valuate/accounting/reconciliation"
)

// buildAccountingCloseSection composes ACCOUNTING_AND_CLOSE from
// Input.Close.Reconciliation/CloseQuality/CloseChecklist — task section
// 26. Every source package's own overall status is echoed verbatim (never
// downgraded) and its Finding/Blocker Severity translated onto this
// package's Severity scale, following the closest existing precedent
// (reporting/management.TopIssue's translation pattern) — never a new
// detection rule of this package's own. This function does not read
// accounting/journaldiagnostics.Result directly (that package produces no
// single-account/single-task granularity this section's Metrics model
// fits) — journal-review findings instead surface through
// closequality.Result, which already mines journaldiagnostics per its own
// documented composition (accounting/closequality/mine_journaldiagnostics.go)
// — this section reads that already-composed closequality.Finding output
// rather than re-mining journaldiagnostics.Result a second time.
func buildAccountingCloseSection(in Input, policy Policy) Section {
	rec := in.Close.Reconciliation
	cq := in.Close.CloseQuality
	cc := in.Close.CloseChecklist

	hasRec := rec.AccountID != "" || rec.AsOfDate != ""
	hasCQ := cq.Status != ""
	hasCC := cc.Readiness != ""

	if !hasRec && !hasCQ && !hasCC {
		return newUnavailableSection(SectionAccountingAndClose, StatusNotSupplied)
	}

	period := currentPeriodLabel(in)
	var metricsOut []Metric
	var highlights []Insight
	var findings []Insight
	var actions []ActionItem
	usedModules := map[string]bool{}

	if hasRec {
		usedModules["reconciliation"] = true
		recPeriod := rec.AsOfDate
		if recPeriod == "" {
			recPeriod = period
		}
		highlights = append(highlights, statusHighlight("Reconciliation Status: "+rec.AccountID, string(rec.Status), string(SectionAccountingAndClose), recPeriod, rec.AccountID, "reconciliation", "status"))
		for _, f := range rec.Findings {
			findings = append(findings, Insight{
				Code: string(f.Code), Category: string(SectionAccountingAndClose), Severity: severityFromReconciliationFinding(f.Severity),
				Title: "Reconciliation finding", Statement: statementReconciliationBlocker(rec.AccountID, f.Message),
				Period: recPeriod, EntityRef: rec.AccountID,
				SourceModule: "reconciliation", SourceCode: string(f.Code),
				SourceRefs: []SourceRef{{Module: "reconciliation", Code: string(f.Code), Ref: rec.AccountID, Period: recPeriod}},
			})
			if f.Code == reconciliation.FindingBalanceMismatch {
				actions = append(actions, newGeneratedAction(actionTemplates[ActionResolveReconciliationBlocker], PriorityHigh, "reconciliation", string(f.Code), []SourceRef{{Module: "reconciliation", Ref: rec.AccountID, Period: recPeriod}}, rec.AccountID, recPeriod))
			}
		}
		if rec.Status == reconciliation.StatusUnreconciled {
			actions = append(actions, newGeneratedAction(actionTemplates[ActionResolveReconciliationBlocker], PriorityHigh, "reconciliation", "unreconciled_status", []SourceRef{{Module: "reconciliation", Ref: rec.AccountID, Period: recPeriod}}, rec.AccountID, recPeriod))
		}
	}

	if hasCQ {
		usedModules["close_quality"] = true
		cqPeriod := cq.Period.Period
		if cqPeriod == "" {
			cqPeriod = period
		}
		highlights = append(highlights, statusHighlight("Close-Quality Status", string(cq.Status), string(SectionAccountingAndClose), cqPeriod, "", "close_quality", "status"))
		for _, f := range cq.Blockers {
			findings = append(findings, Insight{
				Code: string(f.Code), Category: string(SectionAccountingAndClose), Severity: severityFromCloseQualityFinding(f.Severity),
				Title: "Close-quality blocker", Statement: f.Message, Period: cqPeriod,
				SourceModule: "close_quality", SourceCode: string(f.Code),
				SourceRefs: []SourceRef{{Module: "close_quality", Code: string(f.Code), Period: cqPeriod}},
			})
			actions = append(actions, newGeneratedAction(actionTemplates[ActionResolveCloseQualityBlocker], PriorityCritical, "close_quality", string(f.Code), []SourceRef{{Module: "close_quality", Code: string(f.Code), Period: cqPeriod}}, "", cqPeriod))
		}
		for _, f := range cq.Warnings {
			findings = append(findings, Insight{
				Code: string(f.Code), Category: string(SectionAccountingAndClose), Severity: severityFromCloseQualityFinding(f.Severity),
				Title: "Close-quality warning", Statement: f.Message, Period: cqPeriod,
				SourceModule: "close_quality", SourceCode: string(f.Code),
				SourceRefs: []SourceRef{{Module: "close_quality", Code: string(f.Code), Period: cqPeriod}},
			})
		}
	}

	if hasCC {
		usedModules["close_checklist"] = true
		ccPeriod := cc.PeriodID
		if ccPeriod == "" {
			ccPeriod = period
		}
		highlights = append(highlights, statusHighlight("Close Checklist Readiness", string(cc.Readiness), string(SectionAccountingAndClose), ccPeriod, "", "close_checklist", "readiness"))
		metricsOut = append(metricsOut, newMetric("close_task_completion_percent", "Close Task Completion %", AvailableValue(cc.Completion.RequiredCompletionPercent), UnitPercent, ccPeriod, "close_checklist", "completion.required_completion_percent"))

		for _, b := range cc.Blockers {
			if !b.EffectiveBlocking {
				continue
			}
			findings = append(findings, Insight{
				Code: string(b.ReasonCode), Category: string(SectionAccountingAndClose), Severity: SeverityBlocking,
				Title: "Close checklist blocker", Statement: b.Message, Period: ccPeriod, EntityRef: b.TaskCode,
				SourceModule: "close_checklist", SourceCode: string(b.ReasonCode),
				SourceRefs: []SourceRef{{Module: "close_checklist", Code: string(b.ReasonCode), Ref: b.TaskCode, Period: ccPeriod}},
			})
			switch b.ReasonCode {
			case closechecklist.BlockerEvidenceMissing:
				actions = append(actions, newGeneratedAction(actionTemplates[ActionAttachRequiredEvidence], PriorityHigh, "close_checklist", string(b.ReasonCode), []SourceRef{{Module: "close_checklist", Ref: b.TaskCode, Period: ccPeriod}}, b.TaskCode, ccPeriod))
			case closechecklist.BlockerSignOffMissing:
				actions = append(actions, newGeneratedAction(actionTemplates[ActionObtainRequiredSignoff], PriorityHigh, "close_checklist", string(b.ReasonCode), []SourceRef{{Module: "close_checklist", Ref: b.TaskCode, Period: ccPeriod}}, b.TaskCode, ccPeriod))
			default:
				actions = append(actions, newGeneratedAction(actionTemplates[ActionCompleteRequiredCloseTask], PriorityHigh, "close_checklist", string(b.ReasonCode), []SourceRef{{Module: "close_checklist", Ref: b.TaskCode, Period: ccPeriod}}, b.TaskCode, ccPeriod))
			}
		}
	}

	sources := make([]SourceRef, 0, len(usedModules))
	for m := range usedModules {
		sources = append(sources, SourceRef{Module: m, Period: period})
	}

	return Section{
		Code: SectionAccountingAndClose, Availability: StatusAvailable,
		Highlights: capInsightsForSection(highlights, policy),
		Metrics:    metricsOut, Findings: capInsightsForSection(findings, policy), Actions: capActionsForSection(dedupeActions(actions), policy),
		Sources: sortedSourceRefs(sources),
	}
}

// statusHighlight renders a qualitative source status (a Go string enum
// value, e.g. reconciliation.Status/closequality.Status/
// closechecklist.ChecklistReadiness) as an Insight rather than a Metric —
// task section 47's Metric model is numeric-Value-shaped, so a pure
// status label belongs in Insight.Statement, never encoded as a
// fabricated numeric Value. Unavailable Insight.Current is intentional
// here: there is no numeric figure to carry, only a status word.
func statusHighlight(title, status, category, period, entityRef, sourceModule, sourceCode string) Insight {
	return Insight{
		Code: sourceCode + "_status", Category: category, Severity: SeverityInfo,
		Title: title, Statement: title + ": " + status,
		Period: period, EntityRef: entityRef,
		SourceModule: sourceModule, SourceCode: sourceCode,
		SourceRefs: []SourceRef{{Module: sourceModule, Code: sourceCode, Ref: entityRef, Period: period}},
	}
}

func severityFromReconciliationFinding(s reconciliation.Severity) Severity {
	switch s {
	case reconciliation.SeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}

func severityFromCloseQualityFinding(s closequality.Severity) Severity {
	switch s {
	case closequality.SeverityBlocking:
		return SeverityBlocking
	case closequality.SeverityWarning:
		return SeverityMedium
	default:
		return SeverityInfo
	}
}
