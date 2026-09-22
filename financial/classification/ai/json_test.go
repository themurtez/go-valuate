package ai

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
)

// TestJSON_FallbackOutcome_RoundTrip proves a FallbackOutcome carrying a
// full Provenance (including a Disagreement) serializes and deserializes
// cleanly, with every field preserved.
func TestJSON_FallbackOutcome_RoundTrip(t *testing.T) {
	conf := 0.77
	out := FallbackOutcome{
		RowID: "row-1",
		Result: classification.Result{
			RowID: "row-1", Label: "Advertising", Code: financial.CodeOpexMarketing,
			Status: financial.RowStatusNormal, Confidence: 0.98, Source: classification.SourceAlias,
		},
		Provenance: &Provenance{
			Provider: "openai", Model: "gpt-4o-mini", AdapterVersion: "1.0.0",
			RequestSchemaVersion: RequestSchemaVersion, OrchestrationVersion: OrchestrationVersion,
			TriggerMode: AIForce,
			DeterministicResult: DeterministicSummary{
				Code: financial.CodeOpexMarketing, Source: string(classification.SourceAlias), Confidence: 0.98,
			},
			ProposedCode:    financial.CodeCogsDirectLabor,
			Alternatives:    []financial.Code{financial.CodeCogsOther},
			Reason:          "looks like direct labor",
			ModelConfidence: &conf,
			ReviewRequired:  true,
			Disagreement: &Disagreement{
				DeterministicCode: financial.CodeOpexMarketing,
				AICode:            financial.CodeCogsDirectLabor,
			},
		},
		Issues: []Issue{{RowID: "row-1", Code: IssueStructuralRowSkipped, Severity: SeverityWarning, Message: "x"}},
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var round FallbackOutcome
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if round.RowID != out.RowID {
		t.Errorf("RowID: got %q, want %q", round.RowID, out.RowID)
	}
	if round.Result.Code != out.Result.Code {
		t.Errorf("Result.Code: got %q, want %q", round.Result.Code, out.Result.Code)
	}
	if round.Provenance == nil {
		t.Fatal("expected Provenance to survive round-trip")
	}
	if round.Provenance.Disagreement == nil {
		t.Fatal("expected Disagreement to survive round-trip")
	}
	if round.Provenance.Disagreement.AICode != financial.CodeCogsDirectLabor {
		t.Errorf("Disagreement.AICode: got %q, want %q", round.Provenance.Disagreement.AICode, financial.CodeCogsDirectLabor)
	}
	if round.Provenance.ModelConfidence == nil || *round.Provenance.ModelConfidence != conf {
		t.Errorf("ModelConfidence: got %v, want %v", round.Provenance.ModelConfidence, conf)
	}
	if len(round.Issues) != 1 || round.Issues[0].Code != IssueStructuralRowSkipped {
		t.Errorf("Issues: got %+v", round.Issues)
	}
}

// TestJSON_BatchOutcome_RoundTrip proves BatchOutcome (the multi-row
// wrapper) also serializes cleanly.
func TestJSON_BatchOutcome_RoundTrip(t *testing.T) {
	batch := BatchOutcome{
		Outcomes: []FallbackOutcome{
			{RowID: "row-1", Result: classification.Result{RowID: "row-1", Source: classification.SourceUnknown}},
		},
		AIRowsAttempted: 1,
		BudgetExceeded:  false,
	}

	data, err := json.Marshal(batch)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var round BatchOutcome
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if len(round.Outcomes) != 1 || round.Outcomes[0].RowID != "row-1" {
		t.Errorf("expected Outcomes preserved, got %+v", round.Outcomes)
	}
	if round.AIRowsAttempted != 1 {
		t.Errorf("expected AIRowsAttempted preserved, got %d", round.AIRowsAttempted)
	}
}

// TestJSON_NoNaNOrInf proves nothing in this package's JSON output can ever
// contain a NaN/Inf token, mirroring review's identical comprehensive check
// (see review/roundtrip_test.go's TestJSON_NoNaNOrInf_Comprehensive) —
// ValidateResponse already rejects a non-finite RawConfidence outright, so
// this pins that guarantee at the JSON-output level too.
func TestJSON_NoNaNOrInf(t *testing.T) {
	badConf := math.NaN()
	resp := Response{Code: financial.CodeOpexOther, RawConfidence: &badConf}
	req := Request{AllowedCodes: BuildAllowedCodes(financial.AllCodes())}

	if issue := ValidateResponse(req, resp); issue == nil {
		t.Fatal("expected ValidateResponse to reject a NaN confidence")
	}

	// Even if a caller bypassed validation and serialized a Provenance
	// carrying a non-finite ModelConfidence directly, prove that scenario
	// is at least detectable via a JSON string scan (defense in depth: this
	// package's own orchestration never does this, since resolveOutcome
	// only builds Provenance after ValidateResponse passes).
	prov := Provenance{ModelConfidence: &badConf}
	data, err := json.Marshal(prov)
	if err == nil && (strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf")) {
		t.Fatalf("a NaN/Inf literal must never appear in JSON output, got: %s", data)
	}
}
