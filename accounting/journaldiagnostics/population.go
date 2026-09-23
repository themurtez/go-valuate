package journaldiagnostics

import (
	"sort"

	"github.com/themurtez/go-valuate/accounting/ledger"
)

// EntryMagnitude is this package's one documented entry-magnitude
// convention: an entry's total debits (equal to total credits for a
// balanced entry). Never debits + credits — see the package doc comment's
// entry-magnitude note. Used consistently by every amount-based rule in
// this package (round-dollar, large-entry, account-relative, etc.).
func EntryMagnitude(e ledger.JournalEntry) float64 {
	return e.TotalDebits()
}

// analyzedEntry bundles one JournalEntry with its resolved metadata and
// effective date — the unit every rule family in this package operates on.
// Built once by buildPopulation and passed by value (never a pointer into
// caller-owned data) to every rule function.
type analyzedEntry struct {
	entry     ledger.JournalEntry
	meta      EntryMetadata // zero value if no metadata was supplied
	hasMeta   bool
	date      string // resolved effective date, "YYYY-MM-DD" — see resolveDate
	period    string
	magnitude float64
}

// entryAccountIDs returns the sorted, deduplicated Account.ID values posted
// to by e's lines — the shared helper every rule family uses to populate
// Finding.AccountIDs.
func entryAccountIDs(e ledger.JournalEntry) []string {
	seen := make(map[string]bool, len(e.Lines))
	var out []string
	for _, l := range e.Lines {
		if !seen[l.AccountID] {
			seen[l.AccountID] = true
			out = append(out, l.AccountID)
		}
	}
	sort.Strings(out)
	return out
}

// resolveDate returns e.Date if set, otherwise "" — this package never
// derives a date from Period (a caller wanting date-based diagnostics must
// supply Date; a caller with only Period-based entries simply finds
// date-based rules unavailable for those entries, exactly like
// ledger.PeriodRange.Matches' identical rule).
func resolveDate(e ledger.JournalEntry) string {
	return e.Date
}

// Population is the resolved set of entries this Result's diagnostics were
// computed from, after posted-scope filtering and validation exclusion —
// see the package doc's "Posted-entry scope" and "Input validation"
// sections.
type PopulationSummary struct {
	// TotalSuppliedEntries is len(Ledger.Entries) before any filtering.
	TotalSuppliedEntries int `json:"total_supplied_entries"`
	// InScopeEntries is the count after posted-status scope filtering
	// (before validation exclusion).
	InScopeEntries int `json:"in_scope_entries"`
	// AnalyzedEntries is the count actually used for diagnostics: in-scope
	// AND passing validation (see excludedFromDiagnostics).
	AnalyzedEntries int `json:"analyzed_entries"`
	// ExcludedEntries is InScopeEntries - AnalyzedEntries: entries that were
	// in the posted-status scope but excluded from statistical diagnostics
	// because ledger.ValidateEntries flagged a SeverityError issue against
	// them — see the package doc's "excluded from diagnostics, but surfaced
	// in Result.Issues via LedgerIssues" policy.
	ExcludedEntries int `json:"excluded_entries"`
	// TotalAnalyzedAmount is the sum of EntryMagnitude across every analyzed
	// entry.
	TotalAnalyzedAmount float64 `json:"total_analyzed_amount"`
	// ManualEntryCount and ManualEntryAmount are the count/summed magnitude
	// of analyzed entries whose resolved EntryMetadata.Source ==
	// SourceManual. Zero (not unavailable) when no metadata was supplied —
	// see Coverage.PercentWithSource for whether that zero is meaningful.
	ManualEntryCount  int     `json:"manual_entry_count"`
	ManualEntryAmount float64 `json:"manual_entry_amount"`
	// PostedEntryCount and PostedEntryAmount are the count/summed magnitude
	// of analyzed entries with ledger.StatusPosted specifically (a subset
	// of AnalyzedEntries, which may also include StatusReversed entries
	// under the default IncludeStatuses).
	PostedEntryCount  int     `json:"posted_entry_count"`
	PostedEntryAmount float64 `json:"posted_entry_amount"`
	// AccountsTouched is the number of distinct Account.ID values posted to
	// by any analyzed entry's lines.
	AccountsTouched int `json:"accounts_touched"`
}

// Coverage reports what fraction of analyzed entries actually had each
// optional metadata field populated, so a caller can see why a given rule
// is unavailable rather than silently trusting an empty finding list — see
// the package doc's "Optional metadata" section and RuleAvailability.
type Coverage struct {
	EntriesAnalyzed       int     `json:"entries_analyzed"`
	EntriesExcluded       int     `json:"entries_excluded"`
	PercentWithSource     float64 `json:"percent_with_source"`
	PercentWithCreatedAt  float64 `json:"percent_with_created_at"`
	PercentWithPostedAt   float64 `json:"percent_with_posted_at"`
	PercentWithPreparerID float64 `json:"percent_with_preparer_id"`
	PercentWithApproverID float64 `json:"percent_with_approver_id"`
}

