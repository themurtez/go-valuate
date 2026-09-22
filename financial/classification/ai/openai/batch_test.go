package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification"
	"github.com/themurtez/go-valuate/financial/classification/ai"
)

func batchEnvelopeBody(results ...batchResultItem) string {
	env := batchResponseEnvelope{Results: results}
	data, _ := json.Marshal(env)
	return chatCompletionBody(string(data))
}

func floatPtr(f float64) *float64 { return &f }

// TestClassifyBatch_ImplementsBatchClassifier proves *Classifier satisfies
// ai.BatchClassifier at compile time via a runtime type assertion too, so
// ai.ClassifyBatchWithFallback actually routes through the batch path for
// this adapter rather than silently falling back to sequential Classify
// calls.
func TestClassifyBatch_ImplementsBatchClassifier(t *testing.T) {
	var c any = &Classifier{}
	if _, ok := c.(ai.BatchClassifier); !ok {
		t.Fatal("expected *Classifier to implement ai.BatchClassifier")
	}
}

// TestClassifyBatch_MultipleValidRows proves a well-formed multi-row batch
// response maps every result back to its correct request by id, preserving
// input order in the returned slice.
func TestClassifyBatch_MultipleValidRows(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "OPEX_MARKETING", Alternatives: []string{"OPEX_OTHER"}, Reason: "ads", Confidence: floatPtr(0.9)},
		batchResultItem{ID: "row-1", Code: "COGS_DIRECT_LABOR", Alternatives: nil, Reason: "labor", Confidence: floatPtr(0.75)},
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{
		{RawLabel: "Advertising", AllowedCodes: ai.BuildAllowedCodes(financial.AllCodes())},
		{RawLabel: "Field Labor", AllowedCodes: ai.BuildAllowedCodes(financial.AllCodes())},
	}
	results, errs := c.ClassifyBatch(context.Background(), reqs)

	if len(results) != 2 || len(errs) != 2 {
		t.Fatalf("expected 2 results/errs, got %d/%d", len(results), len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("row %d: unexpected error: %v", i, err)
		}
	}
	if results[0].Code != financial.CodeOpexMarketing {
		t.Errorf("row 0: expected OPEX_MARKETING, got %s", results[0].Code)
	}
	if results[1].Code != financial.CodeCogsDirectLabor {
		t.Errorf("row 1: expected COGS_DIRECT_LABOR, got %s", results[1].Code)
	}
	for i, r := range results {
		if r.Provider != providerName {
			t.Errorf("row %d: expected provider %q, got %q", i, providerName, r.Provider)
		}
		if r.AdapterVersion != AdapterVersion {
			t.Errorf("row %d: expected AdapterVersion %s, got %s", i, AdapterVersion, r.AdapterVersion)
		}
		if r.Model == "" {
			t.Errorf("row %d: expected a non-empty model identifier", i)
		}
	}
}

// TestClassifyBatch_MixedUnknownAndValid proves a batch may legitimately mix
// a literal "UNKNOWN" answer for one row with a real code for another,
// exactly like the single-row path allows per row.
func TestClassifyBatch_MixedUnknownAndValid(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "UNKNOWN", Alternatives: []string{}, Reason: "unclear"},
		batchResultItem{ID: "row-1", Code: "OPEX_OTHER", Alternatives: []string{}, Reason: "misc"},
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{{RawLabel: "Mystery"}, {RawLabel: "Misc Expense"}}
	results, errs := c.ClassifyBatch(context.Background(), reqs)

	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("expected no errors, got %v / %v", errs[0], errs[1])
	}
	if results[0].Code != ai.CodeUnknown {
		t.Errorf("row 0: expected CodeUnknown, got %s", results[0].Code)
	}
	if results[1].Code != financial.CodeOpexOther {
		t.Errorf("row 1: expected OPEX_OTHER, got %s", results[1].Code)
	}
}

