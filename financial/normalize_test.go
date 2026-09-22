package financial

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalize_BasicAggregation(t *testing.T) {
	items := []MappedLineItem{
		{
			SourceID: "row-1",
			Code:     CodeRevProduct,
			Values:   map[Period]float64{"2025": 100},
		},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	item, ok := ds.ByCodeAndPeriod(CodeRevProduct, "2025")
	if !ok {
		t.Fatalf("expected REV_PRODUCT/2025 in dataset, got %+v", ds.Items)
	}
	if item.Amount != 100 {
		t.Errorf("amount = %v, want 100", item.Amount)
	}
	if ds.Currency != "USD" {
		t.Errorf("currency = %q, want USD", ds.Currency)
	}
}

func TestNormalize_MultipleRowsIntoOneCanonicalAccount(t *testing.T) {
	// Mirrors fixtures/mapped_income_statement.json rows row-40 and row-42:
	// two distinct source rows both classified as OPEX_MARKETING must sum
	// into a single normalized item per period.
	items := []MappedLineItem{
		{SourceID: "row-40", Label: "Online Ads", Code: CodeOpexMarketing, Values: map[Period]float64{"2024": 20000, "2025": 25000}},
		{SourceID: "row-42", Label: "Advertising & Promotion", Code: CodeOpexMarketing, Values: map[Period]float64{"2024": 15000, "2025": 17000}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	if len(ds.Items) != 2 {
		t.Fatalf("expected 2 normalized items (one per period), got %d: %+v", len(ds.Items), ds.Items)
	}

	y2024, ok := ds.ByCodeAndPeriod(CodeOpexMarketing, "2024")
	if !ok || y2024.Amount != 35000 {
		t.Errorf("2024 OPEX_MARKETING = %+v, want amount 35000", y2024)
	}
	y2025, ok := ds.ByCodeAndPeriod(CodeOpexMarketing, "2025")
	if !ok || y2025.Amount != 42000 {
		t.Errorf("2025 OPEX_MARKETING = %+v, want amount 42000", y2025)
	}
}

func TestNormalize_MultiplePeriodsAggregateIndependently(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevProduct, Values: map[Period]float64{"2023": 10, "2024": 20, "2025": 30}},
		{SourceID: "row-2", Code: CodeRevProduct, Values: map[Period]float64{"2024": 5}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	cases := map[Period]float64{"2023": 10, "2024": 25, "2025": 30}
	for period, want := range cases {
		got, ok := ds.ByCodeAndPeriod(CodeRevProduct, period)
		if !ok {
			t.Errorf("missing period %s", period)
			continue
		}
		if got.Amount != want {
			t.Errorf("period %s amount = %v, want %v", period, got.Amount, want)
		}
	}

	periods := ds.Periods()
	if len(periods) != 3 {
		t.Errorf("Periods() = %v, want 3 distinct periods", periods)
	}
}

func TestNormalize_IgnoredRowsAreSkipped(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevProduct, Values: map[Period]float64{"2025": 100}},
		{SourceID: "row-2", Status: RowStatusIgnored, Values: map[Period]float64{"2025": 999999}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	item, ok := ds.ByCodeAndPeriod(CodeRevProduct, "2025")
	if !ok || item.Amount != 100 {
		t.Errorf("expected ignored row to be excluded, got %+v (dataset: %+v)", item, ds.Items)
	}
	if len(ds.Items) != 1 {
		t.Errorf("expected exactly 1 item, got %d: %+v", len(ds.Items), ds.Items)
	}
}

func TestNormalize_SubtotalRowsExcludedFromAggregation(t *testing.T) {
	// A subtotal row shares no code with its constituent rows in practice,
	// but even if it were mistakenly given one, Status must take priority
	// so it is never summed in.
	items := []MappedLineItem{
		{SourceID: "row-40", Code: CodeOpexMarketing, Values: map[Period]float64{"2025": 25000}},
		{SourceID: "row-42", Code: CodeOpexMarketing, Values: map[Period]float64{"2025": 17000}},
		{SourceID: "row-50", Code: CodeOpexMarketing, Status: RowStatusSubtotal, Values: map[Period]float64{"2025": 42000}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	item, ok := ds.ByCodeAndPeriod(CodeOpexMarketing, "2025")
	if !ok {
		t.Fatalf("expected OPEX_MARKETING/2025 present")
	}
	if item.Amount != 42000 {
		t.Errorf("amount = %v, want 42000 (subtotal must not double-count)", item.Amount)
	}
}

func TestNormalize_TotalRowsExcludedFromAggregation(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevProduct, Values: map[Period]float64{"2025": 500000}},
		{SourceID: "row-2", Code: CodeRevService, Values: map[Period]float64{"2025": 135000}},
		{SourceID: "row-3", Status: RowStatusTotal, Values: map[Period]float64{"2025": 635000}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	for _, item := range ds.Items {
		sum := 0.0
		for _, i := range ds.Items {
			sum += i.Amount
		}
		if sum != 635000 {
			t.Errorf("sum of normalized items = %v, want 635000 (total row must be excluded, not summed again)", sum)
		}
		_ = item
	}
	if len(ds.Items) != 2 {
		t.Errorf("expected 2 items (total row excluded), got %d: %+v", len(ds.Items), ds.Items)
	}
}

func TestNormalize_ProvenancePreserved(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-40", Label: "Online Ads", Code: CodeOpexMarketing, Values: map[Period]float64{"2025": 25000}},
		{SourceID: "row-42", Label: "Advertising & Promotion", Code: CodeOpexMarketing, Values: map[Period]float64{"2025": 17000}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD", IncludeProvenance: true})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	item, ok := ds.ByCodeAndPeriod(CodeOpexMarketing, "2025")
	if !ok {
		t.Fatalf("expected OPEX_MARKETING/2025 present")
	}
	if len(item.Sources) != 2 {
		t.Fatalf("expected 2 source refs, got %d: %+v", len(item.Sources), item.Sources)
	}

	found := map[string]float64{}
	for _, ref := range item.Sources {
		found[ref.RowID] = ref.Amount
		if ref.Period != "2025" {
			t.Errorf("source ref period = %s, want 2025", ref.Period)
		}
	}
	if found["row-40"] != 25000 {
		t.Errorf("row-40 source amount = %v, want 25000", found["row-40"])
	}
	if found["row-42"] != 17000 {
		t.Errorf("row-42 source amount = %v, want 17000", found["row-42"])
	}
}

func TestNormalize_ProvenanceOmittedByDefault(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevProduct, Values: map[Period]float64{"2025": 100}},
	}

	ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}

	item, _ := ds.ByCodeAndPeriod(CodeRevProduct, "2025")
	if item.Sources != nil {
		t.Errorf("expected nil Sources when IncludeProvenance is false, got %+v", item.Sources)
	}
}

func TestNormalize_MissingCurrency(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevProduct, Values: map[Period]float64{"2025": 100}},
	}

	_, err := Normalize(items, NormalizeOptions{})
	if !errors.Is(err, ErrMissingCurrency) {
		t.Fatalf("err = %v, want ErrMissingCurrency", err)
	}
}

func TestNormalize_MissingCodeOnNormalRowIsValidationError(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Values: map[Period]float64{"2025": 100}},
	}

	_, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	if err == nil {
		t.Fatal("expected validation error for missing code, got nil")
	}
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("err = %v (%T), want ValidationErrors", err, err)
	}
	if len(verrs) != 1 {
		t.Fatalf("expected 1 validation error, got %d: %v", len(verrs), verrs)
	}
	if verrs[0].SourceID != "row-1" {
		t.Errorf("SourceID = %q, want row-1", verrs[0].SourceID)
	}
	if verrs[0].Code != ErrCodeMissingCode {
		t.Errorf("Code = %q, want %q", verrs[0].Code, ErrCodeMissingCode)
	}
}

