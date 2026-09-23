package statements_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/accounting/ledger"
	"github.com/themurtez/go-valuate/accounting/statements"
	"github.com/themurtez/go-valuate/accounting/statements/fixtures"
	"github.com/themurtez/go-valuate/financial"
)

// TestJSON_RoundTrip_Result marshals and unmarshals a full Build Result
// and asserts field-for-field equality — task section 41's round-trip
// requirement, plus a scan for NaN/Inf anywhere in the numeric output.
func TestJSON_RoundTrip_Result(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.ContraAccountChart(),
		Entries:   fixtures.ContraAccountEntries(),
		Periods:   []financial.Period{"2025-08"},
		Mappings:  fixtures.ContraAccountMappings(),
		Selection: statements.SelectionBoth,
	}
	result := statements.Build(input, statements.Options{})

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var roundTripped statements.Result
	if err := json.Unmarshal(b, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(result, roundTripped) {
		t.Errorf("round-tripped Result does not equal original.\noriginal: %+v\nround-tripped: %+v", result, roundTripped)
	}

	assertNoNaNOrInf(t, b)
}

// TestJSON_RoundTrip_MappingTemplate round-trips a MappingTemplate.
func TestJSON_RoundTrip_MappingTemplate(t *testing.T) {
	template := fixtures.ServiceBusinessMappingTemplate()

	b, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var roundTripped statements.MappingTemplate
	if err := json.Unmarshal(b, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(template, roundTripped) {
		t.Errorf("round-tripped MappingTemplate does not equal original")
	}
}

// TestJSON_RoundTrip_AccountMapping_WithAllocations round-trips an
// allocated (one-to-many) AccountMapping specifically, since Allocations
// is the field most likely to be lost/reordered by a careless struct
// tag.
func TestJSON_RoundTrip_AccountMapping_WithAllocations(t *testing.T) {
	mapping := statements.AccountMapping{
		AccountID: "6300",
		Allocations: []statements.AllocationRule{
			{FinancialCode: financial.CodeOpexOffice, StatementType: financial.StatementIncomeStatement, Percent: 0.6, SignTreatment: statements.SignNatural},
			{FinancialCode: financial.CodeOpexOther, StatementType: financial.StatementIncomeStatement, Percent: 0.4, SignTreatment: statements.SignInvert},
		},
		Source: statements.MappingSourceExplicit,
	}

	b, err := json.Marshal(mapping)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var roundTripped statements.AccountMapping
	if err := json.Unmarshal(b, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(mapping, roundTripped) {
		t.Errorf("round-tripped AccountMapping does not equal original:\n%+v\nvs\n%+v", mapping, roundTripped)
	}
}

// TestJSON_RoundTrip_MappingCoverage round-trips MappingCoverage.
func TestJSON_RoundTrip_MappingCoverage(t *testing.T) {
	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     fixtures.RetailerChart(),
		Entries:   fixtures.RetailerEntries(),
		Periods:   []financial.Period{"2025-03"},
		Mappings:  fixtures.RetailerMappings(),
		Selection: statements.SelectionIncomeOnly,
	}
	result := statements.Build(input, statements.Options{})

	b, err := json.Marshal(result.Coverage)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var roundTripped statements.MappingCoverage
	if err := json.Unmarshal(b, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(result.Coverage, roundTripped) {
		t.Errorf("round-tripped MappingCoverage does not equal original")
	}
}

// assertNoNaNOrInf scans raw JSON bytes for the literal tokens
// encoding/json would produce if a NaN/Inf ever slipped through (it
// cannot actually marshal them — json.Marshal returns an
// UnsupportedValueError instead), as a defense-in-depth check
// consistent with this repository's other json_test.go files (see
// financial, ledger, and every analytics sibling's identical pattern).
// The primary guarantee is that Marshal did not error at all above.
func assertNoNaNOrInf(t *testing.T, b []byte) {
	t.Helper()
	s := string(b)
	for _, token := range []string{"NaN", "Infinity", "-Infinity"} {
		if strings.Contains(s, token) {
			t.Errorf("JSON output contains %q, which should never appear in valid JSON", token)
		}
	}
}

// TestBuild_LargeFiniteBalance_NeverProducesNaNOrInf sanity-checks that
// canonicalAmount's arithmetic (sign flips and allocation-percent
// scaling) stays finite for a large-but-finite input balance, complementing
// assertNoNaNOrInf's JSON-level scan above with a direct numeric check.
func TestBuild_LargeFiniteBalance_NeverProducesNaNOrInf(t *testing.T) {
	chart := fixtures.ContraAccountChart()
	entries := []ledger.JournalEntry{
		{
			ID: "BIG-JE-1", Date: "2025-08-01", Period: "2025-08", Status: ledger.StatusPosted,
			Lines: []ledger.JournalLine{
				{AccountID: "1000", Debit: 1e15},
				{AccountID: "3000", Credit: 1e15},
			},
		},
	}

	input := statements.Input{
		Source:    statements.SourceLedger,
		Chart:     chart,
		Entries:   entries,
		Periods:   []financial.Period{"2025-08"},
		Mappings:  fixtures.ContraAccountMappings(),
		Selection: statements.SelectionBalanceOnly,
	}
	result := statements.Build(input, statements.Options{})

	for _, item := range result.Dataset.Items {
		if math.IsNaN(item.Amount) || math.IsInf(item.Amount, 0) {
			t.Errorf("dataset item %+v has a non-finite amount", item)
		}
	}
}
