package ap

import "github.com/themurtez/go-valuate/analytics/concentration"

// Result is Calculate's top-level output.
type Result struct {
	SchemaVersion  string `json:"schema_version"`
	FormulaVersion string `json:"formula_version"`
	// Available is false only if Calculate could not proceed at all (see
	// IssueInvalidBucketConfiguration on an unusable bucket schema, or a
	// zero-value AsOfDate) — every other field is then zero-value.
	Available bool `json:"available"`

	AsOfDate string             `json:"as_of_date"`
	Basis    AgingBasis         `json:"basis"`
	Buckets  []BucketDefinition `json:"buckets"`

	PortfolioSummary  PortfolioSummary  `json:"portfolio_summary"`
	SupplierSummaries []SupplierSummary `json:"supplier_summaries,omitempty"`

	Concentration ConcentrationSummary `json:"concentration"`

	DPO        DPOResult  `json:"dpo"`
	DPOHistory DPOHistory `json:"dpo_history"`

	AgingTrend AgingTrend      `json:"aging_trend"`
	Migration  MigrationResult `json:"migration"`

	PaymentMetrics PaymentMetrics `json:"payment_metrics"`
	TermsAnalysis  TermsAnalysis  `json:"terms_analysis"`

	DueSchedule     DueSchedule     `json:"due_schedule"`
	PaymentPressure PaymentPressure `json:"payment_pressure"`

	AgingReconciliation          AgingReconciliation          `json:"aging_reconciliation"`
	ControlAccountReconciliation ControlAccountReconciliation `json:"control_account_reconciliation"`

	Dimension DimensionSummary `json:"dimension"`

	Flags  []Flag  `json:"flags,omitempty"`
	Issues []Issue `json:"issues,omitempty"`
}