func TestNormalize_UnrecognizedStatusIsValidationError(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevProduct, Status: "bogus", Values: map[Period]float64{"2025": 100}},
	}

	_, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("err = %v, want ValidationErrors", err)
	}
	if len(verrs) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(verrs))
	}
	if verrs[0].Code != ErrCodeUnrecognizedStatus {
		t.Errorf("Code = %q, want %q", verrs[0].Code, ErrCodeUnrecognizedStatus)
	}
}

func TestNormalize_ValidationErrorCodesAreStableNotMessageDependent(t *testing.T) {
	// A caller must be able to branch on Code alone; Reason is free text
	// and may change wording without notice.
	missingCode := []MappedLineItem{
		{SourceID: "row-1", Values: map[Period]float64{"2025": 100}},
	}
	_, err := Normalize(missingCode, NormalizeOptions{Currency: "USD"})
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("err = %v, want ValidationErrors", err)
	}
	if verrs[0].Code != ErrCodeMissingCode {
		t.Errorf("Code = %q, want %q", verrs[0].Code, ErrCodeMissingCode)
	}

	badStatus := []MappedLineItem{
		{SourceID: "row-2", Code: CodeRevProduct, Status: "bogus", Values: map[Period]float64{"2025": 100}},
	}
	_, err = Normalize(badStatus, NormalizeOptions{Currency: "USD"})
	if !errors.As(err, &verrs) {
		t.Fatalf("err = %v, want ValidationErrors", err)
	}
	if verrs[0].Code != ErrCodeUnrecognizedStatus {
		t.Errorf("Code = %q, want %q", verrs[0].Code, ErrCodeUnrecognizedStatus)
	}
	if verrs[0].Code == ErrCodeMissingCode {
		t.Error("distinct validation problems must not share a Code")
	}
}

