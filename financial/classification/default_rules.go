package classification

import "github.com/themurtez/go-valuate/financial"

// DefaultRulesVersion identifies the fixed rule set DefaultRules returns.
// Bump this whenever a rule is added, removed, or its matching/precedence
// behavior changes in a way that could make a historical classification
// Result not reproduce identically under the new code — see the
// repository README's versioning-strategy section. A caller assembling a
// fully custom Config.Rules (not using DefaultRules at all) is versioning
// its own rule set independently; this constant only describes the
// built-in one.
const DefaultRulesVersion = "1.0.0"

// DefaultRules returns the built-in set of context-aware and phrase/token
// rules, in the precedence order they should be evaluated (earlier rules win
// ties by virtue of running first, though Classify continues scanning all
// rules to build Result.Alternatives). Callers building a Config are free to
// use DefaultRules() as-is, extend it with their own rules appended (lower
// precedence) or prepended (higher precedence), or ignore it entirely and
// supply a fully custom rule set.
//
// Rules here intentionally cover only the illustrative cases from the
// package's design brief. Real-world coverage is expected to grow via
// caller-supplied aliases first (cheap, data-driven, no code changes) and
// via additional Rule values only for genuinely context-dependent cases
// aliases cannot express, such as the "labor" example below where the same
// label maps to different codes depending on its parent section.
func DefaultRules() []Rule {
	return []Rule{
		laborParentAwareRule(),
		accountsReceivableBalanceSheetRule(),
		depreciationPhraseRule(),
		amortizationPhraseRule(),
		interestExpensePhraseRule(),
		interestIncomePhraseRule(),
		marketingPhraseRule(),
		payrollPhraseRule(),
		ownerCompPhraseRule(),
		rentPhraseRule(),
		insurancePhraseRule(),
		utilitiesPhraseRule(),
		softwarePhraseRule(),
		professionalFeesPhraseRule(),
		repairsPhraseRule(),
		vehiclePhraseRule(),
		travelPhraseRule(),
		officeSuppliesPhraseRule(),
		freightPhraseRule(),
	}
}

// laborParentAwareRule demonstrates a context-aware rule: the same label
// ("Field Installers", "Field Labor", "Subcontract Installers", etc.) maps
// to different canonical codes depending on which section of the income
// statement it appears under. Labor under a cost-of-sales/COGS section is
// direct labor; labor under an operating-expenses section is payroll.
// Neither guess is safe without the parent, so this rule requires one.
func laborParentAwareRule() Rule {
	laborTokens := []string{"labor", "installer", "installers", "technician", "technicians", "crew"}
	cogsParentPhrases := []string{"cost of sales", "cost of goods sold", "cogs", "direct costs"}
	opexParentPhrases := []string{"operating expenses", "operating expense", "opex", "sg&a", "sganda"}

	return RuleFunc{
		RuleName: "labor_parent_aware",
		MatchFn: func(in RuleInput) (RuleMatch, bool) {
			if _, ok := containsAnyToken(in.Label.Comparable, laborTokens...); !ok {
				return RuleMatch{}, false
			}
			if in.ParentLabel.Comparable == "" {
				return RuleMatch{}, false
			}
			for _, phrase := range cogsParentPhrases {
				if containsPhrase(in.ParentLabel.Comparable, phrase) {
					return RuleMatch{
						Code:       financial.CodeCogsDirectLabor,
						Confidence: ConfidenceStrongRule,
						Reason:     "labor-related label under a cost-of-sales parent section",
					}, true
				}
			}
			for _, phrase := range opexParentPhrases {
				if containsPhrase(in.ParentLabel.Comparable, phrase) {
					return RuleMatch{
						Code:       financial.CodeOpexPayroll,
						Confidence: ConfidenceStrongRule,
						Reason:     "labor-related label under an operating-expenses parent section",
					}, true
				}
			}
			return RuleMatch{}, false
		},
	}
}

// accountsReceivableBalanceSheetRule requires both the phrase match and the
// correct statement type, since "Accounts Receivable" only means the
// canonical balance-sheet code when the row actually comes from a balance
// sheet.
func accountsReceivableBalanceSheetRule() Rule {
	return RuleFunc{
		RuleName: "accounts_receivable_balance_sheet",
		MatchFn: func(in RuleInput) (RuleMatch, bool) {
			if in.StatementType != financial.StatementBalanceSheet {
				return RuleMatch{}, false
			}
			if !containsPhrase(in.Label.Comparable, "accounts receivable") {
				return RuleMatch{}, false
			}
			return RuleMatch{
				Code:       financial.CodeBsAccountsReceivable,
				Confidence: ConfidenceStrongRule,
				Reason:     "matched \"accounts receivable\" on a balance sheet row",
			}, true
		},
	}
}

// phraseRule builds a simple Rule that matches when any of tokens appears as
// a whole word/phrase in the normalized label, independent of any context.
// Because these rules have no context to lean on, they use the weaker
// ConfidenceWeakRule tier rather than ConfidenceStrongRule.
func phraseRule(name string, code financial.Code, reason string, tokens ...string) Rule {
	return RuleFunc{
		RuleName: name,
		MatchFn: func(in RuleInput) (RuleMatch, bool) {
			if _, ok := containsAnyToken(in.Label.Comparable, tokens...); !ok {
				return RuleMatch{}, false
			}
			return RuleMatch{
				Code:       code,
				Confidence: ConfidenceWeakRule,
				Reason:     reason,
			}, true
		},
	}
}