// Calculate derives a full Result from in under opts. It never mutates
// in.Payables, in.Payments, in.PurchasesHistory, in.Snapshots,
// opts.Buckets, or opts.DueScheduleHorizons, and performs no I/O — see
// immutability_test.go.
func Calculate(in Input, opts Options) Result {
	var issues []Issue

	basis := resolvedAgingBasis(opts.Basis)
	buckets := resolvedBuckets(opts.Buckets)
	bucketIssues := validateBucketDefinitions(buckets, opts.AllowBucketGaps)
	issues = append(issues, bucketIssues...)

	result := Result{
		SchemaVersion:  SchemaVersion,
		FormulaVersion: FormulaVersion,
		Basis:          basis,
		Buckets:        buckets,
	}

	if opts.AsOfDate.IsZero() {
		issues = append(issues, Issue{Code: IssueInvalidDate, Severity: SeverityError, Message: "AsOfDate is required"})
		result.Issues = issues
		return result
	}
	result.AsOfDate = opts.AsOfDate.Format("2006-01-02")

	if HasErrors(bucketIssues) {
		result.Issues = issues
		return result
	}

	sortedBuckets := sortedBucketsByMin(buckets)

	payIssues, excluded := validatePayables(in.Payables, opts.AsOfDate)
	issues = append(issues, payIssues...)

	includeStatuses := resolvedIncludeStatuses(opts.IncludeStatuses)
	reportingCurrency, currencyIssues := resolveReportingCurrency(in.Payables, excluded, opts.ReportingCurrency)
	issues = append(issues, currencyIssues...)

	payablesByID := map[string]Payable{}
	seen := map[string]bool{}
	var rows []payableAging

	for _, p := range in.Payables {
		if seen[p.ID] {
			continue // duplicate; already flagged, keep first occurrence only.
		}
		seen[p.ID] = true
		if p.ID != "" {
			payablesByID[p.ID] = p
		}
		if excluded[p.ID] {
			continue
		}

		if !includedStatus(p.Status, includeStatuses) {
			continue
		}

		included := p.Currency == reportingCurrency
		days := ageDays(opts.AsOfDate, basisDateFor(p, basis))
		bucketCode := ""
		if included {
			code, ok := bucketForDays(sortedBuckets, days)
			if !ok {
				included = false
			} else {
				bucketCode = code
			}
		}

		isCredit := resolvedDocumentType(p.DocumentType) == DocumentTypeVendorCredit

		rows = append(rows, payableAging{
			p:             p,
			daysPastDue:   days,
			bucketCode:    bucketCode,
			isCredit:      isCredit,
			includedInAgg: included,
		})
	}

	netting := resolvedCreditNettingPolicy(opts.CreditNetting)

	portfolio := buildPortfolioSummary(rows, sortedBuckets, netting)
	result.PortfolioSummary = portfolio

	suppliers := buildSupplierSummaries(rows, sortedBuckets)

	result.AgingReconciliation = buildAgingReconciliation(portfolio.TotalOpenPayables, portfolio.Buckets)
	if !result.AgingReconciliation.Balanced {
		issues = append(issues, Issue{Code: IssueUnbalancedAging, Severity: SeverityWarning, Message: "sum of aging buckets does not equal total open payables"})
	}

	result.ControlAccountReconciliation = buildControlAccountReconciliation(portfolio.TotalOpenPayables, opts.ControlAccountBalance, opts.ControlAccountTolerance)
	if result.ControlAccountReconciliation.Available && !result.ControlAccountReconciliation.Reconciled {
		issues = append(issues, Issue{Code: IssueControlAccountMismatch, Severity: SeverityWarning, Message: "subledger AP total does not match supplied control account balance"})
	}

	overdueTotal := portfolio.OverdueTotal
	o60Total := sumBucketsAtLeast(portfolio.Buckets, sortedBuckets, 60)
	o90Total := sumBucketsAtLeast(portfolio.Buckets, sortedBuckets, 90)
	result.Concentration = buildConcentrationSummary(suppliers, sortedBuckets, overdueTotal, o60Total, o90Total, concentrationTopN, concentration.DefaultPolicy())

	if len(in.PurchasesHistory) == 0 {
		issues = append(issues, Issue{Code: IssueMissingDenominatorForDPO, Severity: SeverityWarning, Message: "no purchases/COGS history supplied; DPO unavailable"})
	}
	result.DPO = calculateCurrentDPO(portfolio.TotalOpenPayables, in.PurchasesHistory)
	result.DPOHistory = calculateDPOHistory(in.PurchasesHistory)

	result.AgingTrend = calculateAgingTrend(in.Snapshots, sortedBuckets, basis, includeStatuses)
	result.Migration = calculateMigration(in.Snapshots, rows, sortedBuckets, basis, result.AsOfDate)

	paymentIssues, excludedPayments := validatePayments(in.Payments, seen)
	issues = append(issues, paymentIssues...)
	timing := calculatePaymentTiming(in.Payments, payablesByID, excludedPayments)
	result.TermsAnalysis = calculateTermsAnalysis(rows, timing)
	result.PaymentMetrics = calculatePaymentMetrics(in.Payments, excludedPayments, result.AgingTrend, timing, result.Migration)

	latePaymentCounts := countLatePaymentsBySupplier(in.Payments, excludedPayments, payablesByID)

	// assessMateriality applies its own "either rule triggers" OR logic
	// (absolute threshold, when set, OR percent-of-AP threshold), so it
	// is given opts.MaterialityThreshold directly (0 means "no absolute
	// rule") rather than resolveMaterialityAbsolute's blended fallback
	// figure (that fallback exists only for resolveThresholds' single-
	// figure flag-threshold defaults below, which have no percent-of-AP
	// alternative of their own).
	materialityPercent := resolvedMaterialityPercent(opts.MaterialityPercentOfAP)
	for i := range suppliers {
		suppliers[i].Materiality = assessMateriality(suppliers[i].OverdueTotal, opts.MaterialityThreshold, materialityPercent, portfolio.TotalOpenPayables)
	}
	result.SupplierSummaries = suppliers

	result.DueSchedule = buildDueSchedule(rows, resolvedDueScheduleHorizons(opts.DueScheduleHorizons), opts.AsOfDate)
	result.PaymentPressure = buildPaymentPressure(rows, opts.AsOfDate, opts.PaymentPressure, opts.ExcludeDisputedFromPaymentPressure)

	materialityAbs := resolveMaterialityAbsolute(opts.MaterialityThreshold, opts.MaterialityPercentOfAP, portfolio.TotalOpenPayables)
	thresholds := resolveThresholds(opts.Thresholds, materialityAbs)
	result.Flags = computeFlags(flagInputs{
		suppliers:         suppliers,
		sortedBuckets:     sortedBuckets,
		concentration:     result.Concentration,
		overdueTotal:      overdueTotal,
		o60Total:          o60Total,
		o90Total:          o90Total,
		dpoHistory:        result.DPOHistory,
		agingTrend:        result.AgingTrend,
		latePaymentCounts: latePaymentCounts,
		paymentPressure:   result.PaymentPressure,
		controlReconciled: result.ControlAccountReconciliation,
		thresholds:        thresholds,
	})

	result.Dimension = buildDimensionSummary(rows, sortedBuckets, opts.Dimension)

	result.Issues = issues
	result.Available = true
	return result
}

