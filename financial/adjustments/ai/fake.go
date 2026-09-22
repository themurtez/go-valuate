package ai

import "context"

// FakeSuggester is a fully in-memory, deterministic Suggester implementation
// for tests — no network, no credentials, no external process. Every test in
// this repository that exercises AI-adjustment-suggestion orchestration
// (this package's own tests, review's AI-adjustment integration tests) uses
// FakeSuggester or a caller-supplied func, never a real provider.
type FakeSuggester struct {
	// Response is returned for every Suggest call, unless Err is set.
	Response Response
	// Err, when non-nil, is returned instead of Response.
	Err error
	// Calls records every Request this FakeSuggester received, in call
	// order, so a test can assert on exactly what was sent (e.g. proving a
	// privacy-sensitive field was never populated).
	Calls []Request
}

// Suggest implements Suggester.
func (f *FakeSuggester) Suggest(ctx context.Context, req Request) (Response, error) {
	f.Calls = append(f.Calls, req)
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if f.Err != nil {
		return Response{}, f.Err
	}
	return f.Response, nil
}

var _ Suggester = (*FakeSuggester)(nil)