// TestClassifyBatch_InvalidCodeForOneRow_IsolatedToThatRow proves
// ai.ValidateResponse's closed-set rejection (applied one layer up, in
// ai.ClassifyBatchWithFallback) only ever affects the offending row: this
// adapter itself does not pre-filter by AllowedCodes, so ClassifyBatch
// returns the raw (later-rejected) code here, and the isolation is proven at
// the orchestration boundary via ai.ValidateResponse per row.
func TestClassifyBatch_InvalidCodeForOneRow_IsolatedToThatRow(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "NOT_A_REAL_CODE", Alternatives: []string{}},
		batchResultItem{ID: "row-1", Code: "OPEX_OTHER", Alternatives: []string{}},
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	allowed := ai.BuildAllowedCodes(financial.AllCodes())
	reqs := []ai.Request{
		{RawLabel: "Weird One", AllowedCodes: allowed},
		{RawLabel: "Misc Expense", AllowedCodes: allowed},
	}
	results, errs := c.ClassifyBatch(context.Background(), reqs)
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("ClassifyBatch itself should not reject an out-of-set code (that is ai.ValidateResponse's job); got %v / %v", errs[0], errs[1])
	}

	if issue := ai.ValidateResponse(reqs[0], results[0]); issue == nil || issue.Code != ai.IssueInvalidCode {
		t.Fatalf("expected row 0's out-of-set code to be rejected by ai.ValidateResponse, got %+v", issue)
	}
	if issue := ai.ValidateResponse(reqs[1], results[1]); issue != nil {
		t.Fatalf("expected row 1 to remain valid and unaffected by row 0's invalid code, got %+v", issue)
	}
}

// TestClassifyBatch_DuplicateResultID proves a duplicate id in the model's
// response is rejected for the affected row (the first occurrence wins) and
// never silently overwrites or corrupts any other row's result.
func TestClassifyBatch_DuplicateResultID(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "OPEX_MARKETING"},
		batchResultItem{ID: "row-0", Code: "OPEX_OTHER"}, // duplicate id
		batchResultItem{ID: "row-1", Code: "COGS_DIRECT_LABOR"},
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{{RawLabel: "A"}, {RawLabel: "B"}}
	results, errs := c.ClassifyBatch(context.Background(), reqs)

	if errs[0] != nil {
		t.Fatalf("expected row 0 to keep its first (valid) occurrence, got error: %v", errs[0])
	}
	if results[0].Code != financial.CodeOpexMarketing {
		t.Errorf("expected row 0 to keep the FIRST occurrence's code OPEX_MARKETING, got %s", results[0].Code)
	}
	if errs[1] != nil || results[1].Code != financial.CodeCogsDirectLabor {
		t.Errorf("expected row 1 unaffected by row 0's duplicate id, got code=%s err=%v", results[1].Code, errs[1])
	}
}

// TestClassifyBatch_MissingResultID proves a row the model silently omitted
// from "results" gets its own isolated error, while every row that DID
// receive a result is still returned normally.
func TestClassifyBatch_MissingResultID(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "OPEX_MARKETING"},
		// row-1 deliberately missing entirely.
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{{RawLabel: "A"}, {RawLabel: "B"}}
	results, errs := c.ClassifyBatch(context.Background(), reqs)

	if errs[0] != nil {
		t.Fatalf("expected row 0 to succeed, got error: %v", errs[0])
	}
	if results[0].Code != financial.CodeOpexMarketing {
		t.Errorf("expected row 0 OPEX_MARKETING, got %s", results[0].Code)
	}
	if errs[1] == nil {
		t.Fatal("expected row 1 to have a non-nil error for its missing result id")
	}
	if !strings.Contains(errs[1].Error(), "row-1") {
		t.Errorf("expected row 1's error to mention its row id, got: %v", errs[1])
	}
}

// TestClassifyBatch_UnknownResultID proves a result whose id does not match
// any requested row is discarded rather than guessed at, and every actually
// requested row is still resolved correctly from the remaining results.
func TestClassifyBatch_UnknownResultID(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "OPEX_MARKETING"},
		batchResultItem{ID: "row-99", Code: "OPEX_OTHER"}, // no such requested row
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{{RawLabel: "A"}}
	results, errs := c.ClassifyBatch(context.Background(), reqs)

	if len(results) != 1 || len(errs) != 1 {
		t.Fatalf("expected exactly 1 result/err (matching len(reqs)), got %d/%d", len(results), len(errs))
	}
	if errs[0] != nil {
		t.Fatalf("expected row 0 to succeed despite the unrelated unknown-id result, got error: %v", errs[0])
	}
	if results[0].Code != financial.CodeOpexMarketing {
		t.Errorf("expected row 0 OPEX_MARKETING, got %s", results[0].Code)
	}
}