// contextBoostedPhraseRule is like phraseRule, but for phrases specific
// enough to income-statement expense lines that a stronger confidence is
// warranted even without parent context (e.g. "depreciation expense" is
// unambiguous almost everywhere it appears).
func contextBoostedPhraseRule(name string, code financial.Code, reason string, tokens ...string) Rule {
	return RuleFunc{
		RuleName: name,
		MatchFn: func(in RuleInput) (RuleMatch, bool) {
			if _, ok := containsAnyToken(in.Label.Comparable, tokens...); !ok {
				return RuleMatch{}, false
			}
			return RuleMatch{
				Code:       code,
				Confidence: ConfidenceStrongRule,
				Reason:     reason,
			}, true
		},
	}
}

func depreciationPhraseRule() Rule {
	return contextBoostedPhraseRule(
		"depreciation_phrase",
		financial.CodeDepreciation,
		`matched "depreciation"`,
		"depreciation",
	)
}

func amortizationPhraseRule() Rule {
	return contextBoostedPhraseRule(
		"amortization_phrase",
		financial.CodeAmortization,
		`matched "amortization"`,
		"amortization", "amortisation",
	)
}

func interestExpensePhraseRule() Rule {
	return RuleFunc{
		RuleName: "interest_expense_phrase",
		MatchFn: func(in RuleInput) (RuleMatch, bool) {
			if !containsToken(in.Label.Comparable, "interest") {
				return RuleMatch{}, false
			}
			if containsToken(in.Label.Comparable, "income") {
				return RuleMatch{}, false
			}
			return RuleMatch{
				Code:       financial.CodeInterestExpense,
				Confidence: ConfidenceStrongRule,
				Reason:     `matched "interest" without "income"`,
			}, true
		},
	}
}

func interestIncomePhraseRule() Rule {
	return RuleFunc{
		RuleName: "interest_income_phrase",
		MatchFn: func(in RuleInput) (RuleMatch, bool) {
			if !containsToken(in.Label.Comparable, "interest") || !containsToken(in.Label.Comparable, "income") {
				return RuleMatch{}, false
			}
			return RuleMatch{
				Code:       financial.CodeInterestIncome,
				Confidence: ConfidenceStrongRule,
				Reason:     `matched "interest income"`,
			}, true
		},
	}
}

func marketingPhraseRule() Rule {
	return phraseRule(
		"marketing_phrase",
		financial.CodeOpexMarketing,
		`matched a marketing/advertising-related term`,
		"advertising", "marketing", "promotion", "promotions", "ads",
	)
}

func payrollPhraseRule() Rule {
	return phraseRule(
		"payroll_phrase",
		financial.CodeOpexPayroll,
		`matched a payroll-related term`,
		"payroll", "wages", "salaries", "salary",
	)
}

func ownerCompPhraseRule() Rule {
	return contextBoostedPhraseRule(
		"owner_comp_phrase",
		financial.CodeOpexOwnerComp,
		`matched an owner/officer compensation term`,
		"owner",
	)
}

func rentPhraseRule() Rule {
	return phraseRule(
		"rent_phrase",
		financial.CodeOpexRent,
		`matched "rent"`,
		"rent",
	)
}

func insurancePhraseRule() Rule {
	return phraseRule(
		"insurance_phrase",
		financial.CodeOpexInsurance,
		`matched "insurance"`,
		"insurance",
	)
}

func utilitiesPhraseRule() Rule {
	return phraseRule(
		"utilities_phrase",
		financial.CodeOpexUtilities,
		`matched a utilities-related term`,
		"utilities", "utility", "electric", "electricity", "gas", "water",
	)
}

func softwarePhraseRule() Rule {
	return phraseRule(
		"software_phrase",
		financial.CodeOpexSoftware,
		`matched a software/subscription-related term`,
		"software", "subscription", "subscriptions", "saas",
	)
}

func professionalFeesPhraseRule() Rule {
	return phraseRule(
		"professional_fees_phrase",
		financial.CodeOpexProfessionalFees,
		`matched a professional-services-related term`,
		"legal", "accounting", "consulting", "professional",
	)
}

func repairsPhraseRule() Rule {
	return phraseRule(
		"repairs_phrase",
		financial.CodeOpexRepairs,
		`matched a repairs/maintenance-related term`,
		"repairs", "maintenance", "repair",
	)
}

func vehiclePhraseRule() Rule {
	return phraseRule(
		"vehicle_phrase",
		financial.CodeOpexVehicle,
		`matched a vehicle-related term`,
		"vehicle", "vehicles", "truck", "trucks", "auto", "automobile",
	)
}

func travelPhraseRule() Rule {
	return phraseRule(
		"travel_phrase",
		financial.CodeOpexTravel,
		`matched "travel"`,
		"travel",
	)
}

func officeSuppliesPhraseRule() Rule {
	return phraseRule(
		"office_supplies_phrase",
		financial.CodeOpexOffice,
		`matched an office-supplies-related term`,
		"office", "supplies",
	)
}

func freightPhraseRule() Rule {
	return phraseRule(
		"freight_phrase",
		financial.CodeCogsFreight,
		`matched a freight/shipping-related term`,
		"freight", "shipping",
	)
}