func TestValidationError_JSONRoundTrip(t *testing.T) {
	original := &ValidationError{
		SourceID: "row-1",
		Index:    3,
		Code:     ErrCodeMissingCode,
		Reason:   "missing canonical code for non-ignored, non-subtotal, non-total row",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ValidationError
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded != *original {
		t.Errorf("round-tripped ValidationError = %+v, want %+v", decoded, *original)
	}

	// Code must serialize as its stable string value, not an opaque
	// wrapper, so a non-Go consumer of a persisted result can match on it.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}
	if raw["code"] != "MISSING_CODE" {
		t.Errorf("code field = %v, want MISSING_CODE", raw["code"])
	}
}

func TestValidationErrors_JSONRoundTrip(t *testing.T) {
	original := ValidationErrors{
		{SourceID: "row-1", Index: 0, Code: ErrCodeMissingCode, Reason: "missing code"},
		{SourceID: "row-2", Index: 1, Code: ErrCodeUnrecognizedStatus, Reason: "bad status"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ValidationErrors
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded) != len(original) {
		t.Fatalf("decoded length = %d, want %d", len(decoded), len(original))
	}
	for i := range original {
		if *decoded[i] != *original[i] {
			t.Errorf("decoded[%d] = %+v, want %+v", i, *decoded[i], *original[i])
		}
	}
}

func TestNormalize_CollectsAllValidationErrors(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Values: map[Period]float64{"2025": 100}},
		{SourceID: "row-2", Values: map[Period]float64{"2025": 200}},
	}

	_, err := Normalize(items, NormalizeOptions{Currency: "USD"})
	var verrs ValidationErrors
	if !errors.As(err, &verrs) {
		t.Fatalf("err = %v, want ValidationErrors", err)
	}
	if len(verrs) != 2 {
		t.Fatalf("expected both rows to be reported, got %d errors: %v", len(verrs), verrs)
	}
}

func TestNormalize_EmptyInputProducesEmptyDataset(t *testing.T) {
	ds, err := Normalize(nil, NormalizeOptions{Currency: "USD"})
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if len(ds.Items) != 0 {
		t.Errorf("expected empty dataset, got %+v", ds.Items)
	}
	if ds.Currency != "USD" {
		t.Errorf("currency = %q, want USD", ds.Currency)
	}
}

func TestNormalize_DeterministicOrdering(t *testing.T) {
	items := []MappedLineItem{
		{SourceID: "row-1", Code: CodeRevService, Values: map[Period]float64{"2025": 1, "2024": 1}},
		{SourceID: "row-2", Code: CodeRevProduct, Values: map[Period]float64{"2025": 1, "2024": 1}},
	}

	for i := 0; i < 5; i++ {
		ds, err := Normalize(items, NormalizeOptions{Currency: "USD"})
		if err != nil {
			t.Fatalf("Normalize returned error: %v", err)
		}
		want := []struct {
			code   Code
			period Period
		}{
			{CodeRevProduct, "2024"},
			{CodeRevProduct, "2025"},
			{CodeRevService, "2024"},
			{CodeRevService, "2025"},
		}
		if len(ds.Items) != len(want) {
			t.Fatalf("run %d: got %d items, want %d", i, len(ds.Items), len(want))
		}
		for j, w := range want {
			if ds.Items[j].Code != w.code || ds.Items[j].Period != w.period {
				t.Fatalf("run %d: item[%d] = (%s, %s), want (%s, %s)", i, j, ds.Items[j].Code, ds.Items[j].Period, w.code, w.period)
			}
		}
	}
}