// TestClassifyBatch_ProviderFailure_AffectsWholeChunkNotOtherChunks proves a
// hard provider-call failure (transport error) for one character-bounded
// chunk is reported for every row in THAT chunk, without silently returning
// a zero-length result.
func TestClassifyBatch_ProviderFailure(t *testing.T) {
	rt := &fakeRoundTripper{err: errTransportBoom}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{{RawLabel: "A"}, {RawLabel: "B"}}
	results, errs := c.ClassifyBatch(context.Background(), reqs)

	if len(results) != 2 || len(errs) != 2 {
		t.Fatalf("expected 2 results/errs even on total provider failure, got %d/%d", len(results), len(errs))
	}
	if errs[0] == nil || errs[1] == nil {
		t.Fatalf("expected both rows to carry the provider failure, got %v / %v", errs[0], errs[1])
	}
}

// TestClassifyBatch_EmptyInput proves ClassifyBatch handles a zero-length
// request slice without issuing any provider call and without panicking.
func TestClassifyBatch_EmptyInput(t *testing.T) {
	rt := &fakeRoundTripper{err: errTransportBoom} // would fail the test if ever called
	c := newTestClassifier(t, rt)

	results, errs := c.ClassifyBatch(context.Background(), nil)
	if len(results) != 0 || len(errs) != 0 {
		t.Fatalf("expected empty results/errs for empty input, got %d/%d", len(results), len(errs))
	}
}

// TestClassifyBatch_RowIDsIncludedAndAllowedCodesClosed proves the wire
// request contains a deterministic "id" per row and an "allowed_codes"
// array, and that no code outside financial.AllCodes() ever appears there —
// the model must never be shown (or able to invent) a code outside the
// closed set.
func TestClassifyBatch_RowIDsIncludedAndAllowedCodesClosed(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "UNKNOWN"},
		batchResultItem{ID: "row-1", Code: "UNKNOWN"},
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	allowed := ai.BuildAllowedCodes(financial.AllCodes())
	reqs := []ai.Request{
		{RawLabel: "First Row", AllowedCodes: allowed},
		{RawLabel: "Second Row", AllowedCodes: allowed},
	}
	if _, errs := c.ClassifyBatch(context.Background(), reqs); errs[0] != nil || errs[1] != nil {
		t.Fatalf("unexpected errors: %v / %v", errs[0], errs[1])
	}

	sent := extractUserPayload(t, rt.capturedBody)
	var payload batchRequestPayload
	if err := json.Unmarshal(sent, &payload); err != nil {
		t.Fatalf("expected the sent rows payload to parse as batchRequestPayload, got error: %v, body: %s", err, sent)
	}
	if len(payload.Rows) != 2 {
		t.Fatalf("expected 2 rows in the request, got %d", len(payload.Rows))
	}
	if payload.Rows[0].ID != "row-0" || payload.Rows[1].ID != "row-1" {
		t.Fatalf("expected deterministic positional row ids row-0/row-1, got %q/%q", payload.Rows[0].ID, payload.Rows[1].ID)
	}
	if len(payload.AllowedCodes) == 0 {
		t.Fatal("expected a non-empty allowed_codes closed set in the request")
	}
	validCodes := make(map[financial.Code]bool)
	for _, m := range financial.AllCodes() {
		validCodes[m.Code] = true
	}
	for _, c := range payload.AllowedCodes {
		if !validCodes[c.Code] {
			t.Errorf("request contained a code %q outside financial.AllCodes()", c.Code)
		}
	}
}

// TestClassifyBatch_PrivacyBoundary proves the batch wire payload contains
// only the same permitted fields as the single-row path (section 5's
// privacy boundary applied to a bounded, multi-row request) — no amounts,
// no user/account/customer identity, no tax/bank details.
func TestClassifyBatch_PrivacyBoundary(t *testing.T) {
	body := batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "UNKNOWN"})
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	reqs := []ai.Request{{
		RawLabel: "Field Labor", NormalizedLabel: "field labor", ParentLabel: "Cost of Sales",
		StatementType: "income_statement",
		AllowedCodes:  ai.BuildAllowedCodes(financial.AllCodes()),
	}}
	if _, errs := c.ClassifyBatch(context.Background(), reqs); errs[0] != nil {
		t.Fatalf("unexpected error: %v", errs[0])
	}

	// Scoped to the user message's data payload only (not the fixed system
	// instruction prose, which legitimately discusses "amounts"/"figures" in
	// the abstract) — this is the actual privacy boundary under test.
	sent := string(extractUserPayload(t, rt.capturedBody))
	for _, forbidden := range []string{"ssn", "tax_id", "bank_account", "customer_name", "user_id", "account_id", "\"values\"", "\"amount\""} {
		if containsFold(sent, forbidden) {
			t.Errorf("batch request data payload unexpectedly contains %q, which must never be sent: %s", forbidden, sent)
		}
	}
	if !containsFold(sent, "field labor") {
		t.Error("expected the row's own label to be present in the batch request data payload")
	}
}

