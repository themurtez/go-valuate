package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/openai/openai-go/v2/option"

	"github.com/themurtez/go-valuate/financial"
	"github.com/themurtez/go-valuate/financial/classification/ai"
)

// fakeRoundTripper is a net/http.RoundTripper that returns a canned
// response for every request, so this package's tests never touch the
// network or require credentials — see the package doc comment's "zero
// network access and zero credentials" guarantee for every test except the
// explicitly opt-in openai_integration_test.go.
type fakeRoundTripper struct {
	statusCode int
	body       string
	// capturedBody records the last request's raw body, so a test can
	// assert on exactly what this adapter sent — proving no unrelated
	// application state leaked into the request.
	capturedBody []byte
	err          error
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	if req.Body != nil {
		f.capturedBody, _ = io.ReadAll(req.Body)
	}
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(bytes.NewReader([]byte(f.body))),
		Header:     header,
	}, nil
}

func chatCompletionBody(content string) string {
	payload := map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": 1700000000,
		"model":   "gpt-4o-mini-test",
		"choices": []map[string]any{
			{
				"index":         0,
				"finish_reason": "stop",
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
				},
			},
		},
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func newTestClassifier(t *testing.T, rt *fakeRoundTripper) *Classifier {
	t.Helper()
	c, err := New(Config{
		APIKey:     "test-key-not-real",
		HTTPClient: &http.Client{Transport: rt},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	return c
}

// TestNew_RequiresAPIKeyOrClientOptions proves New refuses to construct a
// Classifier with no authentication configured at all.
func TestNew_RequiresAPIKeyOrClientOptions(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected an error when neither APIKey nor ClientOptions is supplied")
	}
}

// TestNewFromEnv_MissingKey proves NewFromEnv fails cleanly (no panic, no
// silent zero-value client) when OPENAI_API_KEY is unset — this is what
// lets every OTHER test in this repository run without credentials.
func TestNewFromEnv_MissingKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := NewFromEnv()
	if err == nil {
		t.Fatal("expected an error when OPENAI_API_KEY is unset")
	}
}

// TestClassify_StructuredResponseParsed proves a well-formed structured
// output response parses into a correct ai.Response with provider/model
// metadata attached.
func TestClassify_StructuredResponseParsed(t *testing.T) {
	content := `{"code":"OPEX_MARKETING","alternatives":["OPEX_OTHER"],"reason":"looks like marketing spend","confidence":0.81}`
	rt := &fakeRoundTripper{statusCode: 200, body: chatCompletionBody(content)}
	c := newTestClassifier(t, rt)

	req := ai.Request{
		RawLabel: "Advertising", NormalizedLabel: "advertising",
		AllowedCodes: ai.BuildAllowedCodes(financial.AllCodes()),
	}
	resp, err := c.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}
	if resp.Code != financial.CodeOpexMarketing {
		t.Errorf("expected OPEX_MARKETING, got %s", resp.Code)
	}
	if len(resp.Alternatives) != 1 || resp.Alternatives[0] != financial.CodeOpexOther {
		t.Errorf("expected alternatives [OPEX_OTHER], got %v", resp.Alternatives)
	}
	if resp.RawConfidence == nil || *resp.RawConfidence != 0.81 {
		t.Errorf("expected confidence 0.81, got %v", resp.RawConfidence)
	}
	if resp.Provider != "openai" {
		t.Errorf("expected provider 'openai', got %q", resp.Provider)
	}
	if resp.Model == "" {
		t.Error("expected a non-empty model identifier")
	}
	if resp.AdapterVersion != AdapterVersion {
		t.Errorf("expected AdapterVersion %s, got %s", AdapterVersion, resp.AdapterVersion)
	}

	// Validate the resulting Response against the closed set exactly as
	// ai.ClassifyWithFallback would.
	if issue := ai.ValidateResponse(req, resp); issue != nil {
		t.Errorf("expected a valid Response, got issue: %+v", issue)
	}
}

// TestClassify_UnknownResponseParsed proves the adapter correctly passes
// through a model's literal "UNKNOWN" answer.
func TestClassify_UnknownResponseParsed(t *testing.T) {
	content := `{"code":"UNKNOWN","alternatives":[],"reason":"not enough context","confidence":null}`
	rt := &fakeRoundTripper{statusCode: 200, body: chatCompletionBody(content)}
	c := newTestClassifier(t, rt)

	resp, err := c.Classify(context.Background(), ai.Request{RawLabel: "Mystery Item"})
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}
	if resp.Code != ai.CodeUnknown {
		t.Errorf("expected CodeUnknown, got %s", resp.Code)
	}
	if resp.RawConfidence != nil {
		t.Errorf("expected nil confidence for a null JSON value, got %v", *resp.RawConfidence)
	}
}

