package financial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestFixtures_MappedIncomeStatementNormalizesToFixtureDataset verifies that
// fixtures/mapped_income_statement.json, run through Normalize, produces
// exactly fixtures/normalized_dataset.json. This keeps the two fixtures
// honest as documentation: if Normalize's behavior ever changes, this test
// fails instead of silently leaving stale example output on disk.
func TestFixtures_MappedIncomeStatementNormalizesToFixtureDataset(t *testing.T) {
	mappedBytes, err := os.ReadFile(filepath.Join("..", "fixtures", "mapped_income_statement.json"))
	if err != nil {
		t.Fatalf("reading mapped fixture: %v", err)
	}
	var items []MappedLineItem
	if err := json.Unmarshal(mappedBytes, &items); err != nil {
		t.Fatalf("unmarshaling mapped fixture: %v", err)
	}

	got, err := Normalize(items, NormalizeOptions{Currency: "USD", IncludeProvenance: true})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	wantBytes, err := os.ReadFile(filepath.Join("..", "fixtures", "normalized_dataset.json"))
	if err != nil {
		t.Fatalf("reading normalized fixture: %v", err)
	}
	var want FinancialDataset
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("unmarshaling normalized fixture: %v", err)
	}

	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshaling actual result: %v", err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshaling expected fixture: %v", err)
	}

	if string(gotJSON) != string(wantJSON) {
		t.Errorf("Normalize(mapped_income_statement.json) mismatch with normalized_dataset.json\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

// TestFixtures_RawIncomeStatementDecodes verifies that
// fixtures/raw_income_statement.json is valid, well-formed RawLineItem data,
// since it otherwise is not exercised by any code path.
func TestFixtures_RawIncomeStatementDecodes(t *testing.T) {
	rawBytes, err := os.ReadFile(filepath.Join("..", "fixtures", "raw_income_statement.json"))
	if err != nil {
		t.Fatalf("reading raw fixture: %v", err)
	}
	var items []RawLineItem
	if err := json.Unmarshal(rawBytes, &items); err != nil {
		t.Fatalf("unmarshaling raw fixture: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected raw fixture to contain at least one row")
	}
	for _, item := range items {
		if item.ID == "" {
			t.Errorf("row missing id: %+v", item)
		}
		if item.StatementType != StatementIncomeStatement {
			t.Errorf("row %s: statement type = %q, want income_statement", item.ID, item.StatementType)
		}
	}
}