// TestClassifyBatch_MaxBatchCharactersRespected proves a character-budget
// configured via Config.MaxBatchCharacters causes ClassifyBatch to issue
// more than one underlying provider call rather than packing every row into
// one oversized prompt.
func TestClassifyBatch_MaxBatchCharactersRespected(t *testing.T) {
	rt := &countingRoundTripper{
		bodies: []string{
			batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "UNKNOWN"}),
			batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "UNKNOWN"}),
			batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "UNKNOWN"}),
		},
	}
	c, err := New(Config{
		APIKey:             "test-key-not-real",
		HTTPClient:         newHTTPClient(rt),
		MaxBatchCharacters: 200, // deliberately tiny: forces one row per chunk
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	reqs := []ai.Request{
		{RawLabel: "Row Alpha With A Reasonably Long Label To Add Up Characters"},
		{RawLabel: "Row Beta With A Reasonably Long Label To Add Up Characters"},
		{RawLabel: "Row Gamma With A Reasonably Long Label To Add Up Characters"},
	}
	results, errs := c.ClassifyBatch(context.Background(), reqs)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("row %d: unexpected error: %v", i, err)
		}
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if rt.calls < 2 {
		t.Fatalf("expected MaxBatchCharacters=200 to force multiple provider calls for 3 long-label rows, got %d call(s)", rt.calls)
	}
}

// TestClassifyBatch_DeterministicSplitting_PreservesOrder proves that
// however splitBatchByCharacterBudget divides a larger request set across
// multiple underlying provider calls, the final []ai.Response this method
// returns still matches the ORIGINAL input order exactly.
func TestClassifyBatch_DeterministicSplitting_PreservesOrder(t *testing.T) {
	rt := &countingRoundTripper{
		bodies: []string{
			batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "OPEX_MARKETING"}),
			batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "COGS_DIRECT_LABOR"}),
			batchEnvelopeBody(batchResultItem{ID: "row-0", Code: "OPEX_OTHER"}),
		},
	}
	c, err := New(Config{APIKey: "test-key-not-real", HTTPClient: newHTTPClient(rt), MaxBatchCharacters: 120})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	reqs := []ai.Request{
		{RawLabel: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{RawLabel: "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"},
		{RawLabel: "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"},
	}
	results, errs := c.ClassifyBatch(context.Background(), reqs)
	for i, err := range errs {
		if err != nil {
			t.Fatalf("row %d: unexpected error: %v", i, err)
		}
	}
	if results[0].Code != financial.CodeOpexMarketing {
		t.Errorf("expected reqs[0] (A...) to map to OPEX_MARKETING (1st call), got %s", results[0].Code)
	}
	if results[1].Code != financial.CodeCogsDirectLabor {
		t.Errorf("expected reqs[1] (B...) to map to COGS_DIRECT_LABOR (2nd call), got %s", results[1].Code)
	}
	if results[2].Code != financial.CodeOpexOther {
		t.Errorf("expected reqs[2] (C...) to map to OPEX_OTHER (3rd call), got %s", results[2].Code)
	}
}

// countingCapturingRoundTripper is like countingRoundTripper but also
// records each request's raw body, so a test can assert on how many
// distinct rows appeared per underlying provider call — used to prove
// Policy.MaxAIRows/Policy.MaxBatchSize (enforced one layer up, in
// ai.ClassifyBatchWithFallback/classifyMany) are actually respected when
// this adapter's ClassifyBatch is the BatchClassifier being driven.
type countingCapturingRoundTripper struct {
	body  string
	calls int
	rows  []int // rows[i] = number of "rows" entries sent on call i
}

