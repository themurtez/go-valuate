package ai

import "context"

// FakeClassifier is a fully in-memory, deterministic Classifier
// implementation for tests — no network, no credentials, no external
// process. Every test in this repository that exercises AI-fallback
// orchestration (this package's own tests, review's AI-integration tests)
// uses FakeClassifier or a caller-supplied func, never a real provider —
// see the README's "test behavior without credentials" section.
type FakeClassifier struct {
	// Responses maps a Request's RawLabel to the Response FakeClassifier
	// returns for it. A label with no entry falls back to
	// DefaultResponse/DefaultErr below, so a test only needs to specify the
	// rows it actually cares about.
	Responses map[string]Response
	// Errs maps a Request's RawLabel to an error FakeClassifier returns
	// instead of a Response — takes precedence over Responses for that
	// label.
	Errs map[string]error
	// DefaultResponse is returned for any RawLabel not present in Responses
	// or Errs. Defaults to the zero Response ({Code: ""}, which
	// ValidateResponse rejects as IssueEmptyResponse) if never set — a test
	// wanting "always answer UNKNOWN" should set this explicitly to
	// Response{Code: CodeUnknown}.
	DefaultResponse Response
	// DefaultErr, when non-nil, is returned instead of DefaultResponse for
	// any RawLabel not present in Responses or Errs.
	DefaultErr error
	// Calls records every Request this FakeClassifier received, in call
	// order, so a test can assert on exactly what was sent (e.g. proving a
	// privacy-sensitive field was never populated, or that a structural row
	// never reached Classify at all).
	Calls []Request
	// BatchCalls records every ClassifyBatch invocation's full request
	// slice, in call order, for a test exercising BatchClassifier
	// specifically.
	BatchCalls [][]Request
	// SupportsBatch, when true, makes FakeClassifier additionally satisfy
	// BatchClassifier (see AsBatchClassifier) so a test can exercise
	// classifyMany's batching path.
	SupportsBatch bool
}

// NewFakeClassifier returns a ready-to-use FakeClassifier with its maps
// initialized.
func NewFakeClassifier() *FakeClassifier {
	return &FakeClassifier{
		Responses: make(map[string]Response),
		Errs:      make(map[string]error),
	}
}

// Classify implements Classifier.
func (f *FakeClassifier) Classify(ctx context.Context, req Request) (Response, error) {
	f.Calls = append(f.Calls, req)
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if err, ok := f.Errs[req.RawLabel]; ok {
		return Response{}, err
	}
	if resp, ok := f.Responses[req.RawLabel]; ok {
		return resp, nil
	}
	if f.DefaultErr != nil {
		return Response{}, f.DefaultErr
	}
	return f.DefaultResponse, nil
}

// FakeBatchClassifier wraps a *FakeClassifier to additionally satisfy
// BatchClassifier, so tests can exercise ClassifyBatchWithFallback's
// batching path without a real provider. Kept as a separate wrapper type
// (rather than making FakeClassifier itself always satisfy
// BatchClassifier) so a test can deliberately construct a
// non-batch-capable Classifier and prove classifyMany's sequential
// fallback path behaves identically.
type FakeBatchClassifier struct {
	*FakeClassifier
}

// AsBatchClassifier wraps f as a BatchClassifier.
func (f *FakeClassifier) AsBatchClassifier() *FakeBatchClassifier {
	return &FakeBatchClassifier{FakeClassifier: f}
}

// ClassifyBatch implements BatchClassifier by calling Classify for each
// request and recording the whole chunk on BatchCalls. One request's error
// never prevents the rest of the chunk's results/errs from being populated
// — proving FakeBatchClassifier upholds the same per-row isolation
// guarantee any real provider adapter's batch endpoint must.
func (f *FakeBatchClassifier) ClassifyBatch(ctx context.Context, reqs []Request) ([]Response, []error) {
	f.BatchCalls = append(f.BatchCalls, reqs)
	results := make([]Response, len(reqs))
	errs := make([]error, len(reqs))
	for i, req := range reqs {
		results[i], errs[i] = f.Classify(ctx, req)
	}
	return results, errs
}