func buildPortfolioSummary(rows []payableAging, sortedBuckets []BucketDefinition, netting CreditNettingPolicy) PortfolioSummary {
	var grossPayables, openTotal, creditTotal float64
	var disputed float64
	var openCount, overdueCount int
	supplierSet := map[string]bool{}
	var creditCount int
	var weightedDaysSum float64
	oldestDays := -1

	for _, row := range rows {
		supplierSet[row.p.SupplierID] = true
		if row.isCredit {
			creditTotal += row.p.OpenAmount
			creditCount++
			continue
		}
		if !row.includedInAgg {
			continue
		}
		grossPayables += row.p.OriginalAmount
		openTotal += row.p.OpenAmount
		weightedDaysSum += row.p.OpenAmount * float64(row.daysPastDue)
		if row.p.OpenAmount != 0 {
			openCount++
			if row.daysPastDue > oldestDays {
				oldestDays = row.daysPastDue
			}
		}
		if row.daysPastDue > 0 && row.p.OpenAmount != 0 {
			overdueCount++
		}
		if row.p.Status == StatusDisputed {
			disputed += row.p.OpenAmount
		}
	}

	if netting == CreditNetBySupplier {
		openTotal += creditTotal
	}

	buckets := buildBucketAmounts(rows, sortedBuckets, openTotal)
	var current float64
	for _, b := range buckets {
		if bucketIsCurrent(sortedBuckets, b.BucketCode) {
			current += b.Amount
		}
	}

	p := PortfolioSummary{
		TotalGrossPayables: grossPayables,
		TotalOpenPayables:  openTotal,
		Buckets:            buckets,
		OverdueTotal:       openTotal - current,
		OpenBillCount:      openCount,
		OverdueBillCount:   overdueCount,
		SupplierCount:      len(supplierSet),
		DisputedAmount:     disputed,
		VendorCredits: VendorCreditSummary{
			TotalCreditBalance: creditTotal,
			VendorCreditCount:  creditCount,
			NettingApplied:     netting,
		},
	}
	if openTotal != 0 {
		p.CurrentAmount = AvailableAmount(current)
		p.PercentOverdue = AvailableAmount(p.OverdueTotal / openTotal)
		p.WeightedAvgDaysPastDue = AvailableAmount(weightedDaysSum / openTotal)
	}
	if oldestDays >= 0 {
		p.OldestOpenBillDays = AvailableAmount(float64(oldestDays))
	}
	return p
}
