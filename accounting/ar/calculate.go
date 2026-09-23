package ar

import "github.com/themurtez/go-valuate/analytics/concentration"

// Result is Calculate's top-level output — section 27.
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
	CustomerSummaries []CustomerSummary `json:"customer_summaries,omitempty"`

	Concentration ConcentrationSummary `json:"concentration"`

	DSO        DSOResult  `json:"dso"`
	DSOHistory DSOHistory `json:"dso_history"`

	AgingTrend AgingTrend      `json:"aging_trend"`
	Migration  MigrationResult `json:"migration"`

	CollectionMetrics  CollectionMetrics  `json:"collection_metrics"`
	CollectionPriority CollectionPriority `json:"collection_priority"`
	TermsAnalysis      TermsAnalysis      `json:"terms_analysis"`
	WriteOffs          WriteOffSummary    `json:"write_offs"`

	AgingReconciliation          AgingReconciliation          `json:"aging_reconciliation"`
	ControlAccountReconciliation ControlAccountReconciliation `json:"control_account_reconciliation"`

	Dimension DimensionSummary `json:"dimension"`

	Flags  []Flag  `json:"flags,omitempty"`
	Issues []Issue `json:"issues,omitempty"`
}

// Calculate derives a full Result from in under opts. It never mutates
// in.Receivables, in.Payments, in.SalesHistory, in.Snapshots,
// in.WriteOffs, or opts.Buckets, and performs no I/O — see
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

	recvIssues, excluded := validateReceivables(in.Receivables, opts.AsOfDate)
	issues = append(issues, recvIssues...)

	includeStatuses := resolvedIncludeStatuses(opts.IncludeStatuses)
	reportingCurrency, currencyIssues := resolveReportingCurrency(in.Receivables, excluded, opts.ReportingCurrency)
	issues = append(issues, currencyIssues...)

	receivablesByID := map[string]Receivable{}
	seen := map[string]bool{}
	var rows []receivableAging
	var writtenOff float64

	for _, r := range in.Receivables {
		if seen[r.ID] {
			continue // duplicate; already flagged, keep first occurrence only.
		}
		seen[r.ID] = true
		if r.ID != "" {
			receivablesByID[r.ID] = r
		}
		if excluded[r.ID] {
			continue
		}

		if r.Status == StatusWrittenOff {
			writtenOff += r.OpenAmount
			continue
		}
		if !includedStatus(r.Status, includeStatuses) {
			continue
		}

		included := r.Currency == reportingCurrency
		days := ageDays(opts.AsOfDate, basisDateFor(r, basis))
		bucketCode := ""
		if included {
			code, ok := bucketForDays(sortedBuckets, days)
			if !ok {
				included = false
			} else {
				bucketCode = code
			}
		}

		isCredit := resolvedDocumentType(r.DocumentType) == DocumentTypeCreditMemo

		rows = append(rows, receivableAging{
			r:             r,
			daysPastDue:   days,
			bucketCode:    bucketCode,
			isCredit:      isCredit,
			includedInAgg: included,
		})
	}

	netting := resolvedCreditNettingPolicy(opts.CreditNetting)

	portfolio := buildPortfolioSummary(rows, sortedBuckets, writtenOff, netting)
	result.PortfolioSummary = portfolio

	customers := buildCustomerSummaries(rows, sortedBuckets)

	result.AgingReconciliation = buildAgingReconciliation(portfolio.TotalOpenReceivables, portfolio.Buckets)
	if !result.AgingReconciliation.Balanced {
		issues = append(issues, Issue{Code: IssueUnbalancedAging, Severity: SeverityWarning, Message: "sum of aging buckets does not equal total open receivables"})
	}

	result.ControlAccountReconciliation = buildControlAccountReconciliation(portfolio.TotalOpenReceivables, opts.ControlAccountBalance, opts.ControlAccountTolerance)
	if result.ControlAccountReconciliation.Available && !result.ControlAccountReconciliation.Reconciled {
		issues = append(issues, Issue{Code: IssueControlAccountMismatch, Severity: SeverityWarning, Message: "subledger AR total does not match supplied control account balance"})
	}

	overdueTotal := portfolio.OverdueTotal
	o60Total := sumBucketsAtLeast(portfolio.Buckets, sortedBuckets, 60)
	o90Total := sumBucketsAtLeast(portfolio.Buckets, sortedBuckets, 90)
	result.Concentration = buildConcentrationSummary(customers, sortedBuckets, overdueTotal, o60Total, o90Total, concentrationTopN, concentration.DefaultPolicy())

	if len(in.SalesHistory) == 0 {
		issues = append(issues, Issue{Code: IssueMissingSalesForDSO, Severity: SeverityWarning, Message: "no sales history supplied; DSO unavailable"})
	}
	result.DSO = calculateCurrentDSO(portfolio.TotalOpenReceivables, in.SalesHistory)
	result.DSOHistory = calculateDSOHistory(in.SalesHistory)

	result.AgingTrend = calculateAgingTrend(in.Snapshots, sortedBuckets, basis, includeStatuses)
	result.Migration = calculateMigration(in.Snapshots, rows, sortedBuckets, basis, result.AsOfDate)

	paymentIssues, excludedPayments := validatePayments(in.Payments, seen)
	issues = append(issues, paymentIssues...)
	timing := calculatePaymentTiming(in.Payments, receivablesByID, excludedPayments)
	result.TermsAnalysis = calculateTermsAnalysis(rows, timing)
	result.CollectionMetrics = calculateCollectionMetrics(in.Payments, excludedPayments, result.AgingTrend, timing, result.Migration)

	latePaymentCounts := countLatePaymentsByCustomer(in.Payments, excludedPayments, receivablesByID)

	// assessMateriality applies its own "either rule triggers" OR logic
	// (absolute threshold, when set, OR percent-of-AR threshold), so it is
	// given opts.MaterialityThreshold directly (0 means "no absolute rule")
	// rather than resolveMaterialityAbsolute's blended fallback figure
	// (that fallback exists only for resolveThresholds' single-figure
	// flag-threshold defaults below, which have no percent-of-AR
	// alternative of their own).
	materialityPercent := resolvedMaterialityPercent(opts.MaterialityPercentOfAR)
	for i := range customers {
		customers[i].Materiality = assessMateriality(customers[i].OverdueTotal, opts.MaterialityThreshold, materialityPercent, portfolio.TotalOpenReceivables)
	}
	result.CustomerSummaries = customers

	materialityAbs := resolveMaterialityAbsolute(opts.MaterialityThreshold, opts.MaterialityPercentOfAR, portfolio.TotalOpenReceivables)
	thresholds := resolveThresholds(opts.Thresholds, materialityAbs)
	result.Flags = computeFlags(flagInputs{
		customers:         customers,
		sortedBuckets:     sortedBuckets,
		concentration:     result.Concentration,
		overdueTotal:      overdueTotal,
		o60Total:          o60Total,
		o90Total:          o90Total,
		dsoHistory:        result.DSOHistory,
		agingTrend:        result.AgingTrend,
		latePaymentCounts: latePaymentCounts,
		thresholds:        thresholds,
	})

	result.CollectionPriority = calculateCollectionPriority(customers, sortedBuckets, overdueTotal, result.Concentration, opts.PriorityWeights)

	result.WriteOffs = calculateWriteOffSummary(in.WriteOffs, nil)

	result.Dimension = buildDimensionSummary(rows, sortedBuckets, opts.Dimension)

	result.Issues = issues
	result.Available = true
	return result
}