// buildMetadataIndex joins metadata to Ledger.Entries by EntryID, returning
// the resolved map plus Issues for duplicate/unknown entries — see
// IssueDuplicateMetadataEntry and IssueUnknownMetadataEntry. Only the first
// occurrence (input order) of a duplicated EntryID is used.
func buildMetadataIndex(metadata []EntryMetadata, entryIDs map[string]bool) (map[string]EntryMetadata, []Issue) {
	byID := make(map[string]EntryMetadata, len(metadata))
	var issues []Issue
	for _, m := range metadata {
		if m.EntryID == "" {
			continue
		}
		if _, dup := byID[m.EntryID]; dup {
			issues = append(issues, Issue{
				Code: IssueDuplicateMetadataEntry, Severity: IssueSeverityWarning,
				Message: "duplicate metadata for entry " + m.EntryID + "; first occurrence used",
				EntryID: m.EntryID,
			})
			continue
		}
		byID[m.EntryID] = m
		if !entryIDs[m.EntryID] {
			issues = append(issues, Issue{
				Code: IssueUnknownMetadataEntry, Severity: IssueSeverityWarning,
				Message: "metadata supplied for entry " + m.EntryID + " which is not present in the ledger",
				EntryID: m.EntryID,
			})
		}
	}
	return byID, issues
}

// buildPopulation resolves l's entries into the analyzed population per the
// package's posted-scope and validation-exclusion rules (sections 4-5):
// entries outside IncludeStatuses are dropped before validation even runs
// (they are out of scope, not invalid); entries within scope that
// ledger.ValidateEntries flags with a SeverityError issue are excluded from
// the returned analyzed slice (but counted in PopulationSummary.Excluded*
// and surfaced via the returned ledger issues). Returns entries sorted by
// (date, EntryID) for deterministic downstream iteration — a fixed sort key
// independent of caller input order, so accumulation order (and therefore
// float64 summation) is reproducible regardless of how the caller ordered
// Ledger.Entries.
func buildPopulation(l ledger.Ledger, metaByID map[string]EntryMetadata, policy Policy) (analyzed []analyzedEntry, excludedCount int, ledgerIssues []ledger.Issue) {
	chart := l.Chart()
	include := policy.includeStatusSet()

	var inScope []ledger.JournalEntry
	for _, e := range l.Entries {
		if include[e.EffectiveStatus()] {
			inScope = append(inScope, e)
		}
	}

	allIssues := ledger.ValidateEntries(inScope, chart, ledger.ValidateOptions{Tolerance: policy.Tolerance})
	invalidEntry := make(map[string]bool, len(inScope))
	for _, iss := range allIssues {
		if iss.Severity == ledger.SeverityError && iss.Entry != "" {
			invalidEntry[iss.Entry] = true
		}
	}

	for _, e := range inScope {
		if invalidEntry[e.ID] {
			excludedCount++
			continue
		}
		m, hasMeta := metaByID[e.ID]
		analyzed = append(analyzed, analyzedEntry{
			entry:     e,
			meta:      m,
			hasMeta:   hasMeta,
			date:      resolveDate(e),
			period:    e.Period,
			magnitude: EntryMagnitude(e),
		})
	}

	sort.SliceStable(analyzed, func(i, j int) bool {
		if analyzed[i].date != analyzed[j].date {
			return analyzed[i].date < analyzed[j].date
		}
		return analyzed[i].entry.ID < analyzed[j].entry.ID
	})

	return analyzed, excludedCount, allIssues
}

// computePopulationSummary derives PopulationSummary and Coverage from the
// resolved analyzed population.
func computePopulationSummary(all []ledger.JournalEntry, analyzed []analyzedEntry, excludedCount int) (PopulationSummary, Coverage) {
	inScope := len(analyzed) + excludedCount

	sum := PopulationSummary{
		TotalSuppliedEntries: len(all),
		InScopeEntries:       inScope,
		AnalyzedEntries:      len(analyzed),
		ExcludedEntries:      excludedCount,
	}

	accounts := make(map[string]bool)
	var withSource, withCreated, withPosted, withPreparer, withApprover int
	for _, a := range analyzed {
		sum.TotalAnalyzedAmount += a.magnitude
		if a.entry.EffectiveStatus() == ledger.StatusPosted {
			sum.PostedEntryCount++
			sum.PostedEntryAmount += a.magnitude
		}
		if a.hasMeta && a.meta.Source == SourceManual {
			sum.ManualEntryCount++
			sum.ManualEntryAmount += a.magnitude
		}
		for _, l := range a.entry.Lines {
			accounts[l.AccountID] = true
		}
		if a.hasMeta {
			if a.meta.Source != "" {
				withSource++
			}
			if a.meta.CreatedAt != nil {
				withCreated++
			}
			if a.meta.PostedAt != nil {
				withPosted++
			}
			if a.meta.PreparerID != "" {
				withPreparer++
			}
			if a.meta.ApproverID != "" {
				withApprover++
			}
		}
	}
	sum.AccountsTouched = len(accounts)

	cov := Coverage{EntriesAnalyzed: len(analyzed), EntriesExcluded: excludedCount}
	if n := len(analyzed); n > 0 {
		cov.PercentWithSource = float64(withSource) / float64(n)
		cov.PercentWithCreatedAt = float64(withCreated) / float64(n)
		cov.PercentWithPostedAt = float64(withPosted) / float64(n)
		cov.PercentWithPreparerID = float64(withPreparer) / float64(n)
		cov.PercentWithApproverID = float64(withApprover) / float64(n)
	}
	return sum, cov
}
