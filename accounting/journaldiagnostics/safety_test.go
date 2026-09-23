package journaldiagnostics_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/journaldiagnostics"
	"github.com/themurtez/go-valuate/accounting/journaldiagnostics/fixtures"
	"github.com/themurtez/go-valuate/accounting/ledger"
)

// prohibitedTerms are the words this package's generated text must never
// contain, per the package doc's non-fraud boundary — see section 51's
// explicit instruction. Matching is case-insensitive and substring-based
// (so "fraudulent" also fails on "fraud").
var prohibitedTerms = []string{
	"fraud", "fraudulent", "theft", "embezzlement", "stolen", "manipulation", "misconduct",
}

func scanForProhibitedLanguage(t *testing.T, label, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, term := range prohibitedTerms {
		if strings.Contains(lower, term) {
			t.Errorf("%s contains prohibited term %q: %q", label, term, text)
		}
	}
}

// TestSafety_NoFraudLanguageInFindingMessages scans every Finding message
// this package's fixed templates can produce for prohibited language — the
// explicit regression test section 51 requires. It runs against the full
// fixture Result (every rule family that fires at least once) rather than
// enumerating message constants directly, so it also catches a future rule
// added without updating the prohibited-terms list.
func TestSafety_NoFraudLanguageInFindingMessages(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if len(r.Findings) == 0 {
		t.Fatal("expected at least one Finding from the fixture set to make this scan meaningful")
	}
	seen := make(map[journaldiagnostics.FindingCode]bool)
	for _, f := range r.Findings {
		scanForProhibitedLanguage(t, "Finding "+string(f.Code)+" Message", f.Message)
		seen[f.Code] = true
	}
	if len(seen) < 15 {
		t.Errorf("expected the fixture set to exercise at least 15 distinct FindingCodes for full message coverage, got %d: %v", len(seen), seen)
	}
}

// TestSafety_NoFraudLanguageInIssueMessages covers Issue messages too —
// some of which are built with string concatenation (entry/account IDs),
// so this exercises the dynamic path as well as the fixed templates.
func TestSafety_NoFraudLanguageInIssueMessages(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.BrokenLedgerUnbalancedEntry(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	if len(r.Issues) == 0 {
		t.Fatal("expected at least one Issue from the broken-ledger fixture")
	}
	for _, iss := range r.Issues {
		scanForProhibitedLanguage(t, "Issue "+string(iss.Code)+" Message", iss.Message)
	}
}

// TestSafety_ExternalLabelNeverSetByThisPackage confirms Evidence.
// ExternalLabel — the one field explicitly reserved for a caller's own
// external fraud/risk classification — is always empty in this package's
// own output, since this package never concludes one itself.
func TestSafety_ExternalLabelNeverSetByThisPackage(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	for _, f := range r.Findings {
		if f.Evidence.ExternalLabel != "" {
			t.Errorf("Finding %s has a non-empty ExternalLabel %q; this package must never set it itself", f.Code, f.Evidence.ExternalLabel)
		}
	}
}

func TestSafety_NoMutationOfLedgerAccounts(t *testing.T) {
	l := fixtures.Ledger()
	snapshot := make([]ledger.Account, len(l.Accounts))
	copy(snapshot, l.Accounts)

	_ = journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	if !reflect.DeepEqual(l.Accounts, snapshot) {
		t.Error("Calculate must never mutate Ledger.Accounts")
	}
}

func TestSafety_NoMutationOfLedgerEntries(t *testing.T) {
	l := fixtures.Ledger()
	snapshot := make([]ledger.JournalEntry, len(l.Entries))
	copy(snapshot, l.Entries)
	for i := range snapshot {
		snapshot[i].Lines = append([]ledger.JournalLine{}, l.Entries[i].Lines...)
	}

	_ = journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	for i := range l.Entries {
		if !reflect.DeepEqual(l.Entries[i].Lines, snapshot[i].Lines) {
			t.Errorf("Calculate must never mutate JournalEntry.Lines; entry %s changed", l.Entries[i].ID)
		}
		nonLines := l.Entries[i]
		nonLines.Lines = nil
		snapNonLines := snapshot[i]
		snapNonLines.Lines = nil
		if !reflect.DeepEqual(nonLines, snapNonLines) {
			t.Errorf("Calculate must never mutate JournalEntry fields; entry %s changed", l.Entries[i].ID)
		}
	}
}

func TestSafety_NoMutationOfMetadataOrPolicy(t *testing.T) {
	meta := fixtures.Metadata()
	metaSnapshot := make([]journaldiagnostics.EntryMetadata, len(meta))
	copy(metaSnapshot, meta)

	policy := fixtures.Policy()
	policySnapshot := policy

	_ = journaldiagnostics.Calculate(fixtures.Ledger(), meta, fixtures.Window(), policy)

	if !reflect.DeepEqual(meta, metaSnapshot) {
		t.Error("Calculate must never mutate the metadata slice")
	}
	if !reflect.DeepEqual(policy, policySnapshot) {
		t.Error("Calculate must never mutate the caller's Policy value")
	}
}

func TestSafety_Deterministic_RepeatedCallsIdentical(t *testing.T) {
	l := fixtures.Ledger()
	meta := fixtures.Metadata()
	window := fixtures.Window()
	policy := fixtures.Policy()

	r1 := journaldiagnostics.Calculate(l, meta, window, policy)
	r2 := journaldiagnostics.Calculate(l, meta, window, policy)

	b1, err := json.Marshal(r1)
	if err != nil {
		t.Fatalf("marshal r1: %v", err)
	}
	b2, err := json.Marshal(r2)
	if err != nil {
		t.Fatalf("marshal r2: %v", err)
	}
	if string(b1) != string(b2) {
		t.Error("expected byte-for-byte identical JSON across repeated Calculate calls with identical input")
	}
}

func TestSafety_Deterministic_InputOrderIndependent(t *testing.T) {
	l1 := fixtures.Ledger()
	l2 := fixtures.Ledger()
	// Reverse entry order in l2 — the Result must be identical regardless,
	// since buildPopulation sorts to a fixed (date, EntryID) key.
	for i, j := 0, len(l2.Entries)-1; i < j; i, j = i+1, j-1 {
		l2.Entries[i], l2.Entries[j] = l2.Entries[j], l2.Entries[i]
	}

	r1 := journaldiagnostics.Calculate(l1, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	r2 := journaldiagnostics.Calculate(l2, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	b1, _ := json.Marshal(r1)
	b2, _ := json.Marshal(r2)
	if string(b1) != string(b2) {
		t.Error("expected identical Result regardless of caller-supplied Ledger.Entries order")
	}
}

func TestSafety_JSONRoundTrip_Result(t *testing.T) {
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), fixtures.Policy())
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var r2 journaldiagnostics.Result
	if err := json.Unmarshal(b, &r2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b2, err := json.Marshal(r2)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if string(b) != string(b2) {
		t.Error("expected Result to round-trip through JSON byte-for-byte")
	}
}

func TestSafety_JSONRoundTrip_PolicyAndMetadata(t *testing.T) {
	policy := fixtures.Policy()
	b, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("marshal policy: %v", err)
	}
	var p2 journaldiagnostics.Policy
	if err := json.Unmarshal(b, &p2); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}

	meta := fixtures.Metadata()
	bm, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	var m2 []journaldiagnostics.EntryMetadata
	if err := json.Unmarshal(bm, &m2); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if len(m2) != len(meta) {
		t.Errorf("expected %d metadata entries after round-trip, got %d", len(meta), len(m2))
	}
}

