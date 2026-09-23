package statements_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

// TestBuild_Deterministic_RepeatedRuns asserts that calling Build
// repeatedly against byte-identical input produces byte-equivalent JSON
// output every time — task section 40's core requirement.
func TestBuild_Deterministic_RepeatedRuns(t *testing.T) {
	buildInput := func() (statements.Input, statements.Options) {
		return statements.Input{
			Source:    statements.SourceLedger,
			Chart:     fixtures.ServiceBusinessChart(),
			Entries:   fixtures.ServiceBusinessEntries(),
			Periods:   []financial.Period{"2025-01"},
			Mappings:  fixtures.ServiceBusinessMappings(),
			Selection: statements.SelectionBoth,
		}, statements.Options{}
	}

	const runs = 5
	var jsons []string
	for i := 0; i < runs; i++ {
		input, opts := buildInput()
		result := statements.Build(input, opts)
		b, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("run %d: marshal failed: %v", i, err)
		}
		jsons = append(jsons, string(b))
	}
	for i := 1; i < runs; i++ {
		if jsons[i] != jsons[0] {
			t.Fatalf("run %d produced different JSON than run 0", i)
		}
	}
}

// TestBuild_Deterministic_ManyToOne_MapOrderStress stresses the
// many-to-one aggregation path (aggregate.go), which sums across
// multiple accounts via internal maps — exactly the kind of code this
// repository's history shows is prone to float-sum-order or
// iteration-order nondeterminism (see analytics/forecast and
// analytics/revenuequality's own documented map-order bugs). Runs many
// times and asserts identical output every time.
func TestBuild_Deterministic_ManyToOne_MapOrderStress(t *testing.T) {
	var first string
	for i := 0; i < 20; i++ {
		input := statements.Input{
			Source:    statements.SourceLedger,
			Chart:     fixtures.MultiRevenueChart(),
			Entries:   fixtures.MultiRevenueEntries(),
			Periods:   []financial.Period{"2025-07"},
			Mappings:  fixtures.MultiRevenueMappings(),
			Selection: statements.SelectionIncomeOnly,
		}
		result := statements.Build(input, statements.Options{})
		b, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("run %d: marshal failed: %v", i, err)
		}
		if i == 0 {
			first = string(b)
			continue
		}
		if string(b) != first {
			t.Fatalf("run %d produced different JSON than run 0 (many-to-one map-order nondeterminism)", i)
		}
	}
}

// TestBuild_Deterministic_MultiPeriod exercises a multi-period build,
// asserting both determinism across repeated runs and that Statement.
// Periods echoes the caller's own input order rather than a re-sorted
// one.
func TestBuild_Deterministic_MultiPeriod(t *testing.T) {
	chart := fixtures.ServiceBusinessChart()
	entries := fixtures.ServiceBusinessEntries()
	// Add a second period's worth of entries so more than one period has
	// real data.
	entries = append(entries, secondPeriodEntries()...)

	periods := []financial.Period{"2025-02", "2025-01"} // deliberately NOT chronological

	buildOnce := func() statements.Result {
		input := statements.Input{
			Source:    statements.SourceLedger,
			Chart:     chart,
			Entries:   entries,
			Periods:   periods,
			Mappings:  fixtures.ServiceBusinessMappings(),
			Selection: statements.SelectionIncomeOnly,
		}
		return statements.Build(input, statements.Options{})
	}

	first := buildOnce()
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		result := buildOnce()
		b, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("run %d: marshal failed: %v", i, err)
		}
		if string(b) != string(firstJSON) {
			t.Fatalf("run %d produced different JSON", i)
		}
	}

	if !reflect.DeepEqual(first.IncomeStatement.Periods, periods) {
		t.Errorf("Statement.Periods = %v, want caller order %v (never re-sorted)", first.IncomeStatement.Periods, periods)
	}
}

// secondPeriodEntries returns a small, internally-balanced set of
// journal entries for fixtures.ServiceBusinessChart covering "2025-02",
// so TestBuild_Deterministic_MultiPeriod has real (non-zero) data in a
// second period.
func secondPeriodEntries() []ledger.JournalEntry {
	return []ledger.JournalEntry{
		{
			ID: "SVC2-JE-1", Date: "2025-02-10", Period: "2025-02", Status: ledger.StatusPosted,
			Description: "February consulting invoice", Source: "billing",
			Lines: []ledger.JournalLine{
				{AccountID: "1100", Debit: 9000},
				{AccountID: "4000", Credit: 9000},
			},
		},
		{
			ID: "SVC2-JE-2", Date: "2025-02-15", Period: "2025-02", Status: ledger.StatusPosted,
			Description: "February payroll", Source: "payroll",
			Lines: []ledger.JournalLine{
				{AccountID: "6000", Debit: 7000},
				{AccountID: "1000", Credit: 7000},
			},
		},
	}
}
