package advisory

// QuestionCode is a stable identifier for one management-question
// template — task section 41/106. Only codes actually generated are
// defined; see questionTemplates.
type QuestionCode string

const (
	QuestionAROverdueIncreased             QuestionCode = "AR_OVERDUE_INCREASED"
	QuestionSupplierConcentrationIncreased QuestionCode = "SUPPLIER_CONCENTRATION_INCREASED"
	QuestionNegativeContributionEntity     QuestionCode = "NEGATIVE_CONTRIBUTION_ENTITY"
	QuestionMinimumCashBelowThreshold      QuestionCode = "MINIMUM_CASH_BELOW_THRESHOLD"
	QuestionCovenantHeadroomNarrowed       QuestionCode = "COVENANT_HEADROOM_NARROWED"
	QuestionMarginDeclined                 QuestionCode = "MARGIN_DECLINED"
	QuestionCustomerRetentionChanged       QuestionCode = "CUSTOMER_RETENTION_CHANGED"
	QuestionInventoryBuildOutpacingUsage   QuestionCode = "INVENTORY_BUILD_OUTPACING_USAGE"
)

// ManagementQuestion is one deterministic, template-based prompt derived
// from a supported fact — task section 41. Never embeds a conclusion (task
// section 42's "invite human context, do not embed a conclusion in the
// question" rule) — see questionTemplates for the fixed, reviewed
// question text.
type ManagementQuestion struct {
	Code      QuestionCode `json:"code"`
	Question  string       `json:"question"`
	Category  string       `json:"category"`
	Period    string       `json:"period,omitempty"`
	EntityRef string       `json:"entity_ref,omitempty"`

	SourceModule string      `json:"source_module,omitempty"`
	SourceCode   string      `json:"source_code,omitempty"`
	SourceRefs   []SourceRef `json:"source_refs,omitempty"`
}

// questionTemplates is the closed, finite map from a synthesis/source code
// to its fixed question text — task section 106/42. Every question uses
// neutral, context-inviting phrasing; TestNoPrescriptiveLanguage
// (safety_test.go) scans these too.
var questionTemplates = map[QuestionCode]struct {
	Text     string
	Category string
}{
	QuestionAROverdueIncreased: {
		Text:     "What factors explain the increase in receivables over 90 days?",
		Category: string(SectionWorkingCapital),
	},
	QuestionSupplierConcentrationIncreased: {
		Text:     "Is the increase in supplier concentration expected or intentional?",
		Category: string(SectionVendorSpend),
	},
	QuestionNegativeContributionEntity: {
		Text:     "What factors explain the negative contribution result for this entity?",
		Category: string(SectionProfitability),
	},
	QuestionMinimumCashBelowThreshold: {
		Text:     "What near-term actions, if any, are already planned given the projected minimum cash position?",
		Category: string(SectionLiquidity),
	},
	QuestionCovenantHeadroomNarrowed: {
		Text:     "Is the narrowing covenant headroom expected to continue, and has the lender been informed?",
		Category: string(SectionDebtAndCovenants),
	},
	QuestionMarginDeclined: {
		Text:     "What factors are contributing to the change in margin this period?",
		Category: string(SectionProfitability),
	},
	QuestionCustomerRetentionChanged: {
		Text:     "What factors explain the change in customer retention this period?",
		Category: string(SectionRevenue),
	},
	QuestionInventoryBuildOutpacingUsage: {
		Text:     "Is the current pace of inventory build relative to usage expected to continue?",
		Category: string(SectionInventory),
	},
}

// newManagementQuestion builds a ManagementQuestion from code, returning
// ok == false for an unrecognized code (a programming-error guard, since
// every caller passes one of this package's own QuestionCode constants).
func newManagementQuestion(code QuestionCode, period, entityRef, sourceModule, sourceCode string, refs []SourceRef) (ManagementQuestion, bool) {
	tmpl, ok := questionTemplates[code]
	if !ok {
		return ManagementQuestion{}, false
	}
	return ManagementQuestion{
		Code: code, Question: tmpl.Text, Category: tmpl.Category,
		Period: period, EntityRef: entityRef,
		SourceModule: sourceModule, SourceCode: sourceCode, SourceRefs: refs,
	}, true
}

// insightCodeToQuestion is the finite, fixed map from an Insight's own
// Code (this package's StatementCode or a synthesis code) to the
// ManagementQuestion it prompts — task section 106's "finite map: source/
// synthesis code -> question template" instruction. Only Insight codes
// this package actually generates that also have a natural open question
// are mapped; most Insight codes have no corresponding question.
var insightCodeToQuestion = map[string]QuestionCode{
	string(StatementAROver90Increased):              QuestionAROverdueIncreased,
	string(StatementSupplierConcentrationIncreased): QuestionSupplierConcentrationIncreased,
	"NEGATIVE_CONTRIBUTION_ENTITY":                  QuestionNegativeContributionEntity,
	string(StatementMinimumCashBelowThreshold):      QuestionMinimumCashBelowThreshold,
	"COVENANT_HEADROOM_NARROW":                      QuestionCovenantHeadroomNarrowed,
	string(StatementContributionMarginDeclined):     QuestionMarginDeclined,
	"LOST_CUSTOMER_REVENUE":                         QuestionCustomerRetentionChanged,
	string(StatementInventoryBuildOutpacingUsage):   QuestionInventoryBuildOutpacingUsage,
}

// buildManagementQuestions derives the QuestionsForManagement section from
// every built Section's own Findings, via insightCodeToQuestion — task
// section 41/106. Deduplicated by QuestionCode + EntityRef (the same
// underlying condition should prompt one question, not one per
// contributing Insight). Returns nil when policy.IncludeManagementQuestions
// is false — task section 41's "caller can disable this section" rule.
func buildManagementQuestions(sections []Section, policy Policy) []ManagementQuestion {
	if !policy.IncludeManagementQuestions {
		return nil
	}
	type key struct {
		code      QuestionCode
		entityRef string
	}
	seen := make(map[key]bool)
	var out []ManagementQuestion
	for _, s := range sections {
		for _, f := range s.Findings {
			qc, ok := insightCodeToQuestion[f.Code]
			if !ok {
				continue
			}
			k := key{code: qc, entityRef: f.EntityRef}
			if seen[k] {
				continue
			}
			seen[k] = true
			q, ok := newManagementQuestion(qc, f.Period, f.EntityRef, f.SourceModule, f.SourceCode, f.SourceRefs)
			if ok {
				out = append(out, q)
			}
		}
	}
	return out
}
