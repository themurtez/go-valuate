package ingestion_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/ingestion"
	icsv "github.com/themurtez/go-valuate/ingestion/csv"
)

func TestResultJSONRoundTrip(t *testing.T) {
	res, err := icsv.Parse(strings.NewReader("Account,2023,2024\nRevenue,1000,1200\nTotal Revenue,1000,1200\n"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}

	first, jerr := json.Marshal(res)
	if jerr != nil {
		t.Fatalf("Marshal: %v", jerr)
	}
	var decoded ingestion.Result
	if jerr := json.Unmarshal(first, &decoded); jerr != nil {
		t.Fatalf("Unmarshal: %v", jerr)
	}
	second, jerr := json.Marshal(decoded)
	if jerr != nil {
		t.Fatalf("re-Marshal: %v", jerr)
	}
	if string(first) != string(second) {
		t.Errorf("round-trip mismatch:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestToRawLineItemsFieldMapping(t *testing.T) {
	res, err := icsv.Parse(strings.NewReader("Account,2023,2024\nRevenue,1000,1200\n"), ingestion.Options{})
	if err != nil {
		t.Fatalf("Parse: %+v", err)
	}
	items := res.ToRawLineItems()
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	item := items[0]
	if item.ID != "sheet-0-row-1" {
		t.Errorf("ID = %q, want sheet-0-row-1", item.ID)
	}
	if item.StatementType != res.Metadata.StatementType {
		t.Errorf("StatementType = %q, want %q", item.StatementType, res.Metadata.StatementType)
	}
	if item.Label != "Revenue" {
		t.Errorf("Label = %q, want Revenue", item.Label)
	}
	if item.Values[financial.Period("2023")] != 1000 {
		t.Errorf("Values[2023] = %v, want 1000", item.Values[financial.Period("2023")])
	}
}

func TestDefaultLimits(t *testing.T) {
	d := ingestion.DefaultLimits()
	if d.MaxFileSizeBytes <= 0 {
		t.Error("MaxFileSizeBytes should be positive")
	}
	if d.MaxRows <= 0 {
		t.Error("MaxRows should be positive")
	}
	if d.MaxColumns <= 0 {
		t.Error("MaxColumns should be positive")
	}
	if d.MaxSheets <= 0 {
		t.Error("MaxSheets should be positive")
	}
	if d.MaxCellTextLength <= 0 {
		t.Error("MaxCellTextLength should be positive")
	}
}

func TestErrorImplementsError(t *testing.T) {
	var err error = &ingestion.Error{Code: ingestion.ErrCodeInvalidFile, Message: "boom"}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("Error() = %q, want it to contain %q", err.Error(), "boom")
	}
}

func TestErrorWithDetail(t *testing.T) {
	err := &ingestion.Error{Code: ingestion.ErrCodeLimitExceeded, Message: "too big", Detail: "limit is 10 bytes"}
	got := err.Error()
	if !strings.Contains(got, "too big") || !strings.Contains(got, "limit is 10 bytes") {
		t.Errorf("Error() = %q, want it to contain both message and detail", got)
	}
}