// concentrationTopN is the fixed top-N cutoff used for overdue/60+/90+
// concentration ranking (RankedCustomer lists) — kept generous since
// callers can truncate client-side; 10 mirrors
// analytics/concentration.DefaultPolicy's largest default cutoff.
const concentrationTopN = 10

func sumBucketsAtLeast(buckets []BucketAmount, sortedBuckets []BucketDefinition, minDays int) float64 {
	var total float64
	for _, b := range buckets {
		def := bucketDefByCode(sortedBuckets, b.BucketCode)
		if def != nil && def.MinDaysPastDue >= minDays {
			total += b.Amount
		}
	}
	return total
}

func buildPortfolioSummary(rows []receivableAging, sortedBuckets []BucketDefinition, writtenOff float64, netting CreditNettingPolicy) PortfolioSummary {
	var grossReceivables, openTotal, creditTotal float64
	var disputed float64
	var openCount, overdueCount int
	customerSet := map[string]bool{}
	var creditCount int
	var weightedDaysSum float64
	oldestDays := -1

	for _, row := range rows {
		customerSet[row.r.CustomerID] = true
		if row.isCredit {
			creditTotal += row.r.OpenAmount
			creditCount++
			continue
		}
		if !row.includedInAgg {
			continue
		}
		grossReceivables += row.r.OriginalAmount
		openTotal += row.r.OpenAmount
		weightedDaysSum += row.r.OpenAmount * float64(row.daysPastDue)
		if row.r.OpenAmount != 0 {
			openCount++
			if row.daysPastDue > oldestDays {
				oldestDays = row.daysPastDue
			}
		}
		if row.daysPastDue > 0 && row.r.OpenAmount != 0 {
			overdueCount++
		}
		if row.r.Status == StatusDisputed {
			disputed += row.r.OpenAmount
		}
	}

	if netting == CreditNetByCustomer {
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
		TotalGrossReceivables: grossReceivables,
		TotalOpenReceivables:  openTotal,
		Buckets:               buckets,
		OverdueTotal:          openTotal - current,
		OpenInvoiceCount:      openCount,
		OverdueInvoiceCount:   overdueCount,
		CustomerCount:         len(customerSet),
		DisputedAmount:        disputed,
		WrittenOffAmount:      writtenOff,
		Credits: CreditSummary{
			TotalCreditBalance: creditTotal,
			CreditMemoCount:    creditCount,
			NettingApplied:     netting,
		},
	}
	if openTotal != 0 {
		p.CurrentAmount = AvailableAmount(current)
		p.PercentOverdue = AvailableAmount(p.OverdueTotal / openTotal)
		p.WeightedAvgDaysPastDue = AvailableAmount(weightedDaysSum / openTotal)
	}
	if oldestDays >= 0 {
		p.OldestOpenReceivableDays = AvailableAmount(float64(oldestDays))
	}
	return p
}

func countLatePaymentsByCustomer(payments []Payment, excluded map[string]bool, receivablesByID map[string]Receivable) map[string]int {
	counts := map[string]int{}
	for _, p := range payments {
		if excluded[p.ID] {
			continue
		}
		r, ok := receivablesByID[p.ReceivableID]
		if !ok || r.DueDate.IsZero() {
			continue
		}
		if p.Date.After(r.DueDate) {
			counts[p.CustomerID]++
		}
	}
	return counts
}