func (c *countingCapturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := &fakeRoundTripper{statusCode: 200, body: c.body}
	resp, err := rt.RoundTrip(req)
	if err == nil {
		var payload batchRequestPayload
		if userJSON := extractUserPayloadFromRaw(rt.capturedBody); userJSON != nil {
			_ = json.Unmarshal(userJSON, &payload)
		}
		c.rows = append(c.rows, len(payload.Rows))
	}
	c.calls++
	return resp, err
}

// extractUserPayloadFromRaw is extractUserPayload without the *testing.T
// dependency, for use inside a RoundTripper.
func extractUserPayloadFromRaw(rawBody []byte) []byte {
	var envelope struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		return nil
	}
	for _, m := range envelope.Messages {
		if m.Role == "user" {
			const prefix = "Classify each of these line items:\n"
			if idx := strings.Index(m.Content, prefix); idx >= 0 {
				return []byte(m.Content[idx+len(prefix):])
			}
			return []byte(m.Content)
		}
	}
	return nil
}

// resultsForAllUnknown returns a batch envelope answering UNKNOWN for
// row-0..row-(n-1), used where a test only cares about dispatch shape
// (how many rows per call), not the classification outcome itself.
func resultsForAllUnknown(n int) string {
	items := make([]batchResultItem, n)
	for i := range items {
		items[i] = batchResultItem{ID: batchRowID(i), Code: "UNKNOWN"}
	}
	return batchEnvelopeBody(items...)
}

// TestClassifyBatch_MaxAIRowsRespected proves that when this adapter is
// driven through the real ai.ClassifyBatchWithFallback orchestrator,
// Policy.MaxAIRows caps how many rows are ever sent to the provider at all
// — the budget is enforced one layer up (ai.ClassifyBatchWithFallback),
// before ClassifyBatch is even called, so this test exercises that
// end-to-end composition rather than anything internal to this adapter.
func TestClassifyBatch_MaxAIRowsRespected(t *testing.T) {
	rt := &countingCapturingRoundTripper{body: resultsForAllUnknown(2)}
	c, err := New(Config{APIKey: "test-key-not-real", HTTPClient: newHTTPClient(rt)})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	raws := []financial.RawLineItem{
		{ID: "r1", Label: "Unknown One"},
		{ID: "r2", Label: "Unknown Two"},
		{ID: "r3", Label: "Unknown Three"},
	}
	policy := ai.Policy{Mode: ai.AIUnknownOnly, MaxAIRows: 2}
	out := ai.ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, c, policy, nil)

	if out.AIRowsAttempted != 2 {
		t.Fatalf("expected exactly 2 AI attempts under MaxAIRows=2, got %d", out.AIRowsAttempted)
	}
	if !out.BudgetExceeded {
		t.Error("expected BudgetExceeded true")
	}
	totalRowsSent := 0
	for _, n := range rt.rows {
		totalRowsSent += n
	}
	if totalRowsSent != 2 {
		t.Fatalf("expected exactly 2 rows total sent to the provider across all calls, got %d (calls=%v)", totalRowsSent, rt.rows)
	}
}

// TestClassifyBatch_MaxBatchSizeRespected proves that when this adapter is
// driven through the real orchestrator, Policy.MaxBatchSize caps how many
// rows are packed into a single underlying provider call — enforced by
// ai.classifyMany one layer up, chunking before ClassifyBatch is invoked
// per chunk, so this adapter never even sees a larger slice than the
// configured MaxBatchSize.
func TestClassifyBatch_MaxBatchSizeRespected(t *testing.T) {
	rt := &countingCapturingRoundTripper{body: resultsForAllUnknown(1)}
	c, err := New(Config{APIKey: "test-key-not-real", HTTPClient: newHTTPClient(rt)})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	raws := []financial.RawLineItem{
		{ID: "r1", Label: "Unknown One"},
		{ID: "r2", Label: "Unknown Two"},
		{ID: "r3", Label: "Unknown Three"},
		{ID: "r4", Label: "Unknown Four"},
	}
	policy := ai.Policy{Mode: ai.AIUnknownOnly, MaxBatchSize: 1}
	out := ai.ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, c, policy, nil)

	if out.AIRowsAttempted != 4 {
		t.Fatalf("expected all 4 rows attempted, got %d", out.AIRowsAttempted)
	}
	if rt.calls != 4 {
		t.Fatalf("expected 4 separate provider calls under MaxBatchSize=1, got %d", rt.calls)
	}
	for i, n := range rt.rows {
		if n != 1 {
			t.Errorf("call %d: expected exactly 1 row per call under MaxBatchSize=1, got %d", i, n)
		}
	}
}