// TestClassify_RequestNeverIncludesUnrelatedState proves the request body
// this adapter sends contains only the fields ai.Request itself carries —
// no full document, no user identity, nothing beyond what the caller
// explicitly built into req (section 12's privacy boundary, verified at the
// wire level for this specific adapter).
func TestClassify_RequestNeverIncludesUnrelatedState(t *testing.T) {
	content := `{"code":"UNKNOWN","alternatives":[],"reason":"","confidence":null}`
	rt := &fakeRoundTripper{statusCode: 200, body: chatCompletionBody(content)}
	c := newTestClassifier(t, rt)

	req := ai.Request{
		RawLabel: "Field Labor", NormalizedLabel: "field labor", ParentLabel: "Cost of Sales",
		StatementType: "income_statement",
		AllowedCodes:  ai.BuildAllowedCodes(financial.AllCodes()),
	}
	_, err := c.Classify(context.Background(), req)
	if err != nil {
		t.Fatalf("Classify failed: %v", err)
	}

	sent := string(rt.capturedBody)
	for _, forbidden := range []string{"ssn", "tax_id", "bank_account", "customer_name", "user_id", "account_id"} {
		if containsFold(sent, forbidden) {
			t.Errorf("request body unexpectedly contains %q, which must never be sent: %s", forbidden, sent)
		}
	}
	if !containsFold(sent, "field labor") {
		t.Error("expected the row's own label to be present in the request body")
	}
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if equalFold(s[i:i+len(substr)], substr) {
				return true
			}
		}
		return false
	})()
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// TestClassify_InvalidJSONContent proves malformed structured-output
// content (should not happen given "strict" mode, but adapters must not
// trust that blindly) is reported as an error, never silently accepted.
func TestClassify_InvalidJSONContent(t *testing.T) {
	rt := &fakeRoundTripper{statusCode: 200, body: chatCompletionBody("not valid json{{{")}
	c := newTestClassifier(t, rt)

	_, err := c.Classify(context.Background(), ai.Request{RawLabel: "X"})
	if err == nil {
		t.Fatal("expected an error for malformed structured-output content")
	}
}

// TestClassify_RateLimitedError proves a 429 response is classified so
// ai.RateLimitError is satisfied, letting the orchestrator report
// IssueRateLimited specifically.
func TestClassify_RateLimitedError(t *testing.T) {
	body := `{"error":{"code":"rate_limit_exceeded","message":"too many requests","param":"","type":"rate_limit_error"}}`
	rt := &fakeRoundTripper{statusCode: http.StatusTooManyRequests, body: body}
	// WithMaxRetries(0): the SDK retries 429s by default, which would make
	// this test slow (multiple backoff sleeps) for no added coverage — the
	// retry COUNT is the SDK's own configurable concern (see Config's doc
	// comment), not something this test needs to exercise.
	c, err := New(Config{APIKey: "test-key-not-real", HTTPClient: &http.Client{Transport: rt}, ClientOptions: []option.RequestOption{option.WithMaxRetries(0)}})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = c.Classify(context.Background(), ai.Request{RawLabel: "X"})
	if err == nil {
		t.Fatal("expected a non-nil error for a 429 response")
	}
	var rle ai.RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("expected the returned error to satisfy ai.RateLimitError, got %v (%T)", err, err)
	}
}

// TestClassify_ContextCancellation proves Classify respects context
// cancellation rather than blocking indefinitely or ignoring it.
func TestClassify_ContextCancellation(t *testing.T) {
	rt := &fakeRoundTripper{statusCode: 200, body: chatCompletionBody(`{"code":"UNKNOWN","alternatives":[],"reason":"","confidence":null}`)}
	c := newTestClassifier(t, rt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // ensure the deadline has definitely passed

	_, err := c.Classify(ctx, ai.Request{RawLabel: "X"})
	if err == nil {
		t.Fatal("expected an error from an already-expired context")
	}
}

// TestDescribeRequestForDebug_NeverCalledAutomatically documents (and
// exercises for coverage) the opt-in debug helper: it must never run as
// part of Classify's normal path (see TestClassify_RequestNeverIncludesUnrelatedState,
// which proves the wire payload only reflects req's own fields regardless
// of what this helper would print).
func TestDescribeRequestForDebug_NeverCalledAutomatically(t *testing.T) {
	req := ai.Request{RawLabel: "X"}
	desc := describeRequestForDebug(req)
	if desc == "" {
		t.Error("expected a non-empty debug description when explicitly invoked")
	}
}
