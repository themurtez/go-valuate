package closechecklist

// Section codes used by the example templates below. These are ordinary
// caller-defined opaque codes — this package attaches no behavior to any
// particular value (section 3/6).
const (
	SectionCash                = "CASH"
	SectionAR                  = "AR"
	SectionAP                  = "AP"
	SectionPayroll             = "PAYROLL"
	SectionInventory           = "INVENTORY"
	SectionJournalReview       = "JOURNAL_REVIEW"
	SectionFinancialStatements = "FINANCIAL_STATEMENTS"
	SectionManagementReview    = "MANAGEMENT_REVIEW"
	SectionFinalClose          = "FINAL_CLOSE"
)

// ServiceBusinessMonthlyClose returns an ordinary example Template for a
// simple service business's monthly close. This is an example, not an
// accounting standard, and hard-codes no tax/industry-specific
// requirements (section 30).
func ServiceBusinessMonthlyClose() Template {
	sections := []SectionDefinition{
		{SectionCode: SectionCash, Name: "Cash"},
		{SectionCode: SectionAR, Name: "Accounts Receivable"},
		{SectionCode: SectionAP, Name: "Accounts Payable"},
		{SectionCode: SectionPayroll, Name: "Payroll"},
		{SectionCode: SectionJournalReview, Name: "Journal Review"},
		{SectionCode: SectionFinancialStatements, Name: "Financial Statements"},
		{SectionCode: SectionManagementReview, Name: "Management Review"},
		{SectionCode: SectionFinalClose, Name: "Final Close"},
	}

	tasks := []TaskDefinition{
		{
			TaskCode: "bank_reconciliation", SectionCode: SectionCash,
			Name: "Bank reconciliation", Required: true,
			Applicability:  ApplicabilityRule{Type: ApplicabilityAlways},
			EvidencePolicy: EvidencePolicy{Type: EvidenceAtLeastOne, RequiredTypes: nil},
			GateRules:      []GateRule{{GateCode: "reconciliation.cash_main", Require: GateRequirePass, Required: true}},
			DueRule:        DueRule{Type: DueRulePeriodEnd, OffsetDays: 3},
		},
		{
			TaskCode: "credit_card_reconciliation", SectionCode: SectionCash,
			Name: "Credit card reconciliation", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityCallerFlag, FlagKey: "has_credit_card"},
			GateRules:     []GateRule{{GateCode: "reconciliation.credit_card_main", Require: GateRequirePassOrWarning, Required: true}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 3},
		},
		{
			TaskCode: "ar_control_reconciliation", SectionCode: SectionAR,
			Name: "AR control reconciliation", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			GateRules:     []GateRule{{GateCode: "closequality.ar_control", Require: GateRequirePassOrWarning, Required: true}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 4},
		},
		{
			TaskCode: "ap_control_reconciliation", SectionCode: SectionAP,
			Name: "AP control reconciliation", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			GateRules:     []GateRule{{GateCode: "closequality.ap_control", Require: GateRequirePassOrWarning, Required: true}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 4},
		},
		{
			TaskCode: "payroll_review", SectionCode: SectionPayroll,
			Name: "Payroll review", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityCallerFlag, FlagKey: "has_payroll"},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 5},
		},
		{
			TaskCode: "accrual_prepaid_review", SectionCode: SectionJournalReview,
			Name: "Accrual and prepaid review", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies:  []TaskDependency{{DependsOnTaskCode: "bank_reconciliation", Type: DependencyMustBeCompleted}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 5},
		},
		{
			TaskCode: "journal_review", SectionCode: SectionJournalReview,
			Name: "Journal review", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			GateRules:     []GateRule{{GateCode: "closequality.journal_review", Require: GateRequirePassOrWarning, Required: true}},
			Dependencies:  []TaskDependency{{DependsOnTaskCode: "accrual_prepaid_review", Type: DependencyMustBeCompleted}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 6},
		},
		{
			TaskCode: "financial_statement_build", SectionCode: SectionFinancialStatements,
			Name: "Financial statement build", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies: []TaskDependency{
				{DependsOnTaskCode: "ar_control_reconciliation", Type: DependencyMustBeCompleted},
				{DependsOnTaskCode: "ap_control_reconciliation", Type: DependencyMustBeCompleted},
				{DependsOnTaskCode: "journal_review", Type: DependencyMustBeCompleted},
			},
			GateRules: []GateRule{{GateCode: "closequality.statement_integrity", Require: GateRequirePass, Required: true}},
			DueRule:   DueRule{Type: DueRulePeriodEnd, OffsetDays: 7},
		},
		{
			TaskCode: "financial_statement_review", SectionCode: SectionFinancialStatements,
			Name: "Financial statement review", Required: true,
			Applicability:  ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies:   []TaskDependency{{DependsOnTaskCode: "financial_statement_build", Type: DependencyMustBeCompleted}},
			ReviewPolicy:   ReviewPolicy{Type: ReviewPreparerAndReviewer, RequireDistinctActors: true},
			EvidencePolicy: EvidencePolicy{Type: EvidenceAtLeastOne},
			DueRule:        DueRule{Type: DueRulePeriodEnd, OffsetDays: 8},
		},
		{
			TaskCode: "controller_review", SectionCode: SectionManagementReview,
			Name: "Controller review", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies:  []TaskDependency{{DependsOnTaskCode: "financial_statement_review", Type: DependencyMustBeCompleted}},
			ReviewPolicy:  ReviewPolicy{Type: ReviewSpecificRoles, RequiredRoles: []SignOffRole{SignOffController}},
			GateRules:     []GateRule{{GateCode: "closequality.overall", Require: GateRequirePassOrWarning, Required: true}},
			DueRule:       DueRule{Type: DueRuleTargetClose, OffsetDays: -1},
		},
		{
			TaskCode: "final_close_approval", SectionCode: SectionFinalClose,
			Name: "Final close approval", Required: true,
			Applicability:   ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies:    []TaskDependency{{DependsOnTaskCode: "controller_review", Type: DependencyMustBeCompleted}},
			ReviewPolicy:    ReviewPolicy{Type: ReviewSpecificRoles, RequiredRoles: []SignOffRole{SignOffApprover}},
			ExceptionPolicy: ExceptionAllowedWithApproval,
			DueRule:         DueRule{Type: DueRuleTargetClose},
		},
	}

	return Template{
		TemplateID: "service_business_monthly_close",
		Name:       "Service Business Monthly Close",
		Version:    "2025.1",
		Sections:   sections,
		Tasks:      tasks,
	}
}