func TestSafety_NaNInfThresholdsRejected(t *testing.T) {
	policy := fixtures.Policy()
	policy.MaterialAmount = math.NaN()
	r := journaldiagnostics.Calculate(fixtures.Ledger(), fixtures.Metadata(), fixtures.Window(), policy)

	found := false
	for _, iss := range r.Issues {
		if iss.Code == journaldiagnostics.IssueNonFiniteThreshold {
			found = true
		}
	}
	if !found {
		t.Error("expected IssueNonFiniteThreshold for a NaN MaterialAmount")
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "NaN") || strings.Contains(string(b), "Infinity") {
		t.Error("Result JSON must never contain NaN or Infinity")
	}
}

func TestSafety_NonFiniteEntryAmountsExcludedNotPropagated(t *testing.T) {
	l := fixtures.Ledger()
	l.Entries = append(l.Entries, ledger.JournalEntry{
		ID: "JE-NONFINITE-01", Date: "2025-03-19", Period: "2025-03", Status: ledger.StatusPosted,
		Lines: []ledger.JournalLine{
			{ID: "L1", AccountID: "6200", Debit: math.Inf(1)},
			{ID: "L2", AccountID: "1000", Credit: 100},
		},
	})
	r := journaldiagnostics.Calculate(l, fixtures.Metadata(), fixtures.Window(), fixtures.Policy())

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "Inf") {
		t.Error("Result JSON must never contain a non-finite value from a bad entry amount")
	}
	// The entry should be excluded from analyzed population (ledger
	// validation flags non-finite amounts as a SeverityError).
	for _, f := range r.Findings {
		for _, id := range f.EntryIDs {
			if id == "JE-NONFINITE-01" {
				t.Error("a non-finite-amount entry must be excluded from diagnostics, not appear in any Finding")
			}
		}
	}
}

func TestSafety_ConcurrentCallsProduceIdenticalResults(t *testing.T) {
	l := fixtures.Ledger()
	meta := fixtures.Metadata()
	window := fixtures.Window()
	policy := fixtures.Policy()

	const n = 20
	results := make([][]byte, n)
	done := make(chan int, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			r := journaldiagnostics.Calculate(l, meta, window, policy)
			b, _ := json.Marshal(r)
			results[idx] = b
			done <- idx
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}
	for i := 1; i < n; i++ {
		if string(results[i]) != string(results[0]) {
			t.Fatalf("concurrent call %d produced a different result than call 0", i)
		}
	}
}