// TestClassify_SingleRow_StillWorksAlongsideBatch proves the pre-existing
// single-row Classify path is unaffected by this adapter also implementing
// ai.BatchClassifier — both coexist on the same *Classifier value.
func TestClassify_SingleRow_StillWorksAlongsideBatch(t *testing.T) {
	content := `{"code":"OPEX_MARKETING","alternatives":[],"reason":"","confidence":0.5}`
	rt := &fakeRoundTripper{statusCode: 200, body: chatCompletionBody(content)}
	c := newTestClassifier(t, rt)

	resp, err := c.Classify(context.Background(), ai.Request{RawLabel: "Advertising"})
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}
	if resp.Code != financial.CodeOpexMarketing {
		t.Errorf("expected OPEX_MARKETING, got %s", resp.Code)
	}
}

// TestClassifyBatch_EveryResultReviewRequiredDownstream proves an
// AI-produced batch Response, once run through
// ai.ClassifyBatchWithFallback exactly like a real caller would, still
// carries ReviewRequired == true — batching must never weaken the mandatory
// human-review guarantee.
func TestClassifyBatch_EveryResultReviewRequiredDownstream(t *testing.T) {
	body := batchEnvelopeBody(
		batchResultItem{ID: "row-0", Code: "OPEX_MARKETING", Confidence: floatPtr(0.95)},
		batchResultItem{ID: "row-1", Code: "COGS_DIRECT_LABOR", Confidence: floatPtr(0.99)},
	)
	rt := &fakeRoundTripper{statusCode: 200, body: body}
	c := newTestClassifier(t, rt)

	raws := []financial.RawLineItem{
		{ID: "r1", Label: "Advertising", StatementType: financial.StatementIncomeStatement},
		{ID: "r2", Label: "Field Labor", StatementType: financial.StatementIncomeStatement},
	}
	out := ai.ClassifyBatchWithFallback(context.Background(), raws, classification.Config{}, c, ai.Policy{Mode: ai.AIUnknownOnly}, nil)

	for i, outcome := range out.Outcomes {
		if outcome.Provenance == nil {
			t.Fatalf("row %d: expected non-nil Provenance for a validated AI result", i)
		}
		if !outcome.Provenance.ReviewRequired {
			t.Errorf("row %d: expected ReviewRequired true even at high model confidence, got false", i)
		}
		if !outcome.Result.ReviewRequired {
			t.Errorf("row %d: expected Result.ReviewRequired true, got false", i)
		}
	}
}

var errTransportBoom = &transportError{}

type transportError struct{}

func (*transportError) Error() string { return "simulated transport failure" }

// countingRoundTripper returns bodies[call index] for each successive
// request (clamped to the last entry once exhausted) and counts how many
// requests it received — used to prove a character-budget guard actually
// causes multiple underlying provider calls.
type countingRoundTripper struct {
	bodies []string
	calls  int
}

func (c *countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	idx := c.calls
	if idx >= len(c.bodies) {
		idx = len(c.bodies) - 1
	}
	c.calls++
	rt := &fakeRoundTripper{statusCode: 200, body: c.bodies[idx]}
	return rt.RoundTrip(req)
}

func newHTTPClient(rt http.RoundTripper) *http.Client {
	return &http.Client{Transport: rt}
}

// extractUserPayload pulls the JSON object embedded after the fixed
// "Classify each of these line items:\n" prefix out of the raw chat-
// completion request body this adapter sent, so a test can unmarshal
// exactly the batchRequestPayload without re-implementing OpenAI's own
// chat-completion request envelope parsing.
func extractUserPayload(t *testing.T, rawBody []byte) []byte {
	t.Helper()
	var envelope struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rawBody, &envelope); err != nil {
		t.Fatalf("could not parse captured request body as a chat-completion request: %v", err)
	}
	for _, m := range envelope.Messages {
		if m.Role == "user" {
			const prefix = "Classify each of these line items:\n"
			if idx := strings.Index(m.Content, prefix); idx >= 0 {
				return []byte(m.Content[idx+len(prefix):])
			}
			return []byte(m.Content)
		}
	}
	t.Fatal("no user message found in captured request body")
	return nil
}