// InventoryBusinessMonthlyClose extends ServiceBusinessMonthlyClose with
// inventory-specific tasks (section 30). Also an example, not a
// standard.
func InventoryBusinessMonthlyClose() Template {
	base := ServiceBusinessMonthlyClose()

	sections := append(append([]SectionDefinition{}, base.Sections[:len(base.Sections)-1]...),
		SectionDefinition{SectionCode: SectionInventory, Name: "Inventory"},
		base.Sections[len(base.Sections)-1], // FINAL_CLOSE stays last
	)

	inventoryTasks := []TaskDefinition{
		{
			TaskCode: "inventory_reconciliation", SectionCode: SectionInventory,
			Name: "Inventory reconciliation", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			GateRules:     []GateRule{{GateCode: "reconciliation.inventory_control", Require: GateRequirePassOrWarning, Required: true}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 5},
		},
		{
			TaskCode: "inventory_adjustment_review", SectionCode: SectionInventory,
			Name: "Inventory adjustment review", Required: true,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies:  []TaskDependency{{DependsOnTaskCode: "inventory_reconciliation", Type: DependencyMustBeResolved}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 6},
		},
		{
			TaskCode: "inventory_aging_review", SectionCode: SectionInventory,
			Name: "Inventory aging and slow-moving review", Required: false,
			Applicability: ApplicabilityRule{Type: ApplicabilityAlways},
			Dependencies:  []TaskDependency{{DependsOnTaskCode: "inventory_reconciliation", Type: DependencyMustBeResolved}},
			DueRule:       DueRule{Type: DueRulePeriodEnd, OffsetDays: 6},
		},
	}

	tasks := make([]TaskDefinition, 0, len(base.Tasks)+len(inventoryTasks))
	for _, t := range base.Tasks {
		if t.TaskCode == "financial_statement_build" {
			t.Dependencies = append(append([]TaskDependency{}, t.Dependencies...),
				TaskDependency{DependsOnTaskCode: "inventory_adjustment_review", Type: DependencyMustBeCompleted})
		}
		tasks = append(tasks, t)
	}
	tasks = append(tasks, inventoryTasks...)

	return Template{
		TemplateID: "inventory_business_monthly_close",
		Name:       "Inventory Business Monthly Close",
		Version:    "2025.1",
		Sections:   sections,
		Tasks:      tasks,
	}
}
